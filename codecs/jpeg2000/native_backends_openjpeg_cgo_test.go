//go:build openjpeg && cgo

package jpeg2000

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
		{width: 4096, height: 4096, want: 6},
	}

	for _, tt := range tests {
		if got := openjpegResolutionCount(tt.width, tt.height); got != tt.want {
			t.Fatalf("openjpegResolutionCount(%d, %d) = %d, want %d", tt.width, tt.height, got, tt.want)
		}
	}
}

func TestOpenJPEGBackendSelection(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("openjpeg"); err != nil {
		t.Fatalf("expected openjpeg backend to be registered: %v", err)
	}
	if BackendName() != "openjpeg" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	if _, err := nativeenv.LookPath("opj_compress"); err != nil {
		t.Skip("opj_compress not found in PATH")
	}
	if _, err := nativeenv.LookPath("opj_decompress"); err != nil {
		t.Skip("opj_decompress not found in PATH")
	}

	raw := []byte{1, 2, 3, 4}
	var encoded []byte
	var outSize int
	if err := J2Kencode(raw, 2, 2, 1, 8, &encoded, &outSize, 0); err != nil {
		t.Fatalf("unexpected J2Kencode error: %v", err)
	}
	if outSize == 0 || len(encoded) == 0 {
		t.Fatal("expected non-empty encoded output")
	}

	decoded := make([]byte, len(raw))
	if err := J2Kdecode(encoded, uint32(outSize), decoded); err != nil {
		t.Fatalf("unexpected J2Kdecode error: %v", err)
	}
	if !bytes.Equal(raw, decoded) {
		t.Fatalf("decoded payload mismatch: got %v want %v", decoded, raw)
	}
}
