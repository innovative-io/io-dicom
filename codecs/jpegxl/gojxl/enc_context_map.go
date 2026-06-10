package gojxl

// moveToFront applies the forward move-to-front transform in place: the exact
// inverse of inverseMoveToFront. Each entry is replaced by the current index of
// its value in the MTF table, then that value is moved to the front.
func moveToFront(v []uint8) {
	var mtf [256]uint8
	for i := 0; i < 256; i++ {
		mtf[i] = uint8(i)
	}
	for i := range v {
		val := v[i]
		idx := 0
		for mtf[idx] != val {
			idx++
		}
		v[i] = uint8(idx)
		if idx != 0 {
			copy(mtf[1:idx+1], mtf[0:idx])
			mtf[0] = val
		}
	}
}

// writeContextMap writes a context map (the inverse of decodeContextMap) that
// maps each raw context to a cluster id. For a single cluster it uses the
// trivial all-zero simple form; otherwise it MTF-transforms the cluster ids and
// entropy-codes them with a single-context ANS stream, which is tiny because the
// map is highly repetitive.
func writeContextMap(w *bitWriter, ctxMap []uint8, numClusters int) {
	if numClusters <= 1 {
		w.WriteBits(1, 1) // is_simple = 1
		w.WriteBits(0, 2) // bits_per_entry = 0 → all cluster 0
		return
	}

	mtf := append([]uint8(nil), ctxMap...)
	moveToFront(mtf)

	toks := make([]encToken, len(mtf))
	for i, v := range mtf {
		toks[i] = encToken{value: uint32(v)}
	}
	tks, maxTok := tokenizeAll(toks)

	w.WriteBits(0, 1) // is_simple = 0
	w.WriteBits(1, 1) // use_mtf = 1
	revMap, freqs := writeANSRealHeader(w, maxTok+1, 1, tks)
	encodeANSData(w, ansEncState{tokens: tks, revMap: revMap, freqs: freqs})
}
