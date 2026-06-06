package gojxl

import "errors"

// Frame header + TOC parsing (libjxl frame_header.cc, toc.cc, frame_dimensions.h).

// Frame encodings.
const (
	frameVarDCT  = 0
	frameModular = 1
)

// Frame types.
const (
	frameRegular       = 0
	frameDCFrame       = 1
	frameReferenceOnly = 2
	frameSkipProgress  = 3
)

const flagUseDCFrame = 32

// FrameHeader holds the parsed fields we need for decoding.
type FrameHeader struct {
	Type           uint32
	Encoding       uint32 // frameModular / frameVarDCT
	Flags          uint64
	Upsampling     uint32
	GroupSizeShift uint32
	NumPasses      uint32
	PassShift      [kMaxNumPasses]uint32
	IsLast         bool
	CustomSize     bool
	LoopFilter     loopFilter
	XQMScale       uint32
	BQMScale       uint32
	FrameW, FrameH uint32
	OriginX        int32
	OriginY        int32
}

const kMaxNumPasses = 11

// readPasses (frame_header.cc Passes::VisitFields).
func readPasses(b *bitReader, h *FrameHeader) error {
	h.NumPasses = b.ReadU32(u32Val(1), u32Val(2), u32Val(3), u32Off(3, 4))
	if h.NumPasses > kMaxNumPasses {
		return errors.New("gojxl: too many passes")
	}
	if h.NumPasses != 1 {
		numDownsample := b.ReadU32(u32Val(0), u32Val(1), u32Val(2), u32Off(1, 3))
		for i := uint32(0); i < h.NumPasses-1; i++ {
			h.PassShift[i] = b.ReadBits(2)
		}
		for i := uint32(0); i < numDownsample; i++ {
			b.ReadU32(u32Val(1), u32Val(2), u32Val(4), u32Val(8)) // downsample[i]
		}
		for i := uint32(0); i < numDownsample; i++ {
			b.ReadU32(u32Val(0), u32Val(1), u32Val(2), u32Bits(3)) // last_pass[i]
		}
	}
	return nil
}

// readBlendingInfo consumes a BlendingInfo (frame_header.cc).
func readBlendingInfo(b *bitReader, numExtra int, isPartial bool) {
	mode := b.ReadU32(u32Val(0), u32Val(1), u32Val(2), u32Off(2, 3)) // VisitBlendMode
	// BlendMode: 0=Replace,1=Add,2=Blend,3=MulAdd? (kBlend=2,kAlphaWeightedAdd=3,kMul=4)
	const kBlend, kAlphaWeightedAdd, kMul, kReplace = 2, 3, 4, 0
	if numExtra > 0 && (mode == kBlend || mode == kAlphaWeightedAdd) {
		alpha := b.ReadU32(u32Val(0), u32Val(1), u32Val(2), u32Off(3, 3))
		_ = alpha
	}
	if (numExtra > 0 && (mode == kBlend || mode == kAlphaWeightedAdd)) || mode == kMul {
		b.ReadBool() // clamp
	}
	if mode != kReplace || isPartial {
		b.ReadU32(u32Val(0), u32Val(1), u32Val(2), u32Val(3)) // source
	}
}

// readLoopFilter consumes a LoopFilter (loop_filter.cc).
// loopFilter holds the parsed loop-filter parameters needed for reconstruction.
type loopFilter struct {
	gab                                            bool
	gabXW1, gabXW2, gabYW1, gabYW2, gabBW1, gabBW2 float32
	epfIters                                       uint32
}

func defaultLoopFilter() loopFilter {
	w1 := float32(1.1 * 0.104699568)
	w2 := float32(1.1 * 0.055680538)
	return loopFilter{gab: true, gabXW1: w1, gabXW2: w2, gabYW1: w1, gabYW2: w2, gabBW1: w1, gabBW2: w2, epfIters: 2}
}

func readLoopFilter(b *bitReader, isModular bool) loopFilter {
	lf := defaultLoopFilter()
	if b.ReadBool() { // all_default
		return lf
	}
	gab := b.ReadBool() // default true
	lf.gab = gab
	if gab {
		gabCustom := b.ReadBool()
		if gabCustom {
			lf.gabXW1, _ = b.ReadF16()
			lf.gabXW2, _ = b.ReadF16()
			lf.gabYW1, _ = b.ReadF16()
			lf.gabYW2, _ = b.ReadF16()
			lf.gabBW1, _ = b.ReadF16()
			lf.gabBW2, _ = b.ReadF16()
		}
	}
	epfIters := b.ReadBits(2) // default 2
	lf.epfIters = epfIters
	if epfIters > 0 {
		if !isModular {
			if b.ReadBool() { // epf_sharp_custom
				for i := 0; i < 8; i++ { // kEpfSharpEntries = 8
					b.ReadF16()
				}
			}
		}
		if b.ReadBool() { // epf_weight_custom
			for i := 0; i < 5; i++ {
				b.ReadF16()
			}
		}
		if b.ReadBool() { // epf_sigma_custom
			if !isModular {
				b.ReadF16()
			}
			b.ReadF16()
			b.ReadF16()
			b.ReadF16()
		}
		if isModular {
			b.ReadF16()
		}
	}
	skipExtensions(b)
	return lf
}

// readFrameHeader parses a frame header. meta provides codestream-level context.
func readFrameHeader(b *bitReader, meta *ImageMetadata) (FrameHeader, error) {
	var h FrameHeader
	h.Encoding = frameVarDCT
	h.Upsampling = 1
	h.NumPasses = 1
	h.IsLast = true
	if b.ReadBool() { // all_default
		h.Type = frameRegular
		return h, nil
	}

	h.Type = b.ReadU32(u32Val(0), u32Val(1), u32Val(2), u32Val(3)) // VisitFrameType

	isModular := b.ReadBool()
	if isModular {
		h.Encoding = frameModular
	} else {
		h.Encoding = frameVarDCT
	}

	h.Flags = b.ReadU64()

	// Color transform.
	if meta.XYBEncoded {
		// color_transform = kXYB (no bits)
	} else {
		b.ReadBool() // alternate (kYCbCr vs kNone) — no chroma subsampling handling here
	}

	numExtra := len(meta.ExtraChannels)

	// Upsampling.
	if h.Flags&flagUseDCFrame == 0 {
		h.Upsampling = b.ReadU32(u32Val(1), u32Val(2), u32Val(4), u32Val(8))
		for i := 0; i < numExtra; i++ {
			b.ReadU32(u32Val(1), u32Val(2), u32Val(4), u32Val(8)) // ec_upsampling
		}
	}

	// group_size_shift defaults to 1 and is only coded for modular frames; VarDCT
	// frames always use the default (group dim 256). Getting this wrong for
	// VarDCT miscounts the groups/TOC for images between 129 and 256 px.
	h.GroupSizeShift = 1
	if h.Encoding == frameModular {
		h.GroupSizeShift = b.ReadBits(2)
	} else if h.Encoding == frameVarDCT && meta.XYBEncoded {
		h.XQMScale = b.ReadBits(3) // x_qm_scale (default 3)
		h.BQMScale = b.ReadBits(3) // b_qm_scale (default 2)
	} else {
		h.XQMScale = 3
		h.BQMScale = 2
	}

	if h.Type != frameReferenceOnly {
		if err := readPasses(b, &h); err != nil {
			return h, err
		}
	}

	if h.Type == frameDCFrame {
		b.ReadU32(u32Val(1), u32Val(2), u32Val(3), u32Val(4)) // dc_level
	}

	isPartial := false
	if h.Type != frameDCFrame {
		h.CustomSize = b.ReadBool()
		if h.CustomSize {
			enc0, enc1, enc2, enc3 := u32Bits(8), u32Off(11, 256), u32Off(14, 2304), u32Off(30, 18688)
			if h.Type == frameRegular || h.Type == frameSkipProgress {
				ux0 := b.ReadU32(enc0, enc1, enc2, enc3)
				uy0 := b.ReadU32(enc0, enc1, enc2, enc3)
				h.OriginX = unpackSigned(ux0)
				h.OriginY = unpackSigned(uy0)
			}
			h.FrameW = b.ReadU32(enc0, enc1, enc2, enc3)
			h.FrameH = b.ReadU32(enc0, enc1, enc2, enc3)
			if h.Type == frameRegular || h.Type == frameSkipProgress {
				if h.OriginX > 0 || h.OriginY > 0 {
					isPartial = true
				}
			}
		}
	}

	if h.Type == frameRegular || h.Type == frameSkipProgress {
		readBlendingInfo(b, numExtra, isPartial)
		for i := 0; i < numExtra; i++ {
			readBlendingInfo(b, numExtra, isPartial)
		}
		if meta.HaveAnimation {
			// AnimationFrame: duration (+ timecode if have_timecodes).
			b.ReadU32(u32Val(0), u32Val(1), u32Bits(8), u32Bits(32))
		}
		h.IsLast = b.ReadBool() // default true
	} else {
		h.IsLast = false
	}

	if h.Type != frameDCFrame && !h.IsLast {
		b.ReadU32(u32Val(0), u32Val(1), u32Val(2), u32Val(3)) // save_as_reference
	}

	// save_before_color_transform.
	if h.Type != frameDCFrame {
		canBeReferenced := !h.IsLast && h.Type != frameDCFrame
		if canBeReferenced && !isPartial && (h.Type == frameRegular || h.Type == frameSkipProgress) {
			b.ReadBool()
		} else if h.Type == frameReferenceOnly {
			b.ReadBool()
		}
	}

	_ = visitNameString(b) // frame name

	h.LoopFilter = readLoopFilter(b, isModular)

	if err := skipExtensions(b); err != nil {
		return h, err
	}
	if !b.ok() {
		return h, errTruncated
	}
	return h, nil
}

func unpackSigned(u uint32) int32 {
	return int32(u>>1) ^ -int32(u&1)
}

// frameDimensions computes the group layout for a frame.
type frameDimensions struct {
	xsize, ysize             uint32
	groupDim                 uint32
	xsizeGroups, ysizeGroups uint32
	numGroups                uint32
	numDCGroups              uint32
}

func divCeil(a, b uint32) uint32 { return (a + b - 1) / b }

func computeFrameDimensions(xsize, ysize, groupSizeShift, upsampling uint32) frameDimensions {
	var fd frameDimensions
	const kGroupDim = 256
	const kBlockDim = 8
	fd.groupDim = (kGroupDim >> 1) << uint(groupSizeShift)
	fd.xsize = divCeil(xsize, upsampling)
	fd.ysize = divCeil(ysize, upsampling)
	fd.xsizeGroups = divCeil(fd.xsize, fd.groupDim)
	fd.ysizeGroups = divCeil(fd.ysize, fd.groupDim)
	xsizeBlocks := divCeil(fd.xsize, kBlockDim) // max_hshift 0 for modular
	ysizeBlocks := divCeil(fd.ysize, kBlockDim)
	xsizeDCGroups := divCeil(xsizeBlocks, fd.groupDim)
	ysizeDCGroups := divCeil(ysizeBlocks, fd.groupDim)
	fd.numGroups = fd.xsizeGroups * fd.ysizeGroups
	fd.numDCGroups = xsizeDCGroups * ysizeDCGroups
	return fd
}

// acGroupIndex is toc.h AcGroupIndex: the TOC index of AC group `group` in pass
// `pass`. TOC sections [0, 2+numDCGroups) are LfGlobal, DC groups and HfGlobal.
func acGroupIndex(pass, group, numGroups, numDCGroups uint32) uint32 {
	return 2 + numDCGroups + pass*numGroups + group
}

func numTocEntries(numGroups, numDCGroups, numPasses uint32) uint32 {
	if numGroups == 1 && numPasses == 1 {
		return 1
	}
	return acGroupIndex(0, 0, numGroups, numDCGroups) + numGroups*numPasses
}

// readTOC reads the table-of-contents section sizes (toc.cc ReadToc).
func readTOC(b *bitReader, tocEntries uint32) ([]uint32, error) {
	if tocEntries > 65536 {
		return nil, errors.New("gojxl: too many toc entries")
	}
	permuted := b.ReadBits(1) == 1
	if permuted {
		// Permutation (Lehmer-coded). Rare for small images; not yet supported.
		return nil, errors.New("gojxl: TOC permutation not supported")
	}
	if err := b.JumpToByteBoundary(); err != nil {
		return nil, err
	}
	sizes := make([]uint32, tocEntries)
	for i := range sizes {
		sizes[i] = b.ReadU32(u32Bits(10), u32Off(14, 1024), u32Off(22, 17408), u32Off(30, 4211712))
	}
	if err := b.JumpToByteBoundary(); err != nil {
		return nil, err
	}
	if !b.ok() {
		return nil, errTruncated
	}
	return sizes, nil
}
