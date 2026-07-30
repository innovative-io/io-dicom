package gojxl

import (
	"errors"
	"math"
)

// kInvDCQuant / kDCQuant: per-channel DC quantization weights (quant_weights.h).
var kDCQuant = [3]float32{1.0 / 4096.0, 1.0 / 512.0, 1.0 / 256.0}

// resampleScale1D returns the DC->LLF resample scales for the inverse transform:
// DCTResampleScales<FROM=cb, TO=8*cb> (dct_scales.h), the up-sampling ("inverse
// of the above") scales used by LowestFrequenciesFromDC — NOT the down-sampling
// <8*cb, cb> scales. cb is the covered 8x8 blocks on the axis; index 0..cb-1.
// Package-level scale tables. These were slice literals inside resampleScale1D,
// so every call heap-allocated a fresh copy — 32768 allocations for a single
// 512x512 lossy decode. Callers only read them (see the scaleRow/scaleCol use in
// reconstructVarDCT), so one shared copy per size is safe.
var (
	resampleScales1 = []float32{1.0}
	resampleScales2 = []float32{1.0, 1.108937353592731823}
	resampleScales4 = []float32{1.0, 1.025760096781116015, 1.108937353592731823, 1.270559368765487251}
	resampleScales8 = []float32{
		1.0000000000000000, 1.0063534990068217, 1.0257600967811158,
		1.0593017296817173, 1.1089373535927318, 1.1777765381970435,
		1.2705593687654873, 1.3944898413647777,
	}
)

// resampleScales16-- DCTResampleScales<16, 128>
var resampleScales16 = []float32{
	1.0, 1.0015830492062623, 1.0063534990068217, 1.0143759095928793,
	1.0257600967811158, 1.0406645869480142, 1.0593017296817173, 1.0819447744633812,
	1.1089373535927318, 1.1407059950032632, 1.1777765381970435, 1.2207956782315876,
	1.2705593687654873, 1.3280505578213306, 1.3944898413647777, 1.4714043176061107,
}

// resampleScales32-- DCTResampleScales<32, 256>
var resampleScales32 = []float32{
	1.0, 1.0003954307206069, 1.0015830492062623, 1.0035668445360069,
	1.0063534990068217, 1.009952439375063, 1.0143759095928793, 1.0196390660647288,
	1.0257600967811158, 1.0327603660498115, 1.0406645869480142, 1.049501024072585,
	1.0593017296817173, 1.0701028169146336, 1.0819447744633812, 1.0948728278734026,
	1.1089373535927318, 1.124194353004584, 1.1407059950032632, 1.158541237256391,
	1.1777765381970435, 1.1984966740820495, 1.2207956782315876, 1.244777922949508,
	1.2705593687654873, 1.2982690107339132, 1.3280505578213306, 1.3600643892400104,
	1.3944898413647777, 1.4315278911623237, 1.4714043176061107, 1.5143734423314616,
}

func resampleScale1D(cb int) []float32 {
	switch cb {
	case 1:
		return resampleScales1
	case 2:
		return resampleScales2
	case 4:
		return resampleScales4
	case 8:
		return resampleScales8
	case 16: // DCTResampleScales<16, 128>
		return resampleScales16
	case 32: // DCTResampleScales<32, 256>
		return resampleScales32
	default:
		return nil
	}
}

// transposeRect reinterprets a coefficient block from libjxl's storage layout
// into the row-major [ROWS][COLS] layout that idct2d expects. libjxl stores
// coefficients row-major (idx = r*COLS+c) when ROWS<COLS, but transposed
// (idx = c*ROWS+r) when ROWS>=COLS. transpose must be true in the latter case.
func transposeRect(blk []float32, pw, ph int, transpose bool) []float32 {
	if !transpose {
		return blk // already row-major [ROWS][COLS]
	}
	out := make([]float32, pw*ph)
	for r := 0; r < ph; r++ {
		for c := 0; c < pw; c++ {
			out[r*pw+c] = blk[c*ph+r]
		}
	}
	return out
}

// adaptiveDCSmoothing smooths the dequantized DC image (compressed_dc.cc
// AdaptiveDCSmoothing): each interior DC pixel is blended toward a 3x3 weighted
// average by a factor that shrinks where the change is large relative to the DC
// quant step (preserving edges). dcFactor is the per-channel MulDC. Skipped for
// DC images <=2 in either dimension. Planes are X, Y, B.
func adaptiveDCSmoothing(planes [3][]float32, w, h int, dcFactor [3]float32) {
	if h <= 2 || w <= 2 {
		return
	}
	const w1 = float32(0.20345139757231578)
	const w2 = float32(0.0334829185968739)
	const w0 = float32(1.0 - 4.0*(0.20345139757231578+0.0334829185968739))
	var out [3][]float32
	for c := range out {
		out[c] = append([]float32(nil), planes[c]...) // borders stay unchanged
	}
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			var mc, sm [3]float32
			gap := float32(0.5)
			for c := 0; c < 3; c++ {
				p := planes[c]
				tl, tc, tr := p[(y-1)*w+x-1], p[(y-1)*w+x], p[(y-1)*w+x+1]
				ml, m, mr := p[y*w+x-1], p[y*w+x], p[y*w+x+1]
				bl, bc, br := p[(y+1)*w+x-1], p[(y+1)*w+x], p[(y+1)*w+x+1]
				s := w0*m + w1*(ml+mr+tc+bc) + w2*(tl+tr+bl+br)
				mc[c], sm[c] = m, s
				if g := absf(m-s) / dcFactor[c]; g > gap {
					gap = g
				}
			}
			factor := 3.0 - 4.0*gap
			if factor < 0 {
				factor = 0
			}
			for c := 0; c < 3; c++ {
				out[c][y*w+x] = mc[c] + factor*(sm[c]-mc[c])
			}
		}
	}
	for c := 0; c < 3; c++ {
		copy(planes[c], out[c])
	}
}

// plainInverseDCT performs the inverse of libjxl's ComputeScaledIDCT<ROWS,COLS>
// for a dequantized coefficient block in libjxl storage layout: reinterpret to
// row-major [ROWS][COLS], apply the uniform normalization bridge sqrt(ROWS*COLS),
// then the separable orthonormal inverse DCT. pw=COLS (width), ph=ROWS (height).
// The caller must have already placed any LLF/DC into blk. Returns ph*pw pixels
// row-major (pix[r*pw+c]). blk is consumed (modified).
func plainInverseDCT(blk []float32, pw, ph int) []float32 {
	rm := transposeRect(blk, pw, ph, ph >= pw)
	bridge := float32(math.Sqrt(float64(pw * ph)))
	for k := range rm {
		rm[k] *= bridge
	}
	return idct2d(rm, pw, ph)
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
		if resampleScale1D(t.coveredBlocksX()) == nil || resampleScale1D(t.coveredBlocksY()) == nil {
			return nil, errors.New("gojxl: VarDCT reconstruction: unsupported transform size")
		}
	}
	W, H := int(st.sh.Xsize), int(st.sh.Ysize)
	bw, bh := st.acm.bw, st.acm.bh
	q := st.quant
	dcW := st.dc.w

	mul := float32(1.0) / float32(int(1)<<uint(st.dc.extraPrecision))
	invQuantDC := q.invGlobalScale / float32(q.quantDC)
	mulDC := [3]float32{invQuantDC * kDCQuant[0], invQuantDC * kDCQuant[1], invQuantDC * kDCQuant[2]}
	// AC dequant per-channel multipliers for X and B (dec_cache.h): the X/B
	// quant matrices are scaled by pow(1/1.25, qm_scale-2). Defaults x_qm_scale=3
	// (-> 0.8), b_qm_scale=2 (-> 1.0). Applied to the AC dequant only, not DC.
	xDM := float32(math.Pow(1.0/1.25, float64(st.fh.XQMScale)-2.0))
	bDM := float32(math.Pow(1.0/1.25, float64(st.fh.BQMScale)-2.0))
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

	// Dequantize the DC image (with DC chroma-from-luma) once, then adaptively
	// smooth it; the LLF of every varblock reads from this smoothed DC image.
	dcH := st.dc.h
	dcImg := [3][]float32{
		make([]float32, dcW*dcH), make([]float32, dcW*dcH), make([]float32, dcW*dcH),
	}
	for i := 0; i < dcW*dcH; i++ {
		inY := float32(st.dc.y[i]) * mulDC[1] * mul
		inX := float32(st.dc.x[i]) * mulDC[0] * mul
		inB := float32(st.dc.bch[i]) * mulDC[2] * mul
		dcImg[0][i] = inY*cflFacX + inX // X
		dcImg[1][i] = inY               // Y
		dcImg[2][i] = inY*cflFacB + inB // B
	}
	adaptiveDCSmoothing(dcImg, dcW, dcH, mulDC)

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

			qf := st.acm.quantF[idx]
			sdY := q.invGlobalScale / float32(qf)
			sdX, sdB := sdY*xDM, sdY*bDM
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
			// Resample scales are per-axis (vertical=cby rows, horizontal=cbx
			// cols), and the placement uses libjxl's storage layout (transposed
			// when ROWS>=COLS, i.e. cby>=cbx).
			scaleRow := resampleScale1D(cby) // vertical axis
			scaleCol := resampleScale1D(cbx) // horizontal axis
			transpose := cby >= cbx
			dcY := make([]float32, cbx*cby)
			dcX := make([]float32, cbx*cby)
			dcB := make([]float32, cbx*cby)
			for iy := 0; iy < cby; iy++ {
				for ix := 0; ix < cbx; ix++ {
					di := (by+iy)*dcW + (bx + ix)
					p := iy*cbx + ix
					dcX[p] = dcImg[0][di]
					dcY[p] = dcImg[1][di]
					dcB[p] = dcImg[2][di]
				}
			}
			llfNorm := float32(1.0 / math.Sqrt(float64(cbx*cby)))
			placeLLF := func(dc []float32, blk []float32) {
				// dct2d returns the forward DCT in [vert][horiz] = D[r*cbx+c]
				// order; write it into the block in libjxl's coefficient storage
				// layout so it lines up with the dequantized AC before the IDCT.
				D := dct2d(dc, cbx, cby)
				for r := 0; r < cby; r++ {
					for c := 0; c < cbx; c++ {
						val := D[r*cbx+c] * llfNorm * scaleRow[r] * scaleCol[c]
						if transpose {
							blk[c*ph+r] = val
						} else {
							blk[r*pw+c] = val
						}
					}
				}
			}
			placeLLF(dcY, blkY)
			placeLLF(dcX, blkX)
			placeLLF(dcB, blkB)

			// Inverse transform each channel block to pixels. Plain DCTs use the
			// separable IDCT; the special transforms (IDENTITY/DCT2x2/DCT4x4/
			// DCT4x8/DCT8x4) have bespoke inverses reading the block directly.
			inverse := func(blk []float32) []float32 {
				switch t {
				case acDCT2X2:
					return inverseDCT2X2(blk)
				case acIDENTITY:
					return inverseIdentity(blk)
				case acDCT4X4:
					return inverseDCT4X4(blk)
				case acDCT4X8:
					return inverseDCT4X8(blk)
				case acDCT8X4:
					return inverseDCT8X4(blk)
				case acAFV0, acAFV1, acAFV2, acAFV3:
					return inverseAFV(blk, int(t)-int(acAFV0))
				default:
					return plainInverseDCT(blk, pw, ph)
				}
			}
			pixY := inverse(blkY)
			pixX := inverse(blkX)
			pixB := inverse(blkB)

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

	// Interleave the colour channels (1 for grayscale — luminance is carried in
	// the three equal sRGB channels; 3 for RGB) with any decoded extra channels
	// (alpha, ...). Extra channels are full-resolution 8-bit samples.
	gray := st.meta.Color.ColorSpace == csGray
	colorCh := 3
	if gray {
		colorCh = 1
	}
	nc := colorCh + len(st.extra)
	pix := make([]byte, W*H*nc)
	for i := 0; i < W*H; i++ {
		r, g, b := xybToLinearRGB(planeX[i], planeY[i], planeB[i])
		base := i * nc
		pix[base] = clamp8(linearToSRGB(r))
		if !gray {
			pix[base+1] = clamp8(linearToSRGB(g))
			pix[base+2] = clamp8(linearToSRGB(b))
		}
		for ec := range st.extra {
			v := st.extra[ec][i]
			if v < 0 {
				v = 0
			} else if v > 255 {
				v = 255
			}
			pix[base+colorCh+ec] = byte(v)
		}
	}
	return &Image{W: W, H: H, Channels: nc, BitDepth: 8, Pixels: pix}, nil
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
