package gojxl

// AFV (asymmetric corner) inverse transforms, ported from AFVTransformToPixels /
// AFVIDCT4x4 in dec_transforms-inl.h. An AFV block covers a single 8x8 block and
// is split into three regions: a 4x4 AFV corner (even,even coefficient lattice,
// transformed by the 16x16 IAFV basis and flipped into the chosen corner), a 4x4
// DCT (odd,even lattice) and a 4x8 DCT (odd rows). afvKind selects the corner:
// bit0 = x flip, bit1 = y flip (AFV0..AFV3).

// afvIDCT4x4 applies the 16x16 IAFV basis: out[i] = sum_j coeff[j]*basis[j*16+i].
func afvIDCT4x4(coeff []float32) []float32 {
	out := make([]float32, 16)
	for j := 0; j < 16; j++ {
		cf := coeff[j]
		if cf == 0 {
			continue
		}
		base := j * 16
		for i := 0; i < 16; i++ {
			out[i] += cf * k4x4AFVBasis[base+i]
		}
	}
	return out
}

// inverseAFV transforms a 64-entry AFV coefficient block to an 8x8 pixel block.
// afvKind is 0..3 (AFV0..AFV3).
func inverseAFV(coeffs []float32, afvKind int) []float32 {
	const stride = acBlockDim // 8
	afvX := afvKind & 1
	afvY := afvKind / 2
	pix := make([]float32, 64)

	block00, block01, block10 := coeffs[0], coeffs[1], coeffs[8]
	dcs := [3]float32{
		(block00 + block10 + block01) * 4.0,
		block00 + block10 - block01,
		block00 - block10,
	}

	// IAFV on the (even, even) coefficient lattice -> 4x4 corner.
	coeff := make([]float32, 16)
	coeff[0] = dcs[0]
	for iy := 0; iy < 4; iy++ {
		for ix := 0; ix < 4; ix++ {
			if ix == 0 && iy == 0 {
				continue
			}
			coeff[iy*4+ix] = coeffs[iy*2*8+ix*2]
		}
	}
	corner := afvIDCT4x4(coeff)
	for iy := 0; iy < 4; iy++ {
		for ix := 0; ix < 4; ix++ {
			sy, sx := iy, ix
			if afvY == 1 {
				sy = 3 - iy
			}
			if afvX == 1 {
				sx = 3 - ix
			}
			pix[(iy+afvY*4)*stride+afvX*4+ix] = corner[sy*4+sx]
		}
	}

	// IDCT4x4 on the (odd, even) lattice.
	b4 := make([]float32, 16)
	b4[0] = dcs[1]
	for iy := 0; iy < 4; iy++ {
		for ix := 0; ix < 4; ix++ {
			if ix == 0 && iy == 0 {
				continue
			}
			b4[iy*4+ix] = coeffs[iy*2*8+ix*2+1]
		}
	}
	sub44 := plainInverseDCT(b4, 4, 4)
	colOff := 4
	if afvX == 1 {
		colOff = 0
	}
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			pix[(afvY*4+r)*stride+colOff+c] = sub44[r*4+c]
		}
	}

	// IDCT4x8 on the odd rows.
	b8 := make([]float32, 32)
	b8[0] = dcs[2]
	for iy := 0; iy < 4; iy++ {
		for ix := 0; ix < 8; ix++ {
			if ix == 0 && iy == 0 {
				continue
			}
			b8[iy*8+ix] = coeffs[(1+iy*2)*8+ix]
		}
	}
	sub48 := plainInverseDCT(b8, 8, 4)
	rowOff := 4
	if afvY == 1 {
		rowOff = 0
	}
	for r := 0; r < 4; r++ {
		for c := 0; c < 8; c++ {
			pix[(rowOff+r)*stride+c] = sub48[r*8+c]
		}
	}
	return pix
}
