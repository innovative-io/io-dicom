package gojxl

// ANS entropy encoder — the inverse of ans.go / ans_reader.go. It uses flat
// histograms (uniform distributions) for simplicity: valid and exactly
// decodable, just not optimally compressed. Tokens are hybrid-uint encoded and
// rANS-coded backward, matching enc_ans.h ANSCoder::PutSymbol.

const encLogAlpha = 8 // → max alphabet 256, alias table size 256

var encUintConfig = newHybridUintConfig(4, 2, 0)

// encToken is a value to encode in a given raw context.
type encToken struct {
	ctx   int
	value uint32
}

// encodeHybridUint splits value into (token, nbits, extra) — HybridUintConfig::Encode.
func encodeHybridUint(cfg hybridUintConfig, value uint32) (token uint32, nbits int, bits uint32) {
	if value < cfg.splitToken {
		return value, 0, 0
	}
	n := uint32(floorLog2Nonzero(value))
	m := value - (1 << n)
	token = cfg.splitToken +
		((n - cfg.splitExponent) << (cfg.msbInToken + cfg.lsbInToken)) +
		((m >> (n - cfg.msbInToken)) << cfg.lsbInToken) +
		(m & ((1 << cfg.lsbInToken) - 1))
	nb := n - cfg.msbInToken - cfg.lsbInToken
	return token, int(nb), (value >> cfg.lsbInToken) & ((1 << nb) - 1)
}

// writeVarLenUint8 is the inverse of decodeVarLenUint8 (value in [0,255]).
func writeVarLenUint8(w *bitWriter, v int) {
	if v == 0 {
		w.WriteBits(0, 1)
		return
	}
	w.WriteBits(1, 1)
	if v == 1 {
		w.WriteBits(0, 3)
		return
	}
	n := floorLog2Nonzero(uint32(v))
	w.WriteBits(uint64(n), 3)
	w.WriteBits(uint64(v-(1<<uint(n))), n)
}

// writeUintConfig is the inverse of decodeUintConfig.
func writeUintConfig(w *bitWriter, logAlpha int, cfg hybridUintConfig) {
	w.WriteBits(uint64(cfg.splitExponent), ceilLog2Nonzero(uint32(logAlpha+1)))
	if int(cfg.splitExponent) != logAlpha {
		w.WriteBits(uint64(cfg.msbInToken), ceilLog2Nonzero(cfg.splitExponent+1))
		w.WriteBits(uint64(cfg.lsbInToken), ceilLog2Nonzero(cfg.splitExponent-cfg.msbInToken+1))
	}
}

// reverseAliasMap builds reverse_map[sym][offset] = res for an alias table:
// the res value (state&mask) whose lookup yields (sym, offset).
func reverseAliasMap(counts []int32, logAlpha int) (table []aliasEntry, revMap [][]uint16, freqs []uint16) {
	tableSize := 1 << uint(logAlpha)
	table = make([]aliasEntry, tableSize)
	initAliasTable(counts, logAlpha, table)
	logEntrySize := ansLogTabSize - logAlpha
	entrySizeMinus1 := (1 << uint(logEntrySize)) - 1
	// Determine per-symbol frequency from the distribution.
	freqs = make([]uint16, tableSize)
	for s := 0; s < len(counts); s++ {
		freqs[s] = uint16(counts[s])
	}
	revMap = make([][]uint16, tableSize)
	for s := range revMap {
		if int(freqs[s]) > 0 {
			revMap[s] = make([]uint16, freqs[s])
		}
	}
	for res := 0; res < ansTabSize; res++ {
		sym := aliasLookup(table, res, logEntrySize, entrySizeMinus1)
		revMap[sym.value][sym.offset] = uint16(res)
	}
	return table, revMap, freqs
}

// ansEncState carries the tokenized symbols + flat-histogram tables between the
// header write and the (possibly later) data write, which the decoder separates
// (the channel histogram is read before the group header, its ANS data after).
type ansEncState struct {
	tokens []encTokenized
	revMap [][]uint16
	freqs  []uint16
}

type encTokenized struct {
	token uint32
	nbits int
	extra uint32
}

// encodeANSHeader tokenizes the values, builds a flat histogram, and writes the
// entropy code header (LZ77 off, simple all-zero context map → 1 histogram).
func encodeANSHeader(w *bitWriter, tokens []encToken, numContexts int) ansEncState {
	cfg := encUintConfig
	tks := make([]encTokenized, len(tokens))
	maxToken := 0
	for i, t := range tokens {
		tok, nb, ex := encodeHybridUint(cfg, t.value)
		tks[i] = encTokenized{tok, nb, ex}
		if int(tok) > maxToken {
			maxToken = int(tok)
		}
	}
	alphabet := maxToken + 1
	counts := createFlatHistogram(alphabet, ansTabSize)
	_, revMap, freqs := reverseAliasMap(counts, encLogAlpha)

	w.WriteBool(false) // lz77 disabled
	if numContexts > 1 {
		w.WriteBits(1, 1) // context map is_simple
		w.WriteBits(0, 2) // bits_per_entry = 0 → all histogram 0
	}
	w.WriteBits(0, 1)                     // use_prefix_code = 0 (ANS)
	w.WriteBits(uint64(encLogAlpha-5), 2) // log_alpha_size
	writeUintConfig(w, encLogAlpha, cfg)
	w.WriteBits(0, 1) // simple_code = 0
	w.WriteBits(1, 1) // is_flat = 1
	writeVarLenUint8(w, alphabet-1)

	return ansEncState{tokens: tks, revMap: revMap, freqs: freqs}
}

// encodeANSData rANS-codes the tokens (backward) and writes the 32-bit final
// state followed by the bit chunks in decode order.
func encodeANSData(w *bitWriter, st ansEncState) {
	type chunk struct {
		bits  uint64
		nbits int
	}
	var out []chunk
	state := uint32(ansSignature << 16)
	for i := len(st.tokens) - 1; i >= 0; i-- {
		t := st.tokens[i]
		out = append(out, chunk{uint64(t.extra), t.nbits}) // extra bits first (reversed)
		freq := uint32(st.freqs[t.token])
		var ansBits uint64
		ansN := 0
		if (state >> (32 - ansLogTabSize)) >= freq {
			ansBits = uint64(state & 0xFFFF)
			state >>= 16
			ansN = 16
		}
		res := st.revMap[t.token][state%freq]
		state = ((state / freq) << ansLogTabSize) | uint32(res)
		out = append(out, chunk{ansBits, ansN})
	}
	w.WriteBits(uint64(state), 32)
	for i := len(out) - 1; i >= 0; i-- {
		w.WriteBits(out[i].bits, out[i].nbits)
	}
}

// encodeANSStream writes a header + data contiguously (used for the MA tree).
func encodeANSStream(w *bitWriter, tokens []encToken, numContexts int) {
	st := encodeANSHeader(w, tokens, numContexts)
	encodeANSData(w, st)
}
