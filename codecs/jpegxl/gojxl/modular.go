package gojxl

import "errors"

// Modular mode decoding (libjxl modular/encoding/*, modular/transform/*).
// This stage produces the actual pixel channels for lossless (and modular
// lossy) JPEG XL frames.

// Predictors (modular/options.h Predictor).
const (
	predZero     = 0
	predLeft     = 1
	predTop      = 2
	predAverage0 = 3
	predSelect   = 4
	predGradient = 5
	predWeighted = 6
	predTopRight = 7
	predTopLeft  = 8
	predLeftLeft = 9
	predAverage1 = 10
	predAverage2 = 11
	predAverage3 = 12
	predAverage4 = 13
)

// MA tree contexts (ma_common.h).
const (
	ctxSplitVal      = 0
	ctxProperty      = 1
	ctxPredictor     = 2
	ctxOffset        = 3
	ctxMultiplierLog = 4
	ctxMultiplierBit = 5
	numTreeContexts  = 6
)

const kNumStaticProperties = 2

// treeNode is one MA-tree node. For a leaf, property == -1.
type treeNode struct {
	property   int
	splitval   int32
	lchild     int // for inner nodes; for leaves this is leafID
	rchild     int
	predictor  uint32
	predOffset int64
	multiplier uint32
}

func unpackSignedU(u uint32) int64 {
	return int64(u>>1) ^ -int64(u&1)
}

func unpackSigned32(u uint32) int32 { return int32(u>>1) ^ -int32(u&1) }

// decodeWPChannel decodes one channel whose MA tree is a single Weighted-
// predictor leaf (the common lossless case). General trees (property splits,
// other predictors) and inverse transforms are handled separately as they are
// implemented. Returns the channel pixels (row-major, w*h).
func decodeWPChannel(reader *ansSymbolReader, b *bitReader, ctx, w, h int, wpH wpHeader) []int32 {
	wp := newWPState(wpH, w)
	pix := make([]int32, w*h)
	get := func(xx, yy int) int64 { return int64(pix[yy*w+xx]) }
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var N, W, NE, NW, NN int64
			switch {
			case x == 0:
				if y > 0 {
					N = get(0, y-1)
					W = N
					NW = N
					if w > 1 {
						NE = get(1, y-1)
					} else {
						NE = N
					}
					if y > 1 {
						NN = get(0, y-2)
					} else {
						NN = N
					}
				}
			case y == 0:
				W = get(x-1, 0)
				N, NE, NW, NN = W, W, W, W
			default:
				W = get(x-1, y)
				N = get(x, y-1)
				NW = get(x-1, y-1)
				if x < w-1 {
					NE = get(x+1, y-1)
				} else {
					NE = N
				}
				if y > 1 {
					NN = get(x, y-2)
				} else {
					NN = N
				}
			}
			guess := wp.predict(x, y, w, N, W, NE, NW, NN)
			v := reader.readHybridUintClustered(ctx, b)
			val := int64(unpackSigned32(v)) + guess
			pix[y*w+x] = int32(val)
			wp.updateErrors(val, x, y, w)
		}
	}
	return pix
}

// readDequantMatricesDC consumes DequantMatrices::DecodeDC (quant_weights.cc):
// a 1-bit all_default flag, then 3 F16 DC quants if not default. This runs in
// LfGlobal before DecodeGlobalInfo for both modular and VarDCT frames.
func readDequantMatricesDC(b *bitReader) {
	if !b.ReadBool() { // all_default == false
		for c := 0; c < 3; c++ {
			b.ReadF16()
		}
	}
}

// decodeTree decodes the MA context tree (modular/encoding/dec_ma.cc).
func decodeTree(b *bitReader, treeSizeLimit int) ([]treeNode, error) {
	code, ctxMap, err := decodeHistograms(b, numTreeContexts, false)
	if err != nil {
		return nil, err
	}
	if code.degenerateSymbol[ctxMap[ctxProperty]] > 0 {
		return nil, errors.New("gojxl: infinite tree")
	}
	reader := newANSSymbolReader(code, b, 0)

	var tree []treeNode
	leafID := 0
	toDecode := 1
	for toDecode > 0 {
		if !b.ok() {
			return nil, errTruncated
		}
		if len(tree) > treeSizeLimit {
			return nil, errors.New("gojxl: tree too large")
		}
		toDecode--
		prop1 := reader.readHybridUint(ctxProperty, b, ctxMap)
		if prop1 > 256 {
			return nil, errors.New("gojxl: invalid tree property")
		}
		property := int(prop1) - 1
		if property == -1 {
			predictor := reader.readHybridUint(ctxPredictor, b, ctxMap)
			if predictor >= 16 {
				return nil, errors.New("gojxl: invalid predictor")
			}
			predOffset := unpackSignedU(reader.readHybridUint(ctxOffset, b, ctxMap))
			mulLog := reader.readHybridUint(ctxMultiplierLog, b, ctxMap)
			if mulLog >= 31 {
				return nil, errors.New("gojxl: invalid multiplier log")
			}
			mulBits := reader.readHybridUint(ctxMultiplierBit, b, ctxMap)
			if mulBits >= (1<<(31-mulLog))-1 {
				return nil, errors.New("gojxl: invalid multiplier")
			}
			multiplier := (mulBits + 1) << mulLog
			tree = append(tree, treeNode{
				property: -1, lchild: leafID, predictor: predictor,
				predOffset: predOffset, multiplier: multiplier,
			})
			leafID++
			continue
		}
		splitval := int32(unpackSignedU(reader.readHybridUint(ctxSplitVal, b, ctxMap)))
		tree = append(tree, treeNode{
			property: property, splitval: splitval,
			lchild: len(tree) + toDecode + 1, rchild: len(tree) + toDecode + 2,
			multiplier: 1,
		})
		toDecode += 2
	}
	if !reader.checkFinalState() {
		return nil, errors.New("gojxl: tree ANS final state failed")
	}
	return tree, nil
}

// ---- Weighted-predictor header ----

type wpHeader struct {
	p1C, p2C, p3Ca, p3Cb, p3Cc, p3Cd, p3Ce int
	w                                      [4]uint32
}

func defaultWPHeader() wpHeader {
	return wpHeader{p1C: 16, p2C: 10, p3Ca: 7, p3Cb: 7, p3Cc: 7, w: [4]uint32{0xd, 0xc, 0xc, 0xc}}
}

func readWPHeader(b *bitReader) wpHeader {
	if b.ReadBool() { // all_default
		return defaultWPHeader()
	}
	var h wpHeader
	h.p1C = int(b.ReadBits(5))
	h.p2C = int(b.ReadBits(5))
	h.p3Ca = int(b.ReadBits(5))
	h.p3Cb = int(b.ReadBits(5))
	h.p3Cc = int(b.ReadBits(5))
	h.p3Cd = int(b.ReadBits(5))
	h.p3Ce = int(b.ReadBits(5))
	for i := 0; i < 4; i++ {
		h.w[i] = b.ReadBits(4)
	}
	return h
}

// ---- Transforms ----

const (
	transformRCT     = 0
	transformPalette = 1
	transformSqueeze = 2
)

type squeezeParam struct {
	horizontal bool
	inPlace    bool
	beginC     uint32
	numC       uint32
}

type transform struct {
	id        uint32
	beginC    uint32
	rctType   uint32
	numC      uint32
	nbColors  uint32
	nbDeltas  uint32
	predictor uint32
	squeezes  []squeezeParam
}

func readTransform(b *bitReader) (transform, error) {
	var t transform
	t.id = b.ReadU32(u32Val(0), u32Val(1), u32Val(2), u32Val(3))
	if t.id == 3 {
		return t, errors.New("gojxl: invalid transform id")
	}
	if t.id == transformRCT || t.id == transformPalette {
		t.beginC = b.ReadU32(u32Bits(3), u32Off(6, 8), u32Off(10, 72), u32Off(13, 1096))
	}
	if t.id == transformRCT {
		t.rctType = b.ReadU32(u32Val(6), u32Bits(2), u32Off(4, 2), u32Off(6, 10))
		if t.rctType >= 42 {
			return t, errors.New("gojxl: invalid RCT type")
		}
	}
	if t.id == transformPalette {
		t.numC = b.ReadU32(u32Val(1), u32Val(3), u32Val(4), u32Off(13, 1))
		t.nbColors = b.ReadU32(u32Off(8, 0), u32Off(10, 256), u32Off(12, 1280), u32Off(16, 5376))
		t.nbDeltas = b.ReadU32(u32Val(0), u32Off(8, 1), u32Off(10, 257), u32Off(16, 1281))
		t.predictor = b.ReadBits(4)
		if t.predictor >= 14 {
			return t, errors.New("gojxl: invalid palette predictor")
		}
	}
	if t.id == transformSqueeze {
		n := b.ReadU32(u32Val(0), u32Off(4, 1), u32Off(6, 9), u32Off(8, 41))
		for i := uint32(0); i < n; i++ {
			var sq squeezeParam
			sq.horizontal = b.ReadBool()
			sq.inPlace = b.ReadBool()
			sq.beginC = b.ReadU32(u32Bits(3), u32Off(6, 8), u32Off(10, 72), u32Off(13, 1096))
			sq.numC = b.ReadU32(u32Val(1), u32Val(2), u32Val(3), u32Off(4, 4))
			t.squeezes = append(t.squeezes, sq)
		}
	}
	return t, nil
}

// groupHeader (encoding.h GroupHeader::VisitFields).
type groupHeader struct {
	useGlobalTree bool
	wp            wpHeader
	transforms    []transform
}

func readGroupHeader(b *bitReader) (groupHeader, error) {
	var g groupHeader
	g.useGlobalTree = b.ReadBool()
	g.wp = readWPHeader(b)
	n := b.ReadU32(u32Val(0), u32Val(1), u32Off(4, 2), u32Off(8, 18))
	for i := uint32(0); i < n; i++ {
		t, err := readTransform(b)
		if err != nil {
			return g, err
		}
		g.transforms = append(g.transforms, t)
	}
	return g, nil
}
