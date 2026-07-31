package wado_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
)

// TestPathTraversalUIDsRejected pins UID validation on every path handler.
//
// Go's ServeMux percent-decodes path wildcards, so `..%2f..%2f` reaches the
// handler as the literal `../../` and `a%00b` arrives with an embedded NUL.
// Those previously went to the Store verbatim. io-scp's Store resolves UIDs
// through a query engine and is unaffected, but the Store interface documents no
// validation obligation and the canonical DICOM layout is
// {studyUID}/{seriesUID}/{sopUID}.dcm — a filesystem- or object-store-backed
// implementation would inherit an arbitrary read/delete primitive.
func TestPathTraversalUIDsRejected(t *testing.T) {
	h, _ := newTestHandler(t)

	hostile := []string{
		"..%2f..%2f..%2fetc%2fpasswd",
		"%2e%2e%2f%2e%2e%2fsecret",
		"%2fabsolute%2fpath",
		"a%00b",
		"1.2.3%2f..%2f..%2fadmin",
		strings.Repeat("1", 65), // over the 64-character UID limit
		"not-a-uid",
	}

	for _, uid := range hostile {
		for _, tmpl := range []struct{ method, path string }{
			{"GET", "/wado/rs/studies/%s"},
			{"GET", "/wado/rs/studies/%s/metadata"},
			{"GET", "/wado/rs/studies/1.2.3/series/%s"},
			{"DELETE", "/wado/rs/studies/%s"},
			{"DELETE", "/wado/rs/studies/1.2.3/series/1.2.4/instances/%s"},
		} {
			path := fmt.Sprintf(tmpl.path, uid)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tmpl.method, path, nil))
			// Acceptable: 400 from validation, or a 3xx where ServeMux
			// normalised the path away before routing. Anything else — notably
			// 404 — means the hostile string reached the Store and was treated
			// as a legitimate UID.
			redirected := rec.Code >= 300 && rec.Code < 400
			if rec.Code != http.StatusBadRequest && !redirected {
				t.Errorf("%s %s: got %d; the UID reached the Store instead of being rejected",
					tmpl.method, path, rec.Code)
			}
		}
	}
}

// TestValidUIDsStillAccepted guards the validator from rejecting real UIDs.
func TestValidUIDsStillAccepted(t *testing.T) {
	h, _ := newTestHandler(t)
	for _, uid := range []string{
		"1.2.840.10008.1.1",
		"1.2.840.113619.2.30.1.1762295590.1623.978668949.886",
		"1",
		strings.Repeat("1", 64), // exactly the 64-character limit
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/wado/rs/studies/"+uid, nil))
		if rec.Code == http.StatusBadRequest {
			t.Errorf("valid UID %q was rejected as malformed", uid)
		}
	}
}

// multipartBody builds a STOW-RS body from the supplied part payloads.
func multipartBody(parts ...[]byte) (body []byte, contentType string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		w, _ := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type": []string{"application/dicom"},
		})
		_, _ = w.Write(p)
	}
	_ = mw.Close()
	return buf.Bytes(), fmt.Sprintf(`multipart/related; type="application/dicom"; boundary=%s`, mw.Boundary())
}

// TestStoreReportsFailureWhenNothingStored is the data-loss guard.
//
// Every part that failed to read or parse was skipped with `continue`, and the
// handler unconditionally wrote 200 with an empty ReferencedSOPSequence. A
// sending modality that treats 200 as "safely archived" and deletes its local
// copy would lose the study. PS3.18 §10.5 requires a failure status when no
// instances are stored.
func TestStoreReportsFailureWhenNothingStored(t *testing.T) {
	h, store := newTestHandler(t)

	cases := []struct {
		name  string
		parts [][]byte
	}{
		{"all parts unparseable", [][]byte{[]byte("not dicom"), []byte("also not dicom")}},
		{"single garbage part", [][]byte{bytes.Repeat([]byte{0xFF}, 512)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, ct := multipartBody(tc.parts...)
			req := httptest.NewRequest("POST", "/stow/rs/studies", bytes.NewReader(body))
			req.Header.Set("Content-Type", ct)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			// Nothing was stored, so the status must be a failure. 200 and 202
			// both tell a client the upload was accepted; a modality that then
			// deletes its local copy loses the study.
			if rec.Code < 400 {
				t.Fatalf("stored nothing but returned %d — a client would treat the "+
					"study as archived (body: %s)", rec.Code, rec.Body.String())
			}
			if len(store.stored) != 0 {
				t.Fatalf("expected nothing stored, got %d", len(store.stored))
			}
		})
	}
}

// TestStoreRejectsEmptyBody covers the degenerate case: a well-formed multipart
// carrying no parts previously returned 200 with an empty sequence.
func TestStoreRejectsEmptyBody(t *testing.T) {
	h, _ := newTestHandler(t)
	body, ct := multipartBody()
	req := httptest.NewRequest("POST", "/stow/rs/studies", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code < 400 {
		t.Fatalf("an empty upload returned %d: %s", rec.Code, rec.Body.String())
	}
}

// TestStoreRejectsInstanceFromDifferentStudy pins the study-scoped STOW check.
//
// The {studyUID} path segment was ignored entirely, so POSTing to
// /stow/rs/studies/{A} happily stored instances belonging to study B. Beyond the
// PS3.18 §10.5 conformance point, any deployment that authorises STOW per-study
// had that authorisation bypassed: the path said one study while the payload
// wrote another.
func TestStoreRejectsInstanceFromDifferentStudy(t *testing.T) {
	h, store := newTestHandler(t)
	body, ct := buildMultipartDICOMBody(t, loadSampleDICOM(t))

	// A syntactically valid UID that is not the instance's study.
	req := httptest.NewRequest("POST", "/stow/rs/studies/9.9.9.999", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code < 400 {
		t.Fatalf("an instance from a different study was accepted with %d: %s",
			rec.Code, rec.Body.String())
	}
	if len(store.stored) != 0 {
		t.Fatalf("expected nothing stored, got %d", len(store.stored))
	}
}
