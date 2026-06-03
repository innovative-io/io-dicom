package wado_test

import (
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"testing"

	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/wado"
)

func newWADOTestServer(t *testing.T) (*httptest.Server, *mockStore) {
	t.Helper()
	h, store := newTestHandler(t)
	return httptest.NewServer(h), store
}

func TestClient_RetrieveStudy_OK(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	objects, err := client.RetrieveStudy(context.Background(), "1.2.3")
	if err != nil {
		t.Fatalf("RetrieveStudy() error: %v", err)
	}
	if len(objects) == 0 {
		t.Fatal("want at least one object")
	}
}

func TestClient_RetrieveStudy_NotFound(t *testing.T) {
	srv, _ := newWADOTestServer(t)
	defer srv.Close()
	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	_, err := client.RetrieveStudy(context.Background(), "0.0.0.missing")
	if err == nil {
		t.Fatal("want error for missing study")
	}
}

func TestClient_RetrieveSeries_OK(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	objects, err := client.RetrieveSeries(context.Background(), "1.2.3", "1.2.3.1")
	if err != nil {
		t.Fatalf("RetrieveSeries() error: %v", err)
	}
	if len(objects) == 0 {
		t.Fatal("want at least one object")
	}
}

func TestClient_RetrieveInstance_OK(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	obj, err := client.RetrieveInstance(context.Background(), "1.2.3", "1.2.3.1", "1.2.3.1.1")
	if err != nil {
		t.Fatalf("RetrieveInstance() error: %v", err)
	}
	if obj == nil {
		t.Fatal("want non-nil object")
	}
}

func TestClient_RetrieveMetadata_OK(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	meta, err := client.RetrieveMetadata(context.Background(), "1.2.3", "1.2.3.1", "1.2.3.1.1")
	if err != nil {
		t.Fatalf("RetrieveMetadata() error: %v", err)
	}
	if len(meta) == 0 {
		t.Fatal("want non-empty metadata map")
	}
}

func TestClient_StoreInstances_OK(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	obj := loadSampleDICOM(t)

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	err := client.StoreInstances(context.Background(), "", []media.DICOMObject{obj})
	if err != nil {
		t.Fatalf("StoreInstances() error: %v", err)
	}
	if len(store.stored) != 1 {
		t.Fatalf("want 1 stored, got %d", len(store.stored))
	}
}

func TestClient_StoreInstances_WithStudyUID(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	obj := loadSampleDICOM(t)

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	err := client.StoreInstances(context.Background(), "1.2.3", []media.DICOMObject{obj})
	if err != nil {
		t.Fatalf("StoreInstances() error: %v", err)
	}
	if len(store.stored) != 1 {
		t.Fatalf("want 1 stored, got %d", len(store.stored))
	}
}

func TestClient_StoreInstances_NilSlice(t *testing.T) {
	srv, _ := newWADOTestServer(t)
	defer srv.Close()

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	err := client.StoreInstances(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("StoreInstances(nil) error: %v", err)
	}
}

func TestClient_SearchStudies_OK(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	results, err := client.SearchStudies(context.Background(), nil)
	if err != nil {
		t.Fatalf("SearchStudies() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("want at least one result")
	}
}

func TestClient_SearchStudies_WithParams(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	params := url.Values{"PatientName": []string{"TEST"}}
	_, err := client.SearchStudies(context.Background(), params)
	if err != nil {
		t.Fatalf("SearchStudies() error: %v", err)
	}
}

func TestClient_SearchSeries_OK(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	_, err := client.SearchSeries(context.Background(), "1.2.3", nil)
	if err != nil {
		t.Fatalf("SearchSeries() error: %v", err)
	}
}

func TestClient_SearchInstances_OK(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	_, err := client.SearchInstances(context.Background(), "1.2.3", "1.2.3.1", nil)
	if err != nil {
		t.Fatalf("SearchInstances() error: %v", err)
	}
}

func TestClient_RetrieveFrames_OK(t *testing.T) {
	// Serve a stub multipart response with two raw-bytes frame parts.
	frame1 := []byte{0x01, 0x02, 0x03}
	frame2 := []byte{0x04, 0x05, 0x06}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw := multipart.NewWriter(w)
		w.Header().Set("Content-Type", "multipart/related; type=\"application/octet-stream\"; boundary="+mw.Boundary())
		for _, frame := range [][]byte{frame1, frame2} {
			part, _ := mw.CreatePart(textproto.MIMEHeader{
				"Content-Type": []string{"application/octet-stream"},
			})
			_, _ = part.Write(frame)
		}
		_ = mw.Close()
	}))
	defer srv.Close()

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	frames, err := client.RetrieveFrames(context.Background(), "s", "se", "sop", []int{1, 2})
	if err != nil {
		t.Fatalf("RetrieveFrames() error: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}
	if string(frames[0]) != string(frame1) {
		t.Errorf("frame[0] = %v, want %v", frames[0], frame1)
	}
	if string(frames[1]) != string(frame2) {
		t.Errorf("frame[1] = %v, want %v", frames[1], frame2)
	}
}

func TestClient_RetrieveFrames_EmptyList(t *testing.T) {
	client := wado.NewClient(wado.ClientParams{BaseURL: "http://127.0.0.1:1"})
	_, err := client.RetrieveFrames(context.Background(), "s", "se", "sop", nil)
	if err == nil {
		t.Fatal("want error for empty frame list")
	}
}

func TestClient_RetrieveFrames_ViaServer(t *testing.T) {
	srv, store := newWADOTestServer(t)
	defer srv.Close()
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOM(t)}

	client := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	// Frame retrieval may return 0 frames if the sample has no pixel data — that's OK.
	// What matters is no protocol/parse error.
	_, err := client.RetrieveFrames(context.Background(), "1.2.3", "1.2.3.1", "1.2.3.1.1", []int{1})
	if err != nil {
		t.Fatalf("RetrieveFrames() via server error: %v", err)
	}
}

func TestClient_UnreachableServer(t *testing.T) {
	client := wado.NewClient(wado.ClientParams{BaseURL: "http://127.0.0.1:1"})
	_, err := client.RetrieveStudy(context.Background(), "1.2.3")
	if err == nil {
		t.Fatal("want error for unreachable server")
	}
}

// ── Delete via Client ─────────────────────────────────────────────────────────

func startServerForDelete(t *testing.T) (wado.Client, *mockStore) {
	t.Helper()
	store := newMockStore()
	srv := wado.NewServer(wado.ServerParams{Store: store})
	h := srv.Handler()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return wado.NewClient(wado.ClientParams{BaseURL: ts.URL}), store
}

func TestClient_DeleteStudy_OK(t *testing.T) {
	client, store := startServerForDelete(t)
	store.studies["1.2.3"] = []media.DICOMObject{}
	if err := client.DeleteStudy(context.Background(), "1.2.3"); err != nil {
		t.Fatalf("DeleteStudy error: %v", err)
	}
}

func TestClient_DeleteStudy_NotFound(t *testing.T) {
	client, _ := startServerForDelete(t)
	if err := client.DeleteStudy(context.Background(), "9.9.9"); err == nil {
		t.Fatal("want error for missing study")
	}
}

func TestClient_DeleteSeries_OK(t *testing.T) {
	client, store := startServerForDelete(t)
	store.studies["1.2.3"] = []media.DICOMObject{}
	if err := client.DeleteSeries(context.Background(), "1.2.3", "1.2.3.1"); err != nil {
		t.Fatalf("DeleteSeries error: %v", err)
	}
}

func TestClient_DeleteInstance_OK(t *testing.T) {
	client, store := startServerForDelete(t)
	store.studies["1.2.3"] = []media.DICOMObject{}
	if err := client.DeleteInstance(context.Background(), "1.2.3", "1.2.3.1", "1.2.3.1.1"); err != nil {
		t.Fatalf("DeleteInstance error: %v", err)
	}
}

// ensure http and httptest imports are referenced
var _ = http.StatusOK
var _ *httptest.Server

// ── Typed QIDO-RS search ──────────────────────────────────────────────────────

func TestClient_SearchStudiesObjects_OK(t *testing.T) {
	client, store := startServerForDelete(t) // reuse helper (starts full server)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOMFromClient(t)}
	objs, err := client.SearchStudiesObjects(context.Background(), nil)
	if err != nil {
		t.Fatalf("SearchStudiesObjects error: %v", err)
	}
	if len(objs) == 0 {
		t.Fatal("want at least one object, got none")
	}
}

func TestClient_SearchSeriesObjects_OK(t *testing.T) {
	client, store := startServerForDelete(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOMFromClient(t)}
	objs, err := client.SearchSeriesObjects(context.Background(), "1.2.3", nil)
	if err != nil {
		t.Fatalf("SearchSeriesObjects error: %v", err)
	}
	if len(objs) == 0 {
		t.Fatal("want at least one object, got none")
	}
}

func TestClient_SearchInstancesObjects_OK(t *testing.T) {
	client, store := startServerForDelete(t)
	store.studies["1.2.3"] = []media.DICOMObject{loadSampleDICOMFromClient(t)}
	objs, err := client.SearchInstancesObjects(context.Background(), "1.2.3", "1.2.3.1", nil)
	if err != nil {
		t.Fatalf("SearchInstancesObjects error: %v", err)
	}
	if len(objs) == 0 {
		t.Fatal("want at least one object, got none")
	}
}

func loadSampleDICOMFromClient(t *testing.T) media.DICOMObject {
	t.Helper()
	obj, err := media.NewDCMObjFromFile("../testdata/test.dcm")
	if err != nil {
		t.Skipf("sample DICOM unavailable: %v", err)
	}
	return obj
}
