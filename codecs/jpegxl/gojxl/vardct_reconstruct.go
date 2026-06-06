package gojxl

import (
	"errors"
	"math"
)

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

	// Orthonormal->libjxl DCT normalization bridge: scale coeff by 1/(alpha_kx*
	// alpha_ky); alpha(0)=1/sqrt(8), alpha(k>0)=1/2. pixel = idct2d(coeff*bridge).
	var bridge [64]float32
	af := func(k int) float32 {
		if k == 0 {
			return float32(math.Sqrt(8))
		}
		return 2
	}
	for ky := 0; ky < 8; ky++ {
		for kx := 0; kx < 8; kx++ {
			bridge[ky*8+kx] = af(kx) * af(ky)
		}
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

			var blkY, blkX, blkB [64]float32
			for k := 1; k < 64; k++ {
				blkY[k] = adjustQuantBias(1, st.acCoeffs[1][idx*64+k], &kDefaultQuantBias) * mat[1*64+k] * sdY
				blkX[k] = adjustQuantBias(0, st.acCoeffs[0][idx*64+k], &kDefaultQuantBias) * mat[0*64+k] * sdX
				blkB[k] = adjustQuantBias(2, st.acCoeffs[2][idx*64+k], &kDefaultQuantBias) * mat[2*64+k] * sdB
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
	if lf := st.fh.LoopFilter; lf.gab {
		planeX = applyGaborish(planeX, W, H, lf.gabXW1, lf.gabXW2)
		planeY = applyGaborish(planeY, W, H, lf.gabYW1, lf.gabYW2)
		planeB = applyGaborish(planeB, W, H, lf.gabBW1, lf.gabBW2)
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

