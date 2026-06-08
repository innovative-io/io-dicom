package gojxl

import (
	"math"
	"testing"
)

// TestDCTRoundTrip verifies idct2d∘dct2d is identity for the square VarDCT block
// sizes and a couple of rectangular ones.
func TestDCTRoundTrip(t *testing.T) {
	sizes := [][2]int{{8, 8}, {16, 16}, {32, 32}, {8, 16}, {16, 8}, {4, 4}, {2, 2}}
	for _, s := range sizes {
		w, h := s[0], s[1]
		pix := make([]float32, w*h)
		// Deterministic pseudo-random-ish pattern.
		for i := range pix {
			pix[i] = float32(math.Sin(float64(i)*0.7) + 0.3*float64((i*37)%11))
		}
		coeff := dct2d(pix, w, h)
		got := idct2d(coeff, w, h)
		var maxErr float64
		for i := range pix {
			if d := math.Abs(float64(got[i] - pix[i])); d > maxErr {
				maxErr = d
			}
		}
		if maxErr > 1e-3 {
			t.Errorf("%dx%d DCT round-trip max error %.2e", w, h, maxErr)
		}
	}
}

// TestDCTDCOnly checks that a block with only a DC coefficient inverse-transforms
// to a flat block, and that the DC coefficient of a flat block equals the value
// times sqrt(w*h) (orthonormal convention: DC = mean*sqrt(N)).
func TestDCTDCOnly(t *testing.T) {
	w, h := 8, 8
	coeff := make([]float32, w*h)
	coeff[0] = 5.0
	out := idct2d(coeff, w, h)
	want := float32(5.0 / math.Sqrt(float64(w*h)))
	for i, v := range out {
		if absf(v-want) > 1e-4 {
			t.Fatalf("DC-only idct[%d]=%.6f, want flat %.6f", i, v, want)
			break
		}
	}
	// Forward of a flat block: only DC nonzero, equal to value*sqrt(N).
	flat := make([]float32, w*h)
	for i := range flat {
		flat[i] = 2.0
	}
	c := dct2d(flat, w, h)
	wantDC := float32(2.0 * math.Sqrt(float64(w*h)))
	if absf(c[0]-wantDC) > 1e-3 {
		t.Errorf("flat-block DC = %.5f, want %.5f", c[0], wantDC)
	}
	for i := 1; i < len(c); i++ {
		if absf(c[i]) > 1e-3 {
			t.Errorf("flat-block AC[%d] = %.5f, want 0", i, c[i])
		}
	}
}
