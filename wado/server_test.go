package wado_test

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"testing"

	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/wado"
)

// ── mockStore ─────────────────────────────────────────────────────────────────

type mockStore struct {
	studies map[string][]media.DICOMObject // studyUID -> instances
	stored  []media.DICOMObject
}

func newMockStore() *mockStore {
	return &mockStore{studies: make(map[string][]media.DICOMObject)}
}

func (m *mockStore) RetrieveStudy(_ context.Context, studyUID string) ([]media.DICOMObject, error) {
	if objs, ok := m.studies[studyUID]; ok {
		return objs, nil
	}
	return nil, fmt.Errorf("study %q not found", studyUID)
}

func (m *mockStore) RetrieveSeries(_ context.Context, studyUID, _ string) ([]media.DICOMObject, error) {
	return m.RetrieveStudy(context.Background(), studyUID)
}

func (m *mockStore) RetrieveInstance(_ context.Context, studyUID, _, _ string) (media.DICOMObject, error) {
	objs, err := m.RetrieveStudy(context.Background(), studyUID)
	if err != nil || len(objs) == 0 {
		return nil, fmt.Errorf("instance not found")
	}
	return objs[0], nil
}

func (m *mockStore) StoreInstances(_ context.Context, objects []media.DICOMObject) error {
	m.stored = append(m.stored, objects...)
	return nil
}

func (m *mockStore) SearchStudies(_ context.Context, _ url.Values) ([]media.DICOMObject, error) {
	result := make([]media.DICOMObject, 0)
	for _, objs := range m.studies {
		result = append(result, objs...)
	}
	return result, nil
}

func (m *mockStore) SearchSeries(_ context.Context, studyUID string, _ url.Values) ([]media.DICOMObject, error) {
	objs := m.studies[studyUID] // returns nil (not an error) when absent
	return objs, nil
}

func (m *mockStore) SearchInstances(_ context.Context, studyUID, _ string, _ url.Values) ([]media.DICOMObject, error) {
	objs := m.studies[studyUID]
	return objs, nil
}

// ── test helpers ──────────────────────────────────────────────────────────────

func loadSampleDICOM(t *testing.T) media.DICOMObject {
	t.Helper()
	media.InitDict()
	obj, err := media.NewDCMObjFromFile("../testdata/test.dcm")
	if err != nil {
		t.Skipf("sample DICOM unavailable: %v", err)
	}
	return obj
}

func newTestHandler(t *testing.T) (http.Handler, *mockStore) {
	t.Helper()
	store := newMockStore()
	srv := wado.NewServer(wado.ServerParams{Port: 0, Store: store})
	return srv.Handler(), store
}

// ── WADO-RS tests ─────────────────────────────────────────────────────────────

func TestRetrieveStudy_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/wado/rs/studies/1.2.3.missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestRetrieveStudy_OK(t *testing.T) {
	h, store := newTestHandler(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/wado/rs/studies/1.2.3", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "multipart/related") {
		t.Fatalf("want multipart/related, got %q", ct)
	}
}

func TestRetrieveSeries_OK(t *testing.T) {
	h, store := newTestHandler(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/wado/rs/studies/1.2.3/series/1.2.3.1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestRetrieveInstance_OK(t *testing.T) {
	h, store := newTestHandler(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/wado/rs/studies/1.2.3/series/1.2.3.1/instances/1.2.3.1.1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestRetrieveStudyMetadata_OK(t *testing.T) {
	h, store := newTestHandler(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/wado/rs/studies/1.2.3/metadata", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/dicom+json") {
		t.Fatalf("want application/dicom+json, got %q", ct)
	}
}

func TestRetrieveSeriesMetadata_OK(t *testing.T) {
	h, store := newTestHandler(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/wado/rs/studies/1.2.3/series/1.2.3.1/metadata", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestRetrieveInstanceMetadata_OK(t *testing.T) {
	h, store := newTestHandler(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/wado/rs/studies/1.2.3/series/1.2.3.1/instances/1.2.3.1.1/metadata", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

// ── STOW-RS tests ─────────────────────────────────────────────────────────────

func buildMultipartDICOMBody(t *testing.T, obj media.DICOMObject) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": []string{"application/dicom"}})
	if err != nil {
		t.Fatal(err)
	}
	part.Write(obj.WriteToBytes())
	mw.Close()
	contentType := fmt.Sprintf(`multipart/related; type="application/dicom"; boundary=%s`, mw.Boundary())
	return body, contentType
}

func TestStoreInstances_OK(t *testing.T) {
	h, store := newTestHandler(t)
	body, ct := buildMultipartDICOMBody(t, loadSampleDICOM(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/stow/rs/studies", body)
	req.Header.Set("Content-Type", ct)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(store.stored) != 1 {
		t.Fatalf("want 1 stored instance, got %d", len(store.stored))
	}
}

func TestStoreInstances_WithStudyUID(t *testing.T) {
	h, store := newTestHandler(t)
	body, ct := buildMultipartDICOMBody(t, loadSampleDICOM(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/stow/rs/studies/1.2.3", body)
	req.Header.Set("Content-Type", ct)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if len(store.stored) != 1 {
		t.Fatalf("want 1 stored instance, got %d", len(store.stored))
	}
}

func TestStoreInstances_BadContentType(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/stow/rs/studies", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestStoreInstances_MissingBoundary(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/stow/rs/studies", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/related; type=\"application/dicom\"")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

// ── QIDO-RS tests ─────────────────────────────────────────────────────────────

func TestSearchStudies_Empty(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/qido/rs/studies", nil))
	// Per DICOM PS3.18 §10.6.3.3, an empty result set must return 204 No Content.
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}
}

func TestSearchStudies_WithResults(t *testing.T) {
	h, store := newTestHandler(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/qido/rs/studies?PatientName=TEST", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestSearchSeries_OK(t *testing.T) {
	h, store := newTestHandler(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/qido/rs/studies/1.2.3/series", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestSearchSeries_Empty(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/qido/rs/studies/1.2.3/series", nil))
	// Per DICOM PS3.18 §10.6.3.3, an empty result set must return 204 No Content.
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}
}

func TestSearchInstances_OK(t *testing.T) {
	h, store := newTestHandler(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/qido/rs/studies/1.2.3/series/1.2.3.1/instances", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestSearchInstances_Empty(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/qido/rs/studies/1.2.3/series/1.2.3.1/instances", nil))
	// Per DICOM PS3.18 §10.6.3.3, an empty result set must return 204 No Content.
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}
}
