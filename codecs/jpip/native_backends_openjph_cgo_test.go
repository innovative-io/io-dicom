//go:build openjph && cgo

package jpip

import (
	"bytes"
	"testing"

	"github.com/innovative-io/io-dicom/codecs/internal/nativeenv"
)

func TestOpenJPEGResolutionCount(t *testing.T) {
	tests := []struct {
		width  uint16
		height uint16
		want   int
	}{
		{width: 1, height: 1, want: 1},
		{width: 2, height: 2, want: 2},
		{width: 3, height: 3, want: 2},
		{width: 4, height: 4, want: 3},
		{width: 64, height: 64, want: 6},
	}

	for _, tt := range tests {
		if got := openjpegResolutionCount(tt.width, tt.height); got != tt.want {
			t.Fatalf("openjpegResolutionCount(%d, %d) = %d, want %d", tt.width, tt.height, got, tt.want)
		}
	}
}

func TestOpenJPHBackendSelection(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("openjph"); err != nil {
		t.Fatalf("expected openjph backend to be registered: %v", err)
	}
	if BackendName() != "openjph" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	haveOpenJPH := false
	if _, err := nativeenv.LookPath("ojph_compress"); err == nil {
		if _, derr := nativeenv.LookPath("ojph_decompress"); derr == nil {
			haveOpenJPH = true
		}
	}
	haveOpenJPEG := false
	if _, err := nativeenv.LookPath("opj_compress"); err == nil {
		if _, derr := nativeenv.LookPath("opj_decompress"); derr == nil {
			haveOpenJPEG = true
		}
	}
	if !haveOpenJPH && !haveOpenJPEG {
		t.Skip("neither OpenJPH nor OpenJPEG tools found in PATH")
	}

	raw := []byte{1, 2, 3, 4}
	var out []byte
	var outSize int
	err := JPIPencode(raw, 2, 2, 1, 8, &out, &outSize, "1.2.840.10008.1.2.4.204")
	if err != nil {
		t.Fatalf("unexpected JPIPencode error: %v", err)
	}
	if outSize == 0 || len(out) == 0 {
		t.Fatal("expected non-empty encoded output")
	}

	decoded := make([]byte, len(raw))
	if err := JPIPdecode(out, uint32(outSize), decoded, "1.2.840.10008.1.2.4.204"); err != nil {
		t.Fatalf("unexpected JPIPdecode error: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatalf("decoded payload mismatch: got %v want %v", decoded, raw)
	}
}
