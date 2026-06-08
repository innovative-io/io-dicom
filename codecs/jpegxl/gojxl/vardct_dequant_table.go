package gojxl

// DequantMatrices table assembly. Ports ComputeQuantTable (quant_weights.cc):
// for a given quant-table "kind" and its QuantEncoding it builds the per-channel
// per-frequency inverse-quant table (the dequant multipliers). inv_table holds
// the weights directly; table = 1/weights. The lowest-frequency coefficients
// (the covered-block DC region) are zeroed because DC is reconstructed from the
// separate LF image, not from AC coefficients.

// 17 quant-table kinds (QuantTable enum, quant_weights.h).
const kNumQuantTables = 17

const (
	qtDCT        = 0
	qtIDENTITY   = 1
	qtDCT2X2     = 2
	qtDCT4X4     = 3
	qtDCT16X16   = 4
	qtDCT32X32   = 5
	qtDCT8X16    = 6
	qtDCT8X32    = 7
	qtDCT16X32   = 8
	qtDCT4X8     = 9
	qtAFV0       = 10
	qtDCT64X64   = 11
	qtDCT32X64   = 12
	qtDCT128X128 = 13
	qtDCT64X128  = 14
	qtDCT256X256 = 15
	qtDCT128X256 = 16
)

// required_size_x / required_size_y per kind, in 8x8 blocks (quant_weights.h).
var requiredSizeX = [kNumQuantTables]int{1, 1, 1, 1, 2, 4, 1, 1, 2, 1, 1, 8, 4, 16, 8, 32, 16}
var requiredSizeY = [kNumQuantTables]int{1, 1, 1, 1, 2, 4, 2, 4, 4, 1, 1, 8, 8, 16, 16, 32, 32}

// kQuantTable maps each of the 27 AC strategies to one of the 17 quant kinds
// (transposed transforms share a table; quant_weights.cc kQuantTable).
var kQuantTable = [acNumValidStrategies]int{
	qtDCT, qtIDENTITY, qtDCT2X2, qtDCT4X4, qtDCT16X16, qtDCT32X32,
	qtDCT8X16, qtDCT8X16, qtDCT8X32, qtDCT8X32, qtDCT16X32, qtDCT16X32,
	qtDCT4X8, qtDCT4X8, qtAFV0, qtAFV0, qtAFV0, qtAFV0,
	qtDCT64X64, qtDCT32X64, qtDCT32X64, qtDCT128X128, qtDCT64X128, qtDCT64X128,
	qtDCT256X256, qtDCT128X256, qtDCT128X256,
}

// quant encoding modes (QuantEncoding::Mode, quant_weights.h).
const (
	quantModeLibrary = 0
	quantModeID      = 1
	quantModeDCT2    = 2
	quantModeDCT4    = 3
	quantModeDCT4X8  = 4
	quantModeAFV     = 5
	quantModeDCT     = 6
	quantModeRAW     = 7
)

// quantEncoding holds the parameters needed to build one quant table.
type quantEncoding struct {
	mode int
	// DCT / DCT4 / DCT4X8 distance-band params (per channel).
	dctBands [3][]float32
	dctBandN int
	// DCT4 per-quadrant multipliers [c][0..1]; DCT4X8 single multiplier [c].
	dct4mul   [3][2]float32
	dct4x8mul [3]float32
	// IDENTITY weights [c][0..2].
	idWeights [3][3]float32
	// DCT2X2 weights [c][0..5].
	dct2Weights [3][6]float32
	// AFV: nine per-channel weights, plus the 4x4 corner DCT bands (dctBands is
	// the 4x8 DCT bands for AFV).
	afvWeights  [3][9]float32
	afv4x4Bands [3][]float32
	afv4x4BandN int
}

// computeInvQuantTable returns the inverse-quant table (length 3*wrows*wcols)
// for the given kind and encoding, with the low-frequency DC region zeroed.
// Returns false on an invalid (out-of-range) weight. The RAW mode (custom
// per-image matrices) is not yet supported.
func computeInvQuantTable(kind int, enc *quantEncoding) ([]float32, bool) {
	wrows := 8 * requiredSizeX[kind]
	wcols := 8 * requiredSizeY[kind]
	num := wrows * wcols

	var weights []float32
	switch enc.mode {
	case quantModeID:
		if num != 64 {
			return nil, false
		}
		weights = getQuantWeightsIdentity(&enc.idWeights)
	case quantModeDCT2:
		if num != 64 {
			return nil, false
		}
		weights = getQuantWeightsDCT2(&enc.dct2Weights)
	case quantModeDCT4:
		if num != 64 {
			return nil, false
		}
		w4, ok := getQuantWeightsDCT(4, 4, &enc.dctBands, enc.dctBandN)
		if !ok {
			return nil, false
		}
		weights = make([]float32, 3*64)
		for c := 0; c < 3; c++ {
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					weights[c*64+y*8+x] = w4[c*16+(y/2)*4+(x/2)]
				}
			}
			weights[c*64+1] /= enc.dct4mul[c][0]
			weights[c*64+8] /= enc.dct4mul[c][0]
			weights[c*64+9] /= enc.dct4mul[c][1]
		}
	case quantModeDCT4X8:
		if num != 64 {
			return nil, false
		}
		w48, ok := getQuantWeightsDCT(4, 8, &enc.dctBands, enc.dctBandN)
		if !ok {
			return nil, false
		}
		weights = make([]float32, 3*64)
		for c := 0; c < 3; c++ {
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					weights[c*64+y*8+x] = w48[c*32+(y/2)*8+x]
				}
			}
			weights[c*64+8] /= enc.dct4x8mul[c]
		}
	case quantModeDCT:
		w, ok := getQuantWeightsDCT(wrows, wcols, &enc.dctBands, enc.dctBandN)
		if !ok {
			return nil, false
		}
		weights = w
	case quantModeAFV:
		if num != 64 {
			return nil, false
		}
		w, ok := getQuantWeightsAFV(enc)
		if !ok {
			return nil, false
		}
		weights = w
	default:
		return nil, false // RAW / Library not yet handled here
	}

	// Validate and produce inv_table (= weights).
	invTable := make([]float32, 3*num)
	for i := 0; i < 3*num; i++ {
		v := weights[i]
		if v >= 1.0/kQuantAlmostZero || v < kQuantAlmostZero {
			return nil, false
		}
		invTable[i] = v
	}

	// Zero the lowest-frequency (covered-block DC) coefficients.
	xs, ys := requiredSizeX[kind], requiredSizeY[kind]
	// CoefficientLayout: xs = max, ys = min.
	if ys > xs {
		xs, ys = ys, xs
	}
	for c := 0; c < 3; c++ {
		for y := 0; y < ys; y++ {
			for x := 0; x < xs; x++ {
				invTable[c*ys*xs*64+y*8*xs+x] = 0
			}
		}
	}
	return invTable, true
}
