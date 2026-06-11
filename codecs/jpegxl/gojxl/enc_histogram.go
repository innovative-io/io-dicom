package gojxl

// ANS histogram encoder — the inverse of readHistogram's complex (RLE +
// logcounts) path. Using shift = ansLogTabSize gives full population-count
// precision, so any normalized count is represented exactly. This replaces the
// flat histogram for streams where real frequency tables compress better.

// encLogCount[value] = {nbits, code}: the inverse of huffForLogCounts, built at
// init from the static decoder table.
var encLogCount [14]struct {
	nbits uint8
	code  uint16
}

func init() {
	for v := 0; v < 14; v++ {
		encLogCount[v].nbits = 0xFF
	}
	for idx := 0; idx < len(huffForLogCounts); idx++ {
		nbits := huffForLogCounts[idx][0]
		val := huffForLogCounts[idx][1]
		if nbits < encLogCount[val].nbits {
			encLogCount[val].nbits = nbits
			encLogCount[val].code = uint16(idx) & ((1 << nbits) - 1)
		}
	}
}

// normalizeHistogram scales raw symbol frequencies to integer counts summing to
// ansTabSize, keeping every non-zero frequency non-zero (largest-remainder with
// a min-1 floor). The result is a valid ANS distribution.
func normalizeHistogram(freq []int64) []int32 {
	const target = ansTabSize
	var total int64
	for _, f := range freq {
		total += f
	}
	counts := make([]int32, len(freq))
	if len(counts) == 0 {
		counts = make([]int32, 1)
	}
	if total == 0 {
		// Unused context: a degenerate single-symbol distribution (no tokens will
		// reference it), written via the simple-code path.
		counts[0] = ansTabSize
		return counts
	}
	var idxs []int // indices of non-zero symbols
	used := 0
	for i, f := range freq {
		if f == 0 {
			continue
		}
		c := int(float64(f) * target / float64(total))
		if c < 1 {
			c = 1
		}
		if c > target-1 {
			c = target - 1
		}
		counts[i] = int32(c)
		used += c
		idxs = append(idxs, i)
	}
	// A single-symbol (degenerate) distribution takes the whole table; it is
	// written via the simple-code path (a count of `target` would alias the RLE
	// marker in the complex path).
	if len(idxs) == 1 {
		counts[idxs[0]] = target
		return counts
	}
	// Adjust the sum to exactly `target`, one unit at a time on the largest
	// eligible count. Counts stay in [1, target-1] (target == 2^logTabSize would
	// alias the RLE marker).
	for used < target {
		bi, bv := -1, int32(0)
		for _, i := range idxs {
			if counts[i] > bv && counts[i] < target-1 {
				bv, bi = counts[i], i
			}
		}
		if bi < 0 {
			break
		}
		counts[bi]++
		used++
	}
	for used > target {
		bi, bv := -1, int32(1)
		for _, i := range idxs {
			if counts[i] > bv {
				bv, bi = counts[i], i
			}
		}
		if bi < 0 {
			break
		}
		counts[bi]--
		used--
	}
	return counts
}

// writeANSHistogram writes a normalized distribution, choosing the simple-code
// path for a degenerate (single-symbol) distribution and the complex path
// otherwise. It is the inverse of readHistogram for these two cases (is_flat is
// never used).
func writeANSHistogram(w *bitWriter, counts []int32) {
	nz, last := 0, 0
	for i, c := range counts {
		if c != 0 {
			nz++
			last = i
		}
	}
	if nz <= 1 {
		w.WriteBits(1, 1)         // simple_code = 1
		w.WriteBits(0, 1)         // num_symbols - 1 = 0 (single symbol)
		writeVarLenUint8(w, last) // the symbol index
		return
	}
	w.WriteBits(0, 1) // simple_code = 0
	w.WriteBits(0, 1) // is_flat = 0
	writeComplexHistogram(w, counts)
}

// writeHistogramShift encodes the population-count shift (unary length + bits).
func writeHistogramShift(w *bitWriter, shift int) {
	upperBoundLog := floorLog2Nonzero(uint32(ansLogTabSize + 1))
	sp1 := shift + 1
	log := floorLog2Nonzero(uint32(sp1))
	for i := 0; i < log; i++ {
		w.WriteBits(1, 1)
	}
	if log < upperBoundLog {
		w.WriteBits(0, 1)
	}
	w.WriteBits(uint64(sp1&((1<<uint(log))-1)), log)
}

// writeComplexHistogram writes a normalized distribution (summing to ansTabSize)
// using the complex (logcounts) form. No RLE is used.
func writeComplexHistogram(w *bitWriter, counts []int32) {
	// The decoder requires length >= 3; pad the distribution with zero-count
	// symbols if necessary.
	if len(counts) < 3 {
		padded := make([]int32, 3)
		copy(padded, counts)
		counts = padded
	}
	length := len(counts)
	for length > 3 && counts[length-1] == 0 {
		length--
	}
	const shift = ansLogTabSize
	writeHistogramShift(w, shift)
	writeVarLenUint8(w, length-3)

	logc := make([]int, length)
	omitPos, omitLog := 0, -1
	for i := 0; i < length; i++ {
		c := int(counts[i])
		switch {
		case c == 0:
			logc[i] = 0
		case c == 1:
			logc[i] = 1
		default:
			logc[i] = floorLog2Nonzero(uint32(c)) + 1
		}
		if logc[i] > omitLog {
			omitLog = logc[i]
			omitPos = i
		}
	}
	for i := 0; i < length; i++ {
		e := encLogCount[logc[i]]
		w.WriteBits(uint64(e.code), int(e.nbits))
	}
	for i := 0; i < length; i++ {
		if i == omitPos {
			continue
		}
		code := logc[i]
		if code <= 1 {
			continue
		}
		bitcount := int(getPopulationCountPrecision(uint32(code-1), shift))
		pop := (int(counts[i]) - (1 << uint(code-1))) >> uint(code-1-bitcount)
		w.WriteBits(uint64(pop), bitcount)
	}
}
