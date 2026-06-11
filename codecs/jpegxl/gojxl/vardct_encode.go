package gojxl

import (
	"errors"
	"math"
)

// Pure-Go lossy VarDCT JPEG XL encoder. It is the inverse of the VarDCT decoder
// for a minimal but conformant subset: XYB, DCT8x8 for every block, a uniform
// quantizer (no adaptive quant), the default chroma-from-luma (B stored as B-Y),
// loop filters disabled, single group / single pass / single histogram, default
// dequant matrices. The emitted codestream is decodable byte-exact by libjxl
// (djxl) and by this package's decoder.

// kEncQuantDC / kEncQuantField are the fixed DC quant and per-block raw quant
// field used by the uniform quantizer; quality is controlled by globalScale.
const (
	kEncQuantDC    = 64
	kEncQuantField = 1
)

// EncodeVarDCT compresses interleaved 8-bit sRGB samples (grayscale or RGB) into
// a lossy VarDCT JPEG XL codestream. globalScale sets the quality: larger means
// finer quantization (higher quality, larger output). A good default is ~4096.
func EncodeVarDCT(pixels []byte, w, h, channels, globalScale int) ([]byte, error) {
	if w <= 0 || h <= 0 || w > 1<<14 || h > 1<<14 {
		return nil, errors.New("gojxl: VarDCT encode supports 1..16384 px per dimension")
	}
	if channels != 1 && channels != 3 {
		return nil, errors.New("gojxl: VarDCT encode supports 1 or 3 channels")
	}
	if len(pixels) != w*h*channels {
		return nil, errors.New("gojxl: pixel buffer size mismatch")
	}
	if globalScale < 1 || globalScale > 1<<20 {
		return nil, errors.New("gojxl: globalScale out of range")
	}
	gray := channels == 1

	// ----- sRGB -> linear -> XYB planes (full resolution) -----
	planeX := make([]float32, w*h)
	planeY := make([]float32, w*h)
	planeB := make([]float32, w*h)
	for i := 0; i < w*h; i++ {
		var r, g, b float32
		if gray {
			v := srgbToLinear(float32(pixels[i]) / 255)
			r, g, b = v, v, v
		} else {
			r = srgbToLinear(float32(pixels[i*3+0]) / 255)
			g = srgbToLinear(float32(pixels[i*3+1]) / 255)
			b = srgbToLinear(float32(pixels[i*3+2]) / 255)
		}
		planeX[i], planeY[i], planeB[i] = linearRGBToXYB(r, g, b)
	}
	planes := [3][]float32{planeX, planeY, planeB}

	// ----- forward DCT8x8 + quantization -----
	bw := (w + 7) / 8
	bh := (h + 7) / 8
	q := newQuantizer(globalScale, kEncQuantDC)
	mulDC := [3]float32{q.invQuantDC * kDCQuant[0], q.invQuantDC * kDCQuant[1], q.invQuantDC * kDCQuant[2]}
	xDM := float32(math.Pow(1.0/1.25, 3.0-2.0)) // x_qm_scale default 3
	bDM := float32(math.Pow(1.0/1.25, 2.0-2.0)) // b_qm_scale default 2
	dm := [3]float32{xDM, 1.0, bDM}             // applied to X, Y, B (Y unscaled)
	sd := q.invGlobalScale / float32(kEncQuantField)

	// DCT8x8 dequant weights (1/invQuantTable) for the 3 channels.
	inv, ok := computeInvQuantTable(qtDCT, buildDefaultQuantLibrary()[qtDCT])
	if !ok {
		return nil, errors.New("gojxl: failed to build DCT8x8 quant table")
	}
	mat := make([]float32, len(inv))
	for i, v := range inv {
		if v != 0 {
			mat[i] = 1.0 / v
		}
	}
	const bridge = 8.0 // sqrt(64)
	order := naturalCoeffOrder(1, 1)

	// Chroma-from-luma factors (default: X stored directly, B stored as B - Y).
	cfl := defaultColorCorrelation()
	xcc := cfl.ytoXRatio(0)
	bcc := cfl.ytoBRatio(0)

	// dcInt[pc] is the quantized DC image (bw*bh) per XYB plane index (0=X,1=Y,2=B).
	dcInt := [3][]int32{make([]int32, bw*bh), make([]int32, bw*bh), make([]int32, bw*bh)}
	// acCoeff[pc][block] = 64 quantized coefficients (slot 0 = DC, left 0).
	acCoeff := [3][][]int32{
		make([][]int32, bw*bh), make([][]int32, bw*bh), make([][]int32, bw*bh),
	}

	// blkTarget extracts an 8x8 block of plane pc and returns the target
	// dequantized coefficient block in libjxl storage layout (transposed,
	// divided by the normalization bridge).
	blkTarget := func(pc, bx, by int) [64]float32 {
		p := planes[pc]
		var block [64]float32
		for yy := 0; yy < 8; yy++ {
			sy := by*8 + yy
			if sy >= h {
				sy = h - 1
			}
			for xx := 0; xx < 8; xx++ {
				sx := bx*8 + xx
				if sx >= w {
					sx = w - 1
				}
				block[yy*8+xx] = p[sy*w+sx]
			}
		}
		coef := dct2d(block[:], 8, 8)
		var blk [64]float32
		blk[0] = coef[0] / bridge
		for k := 1; k < 64; k++ {
			r, c := k/8, k%8 // storage idx c*8+r is the transpose of row-major k
			blk[k] = coef[c*8+r] / bridge
		}
		return blk
	}

	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			bi := by*bw + bx
			tY := blkTarget(1, bx, by)
			tX := blkTarget(0, bx, by)
			tB := blkTarget(2, bx, by)

			// Y: quantized directly; keep the reconstructed (dequantized) Y for CfL.
			dcY := int32(math.Round(float64(tY[0] / mulDC[1])))
			dcInt[1][bi] = dcY
			inYdc := float32(dcY) * mulDC[1]
			cY := make([]int32, 64)
			yDeq := make([]float32, 64)
			for k := 1; k < 64; k++ {
				step := mat[1*64+k] * sd
				cY[k] = int32(math.Round(float64(tY[k] / step)))
				yDeq[k] = adjustQuantBias(1, cY[k], &kDefaultQuantBias) * step
			}
			acCoeff[1][bi] = cY

			// X and B: subtract the chroma-from-luma contribution before quantizing.
			dcInt[0][bi] = int32(math.Round(float64((tX[0] - xcc*inYdc) / mulDC[0])))
			dcInt[2][bi] = int32(math.Round(float64((tB[0] - bcc*inYdc) / mulDC[2])))
			cX := make([]int32, 64)
			cB := make([]int32, 64)
			for k := 1; k < 64; k++ {
				cX[k] = int32(math.Round(float64((tX[k] - xcc*yDeq[k]) / (mat[0*64+k] * sd * dm[0]))))
				cB[k] = int32(math.Round(float64((tB[k] - bcc*yDeq[k]) / (mat[2*64+k] * sd * dm[2]))))
			}
			acCoeff[0][bi] = cX
			acCoeff[2][bi] = cB
		}
	}
	// Modular DC channel order is Y, X, B.
	dcMod := [3][]int32{dcInt[1], dcInt[0], dcInt[2]}

	return assembleVarDCT(w, h, bw, bh, gray, globalScale, dcMod, acCoeff, order)
}

// assembleVarDCT writes the full VarDCT codestream. Images larger than one
// group (256px) are split into the decoder's section layout (LfGlobal, one DC
// group, HfGlobal, then one section per AC group).
func assembleVarDCT(w, h, bw, bh int, gray bool, globalScale int,
	dcMod [3][]int32, acCoeff [3][][]int32, order []int) ([]byte, error) {

	fd := computeFrameDimensions(uint32(w), uint32(h), 1, 1) // group_size_shift = 1
	numGroups := int(fd.numGroups)

	// ----- modular global sub-streams: DC image + AC metadata -----
	var dcTokens []encToken
	for c := 0; c < 3; c++ {
		dcTokens = gradientTokens(dcMod[c], bw, 0, 0, bw, bh, dcTokens)
	}
	ctW, ctH := (bw+7)>>3, (bh+7)>>3
	count := bw * bh
	acsqf := make([]int32, count*2) // row0 ACS=0 (DCT8x8), row1 QF=0
	var acmTokens []encToken
	acmTokens = gradientTokens(make([]int32, ctW*ctH), ctW, 0, 0, ctW, ctH, acmTokens) // ytox = 0
	acmTokens = gradientTokens(make([]int32, ctW*ctH), ctW, 0, 0, ctW, ctH, acmTokens) // ytob = 0
	acmTokens = gradientTokens(acsqf, count, 0, 0, count, 2, acmTokens)
	acmTokens = gradientTokens(make([]int32, bw*bh), bw, 0, 0, bw, bh, acmTokens) // EPF = 0
	dcTk, dcMax := tokenizeAll(dcTokens)
	acmTk, acmMax := tokenizeAll(acmTokens)
	modAlphabet := maxInt(dcMax, acmMax) + 1

	// ----- AC coefficients, tokenized per group -----
	perGroupAC, acMax := acTokensByGroup(fd, bw, bh, acCoeff, order)
	acAlphabet := acMax + 1
	acCtxCount := defaultBlockCtxMap().numACContexts()

	single := numGroups == 1

	// LfGlobal: global DC info + modular tree + shared modular histogram.
	lf := newBitWriter()
	lf.WriteBool(true) // DequantMatrices DC all_default
	lf.WriteU32(uint32(globalScale), qGlobalScaleDist[0], qGlobalScaleDist[1], qGlobalScaleDist[2], qGlobalScaleDist[3])
	lf.WriteU32(uint32(kEncQuantDC), qQuantDCDist[0], qQuantDCDist[1], qQuantDCDist[2], qQuantDCDist[3])
	lf.WriteBits(1, 1) // block context map: all_default
	lf.WriteBits(1, 1) // CfL: all_default
	lf.WriteBits(1, 1) // has_tree
	encodeANSStream(lf, maTreeTokens, numTreeContexts)
	modRev, modFreq := writeANSFlatHeader(lf, modAlphabet, 1)

	// DC group: VarDCT DC image + AC metadata (separate ANS streams).
	dcw := lf
	if !single {
		dcw = newBitWriter()
	}
	dcw.WriteBits(0, 2) // extra_precision = 0
	writeModularGroupHeader(dcw)
	encodeANSData(dcw, ansEncState{tokens: dcTk, revMap: modRev, freqs: modFreq})
	dcw.WriteBits(uint64(count-1), ceilLog2Nonzero(uint32(bw*bh)))
	writeModularGroupHeader(dcw)
	encodeANSData(dcw, ansEncState{tokens: acmTk, revMap: modRev, freqs: modFreq})

	// HfGlobal: dequant matrices + coeff orders + shared AC histogram.
	hf := lf
	if !single {
		hf = newBitWriter()
	}
	hf.WriteBits(1, 1)                                                     // AC dequant matrices all_default
	hf.WriteBits(0, ceilLog2Nonzero(fd.numGroups))                         // num_histograms - 1 = 0
	hf.WriteU32(0, kOrderEnc[0], kOrderEnc[1], kOrderEnc[2], kOrderEnc[3]) // used_orders = 0 (natural)
	acRev, acFreq := writeANSFlatHeader(hf, acAlphabet, acCtxCount)

	// AC groups.
	if single {
		encodeANSData(lf, ansEncState{tokens: perGroupAC[0], revMap: acRev, freqs: acFreq})
		return finishVarDCT(w, h, gray, [][]byte{lf.Bytes()}, fd)
	}
	sections := make([][]byte, 0, 3+numGroups)
	sections = append(sections, lf.Bytes(), dcw.Bytes(), hf.Bytes())
	for g := 0; g < numGroups; g++ {
		gw := newBitWriter()
		encodeANSData(gw, ansEncState{tokens: perGroupAC[g], revMap: acRev, freqs: acFreq})
		sections = append(sections, gw.Bytes())
	}
	return finishVarDCT(w, h, gray, sections, fd)
}

// finishVarDCT writes the container headers + TOC and assembles the codestream.
// For a single section the TOC has one entry; for multiple sections the order is
// LfGlobal, numDCGroups (=1) DC group, HfGlobal, then the AC groups.
func finishVarDCT(w, h int, gray bool, sections [][]byte, fd frameDimensions) ([]byte, error) {
	m := newBitWriter()
	writeSizeHeader(m, w, h)
	writeVarDCTImageMetadata(m, gray)
	m.WriteBool(true) // transform_data all_default
	m.ZeroPadToByte()
	writeVarDCTFrameHeader(m)
	m.WriteBits(0, 1) // TOC: no permutation
	m.ZeroPadToByte()
	for _, sec := range sections {
		writeTocSize(m, len(sec))
	}
	m.ZeroPadToByte()
	return assemble(m.Bytes(), sections), nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// acTokensByGroup tokenizes the AC coefficients per AC group (raster blocks
// within each group's rect, channels Y,X,B, nzeros then coefficients), matching
// decodeACGroup, and reports the maximum token over all groups.
func acTokensByGroup(fd frameDimensions, bw, bh int, acCoeff [3][][]int32, order []int) ([][]encTokenized, int) {
	gdb := int(fd.groupDim) / 8
	xg := int(fd.xsizeGroups)
	out := make([][]encTokenized, fd.numGroups)
	maxTok := 0
	for g := 0; g < int(fd.numGroups); g++ {
		bx0 := (g % xg) * gdb
		by0 := (g / xg) * gdb
		bx1 := minInt(bx0+gdb, bw)
		by1 := minInt(by0+gdb, bh)
		var toks []encToken
		for by := by0; by < by1; by++ {
			for bx := bx0; bx < bx1; bx++ {
				bi := by*bw + bx
				for _, c := range [3]int{1, 0, 2} {
					toks = appendACBlockTokens(toks, acCoeff[c][bi], order)
				}
			}
		}
		tks, mx := tokenizeAll(toks)
		out[g] = tks
		if mx > maxTok {
			maxTok = mx
		}
	}
	return out, maxTok
}

// appendACBlockTokens emits one block's AC tokens: the non-zero count, then the
// coefficients in natural order up to and including the last non-zero.
func appendACBlockTokens(toks []encToken, blk []int32, order []int) []encToken {
	nz, lastK := 0, 0
	for k := 1; k < 64; k++ {
		if blk[order[k]] != 0 {
			nz++
			lastK = k
		}
	}
	toks = append(toks, encToken{value: uint32(nz)})
	rem := nz
	for k := 1; k <= lastK && rem != 0; k++ {
		v := blk[order[k]]
		toks = append(toks, encToken{value: packSigned(v)})
		if v != 0 {
			rem--
		}
	}
	return toks
}

// writeModularGroupHeader writes a use-global-tree, default-WP, no-transform
// modular group header.
func writeModularGroupHeader(s *bitWriter) {
	s.WriteBool(true) // use_global_tree
	s.WriteBool(true) // wp_header all_default
	s.WriteU32(0, u32Val(0), u32Val(1), u32Off(4, 2), u32Off(8, 18))
}

// writeModularGroupHeaderLocal writes a modular group header that carries its own
// local tree and histograms (use_global_tree = false), default WP, no transform.
// The local tree + data histograms must be written immediately after (matching
// groupTree's decode order).
func writeModularGroupHeaderLocal(s *bitWriter) {
	s.WriteBool(false) // use_global_tree
	s.WriteBool(true)  // wp_header all_default
	s.WriteU32(0, u32Val(0), u32Val(1), u32Off(4, 2), u32Off(8, 18))
}

// writeVarDCTImageMetadata writes ImageMetadata for an XYB VarDCT image.
func writeVarDCTImageMetadata(w *bitWriter, gray bool) {
	w.WriteBool(false)                                             // all_default
	w.WriteBool(false)                                             // extra_fields
	w.WriteBool(false)                                             // floating_point_sample
	w.WriteU32(8, u32Val(8), u32Val(10), u32Val(12), u32Off(6, 1)) // bits_per_sample = 8
	w.WriteBool(true)                                              // modular_16bit_buffer_sufficient
	w.WriteU32(0, u32Val(0), u32Val(1), u32Off(4, 2), u32Off(12, 1))
	w.WriteBool(true) // xyb_encoded = true
	writeColorEncoding(w, gray)
	w.WriteU64(0) // extensions
}

// writeVarDCTFrameHeader writes a regular single-frame VarDCT frame header with
// loop filters disabled (Gaborish off, EPF off) and default qm scales.
func writeVarDCTFrameHeader(w *bitWriter) {
	w.WriteBool(false)                                        // all_default
	w.WriteU32(0, u32Val(0), u32Val(1), u32Val(2), u32Val(3)) // frame_type = kRegularFrame
	w.WriteBool(false)                                        // is_modular = false (VarDCT)
	w.WriteU64(0)                                             // flags
	// color_transform: xyb_encoded is true, so kXYB is implied (no alternate bit).
	w.WriteU32(1, u32Val(1), u32Val(2), u32Val(4), u32Val(8)) // upsampling = 1
	// VarDCT + XYB: x_qm_scale (3), b_qm_scale (2).
	w.WriteBits(3, 3)
	w.WriteBits(2, 3)
	w.WriteU32(1, u32Val(1), u32Val(2), u32Val(3), u32Off(3, 4)) // num_passes = 1
	w.WriteBool(false)                                           // custom_size_or_origin
	w.WriteU32(0, u32Val(0), u32Val(1), u32Val(2), u32Off(2, 3)) // BlendMode kReplace
	w.WriteBool(true)                                            // is_last
	w.WriteU32(0, u32Val(0), u32Bits(4), u32Off(5, 16), u32Off(10, 48))
	// loop_filter: explicitly disabled (default would be Gaborish on, EPF=2).
	w.WriteBool(false) // loop_filter all_default = false
	w.WriteBool(false) // gab (Gaborish) = false
	w.WriteBits(0, 2)  // epf_iters = 0
	w.WriteU64(0)      // loop_filter extensions
	w.WriteU64(0)      // frame extensions
}
