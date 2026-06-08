package jpegxl

import (
	"os"
	"testing"
)

// TestVarDCTThroughBackend decodes a lossy VarDCT codestream end-to-end through
// the public JXLdecode entry point — backend selection, gojpegxlBackend.Decode,
// gojxl.Decode's VarDCT dispatch, and the copy into the caller's output buffer —
// confirming the pure-Go lossy decoder is reachable on the production path that
// the DICOM layer uses (CGO_ENABLED=0). The fixture is the 16x16 RGB gradient
// shared with the gojxl package tests.
func TestVarDCTThroughBackend(t *testing.T) {
	// Select the pure-Go backend explicitly (other tests may leave a custom
	// backend registered/selected), and restore the default afterwards.
	if err := UseBackend("gojpegxl"); err != nil {
		t.Skipf("gojpegxl backend unavailable: %v", err)
	}
	t.Cleanup(func() { SetBackend(nil) })

	data, err := os.ReadFile("gojxl/testdata/vardct_rgb16x16.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	out := make([]byte, 16*16*3)
	if err := JXLdecode(data, uint32(len(data)), out); err != nil {
		t.Fatalf("JXLdecode (lossy VarDCT through backend): %v", err)
	}
	// Source is R=x*16: along row 4 the red channel ramps across the full range.
	if int(out[(4*16+15)*3])-int(out[(4*16+0)*3]) < 180 {
		t.Errorf("red gradient not recovered through the backend path")
	}
}
