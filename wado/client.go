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
}

type wadoClient struct {
	params     ClientParams
	httpClient *http.Client
}

// NewClient creates a new DICOMweb client and initialises the DICOM dictionary.
func NewClient(params ClientParams) Client {
	media.InitDict()
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

// ── helpers ───────────────────────────────────────────────────────────────────

// retrieveMultipart fetches a WADO-RS URL and returns parsed DICOM objects.
func (c *wadoClient) retrieveMultipart(ctx context.Context, rawURL string) ([]media.DICOMObject, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, rawURL, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	mediaType, mparams, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return nil, fmt.Errorf("wado: expected multipart response, got %q",
			resp.Header.Get("Content-Type"))
	}

	mr := multipart.NewReader(resp.Body, mparams["boundary"])
	var objects []media.DICOMObject
	for {
		part, partErr := mr.NextPart()
		if partErr != nil {
			break
		}
		data, readErr := io.ReadAll(part)
		if readErr != nil {
			continue
		}
		obj, parseErr := media.NewDCMObjFromBytes(data)
		if parseErr != nil {
			continue
		}
		objects = append(objects, obj)
	}
	return objects, nil
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
