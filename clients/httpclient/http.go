package httpclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"
)

// HTTPClient interface
type HTTPClient interface {
	Delete() ([]byte, error)
	Get() ([]byte, error)
	Patch(body io.Reader) ([]byte, error)
	Post(body io.Reader) ([]byte, error)
	PostDicom(fieldName string, fileName string, content io.Reader) ([]byte, error)
	PostMulti(fieldName string, fileName string, content io.Reader) ([]byte, error)
	PostMultiContent(fieldName string, fileName string, contentType string, content io.Reader) ([]byte, error)
	Put(body io.Reader) ([]byte, error)
}

type httpClient struct {
	Params HTTPClientParams
}

// HTTPClientParams are connection parameters
type HTTPClientParams struct {
	URL                 string
	Proxy               string
	Timeout             int64
	URLAccessToken      string
	ContentType         string
	AcceptType          string
	DisableCompression  bool
	InsecureTLS         bool // skip TLS certificate verification; use only in development/testing
	AuthorizationBearer string
	AuthorizationKey    string
	AuthorizationToken  string
	BasicAuthUser       string
	BasicAuthPass       string
	Headers             map[string]string
	Queries             map[string]string
}

// NewHTTPClient returns a new http client
func NewHTTPClient(params HTTPClientParams) HTTPClient {
	return &httpClient{
		Params: params,
	}
}

// GetOAuthToken - Gets a token from OAuth2 endpoint
func GetOAuthToken(tokenURL string, form url.Values) (map[string]string, error) {
	params := HTTPClientParams{
		URL:         tokenURL,
		ContentType: "application/x-www-form-urlencoded",
	}

	client := NewHTTPClient(params)

	response, err := client.Post(strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	token := make(map[string]string)
	err = json.Unmarshal(response, &token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// Delete sends DELETE request
func (h *httpClient) Delete() ([]byte, error) {
	request, err := http.NewRequest("DELETE", h.Params.URL, nil)
	if err != nil {
		return nil, err
	}

	return h.sendRequest(request)
}

// Get sends GET request
func (h *httpClient) Get() ([]byte, error) {
	request, err := http.NewRequest("GET", h.Params.URL, nil)
	if err != nil {
		return nil, err
	}

	return h.sendRequest(request)
}

// Patch sends PATCH request
func (h *httpClient) Patch(body io.Reader) ([]byte, error) {
	request, err := http.NewRequest("PATCH", h.Params.URL, body)
	if err != nil {
		return nil, err
	}
	return h.sendRequest(request)
}

// Post sends POST request
func (h *httpClient) Post(body io.Reader) ([]byte, error) {
	request, err := http.NewRequest("POST", h.Params.URL, body)
	if err != nil {
		return nil, err
	}
	return h.sendRequest(request)
}

// PostDicom - sends a multipart post
func (h *httpClient) PostDicom(fieldName string, fileName string, content io.Reader) ([]byte, error) {
	return h.PostMultiContent(fieldName, fileName, "application/dicom", content)
}

// PostMulti - sends a multipart post
func (h *httpClient) PostMulti(fieldName string, fileName string, content io.Reader) ([]byte, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, fieldName)
	if err != nil {
		return nil, err
	}

	byteCount, err := io.Copy(part, content)
	if err != nil {
		return nil, err
	}

	h.Params.ContentType = fmt.Sprintf("%s; boundary=%s", h.Params.ContentType, writer.Boundary())

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	h.Params.Headers["Content-Length"] = fmt.Sprintf("%d", byteCount)

	request, err := http.NewRequest("POST", h.Params.URL, body)
	if err != nil {
		return nil, err
	}
	return h.sendRequest(request)
}

// PostMultiContent - sends a multipart post
func (h *httpClient) PostMultiContent(fieldName string, fileName string, contentType string, content io.Reader) ([]byte, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	mediaHeader := textproto.MIMEHeader{}
	mediaHeader.Set("Content-Disposition", fmt.Sprintf("attachment; name=\"%s\"; filename=\"%s\"", fieldName, fileName))
	mediaHeader.Set("Content-Type", contentType)
	mediaPart, err := writer.CreatePart(mediaHeader)
	if err != nil {
		return nil, err
	}

	byteCount, err := io.Copy(mediaPart, content)
	if err != nil {
		return nil, err
	}

	h.Params.ContentType = fmt.Sprintf("%s; boundary=%s", h.Params.ContentType, writer.Boundary())

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	h.Params.Headers["Content-Length"] = fmt.Sprintf("%d", byteCount)

	request, err := http.NewRequest("POST", h.Params.URL, body)
	if err != nil {
		return nil, err
	}
	return h.sendRequest(request)
}

// Put sends PUT request
func (h *httpClient) Put(body io.Reader) ([]byte, error) {
	request, err := http.NewRequest("PUT", h.Params.URL, body)
	if err != nil {
		return nil, err
	}
	return h.sendRequest(request)
}

func (h *httpClient) setupRequest(request *http.Request) {
	if h.Params.ContentType != "" {
		request.Header.Set("Content-Type", h.Params.ContentType)
	}
	if request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if h.Params.AcceptType != "" {
		request.Header.Set("Accept", h.Params.AcceptType)
	}
	if request.Header.Get("Accept") == "" {
		request.Header.Set("Accept", "application/json")
	}
	if h.Params.AuthorizationBearer != "" {
		request.Header.Set("Authorization", "bearer "+h.Params.AuthorizationBearer)
	}
	if h.Params.AuthorizationKey != "" {
		request.Header.Set("Authorization", "key="+h.Params.AuthorizationKey)
	}
	if h.Params.AuthorizationToken != "" {
		request.Header.Set("Authorization", "token "+h.Params.AuthorizationToken)
	}
	for key, value := range h.Params.Headers {
		request.Header.Set(key, value)
	}
	if h.Params.BasicAuthUser != "" && h.Params.BasicAuthPass != "" {
		request.SetBasicAuth(h.Params.BasicAuthUser, h.Params.BasicAuthPass)
	}

	q := request.URL.Query()
	for key, value := range h.Params.Queries {
		q.Add(key, value)
	}
	if h.Params.URLAccessToken != "" {
		q.Add("access_token", h.Params.URLAccessToken)
	}
	request.URL.RawQuery = q.Encode()
}

func (h *httpClient) sendRequest(request *http.Request) ([]byte, error) {
	h.setupRequest(request)

	// #nosec G402 -- InsecureTLS is an explicit opt-in controlled by the caller
	tlsCfg := &tls.Config{InsecureSkipVerify: h.Params.InsecureTLS} //nolint:gosec

	var client *http.Client
	if h.Params.Proxy != "" {
		proxy, err := url.ParseRequestURI(h.Params.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid Proxy URL found '%s'", h.Params.Proxy)
		}
		t := &http.Transport{
			Proxy:              http.ProxyURL(proxy),
			TLSClientConfig:    tlsCfg,
			DisableCompression: h.Params.DisableCompression,
		}
		client = &http.Client{
			Transport: t,
			Timeout:   time.Duration(h.Params.Timeout) * time.Second,
		}
	} else {
		t := &http.Transport{
			TLSClientConfig:    tlsCfg,
			DisableCompression: h.Params.DisableCompression,
		}
		client = &http.Client{
			Transport: t,
			Timeout:   time.Duration(h.Params.Timeout) * time.Second,
		}
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("received unexpected status code %d:%s", response.StatusCode, response.Status)
		}
		return body, fmt.Errorf("received unexpected status code %d:%s", response.StatusCode, response.Status)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
