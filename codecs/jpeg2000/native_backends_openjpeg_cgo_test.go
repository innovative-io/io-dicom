//go:build openjpeg && cgo

package jpeg2000

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestOpenJPEGBackendSelection(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("openjpeg"); err != nil {
		t.Fatalf("expected openjpeg backend to be registered: %v", err)
	}
	if BackendName() != "openjpeg" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	if _, err := exec.LookPath("opj_compress"); err != nil {
		t.Skip("opj_compress not found in PATH")
	}
	if _, err := exec.LookPath("opj_decompress"); err != nil {
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
