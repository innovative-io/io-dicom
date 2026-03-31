//go:build libjxl && cgo

package jpegxl

import (
	"bytes"
	"os/exec"
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

	if _, err := exec.LookPath("cjxl"); err != nil {
		t.Skip("cjxl not found in PATH")
	}
	if _, err := exec.LookPath("djxl"); err != nil {
		t.Skip("djxl not found in PATH")
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
