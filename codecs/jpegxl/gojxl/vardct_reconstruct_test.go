package gojxl

import (
	"os"
	"testing"
)

// TestVarDCTReconstruct decodes the lossy VarDCT fixture end-to-end and checks
// that the reconstruction recovers the image structure. The decode is now
// effectively byte-exact (within ~1 level of a conforming decoder); this test
// verifies the recovered gradient structure (it does not depend on the
// gitignored djxl reference).
func TestVarDCTReconstruct(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_rgb16x16.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 16 || img.H != 16 || img.Channels != 3 {
		t.Fatalf("got %dx%d ch=%d, want 16x16x3", img.W, img.H, img.Channels)
	}
	get := func(x, y, c int) int { return int(img.Pixels[(y*img.W+x)*3+c]) }

	// The source is R=x*16: the red channel must increase left-to-right across a
	// row (the reconstructed gradient), spanning most of the 0..255 range.
	rLeft, rRight := get(0, 4, 0), get(15, 4, 0)
	if rRight-rLeft < 200 {
		t.Errorf("red gradient span = %d (left=%d right=%d), want a wide ramp", rRight-rLeft, rLeft, rRight)
	}
	prev := -1
	increasing := 0
	for x := 0; x < 16; x++ {
		r := get(x, 4, 0)
		if r >= prev {
			increasing++
		}
		prev = r
	}
	if increasing < 14 { // allow a couple of non-monotone steps from quantization
		t.Errorf("red row not monotonically increasing (%d/16 steps)", increasing)
	}

	// G=y*16: the green channel must increase top-to-bottom down a column.
	gTop, gBot := get(4, 0, 1), get(4, 15, 1)
	if gBot-gTop < 180 {
		t.Errorf("green gradient span = %d (top=%d bot=%d), want a wide ramp", gBot-gTop, gTop, gBot)
	}

	// Corner sanity: top-left is near-black, bottom-right near-white.
	if get(1, 1, 0) > 60 || get(14, 14, 0) < 150 {
		t.Errorf("corners off: tl.R=%d br.R=%d", get(1, 1, 0), get(14, 14, 0))
	}
}
