package gojxl

import "testing"

// defaultDCTEncoding builds the kQuantModeDCT encoding with libjxl's default
// DCT (8x8) distance bands.
func defaultDCTEncoding() *quantEncoding {
	return &quantEncoding{
		mode: quantModeDCT,
		dctBands: [3][]float32{
			{3150.0, 0.0, -0.4, -0.4, -0.4, -2.0},
			{560.0, 0.0, -0.3, -0.3, -0.3, -0.3},
			{512.0, -2.0, -1.0, 0.0, -1.0, -2.0},
		},
		dctBandN: 6,
	}
}

// TestComputeInvQuantTableDCT8x8: the DCT8x8 inv table zeroes only the DC of
// each channel and keeps the interpolated weights elsewhere.
func TestComputeInvQuantTableDCT8x8(t *testing.T) {
	enc := defaultDCTEncoding()
	inv, ok := computeInvQuantTable(qtDCT, enc)
	if !ok {
		t.Fatal("computeInvQuantTable failed")
	}
	if len(inv) != 3*64 {
		t.Fatalf("len=%d, want 192", len(inv))
	}
	// Reference weights (pre-zeroing) from the interpolation.
	ref, _ := getQuantWeightsDCT(8, 8, &enc.dctBands, 6)
	for c := 0; c < 3; c++ {
		if inv[c*64] != 0 {
			t.Errorf("channel %d DC not zeroed: %g", c, inv[c*64])
		}
		// All AC positions equal the reference interpolation.
		for i := 1; i < 64; i++ {
			if absf(inv[c*64+i]-ref[c*64+i]) > 1e-3 {
				t.Fatalf("channel %d AC[%d]=%g, want %g", c, i, inv[c*64+i], ref[c*64+i])
			}
		}
	}
}

// TestComputeInvQuantTableIdentity: IDENTITY zeroes only DC (1x1 covered).
func TestComputeInvQuantTableIdentity(t *testing.T) {
	enc := &quantEncoding{mode: quantModeID, idWeights: defaultIdentityWeights}
	inv, ok := computeInvQuantTable(qtIDENTITY, enc)
	if !ok {
		t.Fatal("computeInvQuantTable failed")
	}
	for c := 0; c < 3; c++ {
		if inv[c*64] != 0 {
			t.Errorf("channel %d DC not zeroed", c)
		}
		// AC[1] should be idWeights[c][1].
		if absf(inv[c*64+1]-defaultIdentityWeights[c][1]) > 1e-3 {
			t.Errorf("channel %d AC[1]=%g, want %g", c, inv[c*64+1], defaultIdentityWeights[c][1])
		}
	}
}

// TestKindMapsConsistent sanity-checks the strategy->kind map and per-kind sizes
// against the AC strategy geometry: a kind's wrows*wcols must equal the coeff
// count of every strategy that maps to it (after orientation normalization).
func TestKindMapsConsistent(t *testing.T) {
	for s := 0; s < acNumValidStrategies; s++ {
		kind := kQuantTable[s]
		if kind < 0 || kind >= kNumQuantTables {
			t.Fatalf("strategy %d maps to invalid kind %d", s, kind)
		}
		num := 64 * requiredSizeX[kind] * requiredSizeY[kind]
		if num != acStrategyType(s).numCoeffs() {
			t.Errorf("strategy %d: kind %d table num=%d, strategy coeffs=%d", s, kind, num, acStrategyType(s).numCoeffs())
		}
	}
}
