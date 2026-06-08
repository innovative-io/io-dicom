package gojxl

import "testing"

func TestGetQuantWeightsIdentity(t *testing.T) {
	w := getQuantWeightsIdentity(&defaultIdentityWeights)
	if len(w) != 3*64 {
		t.Fatalf("len=%d, want 192", len(w))
	}
	for c := 0; c < 3; c++ {
		base := c * 64
		w0, w1, w2 := defaultIdentityWeights[c][0], defaultIdentityWeights[c][1], defaultIdentityWeights[c][2]
		// Special positions.
		if w[base+1] != w1 || w[base+8] != w1 {
			t.Errorf("channel %d: AC[1],[8]=%g,%g, want %g", c, w[base+1], w[base+8], w1)
		}
		if w[base+9] != w2 {
			t.Errorf("channel %d: AC[9]=%g, want %g", c, w[base+9], w2)
		}
		// All other positions = w0.
		for i := 0; i < 64; i++ {
			if i == 1 || i == 8 || i == 9 {
				continue
			}
			if w[base+i] != w0 {
				t.Fatalf("channel %d: pos %d=%g, want bulk %g", c, i, w[base+i], w0)
			}
		}
	}
}

func TestGetQuantWeightsDCT2(t *testing.T) {
	w := getQuantWeightsDCT2(&defaultDCT2Weights)
	for c := 0; c < 3; c++ {
		base := c * 64
		d := defaultDCT2Weights[c]
		if w[base] != idDC {
			t.Errorf("channel %d: DC slot=%g, want sentinel", c, w[base])
		}
		// d0 at (0,1)/(1,0), d1 at (1,1).
		if w[base+1] != d[0] || w[base+8] != d[0] {
			t.Errorf("channel %d: d0 positions wrong", c)
		}
		if w[base+9] != d[1] {
			t.Errorf("channel %d: (1,1)=%g, want d1=%g", c, w[base+9], d[1])
		}
		// Spot-check the four block regions.
		if w[base+0*8+2] != d[2] { // (0,2) off-diag 2x2
			t.Errorf("channel %d: (0,2)=%g, want d2=%g", c, w[base+2], d[2])
		}
		if w[base+2*8+2] != d[3] { // (2,2) diag 2x2
			t.Errorf("channel %d: (2,2)=%g, want d3=%g", c, w[base+2*8+2], d[3])
		}
		if w[base+0*8+4] != d[4] { // (0,4) off-diag 4x4
			t.Errorf("channel %d: (0,4)=%g, want d4=%g", c, w[base+4], d[4])
		}
		if w[base+4*8+4] != d[5] { // (4,4) diag 4x4
			t.Errorf("channel %d: (4,4)=%g, want d5=%g", c, w[base+4*8+4], d[5])
		}
	}
	// Verify full coverage: every non-DC position is one of d0..d5.
	for c := 0; c < 3; c++ {
		base := c * 64
		d := defaultDCT2Weights[c]
		set := map[float32]bool{d[0]: true, d[1]: true, d[2]: true, d[3]: true, d[4]: true, d[5]: true}
		for i := 1; i < 64; i++ {
			if !set[w[base+i]] {
				t.Errorf("channel %d: pos %d=%g not a valid DCT2 weight", c, i, w[base+i])
			}
		}
	}
}
