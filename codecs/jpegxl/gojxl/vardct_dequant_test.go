package gojxl

import (
	"math"
	"testing"
)

func TestDctMult(t *testing.T) {
	cases := []struct{ in, want float32 }{
		{0, 1},
		{0.5, 1.5},
		{2, 3},
		{-1, 0.5},  // 1/(1-(-1)) = 1/2
		{-3, 0.25}, // 1/(1-(-3)) = 1/4
	}
	for _, c := range cases {
		if got := dctMult(c.in); absf(got-c.want) > 1e-6 {
			t.Errorf("dctMult(%g)=%g, want %g", c.in, got, c.want)
		}
	}
}

// TestGetQuantWeightsFlat: a single distance band yields a flat weight matrix
// equal to that band value for all frequencies and channels.
func TestGetQuantWeightsFlat(t *testing.T) {
	db := &[3][]float32{{2.0}, {3.0}, {4.0}}
	w, ok := getQuantWeightsDCT(8, 8, db, 1)
	if !ok {
		t.Fatal("getQuantWeightsDCT failed")
	}
	for c := 0; c < 3; c++ {
		want := db[c][0]
		for i := 0; i < 64; i++ {
			if absf(w[c*64+i]-want) > 1e-6 {
				t.Fatalf("channel %d freq %d = %g, want flat %g", c, i, w[c*64+i], want)
			}
		}
	}
}

// TestGetQuantWeightsTwoBand: with two bands, the DC (freq 0,0) weight equals
// band[0], and the highest-frequency corner equals band[1] (the interpolation
// reaches the last band exactly at the block corner, with zero fraction).
func TestGetQuantWeightsTwoBand(t *testing.T) {
	// band[1] raw param p => band value = band[0]*Mult(p).
	raw := &[3][]float32{{10, 0.5}, {10, 0.5}, {10, 0.5}} // Mult(0.5)=1.5 -> band1=15
	w, ok := getQuantWeightsDCT(8, 8, raw, 2)
	if !ok {
		t.Fatal("getQuantWeightsDCT failed")
	}
	for c := 0; c < 3; c++ {
		base := w[c*64+0]
		corner := w[c*64+8*8-1] // y=7,x=7
		if absf(base-10) > 1e-4 {
			t.Errorf("channel %d DC weight=%g, want 10", c, base)
		}
		if absf(corner-15) > 1e-3 {
			t.Errorf("channel %d corner weight=%g, want 15", c, corner)
		}
		// Mid frequency should be strictly between the two bands (monotone).
		mid := w[c*64+4*8+4]
		if mid <= base || mid >= corner {
			t.Errorf("channel %d mid weight=%g not between %g and %g", c, mid, base, corner)
		}
	}
}

// TestDefaultDCT8x8Weights runs the interpolation with libjxl's real default
// DCT (8x8) distance-band parameters (quant_weights.cc DequantLibrary::DCT) and
// checks the DC weight equals band[0] and the highest-frequency corner equals
// the hand-computed cumulative last band.
func TestDefaultDCT8x8Weights(t *testing.T) {
	// {X,Y,B} raw distance-band params, 6 bands.
	db := &[3][]float32{
		{3150.0, 0.0, -0.4, -0.4, -0.4, -2.0},
		{560.0, 0.0, -0.3, -0.3, -0.3, -0.3},
		{512.0, -2.0, -1.0, 0.0, -1.0, -2.0},
	}
	w, ok := getQuantWeightsDCT(8, 8, db, 6)
	if !ok {
		t.Fatal("getQuantWeightsDCT failed on default DCT params")
	}
	// DC weights = band[0] for each channel.
	wantDC := []float32{3150.0, 560.0, 512.0}
	for c := 0; c < 3; c++ {
		if absf(w[c*64]-wantDC[c]) > 1e-2 {
			t.Errorf("channel %d DC weight=%g, want %g", c, w[c*64], wantDC[c])
		}
	}
	// Corner (y=7,x=7, scaled_distance=5=num_bands-1) = cumulative band[5].
	// X: 3150 * 1 * (1/1.4)^3 * (1/3) ≈ 382.65.
	if got := w[0*64+63]; absf(got-382.65) > 0.5 {
		t.Errorf("X corner weight=%g, want ≈382.65", got)
	}
	// Y: 560 * 1 * (1/1.3)^3 * (1/1.3) ≈ 560 * (1/1.3)^4 ≈ 196.0.
	wantY := float32(560.0) * float32(math.Pow(1.0/1.3, 4))
	if got := w[1*64+63]; absf(got-wantY) > 0.5 {
		t.Errorf("Y corner weight=%g, want ≈%g", got, wantY)
	}
}

// TestInterpolateBand matches the scalar Interpolate definition directly.
func TestInterpolateBand(t *testing.T) {
	bands := []float32{2, 8, 8} // padded
	// At pos 0 -> 2; pos 1 -> 8; pos 0.5 -> 2*(8/2)^0.5 = 2*2 = 4.
	checks := []struct{ pos, want float32 }{{0, 2}, {1, 8}, {0.5, 4}}
	for _, c := range checks {
		got := interpolateBand(c.pos, bands)
		if absf(got-c.want) > 1e-4 {
			t.Errorf("interpolateBand(%g)=%g, want %g", c.pos, got, c.want)
		}
	}
	_ = math.Sqrt2
}
