//go:build openjph && cgo

package jpip

import (
	"bytes"
	"testing"
)

func TestOpenJPHDecompositionCount(t *testing.T) {
	tests := []struct {
		width  uint16
		height uint16
		want   int
	}{
		{width: 1, height: 1, want: 0},
		{width: 2, height: 2, want: 1},
		{width: 3, height: 3, want: 1},
		{width: 4, height: 4, want: 2},
		{width: 64, height: 64, want: 5},
	}

	for _, tt := range tests {
		if got := openjphDecompositionCount(tt.width, tt.height); got != tt.want {
			t.Fatalf("openjphDecompositionCount(%d, %d) = %d, want %d", tt.width, tt.height, got, tt.want)
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

	if err := ValidateBackend("openjph"); err != nil {
		t.Fatalf("expected openjph backend to be ready: %v", err)
	}
}

func TestOpenJPHBackendRoundTrip8Bit(t *testing.T) {
	assertOpenJPHRoundTrip(t, []byte{1, 2, 3, 4}, 2, 2, 1, 8)
}

func TestOpenJPHBackendRoundTrip16BitRGB(t *testing.T) {
	raw := []byte{
		0x00, 0x10, 0x00, 0x20, 0x00, 0x30,
		0x01, 0x10, 0x01, 0x20, 0x01, 0x30,
		0x02, 0x10, 0x02, 0x20, 0x02, 0x30,
		0x03, 0x10, 0x03, 0x20, 0x03, 0x30,
	}
	assertOpenJPHRoundTrip(t, raw, 2, 2, 3, 16)
}

func assertOpenJPHRoundTrip(t *testing.T, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16) {
	t.Helper()
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("openjph"); err != nil {
		t.Fatalf("expected openjph backend to be registered: %v", err)
	}

	var out []byte
	var outSize int
	if err := JPIPencode(raw, width, height, samples, bitsa, &out, &outSize, "1.2.840.10008.1.2.4.204"); err != nil {
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
