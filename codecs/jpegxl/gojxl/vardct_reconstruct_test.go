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

// TestVarDCTReconstructMultiblock decodes a 64x64 lossy fixture that uses
// variable block sizes (DCT8x8/16x16/32x32) end-to-end and checks the recovered
// structure: a smooth gradient over most of the image plus a bright detail patch
// in the bottom-right quadrant. Exercises the multiblock decode + LLF-from-DC
// reconstruction path.
func TestVarDCTReconstructMultiblock(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_rgb64x64.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 64 || img.H != 64 || img.Channels != 3 {
		t.Fatalf("got %dx%d ch=%d, want 64x64x3", img.W, img.H, img.Channels)
	}
	get := func(x, y, c int) int { return int(img.Pixels[(y*img.W+x)*3+c]) }
	// Smooth region: red ramps left-to-right (R=x*4) along a top row.
	if get(60, 2, 0)-get(2, 2, 0) < 150 {
		t.Errorf("red gradient too small: %d..%d", get(2, 2, 0), get(60, 2, 0))
	}
	// Detail patch (x>=32,y>=32 checker of white) makes the bottom-right quadrant
	// substantially brighter on average than the top-left.
	avg := func(x0, y0 int) int {
		s := 0
		for y := y0; y < y0+16; y++ {
			for x := x0; x < x0+16; x++ {
				s += get(x, y, 0) + get(x, y, 1) + get(x, y, 2)
			}
		}
		return s / (16 * 16 * 3)
	}
	if avg(40, 40) <= avg(8, 8) {
		t.Errorf("detail patch not brighter: br=%d tl=%d", avg(40, 40), avg(8, 8))
	}
}

// TestVarDCTReconstructMultiGroup decodes a 300x300 lossy fixture that spans
// FOUR AC groups (groupDim 256 -> 2x2 groups). It exercises the multi-group
// path: per-section TOC offsets, independent ANS streams per AC group, and
// group-local non-zero prediction. The source is an R=x, G=y, B=(x+y)/2 sRGB
// gradient; corners pin the four quadrants so a misplaced group region would
// fail. Decode is byte-exact vs djxl (mean ~0.25) on this fixture.
func TestVarDCTReconstructMultiGroup(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_rgb300x300.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 300 || img.H != 300 || img.Channels != 3 {
		t.Fatalf("got %dx%d ch=%d, want 300x300x3", img.W, img.H, img.Channels)
	}
	get := func(x, y, c int) int { return int(img.Pixels[(y*img.W+x)*3+c]) }
	near := func(name string, x, y, c, want int) {
		if d := get(x, y, c) - want; d < -6 || d > 6 {
			t.Errorf("%s ch%d at (%d,%d) = %d, want ~%d", name, c, x, y, get(x, y, c), want)
		}
	}
	// Four corners pin the four group quadrants (R=x, G=y, B=(x+y)/2).
	near("TL", 2, 2, 0, 1)
	near("TL", 2, 2, 1, 1)
	near("TR", 297, 2, 0, 252) // group (1,0): red high, green low
	near("TR", 297, 2, 1, 1)
	near("BL", 2, 297, 0, 1) // group (0,1): red low, green high
	near("BL", 2, 297, 1, 252)
	near("BR", 297, 297, 0, 253) // group (1,1): both high
	near("BR", 297, 297, 1, 252)
	near("center", 150, 150, 0, 127)
}
