package gojxl

import "errors"

// ANS entropy decoder, ported from libjxl dec_ans.cc / ans_common.cc.

const (
	ansLogTabSize     = 12
	ansTabSize        = 1 << ansLogTabSize
	ansMaxAlphabet    = 256
	prefixMaxAlphabet = 4096
	ansSignature      = 0x13
	kMaxClusters      = 256
)

// ---- helpers ----

func ceilLog2Nonzero(x uint32) int {
	fl := floorLog2Nonzero(x)
	if (x & (x - 1)) != 0 {
		return fl + 1
	}
	return fl
}

func getPopulationCountPrecision(logcount, shift uint32) uint32 {
	r := int(logcount)
	if v := int(shift) - int((ansLogTabSize-logcount)>>1); v < r {
		r = v
	}
	if r < 0 {
		return 0
	}
	return uint32(r)
}

func createFlatHistogram(length, totalCount int) []int32 {
	count := int32(totalCount / length)
	out := make([]int32, length)
	for i := range out {
		out[i] = count
	}
	rem := totalCount % length
	for i := 0; i < rem; i++ {
		out[i]++
	}
	return out
}

func decodeVarLenUint8(b *bitReader) int {
	if b.ReadBits(1) != 0 {
		n := int(b.ReadBits(3))
		if n == 0 {
			return 1
		}
		return int(b.ReadBits(n)) + (1 << uint(n))
	}
	return 0
}

func decodeVarLenUint16(b *bitReader) int {
	if b.ReadBits(1) != 0 {
		n := int(b.ReadBits(4))
		if n == 0 {
			return 1
		}
		return int(b.ReadBits(n)) + (1 << uint(n))
	}
	return 0
}

// ---- alias table ----

type aliasEntry struct {
	cutoff     uint16
	rightValue uint16
	freq0      uint16
	offsets1   uint16
	freq1XOR0  uint16
}

type aliasSymbol struct {
	value  int
	offset int
	freq   int
}

func aliasLookup(table []aliasEntry, value, logEntrySize, entrySizeMinus1 int) aliasSymbol {
	i := value >> uint(logEntrySize)
	pos := value & entrySizeMinus1
	e := table[i]
	greater := pos >= int(e.cutoff)
	var offsets1OrZero, freq1xor0OrZero int
	if greater {
		offsets1OrZero = int(e.offsets1)
		freq1xor0OrZero = int(e.freq1XOR0)
	}
	var s aliasSymbol
	if greater {
		s.value = int(e.rightValue)
	} else {
		s.value = i
	}
	s.offset = offsets1OrZero + pos
	s.freq = int(e.freq0) ^ freq1xor0OrZero
	return s
}

func initAliasTable(distribution []int32, logAlphaSize int, a []aliasEntry) {
	for len(distribution) > 0 && distribution[len(distribution)-1] == 0 {
		distribution = distribution[:len(distribution)-1]
	}
	if len(distribution) == 0 {
		distribution = []int32{ansTabSize}
	}
	tableSize := 1 << uint(logAlphaSize)
	entrySize := ansTabSize >> uint(logAlphaSize) // range>>log = exact

	// Single-symbol special case.
	for sym := 0; sym < len(distribution); sym++ {
		if distribution[sym] == ansTabSize {
			for i := 0; i < tableSize; i++ {
				a[i] = aliasEntry{
					rightValue: uint16(sym),
					cutoff:     0,
					offsets1:   uint16(entrySize * i),
					freq0:      0,
					freq1XOR0:  ansTabSize,
				}
			}
			return
		}
	}

	var underfull, overfull []int
	cutoffs := make([]int, tableSize)
	for i := 0; i < len(distribution); i++ {
		cutoffs[i] = int(distribution[i])
		if cutoffs[i] > entrySize {
			overfull = append(overfull, i)
		} else if cutoffs[i] < entrySize {
			underfull = append(underfull, i)
		}
	}
	for i := len(distribution); i < tableSize; i++ {
		cutoffs[i] = 0
		underfull = append(underfull, i)
	}
	for len(overfull) > 0 {
		oi := overfull[len(overfull)-1]
		overfull = overfull[:len(overfull)-1]
		ui := underfull[len(underfull)-1]
		underfull = underfull[:len(underfull)-1]
		underfullBy := entrySize - cutoffs[ui]
		cutoffs[oi] -= underfullBy
		a[ui].rightValue = uint16(oi)
		a[ui].offsets1 = uint16(cutoffs[oi])
		if cutoffs[oi] < entrySize {
			underfull = append(underfull, oi)
		} else if cutoffs[oi] > entrySize {
			overfull = append(overfull, oi)
		}
	}
	for i := 0; i < tableSize; i++ {
		if cutoffs[i] == entrySize {
			a[i].rightValue = uint16(i)
			a[i].offsets1 = 0
			a[i].cutoff = 0
		} else {
			a[i].offsets1 -= uint16(cutoffs[i])
			a[i].cutoff = uint16(cutoffs[i])
		}
		var freq0 int
		if i < len(distribution) {
			freq0 = int(distribution[i])
		}
		i1 := int(a[i].rightValue)
		var freq1 int
		if i1 < len(distribution) {
			freq1 = int(distribution[i1])
		}
		a[i].freq0 = uint16(freq0)
		a[i].freq1XOR0 = uint16(freq1 ^ freq0)
	}
}

// huffForLogCounts is the static prefix code used while reading histograms.
var huffForLogCounts = [128][2]uint8{
	{3, 10}, {7, 12}, {3, 7}, {4, 3}, {3, 6}, {3, 8}, {3, 9}, {4, 5},
	{3, 10}, {4, 4}, {3, 7}, {4, 1}, {3, 6}, {3, 8}, {3, 9}, {4, 2},
	{3, 10}, {5, 0}, {3, 7}, {4, 3}, {3, 6}, {3, 8}, {3, 9}, {4, 5},
	{3, 10}, {4, 4}, {3, 7}, {4, 1}, {3, 6}, {3, 8}, {3, 9}, {4, 2},
	{3, 10}, {6, 11}, {3, 7}, {4, 3}, {3, 6}, {3, 8}, {3, 9}, {4, 5},
	{3, 10}, {4, 4}, {3, 7}, {4, 1}, {3, 6}, {3, 8}, {3, 9}, {4, 2},
	{3, 10}, {5, 0}, {3, 7}, {4, 3}, {3, 6}, {3, 8}, {3, 9}, {4, 5},
	{3, 10}, {4, 4}, {3, 7}, {4, 1}, {3, 6}, {3, 8}, {3, 9}, {4, 2},
	{3, 10}, {7, 13}, {3, 7}, {4, 3}, {3, 6}, {3, 8}, {3, 9}, {4, 5},
	{3, 10}, {4, 4}, {3, 7}, {4, 1}, {3, 6}, {3, 8}, {3, 9}, {4, 2},
	{3, 10}, {5, 0}, {3, 7}, {4, 3}, {3, 6}, {3, 8}, {3, 9}, {4, 5},
	{3, 10}, {4, 4}, {3, 7}, {4, 1}, {3, 6}, {3, 8}, {3, 9}, {4, 2},
	{3, 10}, {6, 11}, {3, 7}, {4, 3}, {3, 6}, {3, 8}, {3, 9}, {4, 5},
	{3, 10}, {4, 4}, {3, 7}, {4, 1}, {3, 6}, {3, 8}, {3, 9}, {4, 2},
	{3, 10}, {5, 0}, {3, 7}, {4, 3}, {3, 6}, {3, 8}, {3, 9}, {4, 5},
	{3, 10}, {4, 4}, {3, 7}, {4, 1}, {3, 6}, {3, 8}, {3, 9}, {4, 2},
}

func readHistogram(precisionBits int, b *bitReader) ([]int32, error) {
	if b.ReadBits(1) == 1 { // simple_code
		var symbols [2]int
		maxSymbol := 0
		numSymbols := int(b.ReadBits(1)) + 1
		for i := 0; i < numSymbols; i++ {
			symbols[i] = decodeVarLenUint8(b)
			if symbols[i] > maxSymbol {
				maxSymbol = symbols[i]
			}
		}
		counts := make([]int32, maxSymbol+1)
		if numSymbols == 1 {
			counts[symbols[0]] = 1 << uint(precisionBits)
		} else {
			if symbols[0] == symbols[1] {
				return nil, errors.New("gojxl: corrupt histogram")
			}
			counts[symbols[0]] = int32(b.ReadBits(precisionBits))
			counts[symbols[1]] = (1 << uint(precisionBits)) - counts[symbols[0]]
		}
		return counts, nil
	}

	if b.ReadBits(1) == 1 { // is_flat
		alphabetSize := decodeVarLenUint8(b) + 1
		return createFlatHistogram(alphabetSize, 1<<uint(precisionBits)), nil
	}

	// Complex (RLE + logcounts) path.
	var shift uint32
	{
		upperBoundLog := floorLog2Nonzero(uint32(ansLogTabSize + 1))
		log := 0
		for ; log < upperBoundLog; log++ {
			if b.ReadBits(1) == 0 {
				break
			}
		}
		shift = (b.ReadBits(log) | (1 << uint(log))) - 1
		if shift > ansLogTabSize+1 {
			return nil, errors.New("gojxl: invalid shift")
		}
	}
	length := decodeVarLenUint8(b) + 3
	counts := make([]int32, length)
	logcounts := make([]int, length)
	same := make([]int, length)
	omitLog := -1
	omitPos := -1
	for i := 0; i < length; i++ {
		b.Refill()
		idx := b.PeekBits(7)
		b.Consume(int(huffForLogCounts[idx][0]))
		logcounts[i] = int(huffForLogCounts[idx][1])
		if logcounts[i] == ansLogTabSize+1 {
			rle := decodeVarLenUint8(b)
			same[i] = rle + 5
			i += rle + 3
			continue
		}
		if logcounts[i] > omitLog {
			omitLog = logcounts[i]
			omitPos = i
		}
	}
	if omitPos < 0 {
		return nil, errors.New("gojxl: invalid histogram")
	}
	if omitPos+1 < length && logcounts[omitPos+1] == ansTabSize+1 {
		return nil, errors.New("gojxl: invalid histogram")
	}
	prev := int32(0)
	numsame := 0
	total := int32(0)
	for i := 0; i < length; i++ {
		if same[i] != 0 {
			numsame = same[i] - 1
			if i > 0 {
				prev = counts[i-1]
			} else {
				prev = 0
			}
		}
		if numsame > 0 {
			counts[i] = prev
			numsame--
		} else {
			code := logcounts[i]
			switch {
			case i == omitPos:
				total += counts[i]
				continue
			case code == 0:
				continue
			case code == 1:
				counts[i] = 1
			default:
				bitcount := int(getPopulationCountPrecision(uint32(code-1), shift))
				counts[i] = int32((1 << uint(code-1)) + (int(b.ReadBits(bitcount)) << uint(code-1-bitcount)))
			}
		}
		total += counts[i]
	}
	counts[omitPos] = (1 << uint(precisionBits)) - total
	if counts[omitPos] <= 0 {
		return nil, errors.New("gojxl: invalid histogram count")
	}
	return counts, nil
}

// ---- hybrid uint config ----

type hybridUintConfig struct {
	splitExponent uint32
	splitToken    uint32
	msbInToken    uint32
	lsbInToken    uint32
}

func newHybridUintConfig(splitExp, msb, lsb uint32) hybridUintConfig {
	return hybridUintConfig{
		splitExponent: splitExp,
		splitToken:    1 << uint(splitExp),
		msbInToken:    msb,
		lsbInToken:    lsb,
	}
}

func decodeUintConfig(logAlphaSize int, b *bitReader) (hybridUintConfig, error) {
	b.Refill()
	splitExp := b.ReadBits(ceilLog2Nonzero(uint32(logAlphaSize + 1)))
	var msb, lsb uint32
	if int(splitExp) != logAlphaSize {
		nbits := ceilLog2Nonzero(splitExp + 1)
		msb = b.ReadBits(nbits)
		if msb > splitExp {
			return hybridUintConfig{}, errors.New("gojxl: invalid uint config")
		}
		nbits = ceilLog2Nonzero(splitExp - msb + 1)
		lsb = b.ReadBits(nbits)
	}
	if lsb+msb > splitExp {
		return hybridUintConfig{}, errors.New("gojxl: invalid uint config")
	}
	return newHybridUintConfig(splitExp, msb, lsb), nil
}

// ---- LZ77 params ----

type lz77Params struct {
	enabled                  bool
	minSymbol                uint32
	minLength                uint32
	lengthUintConfig         hybridUintConfig
	nonserializedDistanceCtx uint8
}

func readLZ77Params(b *bitReader) lz77Params {
	var p lz77Params
	p.enabled = b.ReadBool()
	if !p.enabled {
		return p
	}
	p.minSymbol = b.ReadU32(u32Val(224), u32Val(512), u32Val(4096), u32Off(15, 8))
	p.minLength = b.ReadU32(u32Val(3), u32Val(4), u32Off(2, 5), u32Off(8, 9))
	return p
}

// ---- ANSCode (the decoded entropy code) ----

type ansCode struct {
	usePrefixCode    bool
	logAlphaSize     int
	uintConfig       []hybridUintConfig
	aliasTables      []aliasEntry // numHistograms * (1<<logAlphaSize)
	huffmanData      []huffmanDecodingData
	lz77             lz77Params
	numHistograms    int
	degenerateSymbol []int
}

func decodeANSCodes(numHistograms, maxAlphabetSize int, b *bitReader, code *ansCode) error {
	code.degenerateSymbol = make([]int, numHistograms)
	for i := range code.degenerateSymbol {
		code.degenerateSymbol[i] = -1
	}
	if code.usePrefixCode {
		code.huffmanData = make([]huffmanDecodingData, numHistograms)
		alphabetSizes := make([]int, numHistograms)
		for c := 0; c < numHistograms; c++ {
			alphabetSizes[c] = decodeVarLenUint16(b) + 1
			if alphabetSizes[c] > maxAlphabetSize {
				return errors.New("gojxl: alphabet too large")
			}
		}
		for c := 0; c < numHistograms; c++ {
			if alphabetSizes[c] > 1 {
				if !code.huffmanData[c].ReadFromBitStream(alphabetSizes[c], b) {
					return errors.New("gojxl: invalid huffman tree")
				}
			} else {
				code.huffmanData[c].table = make([]huffmanCode, 1<<huffmanTableBits)
			}
		}
		return nil
	}

	tableSize := 1 << uint(code.logAlphaSize)
	code.aliasTables = make([]aliasEntry, numHistograms*tableSize)
	for c := 0; c < numHistograms; c++ {
		counts, err := readHistogram(ansLogTabSize, b)
		if err != nil {
			return err
		}
		if len(counts) > maxAlphabetSize {
			return errors.New("gojxl: alphabet too large")
		}
		for len(counts) > 0 && counts[len(counts)-1] == 0 {
			counts = counts[:len(counts)-1]
		}
		degenerate := 0
		if len(counts) != 0 {
			degenerate = len(counts) - 1
		}
		for s := 0; s < degenerate; s++ {
			if counts[s] != 0 {
				degenerate = -1
				break
			}
		}
		code.degenerateSymbol[c] = degenerate
		initAliasTable(counts, code.logAlphaSize, code.aliasTables[c*tableSize:(c+1)*tableSize])
	}
	return nil
}

// decodeHistograms reads a full entropy-code header (LZ77 + context map + ANS
// or prefix codes). Returns the code and the context map.
func decodeHistograms(b *bitReader, numContexts int, disallowLZ77 bool) (*ansCode, []uint8, error) {
	code := &ansCode{}
	code.lz77 = readLZ77Params(b)
	if code.lz77.enabled {
		numContexts++
		cfg, err := decodeUintConfig(8, b)
		if err != nil {
			return nil, nil, err
		}
		code.lz77.lengthUintConfig = cfg
	}
	if code.lz77.enabled && disallowLZ77 {
		return nil, nil, errors.New("gojxl: LZ77 disallowed")
	}
	numHistograms := 1
	contextMap := make([]uint8, numContexts)
	if numContexts > 1 {
		nh, err := decodeContextMap(contextMap, b)
		if err != nil {
			return nil, nil, err
		}
		numHistograms = nh
	}
	code.lz77.nonserializedDistanceCtx = contextMap[len(contextMap)-1]
	code.usePrefixCode = b.ReadBits(1) != 0
	if code.usePrefixCode {
		code.logAlphaSize = prefixMaxBits
	} else {
		code.logAlphaSize = int(b.ReadBits(2)) + 5
	}
	code.uintConfig = make([]hybridUintConfig, numHistograms)
	for i := range code.uintConfig {
		cfg, err := decodeUintConfig(code.logAlphaSize, b)
		if err != nil {
			return nil, nil, err
		}
		code.uintConfig[i] = cfg
	}
	maxAlphabetSize := 1 << uint(code.logAlphaSize)
	if err := decodeANSCodes(numHistograms, maxAlphabetSize, b, code); err != nil {
		return nil, nil, err
	}
	code.numHistograms = numHistograms
	return code, contextMap, nil
}
