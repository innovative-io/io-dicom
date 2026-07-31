package wado_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/innovative-io/io-dicom/wado"
)

// TestClientRejectsURLInjectingUIDs pins UID validation on the client.
//
// UIDs were interpolated into URLs raw, so one arriving from an untrusted
// source — a QIDO result, a worklist entry, a user field — could redirect the
// request. "1.2.3?evil=1" appended a query string and
// "1.2.3/../../../admin/shutdown" reached a different endpoint entirely. On
// DeleteStudy that redirects a destructive call.
func TestClientRejectsURLInjectingUIDs(t *testing.T) {
	var reached []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = append(reached, r.URL.Path+"?"+r.URL.RawQuery)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	ctx := context.Background()

	hostile := []string{
		"1.2.3?evil=1",
		"1.2.3/../../../admin/shutdown",
		"1.2.3/series/9.9.9/instances/8.8.8",
		"../../secret",
		"not-a-uid",
		strings.Repeat("1", 65),
	}

	for _, uid := range hostile {
		if err := c.DeleteStudy(ctx, uid); err == nil {
			t.Errorf("DeleteStudy(%q) was allowed to build a request", uid)
		}
		if _, err := c.RetrieveStudy(ctx, uid); err == nil {
			t.Errorf("RetrieveStudy(%q) was allowed to build a request", uid)
		}
	}

	if len(reached) != 0 {
		t.Fatalf("hostile UIDs reached the server: %v", reached)
	}
}

// TestClientAcceptsValidUIDs guards the validation from rejecting real UIDs.
func TestClientAcceptsValidUIDs(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	const uid = "1.2.840.113619.2.30.1.1762295590.1623.978668949.886"
	if err := c.DeleteStudy(context.Background(), uid); err != nil {
		t.Fatalf("DeleteStudy with a valid UID: %v", err)
	}
	if want := "/wado/rs/studies/" + uid; gotPath != want {
		t.Fatalf("got path %q, want %q", gotPath, want)
	}
}

// TestClientReportsUnparseableParts pins error propagation. Parts that failed to
// read or parse were skipped with `continue` and the call returned nil, so a
// caller could not distinguish "the study has 2 instances" from "the study has
// 40 and 38 were corrupt".
func TestClientReportsUnparseableParts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", `multipart/related; type="application/dicom"; boundary=BOUNDARY`)
		w.WriteHeader(http.StatusOK)
		// Two parts, neither a valid DICOM object.
		_, _ = w.Write([]byte("--BOUNDARY\r\nContent-Type: application/dicom\r\n\r\nnot dicom\r\n" +
			"--BOUNDARY\r\nContent-Type: application/dicom\r\n\r\nalso not dicom\r\n--BOUNDARY--\r\n"))
	}))
	defer srv.Close()

	c := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	objs, err := c.RetrieveStudy(context.Background(), "1.2.3")
	if err == nil {
		t.Fatalf("unparseable parts reported success with %d objects", len(objs))
	}
	if len(objs) != 0 {
		t.Fatalf("expected no objects, got %d", len(objs))
	}
}

// TestClientBoundsPartSize pins the read cap. io.ReadAll had no ceiling on the
// client side, so a hostile or compromised peer could OOM it — an audit measured
// a single 300 MiB part allocating 775 MiB, with 10 GiB equally accepted.
func TestClientBoundsPartSize(t *testing.T) {
	const oversize = 70 << 20 // above the 64 MiB per-part cap
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", `multipart/related; type="application/dicom"; boundary=BOUNDARY`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("--BOUNDARY\r\nContent-Type: application/dicom\r\n\r\n"))
		chunk := make([]byte, 1<<20)
		for sent := 0; sent < oversize; sent += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
		_, _ = w.Write([]byte("\r\n--BOUNDARY--\r\n"))
	}))
	defer srv.Close()

	c := wado.NewClient(wado.ClientParams{BaseURL: srv.URL})
	if _, err := c.RetrieveStudy(context.Background(), "1.2.3"); err == nil {
		t.Fatal("an oversized part was accepted without error")
	}
}
