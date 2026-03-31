//go:build st2110 && cgo

package smpte2110

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestST2110BackendSelection(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("st2110"); err != nil {
		t.Fatalf("expected st2110 backend to be registered: %v", err)
	}
	if BackendName() != "st2110" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found in PATH")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not found in PATH")
	}

	raw := []byte{1, 2, 3, 4}
	var out []byte
	var outSize int
	err := SMPTE2110encode(raw, 2, 2, 1, 8, &out, &outSize, "1.2.840.10008.1.2.7.1")
	if err != nil {
		t.Fatalf("unexpected SMPTE2110encode error: %v", err)
	}
	if outSize == 0 || len(out) == 0 {
		t.Fatal("expected non-empty encoded output")
	}

	decoded := make([]byte, len(raw))
	if err := SMPTE2110decode(out, uint32(outSize), decoded, "1.2.840.10008.1.2.7.1"); err != nil {
		t.Fatalf("unexpected SMPTE2110decode error: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatalf("decoded payload mismatch: got %v want %v", decoded, raw)
	}
}
