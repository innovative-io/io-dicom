package gojxl

import "errors"

// Pure-Go lossless Modular JPEG XL encoder. Scope: single frame, grayscale or
// RGB, 8- or 16-bit, no transforms, a single Gradient-predictor MA tree, flat
// ANS histograms. Images larger than one group are split into byte-aligned
// group sections sharing one global histogram; each group is Gradient-predicted
// independently (matching the decoder). It is the inverse of decodeModularFrame,
// validated by round-tripping through the decoder and byte-exact against djxl.

func packSigned(v int32) uint32 { return (uint32(v) << 1) ^ uint32(v>>31) }

// maTreeTokens is the encoded MA tree: a single Gradient-predictor leaf.
var maTreeTokens = []encToken{
	{ctxProperty, 0}, // property+1 = 0 → leaf
	{ctxPredictor, predGradient},
	{ctxOffset, 0},
	{ctxMultiplierLog, 0},
	{ctxMultiplierBit, 0},
}

// gradientTokens Gradient-predicts the [x0,x0+rw) x [y0,y0+rh) rect of plane
// (full-width w) as an independent sub-image — borders are at the rect edges,
// matching how the decoder predicts each modular group — and emits one residual
// token per pixel in raster order.
func gradientTokens(plane []int32, w, x0, y0, rw, rh int, dst []encToken) []encToken {
	get := func(lx, ly int) int64 { return int64(plane[(y0+ly)*w+(x0+lx)]) }
	for ly := 0; ly < rh; ly++ {
		for lx := 0; lx < rw; lx++ {
			var left, top, topleft int64
			if lx > 0 {
				left = get(lx-1, ly)
			} else if ly > 0 {
				left = get(lx, ly-1)
			}
			top = left
			if ly > 0 {
				top = get(lx, ly-1)
			}
			topleft = left
			if lx > 0 && ly > 0 {
				topleft = get(lx-1, ly-1)
			}
			guess := clampedGradient(left, top, topleft)
			residual := int32(get(lx, ly) - guess)
			dst = append(dst, encToken{ctx: 0, value: packSigned(residual)})
		}
	}
	return dst
}

// Encode compresses interleaved samples (matching Decode's output layout: row-
// major, channel-interleaved, little-endian for >8-bit) into a lossless Modular
// JPEG XL codestream. Images larger than one group are split into groups.
func Encode(pixels []byte, w, h, channels, bitdepth int) ([]byte, error) {
	if w <= 0 || h <= 0 || w > 1<<16 || h > 1<<16 {
		return nil, errors.New("gojxl: encode supports 1..65536 px per dimension")
	}
	if channels != 1 && channels != 3 {
		return nil, errors.New("gojxl: encode supports 1 or 3 channels")
	}
	if bitdepth < 1 || bitdepth > 16 {
		return nil, errors.New("gojxl: encode supports 1..16 bit depth")
	}
	bps := 1
	if bitdepth > 8 {
		bps = 2
	}
	if len(pixels) != w*h*channels*bps {
		return nil, errors.New("gojxl: pixel buffer size mismatch")
	}

	// Deinterleave into channel planes.
	planes := make([][]int32, channels)
	for c := range planes {
		planes[c] = make([]int32, w*h)
	}
	for i := 0; i < w*h; i++ {
		for c := 0; c < channels; c++ {
			idx := (i*channels + c) * bps
			v := uint32(pixels[idx])
			if bps == 2 {
				v |= uint32(pixels[idx+1]) << 8
			}
			planes[c][i] = int32(v)
		}
	}

	gss := groupSizeShiftFor(w, h)
	fd := computeFrameDimensions(uint32(w), uint32(h), gss, 1)
	if fd.numGroups == 1 {
		return encodeSingleGroup(planes, w, h, channels, bitdepth, gss)
	}
	return encodeMultiGroup(planes, w, h, channels, bitdepth, gss, fd)
}

// encodeSingleGroup writes the whole frame as one section (LfGlobal contains the
// tree, histograms and all channel data).
func encodeSingleGroup(planes [][]int32, w, h, channels, bitdepth int, gss uint32) ([]byte, error) {
	var chanTokens []encToken
	for c := 0; c < channels; c++ {
		chanTokens = gradientTokens(planes[c], w, 0, 0, w, h, chanTokens)
	}

	lf := newBitWriter()
	lf.WriteBool(true) // DequantMatrices DC all_default
	lf.WriteBits(1, 1) // has_tree
	encodeANSStream(lf, maTreeTokens, numTreeContexts)
	chState := encodeANSHeader(lf, chanTokens, 1)
	lf.WriteBool(true) // use_global_tree
	lf.WriteBool(true) // wp_header all_default
	lf.WriteU32(0, u32Val(0), u32Val(1), u32Off(4, 2), u32Off(8, 18))
	encodeANSData(lf, chState)
	lfBytes := lf.Bytes()

	m := newBitWriter()
	writeContainerHeaders(m, w, h, channels, bitdepth, gss)
	m.WriteBits(0, 1) // TOC: no permutation
	m.ZeroPadToByte()
	writeTocSize(m, len(lfBytes))
	m.ZeroPadToByte()

	return assemble(m.Bytes(), [][]byte{lfBytes}), nil
}

// encodeMultiGroup writes the LfGlobal section (tree + global histogram + group
// header, no pixel data) followed by one byte-aligned section per group, each
// Gradient-predicted independently and rANS-coded with the shared histogram.
func encodeMultiGroup(planes [][]int32, w, h, channels, bitdepth int, gss uint32, fd frameDimensions) ([]byte, error) {
	groupDim := int(fd.groupDim)
	numGroups := int(fd.numGroups)
	xg := int(fd.xsizeGroups)

	// Per-group token lists, in decode order (channel 0..N within the group's
	// rect), and the global maximum token for the shared flat histogram.
	perGroup := make([][]encTokenized, numGroups)
	maxToken := 0
	for g := 0; g < numGroups; g++ {
		x0 := (g % xg) * groupDim
		y0 := (g / xg) * groupDim
		rw := minInt(groupDim, w-x0)
		rh := minInt(groupDim, h-y0)
		var toks []encToken
		for c := 0; c < channels; c++ {
			toks = gradientTokens(planes[c], w, x0, y0, rw, rh, toks)
		}
		tks, mx := tokenizeAll(toks)
		perGroup[g] = tks
		if mx > maxToken {
			maxToken = mx
		}
	}

	// ----- LfGlobal section: tree + shared histogram header + group header -----
	lf := newBitWriter()
	lf.WriteBool(true) // DequantMatrices DC all_default
	lf.WriteBits(1, 1) // has_tree
	encodeANSStream(lf, maTreeTokens, numTreeContexts)
	revMap, freqs := writeANSFlatHeader(lf, maxToken+1, 1)
	lf.WriteBool(true) // use_global_tree
	lf.WriteBool(true) // wp_header all_default
	lf.WriteU32(0, u32Val(0), u32Val(1), u32Off(4, 2), u32Off(8, 18))
	lfBytes := lf.Bytes()

	// ----- per-group sections: group header + rANS data (shared histogram) -----
	groupBytes := make([][]byte, numGroups)
	for g := 0; g < numGroups; g++ {
		gw := newBitWriter()
		gw.WriteBool(true) // use_global_tree
		gw.WriteBool(true) // wp_header all_default
		gw.WriteU32(0, u32Val(0), u32Val(1), u32Off(4, 2), u32Off(8, 18))
		encodeANSData(gw, ansEncState{tokens: perGroup[g], revMap: revMap, freqs: freqs})
		groupBytes[g] = gw.Bytes()
	}

	// ----- main stream: container headers + multi-section TOC -----
	m := newBitWriter()
	writeContainerHeaders(m, w, h, channels, bitdepth, gss)
	m.WriteBits(0, 1) // TOC: no permutation
	m.ZeroPadToByte()
	// TOC order: LfGlobal, then numDCGroups+1 empty DC/HfGlobal sections, then the
	// modular groups (toc.cc / acGroupIndex = 2 + numDCGroups + g).
	writeTocSize(m, len(lfBytes))
	for i := 0; i < int(fd.numDCGroups)+1; i++ {
		writeTocSize(m, 0)
	}
	sections := make([][]byte, 0, 1+numGroups)
	sections = append(sections, lfBytes)
	for g := 0; g < numGroups; g++ {
		writeTocSize(m, len(groupBytes[g]))
		sections = append(sections, groupBytes[g])
	}
	m.ZeroPadToByte()

	return assemble(m.Bytes(), sections), nil
}

// writeContainerHeaders writes the size header, image metadata, transform data
// and frame header (everything before the TOC).
func writeContainerHeaders(m *bitWriter, w, h, channels, bitdepth int, gss uint32) {
	writeSizeHeader(m, w, h)
	writeImageMetadata(m, channels, bitdepth)
	m.WriteBool(true) // transform_data all_default
	m.ZeroPadToByte()
	writeFrameHeader(m, gss)
}

// writeTocSize writes one TOC entry size with the toc.cc U32 distribution.
func writeTocSize(m *bitWriter, n int) {
	m.WriteU32(uint32(n), u32Bits(10), u32Off(14, 1024), u32Off(22, 17408), u32Off(30, 4211712))
}

// assemble concatenates the codestream signature, main stream and the section
// bytes (in section-index order).
func assemble(mainBytes []byte, sections [][]byte) []byte {
	total := 2 + len(mainBytes)
	for _, s := range sections {
		total += len(s)
	}
	out := make([]byte, 0, total)
	out = append(out, 0xFF, 0x0A)
	out = append(out, mainBytes...)
	for _, s := range sections {
		out = append(out, s...)
	}
	return out
}

func groupSizeShiftFor(w, h int) uint32 {
	d := w
	if h > d {
		d = h
	}
	for gss := uint32(0); gss <= 3; gss++ {
		if (128 << gss) >= d {
			return gss
		}
	}
	return 3
}

func writeSizeHeader(w *bitWriter, xs, ys int) {
	off := []u32d{u32Off(9, 1), u32Off(13, 1), u32Off(18, 1), u32Off(30, 1)}
	w.WriteBool(false) // small
	w.WriteU32(uint32(ys), off[0], off[1], off[2], off[3])
	w.WriteBits(0, 3) // ratio = 0
	w.WriteU32(uint32(xs), off[0], off[1], off[2], off[3])
}

func writeImageMetadata(w *bitWriter, channels, bitdepth int) {
	w.WriteBool(false) // all_default
	w.WriteBool(false) // extra_fields (orientation 1, no preview/animation)
	// BitDepth: integer.
	w.WriteBool(false) // floating_point_sample
	w.WriteU32(uint32(bitdepth), u32Val(8), u32Val(10), u32Val(12), u32Off(6, 1))
	w.WriteBool(true)                                                // modular_16bit_buffer_sufficient
	w.WriteU32(0, u32Val(0), u32Val(1), u32Off(4, 2), u32Off(12, 1)) // num_extra_channels = 0
	w.WriteBool(false)                                               // xyb_encoded
	writeColorEncoding(w, channels == 1)
	// extra_fields is false → no tone_mapping.
	w.WriteU64(0) // extensions
}

func writeColorEncoding(w *bitWriter, gray bool) {
	w.WriteBool(false) // all_default
	w.WriteBool(false) // want_icc
	if gray {
		w.WriteEnum(csGray)
	} else {
		w.WriteEnum(csRGB)
	}
	// !want_icc:
	w.WriteEnum(1) // white_point = kD65 (not implicit since not XYB)
	if !gray {
		w.WriteEnum(1) // primaries = kSRGB
	}
	// transfer function (not XYB):
	w.WriteBool(false) // have_gamma
	w.WriteEnum(13)    // transfer_function = kSRGB
	w.WriteEnum(1)     // rendering_intent = kRelative
}

func writeFrameHeader(w *bitWriter, gss uint32) {
	w.WriteBool(false)                                        // all_default
	w.WriteU32(0, u32Val(0), u32Val(1), u32Val(2), u32Val(3)) // frame_type = kRegularFrame
	w.WriteBool(true)                                         // is_modular
	w.WriteU64(0)                                             // flags
	w.WriteBool(false)                                        // color_transform alternate (kNone; xyb_encoded is false)
	w.WriteU32(1, u32Val(1), u32Val(2), u32Val(4), u32Val(8)) // upsampling = 1
	w.WriteBits(uint64(gss), 2)                               // group_size_shift (modular)
	// passes:
	w.WriteU32(1, u32Val(1), u32Val(2), u32Val(3), u32Off(3, 4)) // num_passes = 1
	w.WriteBool(false)                                           // custom_size_or_origin
	// blending (regular frame):
	w.WriteU32(0, u32Val(0), u32Val(1), u32Val(2), u32Off(2, 3)) // BlendMode kReplace
	w.WriteBool(true)                                            // is_last
	// name:
	w.WriteU32(0, u32Val(0), u32Bits(4), u32Off(5, 16), u32Off(10, 48)) // empty name
	// loop_filter: must be explicitly disabled — the default (Gaborish on, EPF=2)
	// would be applied by a conforming decoder and is not lossless.
	w.WriteBool(false) // loop_filter all_default = false
	w.WriteBool(false) // gab (Gaborish) = false
	w.WriteBits(0, 2)  // epf_iters = 0
	w.WriteU64(0)      // loop_filter extensions
	w.WriteU64(0)      // frame extensions
}
