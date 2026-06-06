package gojxl

// VarDCT quantizer and coefficient dequantization helpers, ported from
// lib/jxl/quantizer.h and the dequant path in lib/jxl/dec_group.cc.
//
// An AC coefficient is dequantized as
//
//	coeff[c][k] = adjustQuantBias(c, q[c][k]) * dequantMatrix[c][k] * scaledDequant[c]
//
// where scaledDequant[Y] = invGlobalScale / blockQuant, scaledDequant[X|B] is
// that times the per-frame x_dm_multiplier / b_dm_multiplier, and dequantMatrix
// is the per-(strategy,channel,frequency) weight (DequantMatrices, separate).

const (
	kGlobalScaleDenom     = 1 << 16 // 65536
	kGlobalScaleNumerator = 4096
	kBiasNumerator        = 0.145
)

// kDefaultQuantBias = opsin_params.quant_biases default (quantizer.h). Indices
// 0..2 are the per-channel single-step biases; index 3 is the divisor numerator
// for larger magnitudes.
var kDefaultQuantBias = [4]float32{
	1.0 - 0.05465007330715401,
	1.0 - 0.07005449891748593,
	1.0 - 0.049935103337343655,
	0.145,
}

// adjustQuantBias maps a quantized integer coefficient to its dequantized
// magnitude prior to the matrix/scale multiply (AdjustQuantBias, quantizer-inl.h):
//
//	q == 0  -> 0
//	q == 1  -> biases[c]
//	q == -1 -> -biases[c]
//	else    -> q - biases[3]/q
//
// (libjxl uses an approximate reciprocal for the division; this uses exact
// division, ~2e-5 closer to the ideal.)
func adjustQuantBias(c int, q int32, biases *[4]float32) float32 {
	switch q {
	case 0:
		return 0
	case 1:
		return biases[c]
	case -1:
		return -biases[c]
	default:
		fq := float32(q)
		return fq - biases[3]/fq
	}
}

// quantizer holds the frame-global quantization scale (quantizer.h).
type quantizer struct {
	globalScale    int
	quantDC        int
	globalScaleF   float32
	invGlobalScale float32
	invQuantDC     float32
}

func newQuantizer(globalScale, quantDC int) *quantizer {
	q := &quantizer{globalScale: globalScale, quantDC: quantDC}
	q.recompute()
	return q
}

func (q *quantizer) recompute() {
	q.globalScaleF = float32(q.globalScale) * (1.0 / kGlobalScaleDenom)
	q.invGlobalScale = float32(kGlobalScaleDenom) / float32(q.globalScale)
	q.invQuantDC = q.invGlobalScale / float32(q.quantDC)
}

// scale returns the factor s.t. scale()*rawQuantField recovers the AC quant
// step, mirroring Quantizer::Scale().
func (q *quantizer) scale() float32 { return q.globalScaleF }

// invQuantAC is the AC dequant step for a per-block quant multiplier.
func (q *quantizer) invQuantAC(blockQuant int32) float32 {
	return q.invGlobalScale / float32(blockQuant)
}

// QuantizerParams field distributions (quantizer.cc QuantizerParams::VisitFields).
var (
	qGlobalScaleDist = [4]u32d{u32Off(11, 1), u32Off(11, 2049), u32Off(12, 4097), u32Off(16, 8193)}
	qQuantDCDist     = [4]u32d{u32Val(16), u32Off(5, 1), u32Off(8, 1), u32Off(16, 1)}
)

// decodeQuantizer reads the QuantizerParams bundle (global_scale, quant_dc) from
// the LfGlobal section (Quantizer::Decode) and returns the configured quantizer.
func decodeQuantizer(b *bitReader) *quantizer {
	gs := b.ReadU32(qGlobalScaleDist[0], qGlobalScaleDist[1], qGlobalScaleDist[2], qGlobalScaleDist[3])
	qdc := b.ReadU32(qQuantDCDist[0], qQuantDCDist[1], qQuantDCDist[2], qQuantDCDist[3])
	return newQuantizer(int(gs), int(qdc))
}
