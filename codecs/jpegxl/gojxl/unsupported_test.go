package gojxl

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUnsupportedInputsRejected verifies that inputs outside the supported
// lossless-Modular subset return a clear error (never a panic or garbage), so
// the codec backend can degrade gracefully / fall back to libjxl.
func TestUnsupportedInputsRejected(t *testing.T) {
	t.Run("vardct", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join("testdata", "lossy_vardct.jxl"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Decode(data); err == nil {
			t.Fatal("expected error decoding a VarDCT (lossy) frame")
		}
	})
	t.Run("garbage", func(t *testing.T) {
		// Random/truncated bytes must error, not panic.
		for _, b := range [][]byte{{0xFF, 0x0A, 0x00}, {0x00}, {0xFF, 0x0A}, []byte("not a jxl")} {
			if _, err := Decode(b); err == nil {
				t.Errorf("expected error for %v", b)
			}
		}
	})
}
