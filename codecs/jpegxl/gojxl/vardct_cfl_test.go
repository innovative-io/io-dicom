package gojxl

import "testing"

func TestCfLRatios(t *testing.T) {
	c := defaultColorCorrelation()
	// Defaults: baseX=0, baseB=1, scale=1/84.
	if c.ytoXRatio(0) != 0 {
		t.Errorf("ytoXRatio(0)=%g, want 0", c.ytoXRatio(0))
	}
	if absf(c.ytoXRatio(84)-1) > 1e-6 {
		t.Errorf("ytoXRatio(84)=%g, want 1", c.ytoXRatio(84))
	}
	if absf(c.ytoXRatio(-84)+1) > 1e-6 {
		t.Errorf("ytoXRatio(-84)=%g, want -1", c.ytoXRatio(-84))
	}
	if absf(c.ytoBRatio(0)-1) > 1e-6 {
		t.Errorf("ytoBRatio(0)=%g, want 1 (baseB)", c.ytoBRatio(0))
	}
	if absf(c.ytoBRatio(84)-2) > 1e-6 {
		t.Errorf("ytoBRatio(84)=%g, want 2", c.ytoBRatio(84))
	}
}

func TestCfLApply(t *testing.T) {
	c := defaultColorCorrelation()
	// Default X factor 0 -> X unchanged; default B (factor 0, ratio 1) -> b += y.
	x, b := c.applyCfL(3.0, 10.0, 5.0, 0, 0)
	if absf(x-3.0) > 1e-6 {
		t.Errorf("X with factor 0 changed: %g, want 3", x)
	}
	if absf(b-15.0) > 1e-6 {
		t.Errorf("B = ratio(1)*10 + 5 = 15, got %g", b)
	}
	// X factor 84 (ratio 1): x' = y + x.
	x2, _ := c.applyCfL(3.0, 10.0, 5.0, 84, 0)
	if absf(x2-13.0) > 1e-6 {
		t.Errorf("X with factor 84 = 13, got %g", x2)
	}
}

func TestDecodeCfLDCDefault(t *testing.T) {
	// A single set bit -> all-default.
	w := newBitWriter()
	w.WriteBits(1, 1)
	w.ZeroPadToByte()
	b := newBitReader(w.Bytes())
	c, err := decodeCfLDC(b)
	if err != nil {
		t.Fatal(err)
	}
	def := defaultColorCorrelation()
	if c != def {
		t.Errorf("default decode = %+v, want %+v", c, def)
	}
}

func TestDecodeCfLDCRoundTrip(t *testing.T) {
	w := newBitWriter()
	w.WriteBits(0, 1) // not all-default
	// color_factor via kColorFactorDist: write Val(256) selector (index 1).
	w.WriteU32(256, kColorFactorDist[0], kColorFactorDist[1], kColorFactorDist[2], kColorFactorDist[3])
	w.WriteBits(0x3800, 16) // baseX = 0.5 (IEEE half)
	w.WriteBits(0x3D00, 16) // baseB = 1.25 (IEEE half)
	w.WriteBits(uint64(int32(7)+128), 8)  // ytox_dc = 7
	w.WriteBits(uint64(int32(-3)+128), 8) // ytob_dc = -3
	w.ZeroPadToByte()

	b := newBitReader(w.Bytes())
	c, err := decodeCfLDC(b)
	if err != nil {
		t.Fatal(err)
	}
	if c.colorFactor != 256 {
		t.Errorf("colorFactor=%d, want 256", c.colorFactor)
	}
	if absf(c.baseX-0.5) > 1e-3 {
		t.Errorf("baseX=%g, want 0.5", c.baseX)
	}
	if absf(c.baseB-1.25) > 1e-3 {
		t.Errorf("baseB=%g, want 1.25", c.baseB)
	}
	if c.ytoxDC != 7 || c.ytobDC != -3 {
		t.Errorf("ytox/ytob DC = %d,%d, want 7,-3", c.ytoxDC, c.ytobDC)
	}
	if absf(c.colorScale-1.0/256) > 1e-9 {
		t.Errorf("colorScale=%g, want 1/256", c.colorScale)
	}
}
