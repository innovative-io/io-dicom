package gojxl

import "testing"

// encodePermutationStream emits the ANS-coded token stream for a permutation,
// mirroring what the encoder writes (enc_coeff_order.cc EncodeCoeffOrder).
func encodePermutationStream(perm []uint32, skip, size int) []byte {
	lehmer := computeLehmerCode(perm, size)
	end := size
	for end > skip && lehmer[end-1] == 0 {
		end--
	}
	tokens := []encToken{{ctx: int(coeffOrderContext(uint32(size))), value: uint32(end - skip)}}
	last := uint32(0)
	for i := skip; i < end; i++ {
		tokens = append(tokens, encToken{ctx: int(coeffOrderContext(last)), value: lehmer[i]})
		last = lehmer[i]
	}
	w := newBitWriter()
	encodeANSStream(w, tokens, kPermutationContexts)
	w.ZeroPadToByte()
	return w.Bytes()
}

func decodePermStream(data []byte, skip, size int, t *testing.T) []uint32 {
	t.Helper()
	b := newBitReader(data)
	code, ctxMap, err := decodeHistograms(b, kPermutationContexts, false)
	if err != nil {
		t.Fatalf("decodeHistograms: %v", err)
	}
	reader := newANSSymbolReader(code, b, 0)
	got, err := readPermutation(skip, size, reader, b, ctxMap)
	if err != nil {
		t.Fatalf("readPermutation: %v", err)
	}
	if !reader.checkFinalState() {
		t.Fatal("ANS final state check failed")
	}
	return got
}

func TestReadPermutationRoundTrip(t *testing.T) {
	cases := []struct {
		perm []uint32
		skip int
	}{
		{[]uint32{0, 2, 1, 3, 5, 4, 7, 6, 8, 10, 9, 11, 13, 12, 15, 14}, 0},
		{[]uint32{0, 1, 2, 3, 4, 5, 6, 7}, 0}, // identity -> end trims to 0
		{[]uint32{0, 3, 1, 2, 6, 4, 5, 7}, 1}, // skip=1 (perm[0]=0 fixed)
	}
	for ci, c := range cases {
		size := len(c.perm)
		data := encodePermutationStream(c.perm, c.skip, size)
		got := decodePermStream(data, c.skip, size, t)
		for i := range c.perm {
			if got[i] != c.perm[i] {
				t.Fatalf("case %d pos %d: got %d want %d (full %v)", ci, i, got[i], c.perm[i], got)
			}
		}
	}
}

// TestDecodeCoeffOrderComposesNatural verifies decodeCoeffOrder composes the
// decoded permutation with the natural order: an identity permutation yields
// exactly the natural coefficient order for that strategy.
func TestDecodeCoeffOrderComposesNatural(t *testing.T) {
	strat := acDCT // 8x8, llf=1, size=64
	size := strat.numCoeffs()
	identity := make([]uint32, size)
	for i := range identity {
		identity[i] = uint32(i)
	}
	data := encodePermutationStream(identity, 1, size) // skip=llf=1
	b := newBitReader(data)
	code, ctxMap, err := decodeHistograms(b, kPermutationContexts, false)
	if err != nil {
		t.Fatal(err)
	}
	reader := newANSSymbolReader(code, b, 0)
	order, err := decodeCoeffOrder(strat, reader, b, ctxMap)
	if err != nil {
		t.Fatal(err)
	}
	natural := naturalCoeffOrder(1, 1)
	for i := range order {
		if order[i] != natural[i] {
			t.Fatalf("pos %d: order=%d natural=%d", i, order[i], natural[i])
		}
	}
}
