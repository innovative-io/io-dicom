//go:build libjpeg && cgo

package jpeg

import (
	"bytes"
	"testing"
)

func TestLibJPEGBackendSelection(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("libjpeg"); err != nil {
		t.Fatalf("expected libjpeg backend to be registered: %v", err)
	}
	if BackendName() != "libjpeg" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	raw12 := []byte{0x10, 0x00, 0x20, 0x00, 0x30, 0x00, 0x40, 0x00}
	var out []byte
	var outSize int
	err := EIJG12encode(raw12, 2, 2, 1, &out, &outSize, 0)
	if err != nil {
		t.Fatalf("unexpected EIJG12encode error: %v", err)
	}
	if outSize == 0 || len(out) == 0 {
		t.Fatal("expected non-empty EIJG12encode output")
	}

	decoded := make([]byte, len(raw12))
	if err := DIJG12decode(out, uint32(outSize), decoded, uint32(len(decoded))); err != nil {
		t.Fatalf("unexpected DIJG12decode error: %v", err)
	}
	if !bytes.Equal(decoded, raw12) {
		t.Fatalf("unexpected DIJG12decode payload: got=%v want=%v", decoded, raw12)
	}

	raw16 := []byte{0x11, 0x00, 0x22, 0x00, 0x33, 0x00, 0x44, 0x00}
	out = out[:0]
	outSize = 0
	if err := EIJG16encode(raw16, 2, 2, 1, &out, &outSize, 0); err != nil {
		t.Fatalf("unexpected EIJG16encode error: %v", err)
	}
	if outSize == 0 || len(out) == 0 {
		t.Fatal("expected non-empty EIJG16encode output")
	}
	decoded = make([]byte, len(raw16))
	if err := DIJG16decode(out, uint32(outSize), decoded, uint32(len(decoded))); err != nil {
		t.Fatalf("unexpected DIJG16decode error: %v", err)
	}
	if !bytes.Equal(decoded, raw16) {
		t.Fatalf("unexpected DIJG16decode payload: got=%v want=%v", decoded, raw16)
	}
}
