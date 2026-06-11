package gojxl

import (
	"errors"
	"fmt"
)

// kJpegCFLPrecision and kJpegColorFactor mirror libjxl's chroma-from-luma fixed
// point used when reconstructing JPEG DCT coefficients.
const (
	kJpegCFLPrecision = 11
	kJpegColorFactor  = 84
)

// chroma subsampling shift tables (frame_header.cc), indexed by channel mode.
var jpegKHShift = [4]int{0, 1, 1, 0}
var jpegKVShift = [4]int{0, 1, 0, 1}

// subsamplingShifts returns the per-channel H/V shifts and raw shifts from the
// frame's chroma subsampling modes (JXL channel order: 0=Cb,1=Y,2=Cr).
func subsamplingShifts(cs [3]uint32) (hshift, vshift, rawH, rawV [3]int) {
	maxH, maxV := 0, 0
	for c := 0; c < 3; c++ {
		if jpegKHShift[cs[c]] > maxH {
			maxH = jpegKHShift[cs[c]]
		}
		if jpegKVShift[cs[c]] > maxV {
			maxV = jpegKVShift[cs[c]]
		}
	}
	for c := 0; c < 3; c++ {
		rawH[c] = jpegKHShift[cs[c]]
		rawV[c] = jpegKVShift[cs[c]]
		hshift[c] = maxH - rawH[c]
		vshift[c] = maxV - rawV[c]
	}
	return
}

// ReconstructJPEG reconstructs the original JPEG file bytes from a JPEG XL file
// that losslessly transcoded a JPEG (a container with a jbrd box). It is the
// inverse of cjxl's lossless JPEG transcoding. Returns an error for inputs that
// are not JPEG transcodes or that use an unsupported feature (e.g. progressive
// scans).
func ReconstructJPEG(data []byte) ([]byte, error) {
	jd, err := DecodeJPEGFromJXL(data)
	if err != nil {
		return nil, err
	}
	return EncodeJPEG(jd)
}

// IsJPEGRecompression reports whether data is a JPEG XL container carrying a
// JPEG reconstruction (jbrd) box.
func IsJPEGRecompression(data []byte) bool {
	return extractJBRDBox(data) != nil
}

// DecodeJPEGFromJXL reconstructs the JPEGData (metadata + quantized DCT
// coefficients + quant tables) from a JPEG XL file that losslessly transcoded a
// JPEG (i.e. a container with a `jbrd` box and a JPEG-mode VarDCT frame). The
// returned structure is sufficient to write the original JPEG bitstream.
//
// Current scope: baseline (DCT-8 only) JPEGs, with or without chroma subsampling
// (4:4:4 / 4:4:0 / 4:2:2 / 4:2:0 and grayscale).
func DecodeJPEGFromJXL(data []byte) (*jpegData, error) {
	box := extractJBRDBox(data)
	if box == nil {
		return nil, errors.New("gojxl: no jbrd box (not a JPEG transcode)")
	}
	jd, err := decodeJPEGData(box)
	if err != nil {
		return nil, err
	}
	st, err := decodeVarDCTFrame(data)
	if err != nil {
		return nil, err
	}
	if err := populateJPEGCoefficients(st, jd); err != nil {
		return nil, err
	}
	return jd, nil
}

// populateJPEGCoefficients fills jd.components[].coeffs and jd.quant[].values
// from the decoded VarDCT state, porting the JPEG paths of dec_group.cc and
// dec_frame.cc (DC offset, transpose, chroma-from-luma restoration).
func populateJPEGCoefficients(st *vardctState, jd *jpegData) error {
	if st.meta.XYBEncoded {
		return errors.New("gojxl: JPEG reconstruction requires a non-XYB frame")
	}
	if st.fh.ColorTransform != ctYCbCr && st.fh.ColorTransform != ctNone {
		return errors.New("gojxl: unexpected color transform for JPEG reconstruction")
	}
	numComp := len(jd.components)
	if numComp != 1 && numComp != 3 {
		return errors.New("gojxl: JPEG reconstruction needs 1 or 3 components")
	}
	isGray := numComp == 1

	// Only DCT-8 blocks are representable as JPEG.
	for i := range st.acm.valid {
		if st.acm.valid[i] && st.acm.strategy[i] != acDCT {
			return errors.New("gojxl: JPEG reconstruction supports only DCT-8 blocks")
		}
	}

	qe := st.quantLib[0]
	if qe == nil || qe.mode != quantModeRAW || len(qe.rawQtable) != 3*64 {
		return errors.New("gojxl: JPEG reconstruction requires a RAW quant table")
	}
	qt := qe.rawQtable

	jpegCMap := [3]int{1, 0, 2}
	if isGray {
		jpegCMap = [3]int{0, 0, 0}
	}

	hshift, vshift, rawH, rawV := subsamplingShifts(st.fh.ChromaSubsampling)
	if isGray {
		hshift, vshift, rawH, rawV = [3]int{}, [3]int{}, [3]int{}, [3]int{}
	}

	bw, bh := st.acm.bw, st.acm.bh
	jd.width = int(st.sh.Xsize)
	jd.height = int(st.sh.Ysize)

	// Component geometry + coefficient storage (dec_frame.cc).
	for c := 0; c < numComp; c++ {
		comp := &jd.components[jpegCMap[c]]
		comp.widthInBlocks = uint32(bw >> uint(hshift[c]))
		comp.heightInBlks = uint32(bh >> uint(vshift[c]))
		comp.hSampFactor = 1 << uint(rawH[c])
		comp.vSampFactor = 1 << uint(rawV[c])
		comp.coeffs = make([]int16, int(comp.widthInBlocks)*int(comp.heightInBlks)*64)
	}

	// JPEG quant tables (dec_frame.cc): values[x*8+y] = qtable[quant_c*64+y*8+x].
	qtSet := uint32(0)
	for c := 0; c < numComp; c++ {
		quantC := c
		if isGray {
			quantC = 1
		}
		qpos := jd.components[jpegCMap[c]].quantIdx
		if int(qpos) >= len(jd.quant) {
			return errors.New("gojxl: quant table index out of range")
		}
		qtSet |= 1 << qpos
		for x := 0; x < 8; x++ {
			for y := 0; y < 8; y++ {
				jd.quant[qpos].values[x*8+y] = qt[quantC*64+y*8+x]
			}
		}
	}
	for i := range jd.quant {
		if qtSet&(1<<uint(i)) != 0 {
			continue
		}
		if i == 0 {
			return errors.New("gojxl: first quant table unused")
		}
		jd.quant[i].values = jd.quant[i-1].values
	}

	if isGray {
		return populateGrayCoefficients(st, jd, qt)
	}

	// Chroma-from-luma is only applied when the image is 4:4:4; subsampled chroma
	// is stored directly (dec_group.cc, the !Is444() branch).
	is444 := st.csHShift[0] == 0 && st.csVShift[0] == 0

	// Per-(channel, JPEG sub-block) scaled quant table for CfL, at the transposed
	// coefficient position (dec_group.cc scaled_qtable).
	transpose := func(j int) int { return (j%8)*8 + (j / 8) }
	var scaledQ [3][64]int64
	if is444 {
		for c := 0; c < 3; c++ {
			for j := 0; j < 64; j++ {
				ti := transpose(j)
				scaledQ[c][j] = (int64(1) << kJpegCFLPrecision) * int64(qt[64+ti]) / int64(qt[c*64+ti])
			}
		}
	}
	const round = int64(1) << (kJpegCFLPrecision - 1)

	transposeBlock := func(src []int32) [64]int32 {
		var t [64]int32
		for r := 0; r < 8; r++ {
			for cc := 0; cc < 8; cc++ {
				t[r*8+cc] = src[cc*8+r]
			}
		}
		return t
	}
	clampDC := func(d int32) int32 {
		if d < -2047 {
			return -2047
		}
		if d > 2047 {
			return 2047
		}
		return d
	}
	store := func(comp *jpegComponent, bx, by int, blk *[64]int32) error {
		base := (by*int(comp.widthInBlocks) + bx) * 64
		for j := 0; j < 64; j++ {
			v := blk[j]
			if v < -4095 || v > 4095 {
				return fmt.Errorf("gojxl: JPEG coefficient %d out of range", v)
			}
			comp.coeffs[base+j] = int16(v)
		}
		return nil
	}

	cw := st.dc.cw
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			lumaIdx := by*bw + bx

			// Luma (Y): channel 1 -> JPEG component jpegCMap[1].
			tY := transposeBlock(st.acCoeffs[1][lumaIdx])
			yBlk := tY
			yBlk[0] = clampDC(st.dc.y[lumaIdx])
			if err := store(&jd.components[jpegCMap[1]], bx, by, &yBlk); err != nil {
				return err
			}

			// Chroma (Cb=channel 0, Cr=channel 2): only at the top-left luma
			// block of each chroma block.
			for _, c := range [2]int{0, 2} {
				hs, vs := st.csHShift[c], st.csVShift[c]
				sbx, sby := bx>>uint(hs), by>>uint(vs)
				if (sbx<<uint(hs) != bx) || (sby<<uint(vs) != by) {
					continue
				}
				blk := transposeBlock(st.acCoeffs[c][sby*(bw>>uint(hs))+sbx])
				if is444 {
					tile := (by/kColorTileDimInBlocks)*st.acm.ctW + bx/kColorTileDimInBlocks
					var factor int64
					if c == 0 {
						factor = int64(st.acm.ytoxMap[tile])
					} else {
						factor = int64(st.acm.ytobMap[tile])
					}
					scale := factor * (1 << kJpegCFLPrecision) / kJpegColorFactor
					for j := 0; j < 64; j++ {
						cs := (scaledQ[c][j]*scale + round) >> kJpegCFLPrecision
						blk[j] += int32((int64(tY[j])*cs + round) >> kJpegCFLPrecision)
					}
				}
				var dc int32
				if c == 0 {
					dc = st.dc.x[sby*cw+sbx]
				} else {
					dc = st.dc.bch[sby*cw+sbx]
				}
				blk[0] = clampDC(dc)
				if err := store(&jd.components[jpegCMap[c]], sbx, sby, &blk); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// populateGrayCoefficients handles the single-component (grayscale) case, where
// only the Y channel is present and there is no chroma-from-luma.
func populateGrayCoefficients(st *vardctState, jd *jpegData, qt []int32) error {
	bw, bh := st.acm.bw, st.acm.bh
	comp := &jd.components[0]
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			idx := by*bw + bx
			src := st.acCoeffs[1][idx]
			base := (by*int(comp.widthInBlocks) + bx) * 64
			for r := 0; r < 8; r++ {
				for cc := 0; cc < 8; cc++ {
					comp.coeffs[base+r*8+cc] = int16(src[cc*8+r])
				}
			}
			d := st.dc.y[idx]
			if d < -2047 {
				d = -2047
			} else if d > 2047 {
				d = 2047
			}
			comp.coeffs[base] = int16(d)
		}
	}
	return nil
}
