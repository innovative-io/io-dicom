package gojxl

import "errors"

// Pure-Go lossless Modular JPEG XL encoder. Scope: single frame, single group
// (images up to 1024x1024), grayscale or RGB, 8- or 16-bit, no transforms, a
// single Gradient-predictor MA tree, flat ANS histograms. It is the inverse of
// decodeModularFrame and is validated by round-tripping through the decoder.

func packSigned(v int32) uint32 { return (uint32(v) << 1) ^ uint32(v>>31) }

// Encode compresses interleaved samples (matching Decode's output layout: row-
// major, channel-interleaved, little-endian for >8-bit) into a lossless Modular
// JPEG XL codestream.
func Encode(pixels []byte, w, h, channels, bitdepth int) ([]byte, error) {
	if w <= 0 || h <= 0 || w > 1024 || h > 1024 {
		return nil, errors.New("gojxl: encode supports 1..1024 px per dimension")
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

	// Tokenize all channels (decode order: channel 0 fully, then channel 1...),
	// using the Gradient predictor with the decoder's border rules.
	var chanTokens []encToken
	for c := 0; c < channels; c++ {
		p := planes[c]
		get := func(x, y int) int64 { return int64(p[y*w+x]) }
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				var left, top, topleft int64
				if x > 0 {
					left = get(x-1, y)
				} else if y > 0 {
					left = get(x, y-1)
				}
				top = left
				if y > 0 {
					top = get(x, y-1)
				}
				topleft = left
				if x > 0 && y > 0 {
					topleft = get(x-1, y-1)
				}
				guess := clampedGradient(left, top, topleft)
				residual := int32(get(x, y) - guess)
				chanTokens = append(chanTokens, encToken{ctx: 0, value: packSigned(residual)})
			}
		}
	}

	// ----- frame data section (LfGlobal) -----
	fd := newBitWriter()
	fd.WriteBool(true) // DequantMatrices DC all_default
	fd.WriteBits(1, 1) // has_tree
	// MA tree: a single Gradient leaf.
	treeTokens := []encToken{
		{ctxProperty, 0}, // property+1 = 0 → leaf
		{ctxPredictor, predGradient},
		{ctxOffset, 0},
		{ctxMultiplierLog, 0},
		{ctxMultiplierBit, 0},
	}
	encodeANSStream(fd, treeTokens, numTreeContexts)
	// Channel entropy header (before the group header), data (after it).
	chState := encodeANSHeader(fd, chanTokens, 1)
	// Group header: use global tree, default WP, no transforms.
	fd.WriteBool(true) // use_global_tree
	fd.WriteBool(true) // wp_header all_default
	fd.WriteU32(0, u32Val(0), u32Val(1), u32Off(4, 2), u32Off(8, 18))
	encodeANSData(fd, chState)
	fdBytes := fd.Bytes()

	// ----- main stream -----
	m := newBitWriter()
	writeSizeHeader(m, w, h)
	writeImageMetadata(m, channels, bitdepth)
	m.WriteBool(true) // transform_data all_default
	m.ZeroPadToByte()
	gss := groupSizeShiftFor(w, h)
	writeFrameHeader(m, gss)
	// TOC (single section).
	m.WriteBits(0, 1) // no permutation
	m.ZeroPadToByte()
	m.WriteU32(uint32(len(fdBytes)), u32Bits(10), u32Off(14, 1024), u32Off(22, 17408), u32Off(30, 4211712))
	m.ZeroPadToByte()
	mainBytes := m.Bytes()

	out := make([]byte, 0, 2+len(mainBytes)+len(fdBytes))
	out = append(out, 0xFF, 0x0A)
	out = append(out, mainBytes...)
	out = append(out, fdBytes...)
	return out, nil
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
