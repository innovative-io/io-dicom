package gojxl

import "errors"

// kInvDCQuant / kDCQuant: per-channel DC quantization weights (quant_weights.h).
var kDCQuant = [3]float32{1.0 / 4096.0, 1.0 / 512.0, 1.0 / 256.0}

// reconstructVarDCT turns the decoded VarDCT state into an 8-bit sRGB image.
// Restricted to the all-DCT8x8 / XYB / single-group subset. Pipeline: dequant
// (AC matrices + DC), chroma-from-luma, inverse DCT (with the orthonormal->
// libjxl normalization bridge), then XYB->linear->sRGB.
func reconstructVarDCT(st *vardctState) (*Image, error) {
	for i := range st.acm.strategy {
		if st.acm.valid[i] && st.acm.strategy[i] != acDCT {
			return nil, errors.New("gojxl: VarDCT reconstruction supports only DCT8x8 blocks")
		}
	}
	if !st.meta.XYBEncoded {
		return nil, errors.New("gojxl: non-XYB VarDCT not yet supported")
	}
	W, H := int(st.sh.Xsize), int(st.sh.Ysize)
	bw, bh := st.acm.bw, st.acm.bh
	q := st.quant

	mul := float32(1.0) / float32(int(1)<<uint(st.dc.extraPrecision))
	invQuantDC := q.invGlobalScale / float32(q.quantDC)
	mulDC := [3]float32{invQuantDC * kDCQuant[0], invQuantDC * kDCQuant[1], invQuantDC * kDCQuant[2]}
	cflFacX := st.cmap.ytoXRatio(st.cmap.ytoxDC)
	cflFacB := st.cmap.ytoBRatio(st.cmap.ytobDC)

	invMat, ok := computeInvQuantTable(qtDCT, st.quantLib[qtDCT])
	if !ok {
		return nil, errors.New("gojxl: failed to build DCT8x8 dequant table")
	}
	// computeInvQuantTable returns inv_table (the weights); the dequant
	// multiplier is table = 1/weights. DC entries are zeroed (DC from the image).
	var mat [3 * 64]float32
	for i := range invMat {
		if invMat[i] != 0 {
			mat[i] = 1.0 / invMat[i]
		}
	}

	// Orthonormal->libjxl DCT normalization bridge. libjxl's IDCT has DC gain 1
	// and an extra sqrt(2) per AC axis relative to this decoder's orthonormal
	// idct2d; that is exactly a uniform per-axis factor of sqrt(blockDim), i.e.
	// bridge = blockDim for every coefficient. pixel = idct2d(coeff*bridge).
	const bridgeVal = float32(acBlockDim) // 8 for DCT8x8
	var bridge [64]float32
	for k := range bridge {
		bridge[k] = bridgeVal
	}

	planeX := make([]float32, W*H)
	planeY := make([]float32, W*H)
	planeB := make([]float32, W*H)

	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			idx := by*bw + bx
			qf := st.acm.quantF[idx]
			sdY := q.invGlobalScale / float32(qf)
			sdX := sdY // x_dm_multiplier defaults to 1
			sdB := sdY // b_dm_multiplier defaults to 1

			tile := (by/8)*st.acm.ctW + (bx / 8)
			xcc := st.cmap.ytoXRatio(st.acm.ytoxMap[tile])
			bcc := st.cmap.ytoBRatio(st.acm.ytobMap[tile])

			cX, cY, cB := st.acCoeffs[0][idx], st.acCoeffs[1][idx], st.acCoeffs[2][idx]
			var blkY, blkX, blkB [64]float32
			for k := 1; k < 64; k++ {
				blkY[k] = adjustQuantBias(1, cY[k], &kDefaultQuantBias) * mat[1*64+k] * sdY
				blkX[k] = adjustQuantBias(0, cX[k], &kDefaultQuantBias) * mat[0*64+k] * sdX
				blkB[k] = adjustQuantBias(2, cB[k], &kDefaultQuantBias) * mat[2*64+k] * sdB
			}
			// Chroma-from-luma on AC coefficients.
			for k := 1; k < 64; k++ {
				blkX[k] += xcc * blkY[k]
				blkB[k] += bcc * blkY[k]
			}
			// DC (LLF) from the DC image, with its own CfL DC factors.
			inY := float32(st.dc.y[idx]) * mulDC[1] * mul
			inX := float32(st.dc.x[idx]) * mulDC[0] * mul
			inB := float32(st.dc.bch[idx]) * mulDC[2] * mul
			blkY[0] = inY
			blkX[0] = inY*cflFacX + inX
			blkB[0] = inY*cflFacB + inB

			// libjxl's coefficient block is laid out transposed relative to this
			// decoder's idct2d (column/row) convention; transpose before the IDCT.
			transposeBlock8(&blkY)
			transposeBlock8(&blkX)
			transposeBlock8(&blkB)

			for k := 0; k < 64; k++ {
				blkY[k] *= bridge[k]
				blkX[k] *= bridge[k]
				blkB[k] *= bridge[k]
			}
			pixY := idct2d(blkY[:], 8, 8)
			pixX := idct2d(blkX[:], 8, 8)
			pixB := idct2d(blkB[:], 8, 8)

			for yy := 0; yy < 8; yy++ {
				py := by*8 + yy
				if py >= H {
					continue
				}
				for xx := 0; xx < 8; xx++ {
					px := bx*8 + xx
					if px >= W {
						continue
					}
					planeY[py*W+px] = pixY[yy*8+xx]
					planeX[py*W+px] = pixX[yy*8+xx]
					planeB[py*W+px] = pixB[yy*8+xx]
				}
			}
		}
	}

	// Gaborish loop filter (applied on the XYB planes before color conversion).
	lf := st.fh.LoopFilter
	if lf.gab {
		planeX = applyGaborish(planeX, W, H, lf.gabXW1, lf.gabXW2)
		planeY = applyGaborish(planeY, W, H, lf.gabYW1, lf.gabYW2)
		planeB = applyGaborish(planeB, W, H, lf.gabBW1, lf.gabBW2)
	}

	// EPF (edge-preserving filter): for epf_iters>=1 run EPF1, >=2 also EPF2.
	if lf.epfIters > 0 {
		ep := defaultEPFParams()
		quantScale := st.quant.scale()
		sigmaGrid := make([]float32, bw*bh)
		for i := 0; i < bw*bh; i++ {
			sharp := int(st.acm.epf[i])
			if sharp < 0 || sharp >= kEpfSharpEntries {
				sharp = 0
			}
			sigmaGrid[i] = computeEPFSigma(quantScale, st.acm.quantF[i], sharp, &ep)
		}
		planes := [3][]float32{planeX, planeY, planeB}
		if lf.epfIters >= 1 {
			planes = applyEPF1(&planes, W, H, sigmaGrid, bw, &ep)
		}
		if lf.epfIters >= 2 {
			planes = applyEPF2(&planes, W, H, sigmaGrid, bw, &ep)
		}
		planeX, planeY, planeB = planes[0], planes[1], planes[2]
	}

	pix := make([]byte, W*H*3)
	for i := 0; i < W*H; i++ {
		r, g, b := xybToLinearRGB(planeX[i], planeY[i], planeB[i])
		pix[i*3+0] = clamp8(linearToSRGB(r))
		pix[i*3+1] = clamp8(linearToSRGB(g))
		pix[i*3+2] = clamp8(linearToSRGB(b))
	}
	return &Image{W: W, H: H, Channels: 3, BitDepth: 8, Pixels: pix}, nil
}

// transposeBlock8 transposes an 8x8 coefficient block in place.
func transposeBlock8(b *[64]float32) {
	for y := 0; y < 8; y++ {
		for x := y + 1; x < 8; x++ {
			b[y*8+x], b[x*8+y] = b[x*8+y], b[y*8+x]
		}
	}
}

func clamp8(v float32) byte {
	x := v*255.0 + 0.5
	if x < 0 {
		return 0
	}
	if x > 255 {
		return 255
	}
	return byte(x)
}

