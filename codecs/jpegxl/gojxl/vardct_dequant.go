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
