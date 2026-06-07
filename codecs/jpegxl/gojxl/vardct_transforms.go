package gojxl

// Bespoke VarDCT inverse transforms that do not use a DCT, ported from
// TransformToPixels in dec_transforms-inl.h: IDENTITY and DCT2X2. These map an
// 8x8 coefficient block to an 8x8 pixel block. The DCT-family small transforms
// (DCT4X4, DCT4X8, DCT8X4, AFV) additionally invoke the scaled IDCT and depend
// on the DCT-normalization bridge, so they are added once that is resolved.

// idct2TopBlock applies the 2x2 Hadamard butterfly over the top SxS region of an
// 8x8 block (IDCT2TopBlock<S>), writing to out with the given row stride. out may
// alias block (an internal temp makes it safe). S must be even and divide 8.
func idct2TopBlock(s int, block []float32, strideOut int, out []float32) {
	const bd = acBlockDim // 8
	var temp [64]float32
	num2x2 := s / 2
	for y := 0; y < num2x2; y++ {
		for x := 0; x < num2x2; x++ {
			c00 := block[y*bd+x]
			c01 := block[y*bd+num2x2+x]
			c10 := block[(y+num2x2)*bd+x]
			c11 := block[(y+num2x2)*bd+num2x2+x]
			temp[y*2*bd+x*2] = c00 + c01 + c10 + c11
			temp[y*2*bd+x*2+1] = c00 + c01 - c10 - c11
			temp[(y*2+1)*bd+x*2] = c00 - c01 + c10 - c11
			temp[(y*2+1)*bd+x*2+1] = c00 - c01 - c10 + c11
		}
	}
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			out[y*strideOut+x] = temp[y*bd+x]
		}
	}
}

// inverseDCT2X2 transforms a 64-entry DCT2X2 coefficient block to an 8x8 pixel
// block via successive 2x2 Hadamard butterflies at scales 2, 4, 8.
func inverseDCT2X2(coeffs []float32) []float32 {
	c := make([]float32, 64)
	copy(c, coeffs)
	idct2TopBlock(2, c, acBlockDim, c)
	idct2TopBlock(4, c, acBlockDim, c)
	idct2TopBlock(8, c, acBlockDim, c)
	return c
}

// inverseDCT8X4 transforms a 64-entry DCT8X4 coefficient block to an 8x8 pixel
// block: two side-by-side 8x4 scaled IDCTs whose DCs are the sum/difference of
// coefficients[0] and coefficients[8] (TransformToPixels DCT8X4).
func inverseDCT8X4(coeffs []float32) []float32 {
	const stride = acBlockDim
	pix := make([]float32, 64)
	dcs := [2]float32{coeffs[0] + coeffs[8], coeffs[0] - coeffs[8]}
	for x := 0; x < 2; x++ {
		block := make([]float32, 4*8)
		block[0] = dcs[x]
		for iy := 0; iy < 4; iy++ {
			for ix := 0; ix < 8; ix++ {
				if ix == 0 && iy == 0 {
					continue
				}
				block[iy*8+ix] = coeffs[(x+iy*2)*8+ix]
			}
		}
		// ComputeScaledIDCT<8,4>: 8 rows x 4 cols.
		sub := plainInverseDCT(block, 4, 8)
		for r := 0; r < 8; r++ {
			for c := 0; c < 4; c++ {
				pix[r*stride+x*4+c] = sub[r*4+c]
			}
		}
	}
	return pix
}

// inverseDCT4X8 transforms a 64-entry DCT4X8 coefficient block to an 8x8 pixel
// block: two stacked 4x8 scaled IDCTs whose DCs are the sum/difference of
// coefficients[0] and coefficients[8] (TransformToPixels DCT4X8).
func inverseDCT4X8(coeffs []float32) []float32 {
	const stride = acBlockDim
	pix := make([]float32, 64)
	dcs := [2]float32{coeffs[0] + coeffs[8], coeffs[0] - coeffs[8]}
	for y := 0; y < 2; y++ {
		block := make([]float32, 4*8)
		block[0] = dcs[y]
		for iy := 0; iy < 4; iy++ {
			for ix := 0; ix < 8; ix++ {
				if ix == 0 && iy == 0 {
					continue
				}
				block[iy*8+ix] = coeffs[(y+iy*2)*8+ix]
			}
		}
		// ComputeScaledIDCT<4,8>: 4 rows x 8 cols.
		sub := plainInverseDCT(block, 8, 4)
		for r := 0; r < 4; r++ {
			for c := 0; c < 8; c++ {
				pix[(y*4+r)*stride+c] = sub[r*8+c]
			}
		}
	}
	return pix
}

// inverseDCT4X4 transforms a 64-entry DCT4X4 coefficient block to an 8x8 pixel
// block: four 4x4 scaled IDCTs whose DCs are the 2x2 Hadamard of coefficients
// [0],[1],[8],[9] (TransformToPixels DCT4X4).
func inverseDCT4X4(coeffs []float32) []float32 {
	const stride = acBlockDim
	pix := make([]float32, 64)
	b00, b01, b10, b11 := coeffs[0], coeffs[1], coeffs[8], coeffs[9]
	dcs := [4]float32{b00 + b01 + b10 + b11, b00 + b01 - b10 - b11, b00 - b01 + b10 - b11, b00 - b01 - b10 + b11}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			block := make([]float32, 16)
			block[0] = dcs[y*2+x]
			for iy := 0; iy < 4; iy++ {
				for ix := 0; ix < 4; ix++ {
					if ix == 0 && iy == 0 {
						continue
					}
					block[iy*4+ix] = coeffs[(y+iy*2)*8+x+ix*2]
				}
			}
			sub := plainInverseDCT(block, 4, 4)
			for r := 0; r < 4; r++ {
				for c := 0; c < 4; c++ {
					pix[(y*4+r)*stride+x*4+c] = sub[r*4+c]
				}
			}
		}
	}
	return pix
}

// inverseIdentity transforms a 64-entry IDENTITY coefficient block to an 8x8
// pixel block (residual-from-block-DC coding over four 4x4 sub-blocks).
func inverseIdentity(coeffs []float32) []float32 {
	const stride = acBlockDim
	pix := make([]float32, 64)
	b00, b01, b10, b11 := coeffs[0], coeffs[1], coeffs[8], coeffs[9]
	dcs := [4]float32{
		b00 + b01 + b10 + b11,
		b00 + b01 - b10 - b11,
		b00 - b01 + b10 - b11,
		b00 - b01 - b10 + b11,
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			blockDC := dcs[y*2+x]
			var residualSum float32
			for iy := 0; iy < 4; iy++ {
				for ix := 0; ix < 4; ix++ {
					if ix == 0 && iy == 0 {
						continue
					}
					residualSum += coeffs[(y+iy*2)*8+x+ix*2]
				}
			}
			center := blockDC - residualSum*(1.0/16)
			pix[(4*y+1)*stride+4*x+1] = center
			for iy := 0; iy < 4; iy++ {
				for ix := 0; ix < 4; ix++ {
					if ix == 1 && iy == 1 {
						continue
					}
					pix[(y*4+iy)*stride+x*4+ix] = coeffs[(y+iy*2)*8+x+ix*2] + center
				}
			}
			pix[y*4*stride+x*4] = coeffs[(y+2)*8+x+2] + center
		}
	}
	return pix
}
