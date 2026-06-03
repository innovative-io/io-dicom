package wado

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/innovative-io/io-dicom/media"
)

// ClientParams configures a DICOMweb client.
type ClientParams struct {
	// BaseURL is the server root, e.g. "https://dicom.example.com".
	// WADO-RS requests append /wado/rs/..., STOW-RS /stow/rs/...,
	// QIDO-RS /qido/rs/...
	BaseURL string
	// Timeout for HTTP requests. 0 means no timeout.
	Timeout time.Duration
	// InsecureTLS disables TLS certificate verification. Use only in development.
	InsecureTLS bool
	// Bearer is an optional Authorization: Bearer token.
	Bearer string
	// BasicUser and BasicPass are optional HTTP Basic Auth credentials.
	BasicUser string
	BasicPass string
}

// Client provides DICOMweb operations: WADO-RS retrieve, STOW-RS store,
// and QIDO-RS search.
type Client interface {
	// RetrieveStudy fetches all instances in a study.
	RetrieveStudy(ctx context.Context, studyUID string) ([]media.DICOMObject, error)
	// RetrieveSeries fetches all instances in a series.
	RetrieveSeries(ctx context.Context, studyUID, seriesUID string) ([]media.DICOMObject, error)
	// RetrieveInstance fetches a single DICOM instance.
	RetrieveInstance(ctx context.Context, studyUID, seriesUID, sopInstanceUID string) (media.DICOMObject, error)
	// RetrieveMetadata returns the DICOMweb JSON metadata for one instance.
	RetrieveMetadata(ctx context.Context, studyUID, seriesUID, sopInstanceUID string) (map[string]interface{}, error)
	// StoreInstances sends instances to the server via STOW-RS.
	// studyUID may be empty to address the generic STOW-RS endpoint.
	StoreInstances(ctx context.Context, studyUID string, objects []media.DICOMObject) error
	// SearchStudies queries for matching studies via QIDO-RS.
	SearchStudies(ctx context.Context, params url.Values) ([]map[string]interface{}, error)
	// SearchSeries queries for matching series within a study via QIDO-RS.
	SearchSeries(ctx context.Context, studyUID string, params url.Values) ([]map[string]interface{}, error)
	// SearchInstances queries for matching instances via QIDO-RS.
	SearchInstances(ctx context.Context, studyUID, seriesUID string, params url.Values) ([]map[string]interface{}, error)
	// SearchStudiesObjects queries for matching studies and returns the results
	// as DICOMObject values, converting the DICOMweb JSON representation.
	SearchStudiesObjects(ctx context.Context, params url.Values) ([]media.DICOMObject, error)
	// SearchSeriesObjects queries for matching series within a study and
	// returns the results as DICOMObject values.
	SearchSeriesObjects(ctx context.Context, studyUID string, params url.Values) ([]media.DICOMObject, error)
	// SearchInstancesObjects queries for matching instances and returns the
	// results as DICOMObject values.
	SearchInstancesObjects(ctx context.Context, studyUID, seriesUID string, params url.Values) ([]media.DICOMObject, error)
	// RetrieveFrames fetches the raw pixel data for specific frames of a DICOM
	// instance via WADO-RS. frames is a 1-based list of frame numbers matching
	// the DICOM standard. Each element of the returned slice is the uncompressed
	// or compressed pixel data for the corresponding requested frame.
	RetrieveFrames(ctx context.Context, studyUID, seriesUID, sopInstanceUID string, frames []int) ([][]byte, error)
	// DeleteStudy removes a study and all its series and instances via WADO-RS.
	DeleteStudy(ctx context.Context, studyUID string) error
	// DeleteSeries removes a series and all its instances via WADO-RS.
	DeleteSeries(ctx context.Context, studyUID, seriesUID string) error
	// DeleteInstance removes a single instance via WADO-RS.
	DeleteInstance(ctx context.Context, studyUID, seriesUID, sopInstanceUID string) error
}

type wadoClient struct {
	params     ClientParams
	httpClient *http.Client
}

// NewClient creates a new DICOMweb client.
func NewClient(params ClientParams) Client {
	// #nosec G402 -- InsecureTLS is an explicit opt-in controlled by the caller.
	tlsCfg := &tls.Config{InsecureSkipVerify: params.InsecureTLS} //nolint:gosec
	return &wadoClient{
		params: params,
		httpClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
			Timeout:   params.Timeout,
		},
	}
}

// ── WADO-RS retrieve ──────────────────────────────────────────────────────────

func (c *wadoClient) RetrieveStudy(ctx context.Context, studyUID string) ([]media.DICOMObject, error) {
	u := fmt.Sprintf("%s/wado/rs/studies/%s", c.params.BaseURL, studyUID)
	return c.retrieveMultipart(ctx, u)
}

func (c *wadoClient) RetrieveSeries(ctx context.Context, studyUID, seriesUID string) ([]media.DICOMObject, error) {
	u := fmt.Sprintf("%s/wado/rs/studies/%s/series/%s", c.params.BaseURL, studyUID, seriesUID)
	return c.retrieveMultipart(ctx, u)
}

func (c *wadoClient) RetrieveInstance(ctx context.Context, studyUID, seriesUID, sopInstanceUID string) (media.DICOMObject, error) {
	u := fmt.Sprintf("%s/wado/rs/studies/%s/series/%s/instances/%s",
		c.params.BaseURL, studyUID, seriesUID, sopInstanceUID)
	objects, err := c.retrieveMultipart(ctx, u)
	if err != nil {
		return nil, err
	}
	if len(objects) == 0 {
		return nil, fmt.Errorf("wado: no instance returned for %s", sopInstanceUID)
	}
	return objects[0], nil
}

func (c *wadoClient) RetrieveMetadata(ctx context.Context, studyUID, seriesUID, sopInstanceUID string) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/wado/rs/studies/%s/series/%s/instances/%s/metadata",
		c.params.BaseURL, studyUID, seriesUID, sopInstanceUID)
	resp, err := c.doRequest(ctx, http.MethodGet, u, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("wado: decode metadata: %w", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("wado: no metadata returned")
	}
	return result[0], nil
}

// ── STOW-RS store ─────────────────────────────────────────────────────────────

func (c *wadoClient) StoreInstances(ctx context.Context, studyUID string, objects []media.DICOMObject) error {
	// Stream the multipart body through a pipe to avoid buffering all objects
	// in memory at once.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := fmt.Sprintf(`multipart/related; type="application/dicom"; boundary=%s`, mw.Boundary())

	go func() {
		var writeErr error
		for _, obj := range objects {
			part, err := mw.CreatePart(textproto.MIMEHeader{
				"Content-Type": []string{"application/dicom"},
			})
			if err != nil {
				writeErr = err
				break
			}
			payload := obj.WriteToBytes()
			if len(payload) == 0 {
				if err := media.ValidateFileWrite(obj); err != nil {
					writeErr = err
				} else {
					writeErr = fmt.Errorf("empty DICOM payload")
				}
				break
			}
			if _, err := part.Write(payload); err != nil {
				writeErr = err
				break
			}
		}
		_ = mw.Close()
		pw.CloseWithError(writeErr)
	}()

	var u string
	if studyUID != "" {
		u = fmt.Sprintf("%s/stow/rs/studies/%s", c.params.BaseURL, studyUID)
	} else {
		u = fmt.Sprintf("%s/stow/rs/studies", c.params.BaseURL)
	}
	resp, err := c.doRequest(ctx, http.MethodPost, u, contentType, pr)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// ── QIDO-RS search ────────────────────────────────────────────────────────────

func (c *wadoClient) SearchStudies(ctx context.Context, params url.Values) ([]map[string]interface{}, error) {
	u := fmt.Sprintf("%s/qido/rs/studies", c.params.BaseURL)
	return c.searchJSON(ctx, u, params)
}

func (c *wadoClient) SearchSeries(ctx context.Context, studyUID string, params url.Values) ([]map[string]interface{}, error) {
	u := fmt.Sprintf("%s/qido/rs/studies/%s/series", c.params.BaseURL, studyUID)
	return c.searchJSON(ctx, u, params)
}

func (c *wadoClient) SearchInstances(ctx context.Context, studyUID, seriesUID string, params url.Values) ([]map[string]interface{}, error) {
	u := fmt.Sprintf("%s/qido/rs/studies/%s/series/%s/instances", c.params.BaseURL, studyUID, seriesUID)
	return c.searchJSON(ctx, u, params)
}

// ── Typed QIDO-RS search ──────────────────────────────────────────────────────

func (c *wadoClient) SearchStudiesObjects(ctx context.Context, params url.Values) ([]media.DICOMObject, error) {
	u := fmt.Sprintf("%s/qido/rs/studies", c.params.BaseURL)
	return c.searchObjects(ctx, u, params)
}

func (c *wadoClient) SearchSeriesObjects(ctx context.Context, studyUID string, params url.Values) ([]media.DICOMObject, error) {
	u := fmt.Sprintf("%s/qido/rs/studies/%s/series", c.params.BaseURL, studyUID)
	return c.searchObjects(ctx, u, params)
}

func (c *wadoClient) SearchInstancesObjects(ctx context.Context, studyUID, seriesUID string, params url.Values) ([]media.DICOMObject, error) {
	u := fmt.Sprintf("%s/qido/rs/studies/%s/series/%s/instances", c.params.BaseURL, studyUID, seriesUID)
	return c.searchObjects(ctx, u, params)
}

// searchObjects fetches a QIDO-RS endpoint and converts each DICOMweb JSON
// entry to a DICOMObject.
func (c *wadoClient) searchObjects(ctx context.Context, rawURL string, params url.Values) ([]media.DICOMObject, error) {
	rows, err := c.searchJSON(ctx, rawURL, params)
	if err != nil {
		return nil, err
	}
	objects := make([]media.DICOMObject, 0, len(rows))
	for _, row := range rows {
		objects = append(objects, qidoRowToObject(row))
	}
	return objects, nil
}

// qidoRowToObject converts a single DICOMweb JSON tag-map (as returned by
// QIDO-RS) to a DICOMObject.  Each key is an 8-hex-digit group/element string;
// each value is {"vr": "XX", "Value": [...]}.
func qidoRowToObject(row map[string]interface{}) media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	for key, rawTag := range row {
		if len(key) != 8 {
			continue
		}
		var grp, elem uint16
		if n, err := strconv.ParseUint(key[:4], 16, 16); err == nil {
			grp = uint16(n)
		} else {
			continue
		}
		if n, err := strconv.ParseUint(key[4:], 16, 16); err == nil {
			elem = uint16(n)
		} else {
			continue
		}
		tagMap, ok := rawTag.(map[string]interface{})
		if !ok {
			continue
		}
		vr, _ := tagMap["vr"].(string)
		values, _ := tagMap["Value"].([]interface{})
		var strVal string
		if len(values) > 0 {
			if s, ok := values[0].(string); ok {
				strVal = s
			} else if n, ok := jsonNumber(values[0]); ok {
				strVal = fmt.Sprintf("%d", n)
			}
		}
		obj.Add(media.NewStringTag(grp, elem, vr, strVal))
	}
	return obj
}

// jsonNumber coerces JSON numeric types (float64 from encoding/json) to int64.
func jsonNumber(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

// RetrieveFrames fetches raw pixel data for the given 1-based frame numbers.
func (c *wadoClient) RetrieveFrames(ctx context.Context, studyUID, seriesUID, sopInstanceUID string, frames []int) ([][]byte, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("wado: RetrieveFrames: frames list must not be empty")
	}
	parts := make([]string, len(frames))
	for i, f := range frames {
		parts[i] = fmt.Sprintf("%d", f)
	}
	u := fmt.Sprintf("%s/wado/rs/studies/%s/series/%s/instances/%s/frames/%s",
		c.params.BaseURL, studyUID, seriesUID, sopInstanceUID,
		strings.Join(parts, ","))
	return c.retrieveMultipartBytes(ctx, u)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// ── WADO-RS delete ────────────────────────────────────────────────────────────

func (c *wadoClient) DeleteStudy(ctx context.Context, studyUID string) error {
	u := fmt.Sprintf("%s/wado/rs/studies/%s", c.params.BaseURL, studyUID)
	resp, err := c.doRequest(ctx, http.MethodDelete, u, "", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *wadoClient) DeleteSeries(ctx context.Context, studyUID, seriesUID string) error {
	u := fmt.Sprintf("%s/wado/rs/studies/%s/series/%s", c.params.BaseURL, studyUID, seriesUID)
	resp, err := c.doRequest(ctx, http.MethodDelete, u, "", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *wadoClient) DeleteInstance(ctx context.Context, studyUID, seriesUID, sopInstanceUID string) error {
	u := fmt.Sprintf("%s/wado/rs/studies/%s/series/%s/instances/%s",
		c.params.BaseURL, studyUID, seriesUID, sopInstanceUID)
	resp, err := c.doRequest(ctx, http.MethodDelete, u, "", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// eachMultipartPart fetches a WADO-RS URL and calls fn for each successfully read part.
func (c *wadoClient) eachMultipartPart(ctx context.Context, rawURL string, fn func([]byte)) error {
	resp, err := c.doRequest(ctx, http.MethodGet, rawURL, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	mediaType, mparams, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return fmt.Errorf("wado: expected multipart response, got %q", resp.Header.Get("Content-Type"))
	}

	mr := multipart.NewReader(resp.Body, mparams["boundary"])
	for {
		part, partErr := mr.NextPart()
		if partErr != nil {
			break
		}
		data, readErr := io.ReadAll(part)
		if readErr != nil {
			continue
		}
		fn(data)
	}
	return nil
}

// retrieveMultipartBytes fetches a WADO-RS URL and returns each part as raw bytes.
// Used for frame retrieval where each part is pixel data, not a DICOM object.
func (c *wadoClient) retrieveMultipartBytes(ctx context.Context, rawURL string) ([][]byte, error) {
	var results [][]byte
	err := c.eachMultipartPart(ctx, rawURL, func(data []byte) {
		results = append(results, data)
	})
	return results, err
}

// retrieveMultipart fetches a WADO-RS URL and returns parsed DICOM objects.
func (c *wadoClient) retrieveMultipart(ctx context.Context, rawURL string) ([]media.DICOMObject, error) {
	var objects []media.DICOMObject
	err := c.eachMultipartPart(ctx, rawURL, func(data []byte) {
		if obj, parseErr := media.NewDCMObjFromBytes(data); parseErr == nil {
			objects = append(objects, obj)
		}
	})
	return objects, err
}

// searchJSON performs a QIDO-RS GET and decodes the JSON response.
func (c *wadoClient) searchJSON(ctx context.Context, rawURL string, params url.Values) ([]map[string]interface{}, error) {
	if len(params) > 0 {
		rawURL = rawURL + "?" + params.Encode()
	}
	resp, err := c.doRequest(ctx, http.MethodGet, rawURL, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("wado: decode QIDO response: %w", err)
	}
	return result, nil
}

// doRequest executes an HTTP request with the configured auth/headers.
func (c *wadoClient) doRequest(ctx context.Context, method, rawURL, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("wado: build request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/dicom+json")
	if c.params.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.params.Bearer)
	}
	if c.params.BasicUser != "" {
		req.SetBasicAuth(c.params.BasicUser, c.params.BasicPass)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wado: http: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("wado: unexpected status %d", resp.StatusCode)
	}
	return resp, nil
}
