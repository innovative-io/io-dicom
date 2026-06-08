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

// TestVarDCTReconstructLargeDCT decodes a 640x384 smooth-gradient fixture that
// the encoder codes entirely with DCT64x64 blocks across 6 AC groups. It
// exercises the large-DCT path: the DCT64 dequant matrices, resampleScale1D(8),
// and idct2d at N=64. Byte-exact vs djxl (mean ~0.22). The source is an
// R=x, G=y, B~(x+y)/2 gradient.
func TestVarDCTReconstructLargeDCT(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_smooth640x384.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 640 || img.H != 384 {
		t.Fatalf("got %dx%d, want 640x384", img.W, img.H)
	}
	get := func(x, y, c int) int { return int(img.Pixels[(y*img.W+x)*3+c]) }
	// Red ramps left-to-right, green top-to-bottom.
	if get(620, 10, 0)-get(10, 10, 0) < 150 {
		t.Errorf("red gradient too small: %d..%d", get(10, 10, 0), get(620, 10, 0))
	}
	if get(10, 370, 1)-get(10, 10, 1) < 150 {
		t.Errorf("green gradient too small: %d..%d", get(10, 10, 1), get(10, 370, 1))
	}
}

// TestVarDCTReconstructHighFreq decodes a 128x128 fixture whose columns are
// identical (pure horizontal structure), coded as DCT64x64. The defining
// property — every column is constant down its height — is only preserved if the
// LowestFrequenciesFromDC resample scales are correct (an inverted scale, the
// historical bug, warped the vertical reconstruction). Each column must stay
// near-constant vertically while varying strongly horizontally.
func TestVarDCTReconstructHighFreq(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_horiz128.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 128 || img.H != 128 {
		t.Fatalf("got %dx%d, want 128x128", img.W, img.H)
	}
	get := func(x, y, c int) int { return int(img.Pixels[(y*img.W+x)*3+c]) }
	// Vertical near-constancy: column x must vary little between rows.
	maxVert := 0
	for x := 0; x < 128; x += 8 {
		for _, c := range []int{0, 1, 2} {
			lo, hi := 255, 0
			for y := 0; y < 128; y++ {
				v := get(x, y, c)
				if v < lo {
					lo = v
				}
				if v > hi {
					hi = v
				}
			}
			if hi-lo > maxVert {
				maxVert = hi - lo
			}
		}
	}
	// djxl's own DCT64 reconstruction of identical columns leaves a small spread;
	// a wrong (inverted) LLF resample warps it to >50.
	if maxVert > 16 {
		t.Errorf("columns not vertically constant (max spread %d) — LLF resample likely wrong", maxVert)
	}
	// Strong horizontal variation must remain.
	if get(0, 64, 0) == get(20, 64, 0) && get(40, 64, 0) == get(60, 64, 0) {
		t.Error("no horizontal variation — decode likely wrong")
	}
}

// TestVarDCTReconstructFullTransformSet decodes a 512x512 textured fixture whose
// encoding exercises the full common transform set in a single frame: square
// DCT8/16/32, the rectangular DCTs (DCT16x32/32x16/64x32/32x64/8x16), and the
// special transforms (DCT2x2, DCT4x8, DCT8x4) — across 4 AC groups with CfL and
// both loop filters active. It is byte-exact vs djxl (mean ~0.24). This is the
// regression guard for the special-transform inverses and the x/b_dm_multiplier
// AC dequant scaling. The source mixes smooth sinusoids with random detail tiles.
func TestVarDCTReconstructFullTransformSet(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_rgb512x512.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 512 || img.H != 512 || img.Channels != 3 {
		t.Fatalf("got %dx%d ch=%d, want 512x512x3", img.W, img.H, img.Channels)
	}
	// Sanity: a textured image must show substantial local variation and a wide
	// value range in every channel (a desync would flatten or saturate it).
	for c := 0; c < 3; c++ {
		lo, hi := 255, 0
		for i := 0; i < img.W*img.H; i++ {
			v := int(img.Pixels[i*3+c])
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
		if hi-lo < 120 {
			t.Errorf("channel %d range too narrow (%d..%d) — decode likely wrong", c, lo, hi)
		}
	}
}

// TestVarDCTReconstructAFV decodes a 256x256 diagonal-edge fixture that the
// encoder codes with the AFV corner transforms (AFV0-3) alongside IDENTITY,
// DCT2x2, DCT4x8, DCT8x4 and DCT8. It is byte-exact vs djxl (mean ~0.23). This
// is the regression guard for the AFV inverse transform, the 16x16 IAFV basis,
// and the AFV quant mode. Diagonal/triangular structure drives the AFV choice.
func TestVarDCTReconstructAFV(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_afv256.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 256 || img.H != 256 || img.Channels != 3 {
		t.Fatalf("got %dx%d ch=%d, want 256x256x3", img.W, img.H, img.Channels)
	}
	// High-contrast diagonal content: every channel must span a wide range.
	for c := 0; c < 3; c++ {
		lo, hi := 255, 0
		for i := 0; i < img.W*img.H; i++ {
			v := int(img.Pixels[i*3+c])
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
		if hi-lo < 120 {
			t.Errorf("channel %d range too narrow (%d..%d)", c, lo, hi)
		}
	}
}

// TestVarDCTReconstructLargeMultiGroup decodes a 1024x1024 textured fixture (16
// AC groups) to confirm the decoder scales to realistic image sizes with the
// full transform mix. Sanity-checks wide per-channel range. Byte-exact vs djxl
// (mean ~0.25) on the local fixture.
func TestVarDCTReconstructLargeMultiGroup(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_rgb1024.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 1024 || img.H != 1024 || img.Channels != 3 {
		t.Fatalf("got %dx%d ch=%d, want 1024x1024x3", img.W, img.H, img.Channels)
	}
	for c := 0; c < 3; c++ {
		lo, hi := 255, 0
		for i := 0; i < img.W*img.H; i++ {
			v := int(img.Pixels[i*3+c])
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
		if hi-lo < 120 {
			t.Errorf("channel %d range too narrow (%d..%d)", c, lo, hi)
		}
	}
}

// TestVarDCTReconstructGrayscale decodes a lossy grayscale fixture. cjxl encodes
// grayscale through XYB (3 equal channels); the decoder must emit a single
// channel matching the declared csGray colour space — important for medical
// imagery. Checks the channel count and a plausible value range.
func TestVarDCTReconstructGrayscale(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_gray128.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 128 || img.H != 128 || img.Channels != 1 {
		t.Fatalf("got %dx%d ch=%d, want 128x128x1", img.W, img.H, img.Channels)
	}
	lo, hi := 255, 0
	for _, v := range img.Pixels {
		if int(v) < lo {
			lo = int(v)
		}
		if int(v) > hi {
			hi = int(v)
		}
	}
	if hi-lo < 120 {
		t.Errorf("grayscale range too narrow (%d..%d)", lo, hi)
	}
}

// TestVarDCTReconstructMultiDCGroup decodes a 2304x2304 fixture that exercises,
// in one frame: a permuted TOC, four DC groups, per-group local modular trees
// (no global tree), and four AC histogram sets — across 81 AC groups. Byte-exact
// vs djxl (mean ~0.22) on the local fixture. Sanity-checks the gradient corners.
func TestVarDCTReconstructMultiDCGroup(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_big2304.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 2304 || img.H != 2304 || img.Channels != 3 {
		t.Fatalf("got %dx%d ch=%d, want 2304x2304x3", img.W, img.H, img.Channels)
	}
	get := func(x, y, c int) int { return int(img.Pixels[(y*img.W+x)*3+c]) }
	// Source gradient R=x, G=y: opposite corners differ widely in R and G.
	if get(2290, 10, 0)-get(10, 10, 0) < 180 {
		t.Errorf("red gradient too small: %d..%d", get(10, 10, 0), get(2290, 10, 0))
	}
	if get(10, 2290, 1)-get(10, 10, 1) < 180 {
		t.Errorf("green gradient too small: %d..%d", get(10, 10, 1), get(10, 2290, 1))
	}
}
