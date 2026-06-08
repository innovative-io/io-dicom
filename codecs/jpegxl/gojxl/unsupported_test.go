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

// TestVarDCTCodedBlockCtxMap decodes a 1280x1280 -e8 fixture whose frame header
// is all-default (so group_size_shift takes its default of 1) and whose AC
// block-context map is coded (non-default, 6 contexts). It is the regression
// guard for the group_size_shift default in the all-default frame-header path —
// getting it wrong miscounts the TOC entries and desyncs the whole frame.
func TestVarDCTCodedBlockCtxMap(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "vardct_big1280.jxl"))
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if img.W != 1280 || img.H != 1280 || img.Channels != 3 {
		t.Fatalf("got %dx%d ch=%d, want 1280x1280x3", img.W, img.H, img.Channels)
	}
	// Source is an R=x, G=y gradient: corners must differ widely.
	get := func(x, y, c int) int { return int(img.Pixels[(y*img.W+x)*3+c]) }
	if get(1270, 10, 0)-get(10, 10, 0) < 180 {
		t.Errorf("red gradient too small: %d..%d", get(10, 10, 0), get(1270, 10, 0))
	}
}
