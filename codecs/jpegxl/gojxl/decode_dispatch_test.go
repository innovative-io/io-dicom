package gojxl

import (
	"os"
	"testing"
)

// TestDecodeDispatchVarDCT verifies that the public Decode entry point routes
// lossy VarDCT frames to the VarDCT decoder (rather than the Modular path),
// across single-group, multi-group, and full-transform-set fixtures.
func TestDecodeDispatchVarDCT(t *testing.T) {
	for _, f := range []string{
		"vardct_rgb16x16.jxl",
		"vardct_rgb300x300.jxl",
		"vardct_rgb512x512.jxl",
	} {
		data, err := os.ReadFile("testdata/" + f)
		if err != nil {
			t.Skipf("fixture %s unavailable: %v", f, err)
		}
		img, err := Decode(data) // public entry, not DecodeVarDCT directly
		if err != nil {
			t.Errorf("%s: Decode failed/misrouted: %v", f, err)
			continue
		}
		if img.Channels != 3 || img.BitDepth != 8 || img.W == 0 || img.H == 0 {
			t.Errorf("%s: implausible image ch=%d bd=%d %dx%d", f, img.Channels, img.BitDepth, img.W, img.H)
		}
	}
}
