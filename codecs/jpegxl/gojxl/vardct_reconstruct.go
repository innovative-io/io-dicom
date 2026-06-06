package gojxl

import (
	"errors"
	"math"
)

// kInvDCQuant / kDCQuant: per-channel DC quantization weights (quant_weights.h).
var kDCQuant = [3]float32{1.0 / 4096.0, 1.0 / 512.0, 1.0 / 256.0}

// resampleScale1D returns the DC->LLF resample scales (DCTResampleScales) for a
// square DCT covering cb 8x8 blocks per axis. Index 0..cb-1.
func resampleScale1D(cb int) []float32 {
	switch cb {
	case 1:
		return []float32{1.0}
	case 2:
		return []float32{1.0, 0.901764195028874394}
	case 4:
		return []float32{1.0, 0.974886821136879522, 0.901764195028874394, 0.787054918159101335}
	default:
		return nil
	}
}

// transposeSquare transposes an n x n block in place.
func transposeSquare(b []float32, n int) {
	for y := 0; y < n; y++ {
		for x := y + 1; x < n; x++ {
			b[y*n+x], b[x*n+y] = b[x*n+y], b[y*n+x]
		}
	}
}

// reconstructVarDCT turns the decoded VarDCT state into an 8-bit sRGB image.
// Supports the square DCT strategies (DCT8x8/16x16/32x32), XYB, single group.
// Pipeline per varblock: dequant AC -> chroma-from-luma -> LLF from the DC image
// -> transpose -> normalization bridge -> inverse DCT -> place; then Gaborish/
// EPF on the XYB planes -> XYB->linear->sRGB.
func reconstructVarDCT(st *vardctState) (*Image, error) {
	if !st.meta.XYBEncoded {
		return nil, errors.New("gojxl: non-XYB VarDCT not yet supported")
	}
	for i := range st.acm.strategy {
		if !st.acm.valid[i] {
			continue
		}
		t := st.acm.strategy[i]
		if t != acDCT && t != acDCT16X16 && t != acDCT32X32 {
			return nil, errors.New("gojxl: VarDCT reconstruction supports only square DCT (8/16/32) blocks")
		}
	}
	W, H := int(st.sh.Xsize), int(st.sh.Ysize)
	bw, bh := st.acm.bw, st.acm.bh
	q := st.quant
	dcW := st.dc.w

	mul := float32(1.0) / float32(int(1)<<uint(st.dc.extraPrecision))
	invQuantDC := q.invGlobalScale / float32(q.quantDC)
	mulDC := [3]float32{invQuantDC * kDCQuant[0], invQuantDC * kDCQuant[1], invQuantDC * kDCQuant[2]}
	cflFacX := st.cmap.ytoXRatio(st.cmap.ytoxDC)
	cflFacB := st.cmap.ytoBRatio(st.cmap.ytobDC)

	// Cache the (1/weight) dequant tables per quant kind.
	matCache := map[int][]float32{}
	getMat := func(kind int) ([]float32, error) {
		if m, ok := matCache[kind]; ok {
			return m, nil
		}
		inv, ok := computeInvQuantTable(kind, st.quantLib[kind])
		if !ok {
			return nil, errors.New("gojxl: failed to build dequant table")
		}
		m := make([]float32, len(inv))
		for i, v := range inv {
			if v != 0 {
				m[i] = 1.0 / v
			}
		}
		matCache[kind] = m
		return m, nil
	}

	planeX := make([]float32, W*H)
	planeY := make([]float32, W*H)
	planeB := make([]float32, W*H)

	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			idx := by*bw + bx
			if !st.acm.valid[idx] {
				continue
			}
			t := st.acm.strategy[idx]
			cbx, cby := t.coveredBlocksX(), t.coveredBlocksY()
			pw, ph := cbx*acBlockDim, cby*acBlockDim
			nc := pw * ph
			kind := kQuantTable[t]
			mat, err := getMat(kind)
			if err != nil {
				return nil, err
			}
			bridge := float32(math.Sqrt(float64(nc)))

			qf := st.acm.quantF[idx]
			sdY := q.invGlobalScale / float32(qf)
			sdX, sdB := sdY, sdY // x/b_dm_multiplier default 1
			tile := (by/kColorTileDimInBlocks)*st.acm.ctW + bx/kColorTileDimInBlocks
			xcc := st.cmap.ytoXRatio(st.acm.ytoxMap[tile])
			bcc := st.cmap.ytoBRatio(st.acm.ytobMap[tile])

			cX, cY, cB := st.acCoeffs[0][idx], st.acCoeffs[1][idx], st.acCoeffs[2][idx]
			blkX := make([]float32, nc)
			blkY := make([]float32, nc)
			blkB := make([]float32, nc)
			for k := 0; k < nc; k++ {
				blkY[k] = adjustQuantBias(1, cY[k], &kDefaultQuantBias) * mat[1*nc+k] * sdY
				blkX[k] = adjustQuantBias(0, cX[k], &kDefaultQuantBias) * mat[0*nc+k] * sdX
				blkB[k] = adjustQuantBias(2, cB[k], &kDefaultQuantBias) * mat[2*nc+k] * sdB
			}
			for k := 0; k < nc; k++ {
				blkX[k] += xcc * blkY[k]
				blkB[k] += bcc * blkY[k]
			}

			// LLF: forward-DCT the dequantized DC of the covered cby x cbx blocks
			// (with DC chroma-from-luma) and place at the top-left LLF positions.
			scale := resampleScale1D(cbx)
			dcY := make([]float32, cbx*cby)
			dcX := make([]float32, cbx*cby)
			dcB := make([]float32, cbx*cby)
			for iy := 0; iy < cby; iy++ {
				for ix := 0; ix < cbx; ix++ {
					di := (by+iy)*dcW + (bx + ix)
					inY := float32(st.dc.y[di]) * mulDC[1] * mul
					inX := float32(st.dc.x[di]) * mulDC[0] * mul
					inB := float32(st.dc.bch[di]) * mulDC[2] * mul
					p := iy*cbx + ix
					dcY[p] = inY
					dcX[p] = inY*cflFacX + inX
					dcB[p] = inY*cflFacB + inB
				}
			}
			llfNorm := float32(1.0 / math.Sqrt(float64(cbx*cby)))
			placeLLF := func(dc []float32, blk []float32) {
				D := dct2d(dc, cbx, cby)
				for ly := 0; ly < cby; ly++ {
					for lx := 0; lx < cbx; lx++ {
						blk[ly*pw+lx] = D[ly*cbx+lx] * llfNorm * scale[ly] * scale[lx]
					}
				}
			}
			placeLLF(dcY, blkY)
			placeLLF(dcX, blkX)
			placeLLF(dcB, blkB)

			transposeSquare(blkY, pw)
			transposeSquare(blkX, pw)
			transposeSquare(blkB, pw)
			for k := 0; k < nc; k++ {
				blkY[k] *= bridge
				blkX[k] *= bridge
				blkB[k] *= bridge
			}
			pixY := idct2d(blkY, pw, ph)
			pixX := idct2d(blkX, pw, ph)
			pixB := idct2d(blkB, pw, ph)

			for yy := 0; yy < ph; yy++ {
				py := by*acBlockDim + yy
				if py >= H {
					continue
				}
				for xx := 0; xx < pw; xx++ {
					px := bx*acBlockDim + xx
					if px >= W {
						continue
					}
					planeY[py*W+px] = pixY[yy*pw+xx]
					planeX[py*W+px] = pixX[yy*pw+xx]
					planeB[py*W+px] = pixB[yy*pw+xx]
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

