package jpeg

// Pure-Go decoder for DCT-based JPEG at >8-bit precision — specifically JPEG
// Extended Sequential (SOF1), the DICOM "JPEG Extended (Process 2 & 4) 12-bit"
// transfer syntax (1.2.840.10008.1.2.4.51). Baseline 8-bit stays on image/jpeg;
// this fills the 12-bit gap for CGO_ENABLED=0 builds.
//
// DCT decoding is lossy, so unlike the lossless path the output is not required
// to be bit-identical to libjpeg — the IDCT rounding differs between decoders.
// It reuses the bitReader / huffTable / extend helpers from gojpeg_lossless.go.

import "math"

const (
	mSOF0 = 0xC0 // baseline DCT
	mSOF1 = 0xC1 // extended sequential DCT
	mDQT  = 0xDB
)

// zigzag maps a zig-zag scan index to the natural 8x8 (row-major) index.
var zigzag = [64]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}

type dctComponent struct {
	id   byte
	h, v int // sampling factors
	tq   int // quantization table selector
	td   int // DC Huffman selector (from SOS)
	ta   int // AC Huffman selector (from SOS)
	pred int // running DC predictor

	// plane geometry in samples (padded to its block grid)
	planeW, planeH int
	plane          []int
}

type dctFrame struct {
	precision  int
	width      int
	height     int
	comps      []dctComponent
	restartIv  int
	hmax, vmax int
}

// idctCos[u][x] = cos((2x+1)uπ/16) * (u==0 ? 1/√2 : 1), scaled for a separable
// float IDCT. Precomputed once.
var idctCos [8][8]float64

func init() {
	for u := 0; u < 8; u++ {
		cu := 1.0
		if u == 0 {
			cu = 1 / math.Sqrt2
		}
		for x := 0; x < 8; x++ {
			idctCos[u][x] = cu * math.Cos(float64((2*x+1)*u)*math.Pi/16)
		}
	}
}

// idct8x8 performs a separable inverse DCT on natural-order coefficients,
// writing spatial samples (pre level-shift) back into block.
func idct8x8(block *[64]float64) {
	var tmp [64]float64
	// rows
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			s := 0.0
			for u := 0; u < 8; u++ {
				s += idctCos[u][x] * block[y*8+u]
			}
			tmp[y*8+x] = s * 0.5
		}
	}
	// columns
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			s := 0.0
			for v := 0; v < 8; v++ {
				s += idctCos[v][y] * tmp[v*8+x]
			}
			block[y*8+x] = s * 0.5
		}
	}
}

// decodeDCT parses and decodes a SOF0/SOF1 JPEG into per-component planes.
func decodeDCT(data []byte) (dctFrame, error) {
	var frame dctFrame
	if len(data) < 2 || data[0] != 0xFF || data[1] != mSOI {
		return frame, errGoJPEGMalformed
	}
	var quant [4][64]int
	var dcTab, acTab [4]*huffTable
	i := 2
	sawSOF := false
	for {
		if i+1 >= len(data) || data[i] != 0xFF {
			return frame, errGoJPEGMalformed
		}
		marker := data[i+1]
		i += 2
		if marker == mEOI {
			return frame, errGoJPEGMalformed
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			continue
		}
		if i+2 > len(data) {
			return frame, errGoJPEGMalformed
		}
		segLen := int(data[i])<<8 | int(data[i+1])
		if segLen < 2 || i+segLen > len(data) {
			return frame, errGoJPEGMalformed
		}
		seg := data[i+2 : i+segLen]

		switch marker {
		case mDQT:
			if err := parseDQT(seg, &quant); err != nil {
				return frame, err
			}
		case mSOF0, mSOF1:
			if err := parseSOFDCT(seg, &frame); err != nil {
				return frame, err
			}
			sawSOF = true
		case mDHT:
			if err := parseDHTDCT(seg, &dcTab, &acTab); err != nil {
				return frame, err
			}
		case mDRI:
			if len(seg) < 2 {
				return frame, errGoJPEGMalformed
			}
			frame.restartIv = int(seg[0])<<8 | int(seg[1])
		case mSOS:
			if !sawSOF {
				return frame, errGoJPEGMalformed
			}
			if err := parseSOSDCT(seg, &frame, &dcTab, &acTab); err != nil {
				return frame, err
			}
			if err := decodeDCTScan(data[i+segLen:], &frame, &quant, dcTab, acTab); err != nil {
				return frame, err
			}
			return frame, nil
		default:
			if marker >= 0xC0 && marker <= 0xCF && marker != mDHT {
				return frame, errGoJPEGUnsupported // a SOF we don't handle (e.g. progressive)
			}
		}
		i += segLen
	}
}

func parseDQT(seg []byte, quant *[4][64]int) error {
	p := 0
	for p < len(seg) {
		pq := seg[p] >> 4   // 0 = 8-bit, 1 = 16-bit entries
		tq := seg[p] & 0x0F // table id
		p++
		if tq > 3 {
			return errGoJPEGUnsupported
		}
		if pq == 0 {
			if p+64 > len(seg) {
				return errGoJPEGMalformed
			}
			for k := 0; k < 64; k++ {
				quant[tq][k] = int(seg[p+k])
			}
			p += 64
		} else {
			if p+128 > len(seg) {
				return errGoJPEGMalformed
			}
			for k := 0; k < 64; k++ {
				quant[tq][k] = int(seg[p+2*k])<<8 | int(seg[p+2*k+1])
			}
			p += 128
		}
	}
	return nil
}

func parseSOFDCT(seg []byte, frame *dctFrame) error {
	if len(seg) < 6 {
		return errGoJPEGMalformed
	}
	frame.precision = int(seg[0])
	frame.height = int(seg[1])<<8 | int(seg[2])
	frame.width = int(seg[3])<<8 | int(seg[4])
	nf := int(seg[5])
	if nf != 1 && nf != 3 {
		return errGoJPEGUnsupported
	}
	if frame.precision != 8 && frame.precision != 12 {
		return errGoJPEGUnsupported
	}
	if frame.width == 0 || frame.height == 0 || len(seg) < 6+nf*3 {
		return errGoJPEGMalformed
	}
	if frame.width*frame.height*nf > maxGoJPEGSamples {
		return errGoJPEGUnsupported
	}
	frame.comps = make([]dctComponent, nf)
	frame.hmax, frame.vmax = 1, 1
	for c := 0; c < nf; c++ {
		off := 6 + c*3
		frame.comps[c].id = seg[off]
		frame.comps[c].h = int(seg[off+1] >> 4)
		frame.comps[c].v = int(seg[off+1] & 0x0F)
		frame.comps[c].tq = int(seg[off+2])
		if frame.comps[c].h < 1 || frame.comps[c].h > 4 || frame.comps[c].v < 1 || frame.comps[c].v > 4 || frame.comps[c].tq > 3 {
			return errGoJPEGUnsupported
		}
		if frame.comps[c].h > frame.hmax {
			frame.hmax = frame.comps[c].h
		}
		if frame.comps[c].v > frame.vmax {
			frame.vmax = frame.comps[c].v
		}
	}
	return nil
}

func parseDHTDCT(seg []byte, dcTab, acTab *[4]*huffTable) error {
	p := 0
	for p < len(seg) {
		tc := seg[p] >> 4
		th := seg[p] & 0x0F
		p++
		if tc > 1 || th > 3 {
			return errGoJPEGUnsupported
		}
		if p+16 > len(seg) {
			return errGoJPEGMalformed
		}
		var counts [16]byte
		total := 0
		for l := 0; l < 16; l++ {
			counts[l] = seg[p+l]
			total += int(counts[l])
		}
		p += 16
		if p+total > len(seg) {
			return errGoJPEGMalformed
		}
		vals := make([]byte, total)
		copy(vals, seg[p:p+total])
		p += total
		t := buildHuffTable(counts, vals)
		if tc == 0 {
			dcTab[th] = t
		} else {
			acTab[th] = t
		}
	}
	return nil
}

func parseSOSDCT(seg []byte, frame *dctFrame, dcTab, acTab *[4]*huffTable) error {
	if len(seg) < 1 {
		return errGoJPEGMalformed
	}
	ns := int(seg[0])
	if ns != len(frame.comps) || len(seg) < 1+ns*2+3 {
		return errGoJPEGUnsupported
	}
	for c := 0; c < ns; c++ {
		cs := seg[1+c*2]
		td := int(seg[1+c*2+1] >> 4)
		ta := int(seg[1+c*2+1] & 0x0F)
		if td > 3 || ta > 3 {
			return errGoJPEGMalformed // selectors must index the 4-entry table sets
		}
		matched := false
		for fc := range frame.comps {
			if frame.comps[fc].id == cs {
				frame.comps[fc].td = td
				frame.comps[fc].ta = ta
				matched = true
				break
			}
		}
		if !matched {
			return errGoJPEGMalformed
		}
	}
	for c := range frame.comps {
		if dcTab[frame.comps[c].td] == nil || acTab[frame.comps[c].ta] == nil {
			return errGoJPEGMalformed
		}
	}
	return nil
}

// decodeDCTScan decodes the entropy segment into each component's sample plane.
func decodeDCTScan(entropy []byte, frame *dctFrame, quant *[4][64]int, dcTab, acTab [4]*huffTable) error {
	br := &bitReader{data: entropy}
	levelShift := 1 << (frame.precision - 1)
	maxVal := (1 << frame.precision) - 1

	// MCU grid uses the maximum sampling factors.
	mcuW := 8 * frame.hmax
	mcuH := 8 * frame.vmax
	mcusX := (frame.width + mcuW - 1) / mcuW
	mcusY := (frame.height + mcuH - 1) / mcuH

	for c := range frame.comps {
		comp := &frame.comps[c]
		comp.planeW = mcusX * comp.h * 8
		comp.planeH = mcusY * comp.v * 8
		comp.plane = make([]int, comp.planeW*comp.planeH)
		comp.pred = 0
	}

	restartIv := frame.restartIv
	mcuCount := 0
	var block [64]float64
	for my := 0; my < mcusY; my++ {
		for mx := 0; mx < mcusX; mx++ {
			if restartIv > 0 && mcuCount > 0 && mcuCount%restartIv == 0 {
				br.restart()
				for c := range frame.comps {
					frame.comps[c].pred = 0
				}
			}
			for c := range frame.comps {
				comp := &frame.comps[c]
				for by := 0; by < comp.v; by++ {
					for bx := 0; bx < comp.h; bx++ {
						if err := decodeBlock(br, dcTab[comp.td], acTab[comp.ta], &quant[comp.tq], comp, &block); err != nil {
							return err
						}
						idct8x8(&block)
						// place block at component plane position
						px0 := (mx*comp.h + bx) * 8
						py0 := (my*comp.v + by) * 8
						for yy := 0; yy < 8; yy++ {
							for xx := 0; xx < 8; xx++ {
								v := int(math.Round(block[yy*8+xx])) + levelShift
								if v < 0 {
									v = 0
								} else if v > maxVal {
									v = maxVal
								}
								comp.plane[(py0+yy)*comp.planeW+(px0+xx)] = v
							}
						}
					}
				}
			}
			mcuCount++
		}
	}
	segments := 1
	if frame.restartIv > 0 && mcuCount > 0 {
		segments += mcuCount / frame.restartIv
	}
	if err := br.checkNotTruncated(segments); err != nil {
		return err
	}
	return nil
}

// decodeBlock decodes one 8x8 block: DC difference + AC run-length, dequantizes
// into natural order, and returns the result in block (float for the IDCT).
func decodeBlock(br *bitReader, dc, ac *huffTable, quant *[64]int, comp *dctComponent, block *[64]float64) error {
	for i := range block {
		block[i] = 0
	}
	s, err := br.decodeHuff(dc)
	if err != nil {
		return err
	}
	if s > 15 {
		return errGoJPEGMalformed
	}
	diff := extend(br.receive(int(s)), int(s))
	comp.pred += diff
	block[0] = float64(comp.pred * quant[0])

	k := 1
	for k < 64 {
		rs, err := br.decodeHuff(ac)
		if err != nil {
			return err
		}
		r := int(rs >> 4)
		sz := int(rs & 0x0F)
		if sz == 0 {
			if r != 15 {
				break // EOB
			}
			k += 16 // ZRL
			continue
		}
		k += r
		if k >= 64 {
			return errGoJPEGMalformed
		}
		coef := extend(br.receive(sz), sz)
		block[zigzag[k]] = float64(coef * quant[k])
		k++
	}
	return nil
}

// gojpegSOFMarker scans for the first Start-Of-Frame marker and returns its
// type byte (0 if none/malformed), used to route to the right decoder.
func gojpegSOFMarker(data []byte) byte {
	if len(data) < 2 || data[0] != 0xFF || data[1] != mSOI {
		return 0
	}
	i := 2
	for i+1 < len(data) {
		if data[i] != 0xFF {
			return 0
		}
		m := data[i+1]
		i += 2
		if m == mEOI {
			return 0
		}
		if m == 0x01 || (m >= 0xD0 && m <= 0xD7) {
			continue
		}
		if i+2 > len(data) {
			return 0
		}
		// SOF markers, excluding DHT(0xC4), JPG(0xC8), DAC(0xCC).
		if m >= 0xC0 && m <= 0xCF && m != 0xC4 && m != 0xC8 && m != 0xCC {
			return m
		}
		segLen := int(data[i])<<8 | int(data[i+1])
		if segLen < 2 || i+segLen > len(data) {
			return 0
		}
		i += segLen
	}
	return 0
}

// gojpegDecodeInto routes a JPEG payload to the lossless or DCT decoder by its
// SOF type, packing the result into output.
func gojpegDecodeInto(encoded, output []byte) error {
	switch gojpegSOFMarker(encoded) {
	case mSOF3:
		return decodeLosslessInto(encoded, output)
	case mSOF0, mSOF1:
		return decodeDCTInto(encoded, output)
	default:
		return errGoJPEGUnsupported
	}
}

// decodeDCTInto decodes a DCT JPEG and packs grayscale/RGB samples into output
// using the codec's little-endian convention (2 bytes/sample at 12-bit).
func decodeDCTInto(encoded, output []byte) error {
	frame, err := decodeDCT(encoded)
	if err != nil {
		return err
	}
	nc := len(frame.comps)
	if nc != 1 {
		// 3-component DCT at 12-bit (color) is essentially absent in DICOM and
		// needs photometric/transform handling to match the cgo backend; defer
		// to libjpeg when available.
		return errGoJPEGUnsupported
	}
	bps := 1
	if frame.precision > 8 {
		bps = 2
	}
	need := frame.width * frame.height * nc * bps
	if need > len(output) {
		return errGoJPEGOutputSize
	}
	comp := &frame.comps[0]
	for y := 0; y < frame.height; y++ {
		for x := 0; x < frame.width; x++ {
			v := comp.plane[y*comp.planeW+x]
			idx := y*frame.width + x
			if bps == 1 {
				output[idx] = byte(v)
			} else {
				output[idx*2] = byte(v)
				output[idx*2+1] = byte(v >> 8)
			}
		}
	}
	return nil
}
