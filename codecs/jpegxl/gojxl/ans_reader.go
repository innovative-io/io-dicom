package gojxl

// ANSSymbolReader: the rANS/prefix token reader + hybrid-uint reconstruction
// + LZ77 window, ported from libjxl dec_ans.h.

const (
	lz77WindowSize      = 1 << 20
	lz77WindowMask      = lz77WindowSize - 1
	numSpecialDistances = 120
)

var kSpecialDistances = [numSpecialDistances][2]int8{
	{0, 1}, {1, 0}, {1, 1}, {-1, 1}, {0, 2}, {2, 0}, {1, 2}, {-1, 2},
	{2, 1}, {-2, 1}, {2, 2}, {-2, 2}, {0, 3}, {3, 0}, {1, 3}, {-1, 3},
	{3, 1}, {-3, 1}, {2, 3}, {-2, 3}, {3, 2}, {-3, 2}, {0, 4}, {4, 0},
	{1, 4}, {-1, 4}, {4, 1}, {-4, 1}, {3, 3}, {-3, 3}, {2, 4}, {-2, 4},
	{4, 2}, {-4, 2}, {0, 5}, {3, 4}, {-3, 4}, {4, 3}, {-4, 3}, {5, 0},
	{1, 5}, {-1, 5}, {5, 1}, {-5, 1}, {2, 5}, {-2, 5}, {5, 2}, {-5, 2},
	{4, 4}, {-4, 4}, {3, 5}, {-3, 5}, {5, 3}, {-5, 3}, {0, 6}, {6, 0},
	{1, 6}, {-1, 6}, {6, 1}, {-6, 1}, {2, 6}, {-2, 6}, {6, 2}, {-6, 2},
	{4, 5}, {-4, 5}, {5, 4}, {-5, 4}, {3, 6}, {-3, 6}, {6, 3}, {-6, 3},
	{0, 7}, {7, 0}, {1, 7}, {-1, 7}, {5, 5}, {-5, 5}, {7, 1}, {-7, 1},
	{4, 6}, {-4, 6}, {6, 4}, {-6, 4}, {2, 7}, {-2, 7}, {7, 2}, {-7, 2},
	{3, 7}, {-3, 7}, {7, 3}, {-7, 3}, {5, 6}, {-5, 6}, {6, 5}, {-6, 5},
	{8, 0}, {4, 7}, {-4, 7}, {7, 4}, {-7, 4}, {8, 1}, {8, 2}, {6, 6},
	{-6, 6}, {8, 3}, {5, 7}, {-5, 7}, {7, 5}, {-7, 5}, {8, 4}, {6, 7},
	{-6, 7}, {7, 6}, {-7, 6}, {8, 5}, {7, 7}, {-7, 7}, {8, 6}, {8, 7},
}

func specialDistance(index, multiplier int) int {
	d := int(kSpecialDistances[index][0]) + multiplier*int(kSpecialDistances[index][1])
	if d > 1 {
		return d
	}
	return 1
}

type ansSymbolReader struct {
	code            *ansCode
	usePrefixCode   bool
	configs         []hybridUintConfig
	state           uint32
	logAlphaSize    int
	logEntrySize    int
	entrySizeMinus1 int
	aliasTables     []aliasEntry
	huffmanData     []huffmanDecodingData

	// LZ77 state
	lz77Window       []uint32
	lz77Ctx          int
	lz77LengthUint   hybridUintConfig
	lz77Threshold    uint32
	lz77MinLength    uint32
	numToCopy        uint32
	copyPos          uint32
	numDecoded       uint32
	numSpecialDist   int
	specialDistances [numSpecialDistances]int
}

func newANSSymbolReader(code *ansCode, b *bitReader, distanceMultiplier int) *ansSymbolReader {
	r := &ansSymbolReader{
		code:          code,
		usePrefixCode: code.usePrefixCode,
		configs:       code.uintConfig,
		aliasTables:   code.aliasTables,
		huffmanData:   code.huffmanData,
	}
	if !code.usePrefixCode {
		r.state = b.ReadBits(32)
		r.logAlphaSize = code.logAlphaSize
		r.logEntrySize = ansLogTabSize - code.logAlphaSize
		r.entrySizeMinus1 = (1 << uint(r.logEntrySize)) - 1
	} else {
		r.state = ansSignature << 16
	}
	if !code.lz77.enabled {
		return r
	}
	r.lz77Window = make([]uint32, lz77WindowSize)
	r.lz77Ctx = int(code.lz77.nonserializedDistanceCtx)
	r.lz77LengthUint = code.lz77.lengthUintConfig
	r.lz77Threshold = code.lz77.minSymbol
	r.lz77MinLength = code.lz77.minLength
	if distanceMultiplier != 0 {
		r.numSpecialDist = numSpecialDistances
		for i := 0; i < numSpecialDistances; i++ {
			r.specialDistances[i] = specialDistance(i, distanceMultiplier)
		}
	}
	return r
}

func (r *ansSymbolReader) readSymbolANS(histoIdx int, b *bitReader) int {
	res := int(r.state & (ansTabSize - 1))
	table := r.aliasTables[histoIdx<<uint(r.logAlphaSize):]
	sym := aliasLookup(table, res, r.logEntrySize, r.entrySizeMinus1)
	r.state = uint32(sym.freq)*(r.state>>ansLogTabSize) + uint32(sym.offset)
	if r.state < (1 << 16) {
		r.state = (r.state << 16) | b.PeekBits(16)
		b.Consume(16)
	}
	return sym.value
}

func (r *ansSymbolReader) checkFinalState() bool {
	return r.usePrefixCode || r.state == (ansSignature<<16)
}

func readHybridUintConfig(cfg hybridUintConfig, token uint32, b *bitReader) uint32 {
	if token < cfg.splitToken {
		return token
	}
	nbits := cfg.splitExponent - (cfg.msbInToken + cfg.lsbInToken) +
		((token - cfg.splitToken) >> (cfg.msbInToken + cfg.lsbInToken))
	nbits &= 31
	low := token & ((1 << cfg.lsbInToken) - 1)
	token >>= cfg.lsbInToken
	bits := b.PeekBits(int(nbits))
	b.Consume(int(nbits))
	ret := (((((uint32(1) << cfg.msbInToken) | (token & ((1 << cfg.msbInToken) - 1))) << nbits) | bits) << cfg.lsbInToken) | low
	return ret
}

// readHybridUintClustered reads one token from a *clustered* context, applying
// hybrid-uint reconstruction and LZ77 copy handling.
func (r *ansSymbolReader) readHybridUintClustered(ctx int, b *bitReader) uint32 {
	if r.lz77Window != nil {
		if r.numToCopy > 0 {
			ret := r.lz77Window[r.copyPos&lz77WindowMask]
			r.copyPos++
			r.numToCopy--
			r.lz77Window[r.numDecoded&lz77WindowMask] = ret
			r.numDecoded++
			return ret
		}
	}
	b.Refill()
	token := uint32(r.readSymbolNoRefill(ctx, b))
	if r.lz77Window != nil && token >= r.lz77Threshold {
		r.numToCopy = readHybridUintConfig(r.lz77LengthUint, token-r.lz77Threshold, b) + r.lz77MinLength
		b.Refill()
		distTok := uint32(r.readSymbolNoRefill(r.lz77Ctx, b))
		distance := readHybridUintConfig(r.configs[r.lz77Ctx], distTok, b)
		if int(distance) < r.numSpecialDist {
			distance = uint32(r.specialDistances[distance])
		} else {
			distance = distance + 1 - uint32(r.numSpecialDist)
		}
		if distance > r.numDecoded {
			distance = r.numDecoded
		}
		if distance > lz77WindowSize {
			distance = lz77WindowSize
		}
		r.copyPos = r.numDecoded - distance
		if distance == 0 {
			to := r.numToCopy
			if to > lz77WindowSize {
				to = lz77WindowSize
			}
			for i := uint32(0); i < to; i++ {
				r.lz77Window[i] = 0
			}
		}
		if r.numToCopy < r.lz77MinLength {
			return 0
		}
		ret := r.lz77Window[r.copyPos&lz77WindowMask]
		r.copyPos++
		r.numToCopy--
		r.lz77Window[r.numDecoded&lz77WindowMask] = ret
		r.numDecoded++
		return ret
	}
	ret := readHybridUintConfig(r.configs[ctx], token, b)
	if r.lz77Window != nil {
		r.lz77Window[r.numDecoded&lz77WindowMask] = ret
		r.numDecoded++
	}
	return ret
}

func (r *ansSymbolReader) readSymbolNoRefill(histoIdx int, b *bitReader) int {
	if r.usePrefixCode {
		return int(r.huffmanData[histoIdx].ReadSymbol(b))
	}
	return r.readSymbolANS(histoIdx, b)
}

// readHybridUint reads from a raw context via the context map.
func (r *ansSymbolReader) readHybridUint(ctx int, b *bitReader, contextMap []uint8) uint32 {
	return r.readHybridUintClustered(int(contextMap[ctx]), b)
}
