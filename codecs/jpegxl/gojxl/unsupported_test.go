package gojxl

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUnsupportedInputsRejected verifies that malformed inputs return a clear
// error (never a panic or garbage), so the codec backend can degrade gracefully
// / fall back to libjxl. It also checks that a lossy VarDCT frame now decodes
// through the public Decode entry point (it routes to the VarDCT decoder).
func TestUnsupportedInputsRejected(t *testing.T) {
	t.Run("vardct_now_supported", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join("testdata", "lossy_vardct.jxl"))
		if err != nil {
			t.Skipf("fixture unavailable: %v", err)
		}
		img, err := Decode(data)
		if err != nil {
			t.Fatalf("VarDCT frame should now decode: %v", err)
		}
		if img.W == 0 || img.H == 0 || img.Channels == 0 {
			t.Fatalf("implausible decoded image %dx%d ch=%d", img.W, img.H, img.Channels)
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

// TestVarDCTUnsupportedDegradeGracefully verifies that VarDCT frames using
// features the pure-Go decoder does not yet implement (multi-pass / progressive,
// multi-DC-group) return a clear error rather than panicking or emitting
// garbage, so the codec backend can fall back to a native decoder.
func TestVarDCTUnsupportedDegradeGracefully(t *testing.T) {
	for _, f := range []string{"vardct_prog512.jxl", "vardct_big1280.jxl"} {
		data, err := os.ReadFile(filepath.Join("testdata", f))
		if err != nil {
			t.Skipf("fixture %s unavailable: %v", f, err)
		}
		if _, err := Decode(data); err == nil {
			t.Errorf("%s: expected an error for an unsupported VarDCT feature", f)
		}
	}
}
