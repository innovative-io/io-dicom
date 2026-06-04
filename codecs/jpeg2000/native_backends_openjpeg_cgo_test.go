//go:build openjpeg && cgo

package jpeg2000

import (
	"bytes"
	"testing"
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

func TestOpenJPEGEncodeDecode16BitRoundTrip(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("openjpeg"); err != nil {
		t.Fatalf("expected openjpeg backend to be registered: %v", err)
	}

	// Little-endian 16-bit samples (the DICOM uncompressed convention):
	// 16, 256, 2047, 4095 — all within the 12-bit precision encoded below.
	raw := []byte{
		0x10, 0x00,
		0x00, 0x01,
		0xFF, 0x07,
		0xFF, 0x0F,
	}
	var encoded []byte
	var outSize int
	if err := J2Kencode(raw, 2, 2, 1, 12, &encoded, &outSize, 0); err != nil {
		t.Fatalf("unexpected 16-bit J2Kencode error: %v", err)
	}

	decoded := make([]byte, len(raw))
	if err := J2Kdecode(encoded, uint32(outSize), decoded); err != nil {
		t.Fatalf("unexpected 16-bit J2Kdecode error: %v", err)
	}
	if !bytes.Equal(raw, decoded) {
		t.Fatalf("decoded 16-bit payload mismatch: got %v want %v", decoded, raw)
	}
}
