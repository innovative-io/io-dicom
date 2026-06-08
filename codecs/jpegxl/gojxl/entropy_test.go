package gojxl

import "testing"

// TestAliasTableFlat verifies InitAliasTable + aliasLookup: every value in
// [0, ANS_TAB_SIZE) must map to a symbol the correct number of times (equal to
// its frequency), and the per-symbol offsets must form the permutation
// [0, freq).
func TestAliasTableFlat(t *testing.T) {
	for _, k := range []int{1, 2, 3, 5, 17, 100, 256} {
		dist := createFlatHistogram(k, ansTabSize)
		logAlpha := 8
		table := make([]aliasEntry, 1<<logAlpha)
		initAliasTable(dist, logAlpha, table)
		logEntrySize := ansLogTabSize - logAlpha
		entrySizeMinus1 := (1 << uint(logEntrySize)) - 1

		seen := make([][]bool, k)
		for s := range seen {
			seen[s] = make([]bool, dist[s])
		}
		counts := make([]int, k)
		for v := 0; v < ansTabSize; v++ {
			sym := aliasLookup(table, v, logEntrySize, entrySizeMinus1)
			if sym.value < 0 || sym.value >= k {
				t.Fatalf("k=%d v=%d: symbol %d out of range", k, v, sym.value)
			}
			if sym.offset < 0 || sym.offset >= int(dist[sym.value]) {
				t.Fatalf("k=%d v=%d sym=%d: offset %d out of [0,%d)", k, v, sym.value, sym.offset, dist[sym.value])
			}
			if seen[sym.value][sym.offset] {
				t.Fatalf("k=%d: duplicate (sym=%d, offset=%d)", k, sym.value, sym.offset)
			}
			seen[sym.value][sym.offset] = true
			counts[sym.value]++
		}
		for s := 0; s < k; s++ {
			if counts[s] != int(dist[s]) {
				t.Fatalf("k=%d sym=%d: count %d != freq %d", k, s, counts[s], dist[s])
			}
		}
	}
}

// TestReadHybridUintConfig checks token<split returns token verbatim, and the
// extra-bits path against a hand-built bitstream.
func TestReadHybridUintConfig(t *testing.T) {
	cfg := newHybridUintConfig(4, 2, 0) // split_token = 16
	// token < split_token => identity, no bits consumed.
	for _, tok := range []uint32{0, 1, 15} {
		b := newBitReader([]byte{0xFF, 0xFF})
		if got := readHybridUintConfig(cfg, tok, b); got != tok {
			t.Fatalf("identity token %d: got %d", tok, got)
		}
		if b.bitsConsumed() != 0 {
			t.Fatalf("identity token %d consumed %d bits", tok, b.bitsConsumed())
		}
	}
	// token == split_token (16): nbits = 4 - 2 + 0 = 2; msb bits = 1<<2 | (token&3)=4|0=... compute via formula.
	// value = ((((1<<msb)|(token>>lsb)&((1<<msb)-1)) << nbits) | bits) << lsb | low
	// token=16: low=0, token>>=0 ->16; msb_in=2 -> (1<<2)|(16&3)=4|0=4; nbits=2; bits=read2.
	b := newBitReader([]byte{0b11}) // bits=0b11=3 (LSB-first: first 2 bits =11)
	got := readHybridUintConfig(cfg, 16, b)
	want := uint32(((4 << 2) | 3) << 0) // ((4<<2)|3) = 19
	if got != want {
		t.Fatalf("hybrid token 16: got %d want %d", got, want)
	}
}

// TestInverseMTF checks the inverse move-to-front transform.
func TestInverseMTF(t *testing.T) {
	// Forward MTF of [2,0,0,1] given identity start:
	//   indices encode positions; verify the documented inverse behavior on a
	//   small known vector.
	v := []uint8{0, 1, 0, 2}
	inverseMoveToFront(v)
	// Hand-computed expectation:
	// mtf=[0,1,2,3,...]; i0: idx0 -> v=0, no move. mtf unchanged.
	// i1: idx1 -> v=mtf[1]=1, move 1 to front: mtf=[1,0,2,3,...]
	// i2: idx0 -> v=mtf[0]=1, no move.
	// i3: idx2 -> v=mtf[2]=2, move to front: mtf=[2,1,0,3..]
	want := []uint8{0, 1, 1, 2}
	for i := range want {
		if v[i] != want[i] {
			t.Fatalf("inverseMTF[%d]=%d want %d (got %v)", i, v[i], want[i], v)
		}
	}
}

func TestCeilLog2Nonzero(t *testing.T) {
	cases := map[uint32]int{1: 0, 2: 1, 3: 2, 4: 2, 5: 3, 8: 3, 9: 4, 16: 4, 17: 5}
	for x, want := range cases {
		if got := ceilLog2Nonzero(x); got != want {
			t.Fatalf("ceilLog2(%d)=%d want %d", x, got, want)
		}
	}
}

// TestHuffmanCanonical builds a 4-symbol, 2-bit canonical prefix code and checks
// that all four codes decode to distinct symbols covering the alphabet.
func TestHuffmanCanonical(t *testing.T) {
	codeLengths := []uint8{2, 2, 2, 2}
	var counts [16]uint16
	for _, c := range codeLengths {
		counts[c]++
	}
	table := make([]huffmanCode, 1<<huffmanTableBits)
	if buildHuffmanTable(table, huffmanTableBits, codeLengths, &counts) == 0 {
		t.Fatal("buildHuffmanTable failed")
	}
	h := &huffmanDecodingData{table: table}
	seen := map[uint16]bool{}
	for code := 0; code < 4; code++ {
		// Feed the 2-bit code (LSB-first) followed by padding.
		b := newBitReader([]byte{byte(code)})
		sym := h.ReadSymbol(b)
		if sym >= 4 {
			t.Fatalf("code %d decoded to out-of-range symbol %d", code, sym)
		}
		seen[sym] = true
		if b.bitsConsumed() != 2 {
			t.Fatalf("code %d consumed %d bits, want 2", code, b.bitsConsumed())
		}
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 distinct symbols, got %d", len(seen))
	}
}
