package gojxl

// Non-DCT quant-weight generators for the small VarDCT transforms, ported from
// lib/jxl/quant_weights.cc (GetQuantWeightsIdentity / GetQuantWeightsDCT2), with
// the default library parameters from DequantLibrary. Each fills a 3*64 weight
// array (3 channels, one 8x8 block). The DC slot (index 0) is unused here — DC
// is quantized separately — and libjxl writes a sentinel there.

// idDC is libjxl's 0xBAD sentinel written into the DC weight slot (never used).
const idDC float32 = 0xBAD

// getQuantWeightsIdentity fills weights for the IDENTITY transform from three
// per-channel params: w0 (bulk), w1 (the two first AC positions), w2 (the
// diagonal AC position).
func getQuantWeightsIdentity(id *[3][3]float32) []float32 {
	w := make([]float32, 3*64)
	for c := 0; c < 3; c++ {
		base := c * 64
		for i := 0; i < 64; i++ {
			w[base+i] = id[c][0]
		}
		w[base+1] = id[c][1]
		w[base+8] = id[c][1]
		w[base+9] = id[c][2]
	}
	return w
}

// getQuantWeightsDCT2 fills weights for the DCT2X2 transform from six per-channel
// params, in the nested-2x2/4x4 pattern of GetQuantWeightsDCT2.
func getQuantWeightsDCT2(d *[3][6]float32) []float32 {
	w := make([]float32, 3*64)
	for c := 0; c < 3; c++ {
		base := c * 64
		w[base] = idDC
		w[base+1] = d[c][0]
		w[base+8] = d[c][0]
		w[base+9] = d[c][1]
		// 2x2 off-diagonal blocks -> d[2]; 2x2 diagonal -> d[3].
		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				w[base+y*8+x+2] = d[c][2]
				w[base+(y+2)*8+x] = d[c][2]
			}
		}
		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				w[base+(y+2)*8+x+2] = d[c][3]
			}
		}
		// 4x4 off-diagonal blocks -> d[4]; 4x4 diagonal -> d[5].
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				w[base+y*8+x+4] = d[c][4]
				w[base+(y+4)*8+x] = d[c][4]
			}
		}
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				w[base+(y+4)*8+x+4] = d[c][5]
			}
		}
	}
	return w
}

// Default library parameters (DequantLibrary in quant_weights.cc).
var defaultIdentityWeights = [3][3]float32{
	{280.0, 3160.0, 3160.0},
	{60.0, 864.0, 864.0},
	{18.0, 200.0, 200.0},
}

var defaultDCT2Weights = [3][6]float32{
	{3840.0, 2560.0, 1280.0, 640.0, 480.0, 300.0},
	{960.0, 640.0, 320.0, 180.0, 140.0, 120.0},
	{640.0, 320.0, 128.0, 64.0, 32.0, 16.0},
}
