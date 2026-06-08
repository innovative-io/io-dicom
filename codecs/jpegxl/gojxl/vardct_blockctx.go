package gojxl

// Block context map for VarDCT AC decoding, ported from lib/jxl/ac_context.h and
// DecodeBlockCtxMap (entropy_coder.cc). The map groups (channel, coefficient
// order, quant-field bucket, DC bucket) into a small set of entropy contexts,
// and provides the non-zero-count and zero-density context derivations used per
// AC block.

const (
	kNumOrders             = 13
	kNonZeroBuckets        = 37
	kZeroDensityContextCnt = 458
)

// Default block context map (clusters all large transforms together).
var kDefaultBlockCtxMap = [3 * kNumOrders]uint8{
	0, 1, 2, 2, 3, 3, 4, 5, 6, 6, 6, 6, 6,
	7, 8, 9, 9, 10, 11, 12, 13, 14, 14, 14, 14, 14,
	7, 8, 9, 9, 10, 11, 12, 13, 14, 14, 14, 14, 14,
}

// kCoeffFreqContext / kCoeffNumNonzeroContext index AC coefficient symbol
// contexts by scan position (ac_context.h).
var kCoeffFreqContext = [64]uint16{
	0xBAD, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14,
	15, 15, 16, 16, 17, 17, 18, 18, 19, 19, 20, 20, 21, 21, 22, 22,
	23, 23, 23, 23, 24, 24, 24, 24, 25, 25, 25, 25, 26, 26, 26, 26,
	27, 27, 27, 27, 28, 28, 28, 28, 29, 29, 29, 29, 30, 30, 30, 30,
}

var kCoeffNumNonzeroContext = [64]uint16{
	0xBAD, 0, 31, 62, 62, 93, 93, 93, 93, 123, 123, 123, 123,
	152, 152, 152, 152, 152, 152, 152, 152, 180, 180, 180, 180, 180,
	180, 180, 180, 180, 180, 180, 180, 206, 206, 206, 206, 206, 206,
	206, 206, 206, 206, 206, 206, 206, 206, 206, 206, 206, 206, 206,
	206, 206, 206, 206, 206, 206, 206, 206, 206, 206, 206, 206,
}

type blockCtxMap struct {
	dcThresholds [3][]int32
	qfThresholds []uint32
	ctxMap       []uint8
	numCtxs      int
	numDCCtxs    int
}

func defaultBlockCtxMap() *blockCtxMap {
	m := &blockCtxMap{
		ctxMap:    append([]uint8(nil), kDefaultBlockCtxMap[:]...),
		numDCCtxs: 1,
	}
	max := uint8(0)
	for _, v := range m.ctxMap {
		if v > max {
			max = v
		}
	}
	m.numCtxs = int(max) + 1
	return m
}

// NumACContexts is the total number of AC entropy contexts in the histograms.
func (m *blockCtxMap) numACContexts() int {
	return m.numCtxs * (kNonZeroBuckets + kZeroDensityContextCnt)
}

// zeroDensityContextsOffset is the offset into the AC contexts for the
// zero-density (coefficient symbol) contexts of a given block context.
func (m *blockCtxMap) zeroDensityContextsOffset(blockCtx int) int {
	return m.numCtxs*kNonZeroBuckets + kZeroDensityContextCnt*blockCtx
}

// nonZeroContext derives the context index for coding the number of non-zeros.
func (m *blockCtxMap) nonZeroContext(nonZeros, blockCtx int) int {
	if nonZeros >= 64 {
		nonZeros = 64
	}
	var ctx int
	if nonZeros < 8 {
		ctx = nonZeros
	} else {
		ctx = 4 + nonZeros/2
	}
	return ctx*m.numCtxs + blockCtx
}

// blockContext maps (dcIdx, quant-field value, order, channel) to a context.
func (m *blockCtxMap) blockContext(dcIdx int, qf uint32, ord, c int) int {
	qfIdx := 0
	for _, t := range m.qfThresholds {
		if qf > t {
			qfIdx++
		}
	}
	var idx int
	if c < 2 {
		idx = c ^ 1
	} else {
		idx = 2
	}
	idx = idx*kNumOrders + ord
	idx = idx*(len(m.qfThresholds)+1) + qfIdx
	idx = idx*m.numDCCtxs + dcIdx
	return int(m.ctxMap[idx])
}

// predictFromTopAndLeft predicts a block's non-zero count from its top and left
// neighbors (entropy_coder.h PredictFromTopAndLeft). rowTop is nil for the first
// block row.
func predictFromTopAndLeft(rowTop, row []int32, x int, def int32) int32 {
	if x == 0 {
		if rowTop == nil {
			return def
		}
		return rowTop[0]
	}
	if rowTop == nil {
		return row[x-1]
	}
	return (rowTop[x] + row[x-1] + 1) / 2
}

// zeroDensityContext is the per-coefficient context for AC symbol decoding
// (ac_context.h ZeroDensityContext).
func zeroDensityContext(nonzerosLeft, k, coveredBlocks, log2CoveredBlocks, prev int) int {
	nonzerosLeft = (nonzerosLeft + coveredBlocks - 1) >> uint(log2CoveredBlocks)
	k >>= uint(log2CoveredBlocks)
	return (int(kCoeffNumNonzeroContext[nonzerosLeft])+int(kCoeffFreqContext[k]))*2 + prev
}

// Threshold distributions (entropy_coder.h).
var (
	kDCThresholdDist = [4]u32d{u32Bits(4), u32Off(8, 16), u32Off(16, 272), u32Off(32, 65808)}
	kQFThresholdDist = [4]u32d{u32Bits(2), u32Off(3, 4), u32Off(5, 12), u32Off(8, 44)}
)

// decodeBlockCtxMap parses the block context map (DecodeBlockCtxMap).
func decodeBlockCtxMap(b *bitReader) (*blockCtxMap, error) {
	if b.ReadBits(1) == 1 {
		return defaultBlockCtxMap(), nil
	}
	m := &blockCtxMap{numDCCtxs: 1}
	for j := 0; j < 3; j++ {
		n := int(b.ReadBits(4))
		m.dcThresholds[j] = make([]int32, n)
		m.numDCCtxs *= n + 1
		for i := 0; i < n; i++ {
			m.dcThresholds[j][i] = unpackSigned32(b.ReadU32(kDCThresholdDist[0], kDCThresholdDist[1], kDCThresholdDist[2], kDCThresholdDist[3]))
		}
	}
	nq := int(b.ReadBits(4))
	m.qfThresholds = make([]uint32, nq)
	for i := 0; i < nq; i++ {
		m.qfThresholds[i] = b.ReadU32(kQFThresholdDist[0], kQFThresholdDist[1], kQFThresholdDist[2], kQFThresholdDist[3]) + 1
	}
	if m.numDCCtxs*(nq+1) > 64 {
		return nil, errInvalidBlockCtx
	}
	m.ctxMap = make([]uint8, 3*kNumOrders*m.numDCCtxs*(nq+1))
	num, err := decodeContextMap(m.ctxMap, b)
	if err != nil {
		return nil, err
	}
	m.numCtxs = num
	if m.numCtxs > 16 {
		return nil, errInvalidBlockCtx
	}
	return m, nil
}
