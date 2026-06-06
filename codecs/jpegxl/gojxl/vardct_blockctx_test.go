package gojxl

import "testing"

func TestDefaultBlockCtxMap(t *testing.T) {
	m := defaultBlockCtxMap()
	if m.numDCCtxs != 1 {
		t.Errorf("numDCCtxs=%d, want 1", m.numDCCtxs)
	}
	if m.numCtxs != 15 { // max(default map)=14 -> +1
		t.Errorf("numCtxs=%d, want 15", m.numCtxs)
	}
	if got := m.numACContexts(); got != 15*(37+458) {
		t.Errorf("numACContexts=%d, want %d", got, 15*(37+458))
	}
}

func TestNonZeroContext(t *testing.T) {
	m := defaultBlockCtxMap()
	// non_zeros<8 -> ctx=non_zeros; else 4+nz/2. Context index = ctx*numCtxs+blockCtx.
	cases := []struct {
		nz, blockCtx, wantCtx int
	}{
		{0, 0, 0},
		{7, 1, 7},
		{8, 0, 4 + 4},   // 8 -> 4+8/2=8
		{20, 2, 4 + 10}, // 20 -> 4+10=14
		{100, 0, 4 + 32},// clamped to 64 -> 4+32=36
	}
	for _, c := range cases {
		want := c.wantCtx*m.numCtxs + c.blockCtx
		if got := m.nonZeroContext(c.nz, c.blockCtx); got != want {
			t.Errorf("nonZeroContext(%d,%d)=%d, want %d", c.nz, c.blockCtx, got, want)
		}
	}
}

func TestBlockContextDefault(t *testing.T) {
	m := defaultBlockCtxMap()
	// With no qf thresholds and 1 dc ctx, blockContext(0, qf, ord, c) =
	// ctxMap[(c<2?c^1:2)*13 + ord].
	for c := 0; c < 3; c++ {
		for ord := 0; ord < kNumOrders; ord++ {
			ci := c ^ 1
			if c >= 2 {
				ci = 2
			}
			want := int(kDefaultBlockCtxMap[ci*kNumOrders+ord])
			if got := m.blockContext(0, 0, ord, c); got != want {
				t.Errorf("blockContext(c=%d,ord=%d)=%d, want %d", c, ord, got, want)
			}
		}
	}
}

func TestDecodeBlockCtxMapDefault(t *testing.T) {
	w := newBitWriter()
	w.WriteBits(1, 1) // is_default
	w.ZeroPadToByte()
	m, err := decodeBlockCtxMap(newBitReader(w.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if m.numCtxs != 15 || m.numDCCtxs != 1 {
		t.Errorf("default decode: numCtxs=%d numDCCtxs=%d", m.numCtxs, m.numDCCtxs)
	}
}

func TestDecodeBlockCtxMapMinimalNonDefault(t *testing.T) {
	w := newBitWriter()
	w.WriteBits(0, 1) // not default
	w.WriteBits(0, 4) // dc thresholds[0] count = 0
	w.WriteBits(0, 4) // dc thresholds[1] count = 0
	w.WriteBits(0, 4) // dc thresholds[2] count = 0
	w.WriteBits(0, 4) // qf thresholds count = 0
	// context map (39 entries): simple, bits_per_entry=0 -> all zeros.
	w.WriteBits(1, 1) // is_simple
	w.WriteBits(0, 2) // bits_per_entry = 0
	w.ZeroPadToByte()

	m, err := decodeBlockCtxMap(newBitReader(w.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if m.numDCCtxs != 1 {
		t.Errorf("numDCCtxs=%d, want 1", m.numDCCtxs)
	}
	if len(m.ctxMap) != 3*kNumOrders { // 39
		t.Errorf("ctxMap len=%d, want 39", len(m.ctxMap))
	}
	if m.numCtxs != 1 {
		t.Errorf("numCtxs=%d, want 1 (all-zero map)", m.numCtxs)
	}
}
