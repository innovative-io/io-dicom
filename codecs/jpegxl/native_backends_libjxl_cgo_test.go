//go:build libjxl && cgo

package jpegxl

import (
	"bytes"
	"testing"
)

func TestLibJXLBackendSelection(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("libjxl"); err != nil {
		t.Fatalf("expected libjxl backend to be registered: %v", err)
	}
	if BackendName() != "libjxl" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	raw := []byte{1, 2, 3, 4}
	var out []byte
	var outSize int
	err := JXLencode(raw, 2, 2, 1, 8, &out, &outSize, true)
	if err != nil {
		t.Fatalf("unexpected JXLencode error: %v", err)
	}
	if outSize == 0 || len(out) == 0 {
		t.Fatal("expected non-empty encoded output")
	}

	decoded := make([]byte, len(raw))
	if err := JXLdecode(out, uint32(outSize), decoded); err != nil {
		t.Fatalf("unexpected JXLdecode error: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatalf("decoded payload mismatch: got %v want %v", decoded, raw)
	}
}

func TestLibJXLBackendSelection16Bit(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("libjxl"); err != nil {
		t.Fatalf("expected libjxl backend to be registered: %v", err)
	}

	// Little-endian 16-bit samples (the DICOM uncompressed convention):
	// 16, 256, 2047, 4095 — all within the 12-bit precision encoded below.
	raw := []byte{0x10, 0x00, 0x00, 0x01, 0xFF, 0x07, 0xFF, 0x0F}
	var out []byte
	var outSize int
	if err := JXLencode(raw, 2, 2, 1, 12, &out, &outSize, true); err != nil {
		t.Fatalf("unexpected 16-bit JXLencode error: %v", err)
	}

	decoded := make([]byte, len(raw))
	if err := JXLdecode(out, uint32(outSize), decoded); err != nil {
		t.Fatalf("unexpected 16-bit JXLdecode error: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatalf("decoded 16-bit payload mismatch: got %v want %v", decoded, raw)
	}
}
