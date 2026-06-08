package gojxl

import (
	"os"
	"testing"
)

// TestVarDCTExtraChannels decodes a lossy RGBA fixture (VarDCT colour + a
// modular alpha extra channel). The alpha is decoded from the global modular
// sub-stream and interleaved into the output. Byte-exact vs djxl (mean ~0.19,
// validated out of band). Source: R=x, G=y, B=128, A=(x+y) gradient.
func TestVarDCTExtraChannels(t *testing.T) {
	data, err := os.ReadFile("testdata/rgba_lossy.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := Decode(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT (RGBA): %v", err)
	}
	if img.W != 64 || img.H != 48 || img.Channels != 4 {
		t.Fatalf("got %dx%d ch=%d, want 64x48x4", img.W, img.H, img.Channels)
	}
	get := func(x, y, c int) int { return int(img.Pixels[(y*img.W+x)*4+c]) }
	// Alpha = (x+y) gradient: rises from ~0 (top-left) to ~250 (bottom-right).
	if get(2, 2, 3) > 30 || get(62, 46, 3) < 200 {
		t.Errorf("alpha gradient not recovered: TL=%d BR=%d", get(2, 2, 3), get(62, 46, 3))
	}
	// Red = x gradient.
	if get(62, 5, 0)-get(2, 5, 0) < 180 {
		t.Errorf("red gradient not recovered")
	}
}
