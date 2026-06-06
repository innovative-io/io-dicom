package gojxl

import "testing"

func TestAdjustQuantBias(t *testing.T) {
	b := &kDefaultQuantBias
	cases := []struct {
		c    int
		q    int32
		want float32
	}{
		{0, 0, 0},
		{1, 0, 0},
		{0, 1, b[0]},
		{1, 1, b[1]},
		{2, 1, b[2]},
		{0, -1, -b[0]},
		{2, -1, -b[2]},
		{0, 2, 2 - b[3]/2},
		{1, 5, 5 - b[3]/5},
		{0, -3, -3 - b[3]/-3},
		{0, 100, 100 - b[3]/100},
	}
	for _, c := range cases {
		got := adjustQuantBias(c.c, c.q, b)
		if absf(got-c.want) > 1e-6 {
			t.Errorf("adjustQuantBias(c=%d,q=%d)=%.7f, want %.7f", c.c, c.q, got, c.want)
		}
	}
}

func TestDecodeQuantizerRoundTrip(t *testing.T) {
	cases := []struct{ gs, qdc int }{
		{1, 16},      // both default-ish (first dist entries)
		{2049, 1},    // global_scale second dist
		{4097, 1},    // global_scale third dist
		{8193, 5000}, // largest dists
	}
	for _, c := range cases {
		w := newBitWriter()
		w.WriteU32(uint32(c.gs), qGlobalScaleDist[0], qGlobalScaleDist[1], qGlobalScaleDist[2], qGlobalScaleDist[3])
		w.WriteU32(uint32(c.qdc), qQuantDCDist[0], qQuantDCDist[1], qQuantDCDist[2], qQuantDCDist[3])
		w.ZeroPadToByte()
		q := decodeQuantizer(newBitReader(w.Bytes()))
		if q.globalScale != c.gs || q.quantDC != c.qdc {
			t.Errorf("decodeQuantizer: got gs=%d qdc=%d, want %d %d", q.globalScale, q.quantDC, c.gs, c.qdc)
		}
		// Derived fields must be consistent.
		if absf(q.invGlobalScale-float32(kGlobalScaleDenom)/float32(c.gs)) > 1e-3 {
			t.Errorf("invGlobalScale mismatch for gs=%d", c.gs)
		}
	}
}

func TestQuantizerScales(t *testing.T) {
	// global_scale default-ish value; quant_dc arbitrary.
	gs, qdc := 8192, 5
	q := newQuantizer(gs, qdc)

	wantScale := float32(gs) / float32(kGlobalScaleDenom)
	if absf(q.scale()-wantScale) > 1e-9 {
		t.Errorf("scale()=%.9f, want %.9f", q.scale(), wantScale)
	}
	wantInv := float32(kGlobalScaleDenom) / float32(gs)
	if absf(q.invGlobalScale-wantInv) > 1e-6 {
		t.Errorf("invGlobalScale=%.6f, want %.6f", q.invGlobalScale, wantInv)
	}
	if absf(q.invQuantDC-wantInv/float32(qdc)) > 1e-6 {
		t.Errorf("invQuantDC=%.6f, want %.6f", q.invQuantDC, wantInv/float32(qdc))
	}
	for _, bq := range []int32{1, 2, 7, 50} {
		if absf(q.invQuantAC(bq)-wantInv/float32(bq)) > 1e-6 {
			t.Errorf("invQuantAC(%d)=%.6f, want %.6f", bq, q.invQuantAC(bq), wantInv/float32(bq))
		}
	}
}
