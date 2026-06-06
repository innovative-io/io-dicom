package gojxl

// Natural (default) coefficient scan order for VarDCT blocks. Port of
// CoeffOrderAndLut / AcStrategy::ComputeNaturalCoeffOrder from libjxl
// (lib/jxl/ac_strategy.cc). The order maps scan position -> coefficient index
// within the (cx*8) x (cy*8) coefficient block; the coded coefficient order in
// the bitstream is a permutation applied on top of this.
//
// cbx/cby are the covered-block counts of the AC strategy (1,1 for DCT8x8;
// 2,2 for DCT16x16; 4,4 for DCT32x32; 2,1 for DCT16x8; etc.). CoefficientLayout
// normalizes so the wider dimension is treated as columns.

const acBlockDim = 8

// naturalCoeffOrder returns order[scanPos] = coeffIndex (length cx*cy*64).
func naturalCoeffOrder(cbx, cby int) []int {
	return coeffOrderAndLut(cbx, cby, false)
}

// naturalCoeffOrderLut returns lut[coeffIndex] = scanPos (the inverse mapping).
func naturalCoeffOrderLut(cbx, cby int) []int {
	return coeffOrderAndLut(cbx, cby, true)
}

func coeffOrderAndLut(cbx, cby int, isLut bool) []int {
	// CoefficientLayout: columns = max, rows = min (cx >= cy).
	cx, cy := cbx, cby
	if cy > cx {
		cx, cy = cy, cx
	}
	size := cx * cy * acBlockDim * acBlockDim
	out := make([]int, size)

	xs := cx / cy
	xsm := xs - 1
	xss := ceilLog2Nonzero(uint32(xs))
	cur := cx * cy

	// First half (top-left triangle).
	for i := 0; i < cx*acBlockDim; i++ {
		for j := 0; j <= i; j++ {
			x, y := j, i-j
			if i%2 == 1 {
				x, y = y, x
			}
			if y&xsm != 0 {
				continue
			}
			y >>= xss
			val := 0
			if x < cx && y < cy {
				val = y*cx + x
			} else {
				val = cur
				cur++
			}
			pos := y*cx*acBlockDim + x
			if isLut {
				out[pos] = val
			} else {
				out[val] = pos
			}
		}
	}
	// Second half (bottom-right triangle).
	for ip := cx*acBlockDim - 1; ip > 0; ip-- {
		i := ip - 1
		for j := 0; j <= i; j++ {
			x := cx*acBlockDim - 1 - (i - j)
			y := cx*acBlockDim - 1 - j
			if i%2 == 1 {
				x, y = y, x
			}
			if y&xsm != 0 {
				continue
			}
			y >>= xss
			val := cur
			cur++
			pos := y*cx*acBlockDim + x
			if isLut {
				out[pos] = val
			} else {
				out[val] = pos
			}
		}
	}
	return out
}
