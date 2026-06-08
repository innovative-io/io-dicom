package gojxl

import "math"

// VarDCT dequantization weight generation. This file implements the DCT-band
// interpolation primitive (GetQuantWeights in lib/jxl/quant_weights.cc) that
// produces per-frequency quant weights for DCT-family transforms from a small
// set of "distance band" parameters. The full DequantMatrices::Compute (default
// library params for all 17 tables, plus the ID/DCT2/DCT4/AFV/RAW modes and the
// table inversion) builds on top of this and is added incrementally.

const kQuantAlmostZero = 1e-8

// dctMult maps a raw distance-band parameter to a multiplicative step:
// v>0 -> 1+v, else 1/(1-v). (Mult in quant_weights.cc.)
func dctMult(v float32) float32 {
	if v > 0 {
		return 1.0 + v
	}
	return 1.0 / (1.0 - v)
}

// interpolateBand evaluates the exponential interpolation of bands at the given
// already-scaled position (Interpolate / InterpolateVec). bands must have at
// least idx+2 entries; callers pad by one. pos is in [0, len-1].
func interpolateBand(pos float32, bands []float32) float32 {
	idx := int(pos)
	frac := pos - float32(idx)
	a := bands[idx]
	b := bands[idx+1]
	return a * float32(math.Pow(float64(b/a), float64(frac)))
}

// getQuantWeightsDCT computes the per-frequency quant weights for a
// ROWS x COLS DCT transform from distanceBands[c][0..numBands-1] for the three
// channels. Output layout is [c*ROWS*COLS + y*COLS + x]. Port of GetQuantWeights.
func getQuantWeightsDCT(rows, cols int, distanceBands *[3][]float32, numBands int) ([]float32, bool) {
	out := make([]float32, 3*rows*cols)
	for c := 0; c < 3; c++ {
		db := distanceBands[c]
		// Build cumulative bands; pad one extra slot so interpolateBand's idx+1
		// access at the maximum position (frac==0) is safe.
		bands := make([]float32, numBands+1)
		bands[0] = db[0]
		if bands[0] < kQuantAlmostZero {
			return nil, false
		}
		for i := 1; i < numBands; i++ {
			bands[i] = bands[i-1] * dctMult(db[i])
			if bands[i] < kQuantAlmostZero {
				return nil, false
			}
		}
		bands[numBands] = bands[numBands-1] // pad (only read with frac 0)

		scale := float32(numBands-1) / (float32(math.Sqrt2) + 1e-6)
		rcpcol := scale / float32(cols-1)
		rcprow := scale / float32(rows-1)
		for y := 0; y < rows; y++ {
			dy := float32(y) * rcprow
			dy2 := dy * dy
			for x := 0; x < cols; x++ {
				dx := float32(x) * rcpcol
				scaledDistance := float32(math.Sqrt(float64(dx*dx + dy2)))
				var w float32
				if numBands == 1 {
					w = bands[0]
				} else {
					w = interpolateBand(scaledDistance, bands)
				}
				out[c*rows*cols+y*cols+x] = w
			}
		}
	}
	return out, true
}

// afvFreqs are the AFV corner frequency positions (kFreqs in quant_weights.cc);
// the 0xBAD entries correspond to the (x<2 && y<2) lattice handled separately.
var afvFreqs = [16]float32{
	0xBAD, 0xBAD, 0.8517778890324296, 5.37778436506804,
	0xBAD, 0xBAD, 4.734747904497923, 5.449245381693219,
	1.6598270267479331, 4, 7.275749096817861, 10.423227632456525,
	2.662932286148962, 7.630657783650829, 8.962388608184032, 12.97166202570235,
}

// getQuantWeightsAFV builds the AFV quant weight matrix (kQuantModeAFV in
// quant_weights.cc): the 4x8 DCT weights in odd rows, the 4x4 corner DCT weights
// in even rows / odd columns, fixed weights for (0,1)/(1,0) and the 3-pixel
// corner, and interpolated weights for the rest of the (even,even) AFV lattice.
// Output is 3*64, channel-major, row-major within each 8x8.
func getQuantWeightsAFV(enc *quantEncoding) ([]float32, bool) {
	w48, ok := getQuantWeightsDCT(4, 8, &enc.dctBands, enc.dctBandN)
	if !ok {
		return nil, false
	}
	w44, ok := getQuantWeightsDCT(4, 4, &enc.afv4x4Bands, enc.afv4x4BandN)
	if !ok {
		return nil, false
	}
	const lo = float32(0.8517778890324296)
	const hi = float32(12.97166202570235 - 0.8517778890324296 + 1e-6)
	weights := make([]float32, 3*64)
	for c := 0; c < 3; c++ {
		var bands [4]float32
		bands[0] = enc.afvWeights[c][5]
		if bands[0] < kQuantAlmostZero {
			return nil, false
		}
		for i := 1; i < 4; i++ {
			bands[i] = bands[i-1] * dctMult(enc.afvWeights[c][i+5])
			if bands[i] < kQuantAlmostZero {
				return nil, false
			}
		}
		start := c * 64
		set := func(x, y int, val float32) { weights[start+y*8+x] = val }
		weights[start] = 1 // DC (unused; zeroed as the LF region later)
		set(0, 1, enc.afvWeights[c][0])
		set(1, 0, enc.afvWeights[c][1])
		set(0, 2, enc.afvWeights[c][2])
		set(2, 0, enc.afvWeights[c][3])
		set(2, 2, enc.afvWeights[c][4])
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				if x < 2 && y < 2 {
					continue
				}
				scaled := (afvFreqs[y*4+x] - lo) * float32(3) / hi
				set(2*x, 2*y, interpolateBand(scaled, bands[:]))
			}
		}
		// 4x8 weights in odd rows (except (0,0)).
		for y := 0; y < 4; y++ {
			for x := 0; x < 8; x++ {
				if x == 0 && y == 0 {
					continue
				}
				weights[start+(2*y+1)*8+x] = w48[c*32+y*8+x]
			}
		}
		// 4x4 weights in even rows / odd columns (except (0,0)).
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				if x == 0 && y == 0 {
					continue
				}
				weights[start+(2*y)*8+2*x+1] = w44[c*16+y*4+x]
			}
		}
	}
	return weights, true
}
