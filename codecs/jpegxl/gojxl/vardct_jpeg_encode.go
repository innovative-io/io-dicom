package gojxl

import (
	"errors"
	"math"
)

// floatToF16 converts a non-negative float to its IEEE-754 half-precision bit
// pattern (the inverse of bitReader.ReadF16, which rejects inf/nan).
func floatToF16(f float32) uint16 {
	b := math.Float32bits(f)
	sign := uint16(b>>16) & 0x8000
	exp := int((b >> 23) & 0xFF)
	mant := b & 0x7FFFFF
	if exp == 0 {
		return sign // zero / subnormal -> 0
	}
	e := exp - 127 + 15
	if e >= 0x1F {
		e = 0x1E
		mant = 0x7FFFFF
	}
	if e <= 0 {
		return sign
	}
	return sign | uint16(e<<10) | uint16(mant>>13)
}

// writeJPEGImageMetadata writes ImageMetadata for a non-XYB (JPEG) image with
// the default sRGB / grayscale color encoding.
func writeJPEGImageMetadata(w *bitWriter, gray bool) {
	w.WriteBool(false) // all_default
	w.WriteBool(false) // extra_fields
	w.WriteBool(false) // floating_point_sample
	w.WriteU32(8, u32Val(8), u32Val(10), u32Val(12), u32Off(6, 1))
	w.WriteBool(true)                                                // modular_16bit_buffer_sufficient
	w.WriteU32(0, u32Val(0), u32Val(1), u32Off(4, 2), u32Off(12, 1)) // num_extra_channels = 0
	w.WriteBool(false)                                               // xyb_encoded = false
	w.WriteBool(true)                                                // color_encoding all_default (sRGB / gray)
	w.WriteU64(0)                                                    // extensions
}

// writeJPEGFrameHeader writes a VarDCT frame header for a non-XYB YCbCr image
// with the given chroma subsampling channel modes (JXL channel order 0=Cb,1=Y,
// 2=Cr) and the kSkipAdaptiveDCSmoothing flag set (as JPEG-mode frames do).
func writeJPEGFrameHeader(w *bitWriter, cs [3]uint32) {
	w.WriteBool(false)                                        // all_default
	w.WriteU32(0, u32Val(0), u32Val(1), u32Val(2), u32Val(3)) // frame_type = kRegularFrame
	w.WriteBool(false)                                        // is_modular = false (VarDCT)
	w.WriteU64(flagSkipAdaptiveDCSmoothing)                   // flags
	w.WriteBool(true)                                         // color_transform alternate -> kYCbCr
	for c := 0; c < 3; c++ {
		w.WriteBits(uint64(cs[c]), 2) // chroma_subsampling channel modes
	}
	w.WriteU32(1, u32Val(1), u32Val(2), u32Val(4), u32Val(8)) // upsampling = 1
	// non-XYB: no x/b_qm_scale.
	w.WriteU32(1, u32Val(1), u32Val(2), u32Val(3), u32Off(3, 4))        // num_passes = 1
	w.WriteBool(false)                                                  // custom_size_or_origin
	w.WriteU32(0, u32Val(0), u32Val(1), u32Val(2), u32Off(2, 3))        // BlendMode kReplace
	w.WriteBool(true)                                                   // is_last
	w.WriteU32(0, u32Val(0), u32Bits(4), u32Off(5, 16), u32Off(10, 48)) // name length 0
	w.WriteBool(false)                                                  // loop_filter all_default = false
	w.WriteBool(false)                                                  // gaborish off
	w.WriteU32(0, u32Val(0), u32Val(1), u32Val(2), u32Off(0, 3))        // epf_iters = 0
	w.WriteU64(0)                                                       // loop filter extensions
	w.WriteU64(0)                                                       // frame header extensions
}

const kJpegEncGlobalScale = 65536

// jpegCFLTranspose maps a JXL coefficient index to the transposed (JPEG) index.
func jpegTranspose(i int) int { return (i%8)*8 + (i / 8) }

// EncodeJXLFromJPEGData encodes a parsed baseline JPEG (jpegData with quantized
// coefficients) into a JPEG XL JPEG-recompression file (.111 container with a
// jbrd box and a JPEG-mode VarDCT codestream). Current scope: 4:4:4 (no chroma
// subsampling).
func EncodeJXLFromJPEGData(jd *jpegData) ([]byte, error) {
	if len(jd.components) != 1 && len(jd.components) != 3 {
		return nil, errors.New("gojxl: JPEG->JXL needs 1 or 3 components")
	}
	gray := len(jd.components) == 1
	// Subsampling check (4:4:4 only for now).
	for i := range jd.components {
		if jd.components[i].hSampFactor != 1 || jd.components[i].vSampFactor != 1 {
			return nil, errors.New("gojxl: JPEG->JXL chroma subsampling not yet supported")
		}
	}

	w, h := jd.width, jd.height
	bw, bh := divCeilInt(w, 8), divCeilInt(h, 8)

	jpegCMap := [3]int{1, 0, 2}
	if gray {
		jpegCMap = [3]int{0, 0, 0}
	}

	// DC image (JXL storage order [Y, X, B]) and AC coefficients (JXL channel
	// order 0=X,1=Y,2=B), both transposed from the JPEG layout.
	dcMod := [3][]int32{make([]int32, bw*bh), make([]int32, bw*bh), make([]int32, bw*bh)}
	acCoeff := [3][][]int32{make([][]int32, bw*bh), make([][]int32, bw*bh), make([][]int32, bw*bh)}
	dcStore := [3]int{1, 0, 2} // dcMod[p] holds JXL channel dcStore[p]
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			idx := by*bw + bx
			for c := 0; c < 3; c++ {
				jc := c
				if gray && c != 1 {
					// gray: only Y is real; X/B are zero.
					acCoeff[c][idx] = make([]int32, 64)
					continue
				}
				comp := &jd.components[jpegCMap[jc]]
				src := comp.coeffs[idx*64 : idx*64+64]
				blk := make([]int32, 64)
				for i := 1; i < 64; i++ {
					blk[i] = int32(src[jpegTranspose(i)])
				}
				acCoeff[c][idx] = blk
			}
			// DC into the DC image planes.
			for p := 0; p < 3; p++ {
				jc := dcStore[p]
				if gray && jc != 1 {
					continue
				}
				dcMod[p][idx] = int32(jd.components[jpegCMap[jc]].coeffs[idx*64])
			}
		}
	}

	// Quantization table (3*64) in the codestream layout.
	qt := make([]int32, 3*64)
	denom := float32(1.0 / (8.0 * 255.0))
	for c := 0; c < 3; c++ {
		jc := c
		if gray {
			jc = 1
		}
		qtbl := &jd.quant[jd.components[jpegCMap[jc]].quantIdx]
		for i := 0; i < 64; i++ {
			qt[c*64+i] = qtbl.values[jpegTranspose(i)]
		}
	}

	cs := [3]uint32{0, 0, 0} // 4:4:4
	codestream, err := assembleJPEGVarDCT(w, h, bw, bh, gray, cs, dcMod, acCoeff, qt, denom)
	if err != nil {
		return nil, err
	}
	jbrd, err := encodeJPEGData(jd)
	if err != nil {
		return nil, err
	}
	return wrapJXLContainer(jbrd, codestream), nil
}

// EncodeJXLFromJPEG parses a baseline JPEG and re-encodes it as a JPEG XL
// JPEG-recompression (.111) file. This is a lossless transcode: decoding the
// result reproduces the original JPEG byte-for-byte.
func EncodeJXLFromJPEG(jpegBytes []byte) ([]byte, error) {
	jd, err := ParseJPEG(jpegBytes)
	if err != nil {
		return nil, err
	}
	return EncodeJXLFromJPEGData(jd)
}
