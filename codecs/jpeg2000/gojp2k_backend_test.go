package jpeg2000

import (
	"os"
	"testing"
)

// TestGoJ2KBackendLiveDecode exercises the pure-Go backend end-to-end through the
// package decode entry, independent of whether openjpeg is compiled in.
func TestGoJ2KBackendLiveDecode(t *testing.T) {
	if err := UseBackend("gojpeg2000"); err != nil {
		t.Fatalf("UseBackend(gojpeg2000): %v", err)
	}
	t.Cleanup(func() { SetBackend(nil) })

	dcm, err := os.ReadFile("../../testdata/cornerstone-CTImage-jpeg2000-lossless.dcm")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	frame := extractFirstJ2KFrame(t, dcm)
	out := make([]byte, 512*512*2)
	if err := J2Kdecode(frame, uint32(len(frame)), out); err != nil {
		t.Fatalf("live decode via backend: %v", err)
	}
	nz := 0
	for _, b := range out {
		if b != 0 {
			nz++
		}
	}
	if nz == 0 {
		t.Fatal("decoded output all zero")
	}
}
