package jpeg

// Pure-Go decoder for JPEG Lossless (ITU-T T.81 process 14, SOF3), covering the
// DICOM transfer syntaxes JPEG Lossless Non-Hierarchical (1.2.840.10008.1.2.4.57)
// and its first-order-prediction selection-value-1 variant (.70). Baseline and
// progressive DCT JPEG continue to use the standard library / libjpeg; this
// decoder fills the lossless gap so CGO_ENABLED=0 builds can decode it.
//
// It is registered as the low-priority "gojpeg" backend so the cgo libjpeg
// backend still wins when compiled in, while this one is selected automatically
// in pure-Go builds (and is always reachable via UseBackend).

import (
	"context"
	"errors"
)

var (
	errGoJPEGMalformed   = errors.New("gojpeg: malformed JPEG payload")
	errGoJPEGUnsupported = errors.New("gojpeg: unsupported JPEG variant (pure-Go decoder handles lossless SOF3 only)")
	errGoJPEGOutputSize  = errors.New("gojpeg: decoded payload does not fit output buffer")
)

const (
	mSOI  = 0xD8
	mEOI  = 0xD9
	mSOF3 = 0xC3 // lossless, Huffman
	mDHT  = 0xC4
	mSOS  = 0xDA
	mDRI  = 0xDD
)

// maxGoJPEGSamples bounds the total sample count derived from frame headers so a
// malformed SOF claiming huge dimensions cannot trigger a giant allocation
// before the output-buffer size is even checked (~256 Mpixel, generous for
// medical imaging).
const maxGoJPEGSamples = 1 << 28

// huffTable is a canonical Huffman decode table built per T.81 Annex C/F.
type huffTable struct {
	minCode [17]int // minCode[l] = smallest code of length l
	maxCode [17]int // maxCode[l] = largest code of length l, or -1 if none
	valPtr  [17]int // index into vals of the first symbol of length l
	vals    []byte  // HUFFVAL, in canonical order
}

func buildHuffTable(counts [16]byte, vals []byte) *huffTable {
	t := &huffTable{vals: vals}
	// huffsize/huffcode generation (T.81 Annex C).
	var code, k int
	for l := 1; l <= 16; l++ {
		n := int(counts[l-1])
		if n == 0 {
			t.maxCode[l] = -1
			code <<= 1
			continue
		}
		t.valPtr[l] = k
		t.minCode[l] = code
		code += n
		k += n
		t.maxCode[l] = code - 1
		code <<= 1
	}
	return t
}

// component holds per-component lossless decode state.
type lcomponent struct {
	id      byte
	dcTable int // Huffman table selector (Td) from the scan header
}

type losslessFrame struct {
	precision int
	width     int
	height    int
	comps     []lcomponent
	pointXfrm int // Al in the scan header (point transform)
	predictor int // Ss in the scan header (1..7)
	restartIv int // DRI restart interval (MCUs), 0 if none
}

// bitReader reads MSB-first bits from the entropy-coded segment, transparently
// removing 0xFF00 byte stuffing and stopping at any marker (0xFFxx, xx != 0).
type bitReader struct {
	data     []byte
	pos      int
	bits     uint32
	nbits    uint
	atMarker bool // a marker (other than stuffed 0x00) was reached
	marker   byte
}

// fill loads one more byte into the bit accumulator. Returns false at a marker
// or end of data (callers treat exhausted bits as zero, matching T.81 padding).
func (br *bitReader) fill() bool {
	if br.atMarker || br.pos >= len(br.data) {
		return false
	}
	b := br.data[br.pos]
	if b == 0xFF {
		if br.pos+1 >= len(br.data) {
			br.pos++
			return false
		}
		next := br.data[br.pos+1]
		if next == 0x00 {
			br.pos += 2 // stuffed 0xFF
		} else {
			br.atMarker = true
			br.marker = next
			return false
		}
	} else {
		br.pos++
	}
	br.bits |= uint32(b) << (24 - br.nbits)
	br.nbits += 8
	return true
}

func (br *bitReader) readBit() int {
	if br.nbits == 0 && !br.fill() {
		return 0
	}
	bit := int((br.bits >> 31) & 1)
	br.bits <<= 1
	br.nbits--
	return bit
}

// receive reads n bits as an unsigned integer (n may be 0).
func (br *bitReader) receive(n int) int {
	v := 0
	for i := 0; i < n; i++ {
		v = v<<1 | br.readBit()
	}
	return v
}

// decodeHuff decodes one symbol using the canonical table.
func (br *bitReader) decodeHuff(t *huffTable) (byte, error) {
	code := 0
	for l := 1; l <= 16; l++ {
		code = code<<1 | br.readBit()
		if t.maxCode[l] >= 0 && code <= t.maxCode[l] {
			idx := t.valPtr[l] + (code - t.minCode[l])
			if idx < 0 || idx >= len(t.vals) {
				return 0, errGoJPEGMalformed
			}
			return t.vals[idx], nil
		}
	}
	return 0, errGoJPEGMalformed
}

// extend sign-extends a v received with t bits, per T.81 figure F.12.
func extend(v, t int) int {
	if t == 0 {
		return 0
	}
	if v < (1 << (t - 1)) {
		return v - (1 << t) + 1
	}
	return v
}

// restart clears the bit accumulator at a restart-interval boundary and
// consumes the RSTn marker that terminates the interval, so decoding can
// continue with the next interval.
//
// Both cases matter. If a previous fill() already latched the marker, it is
// consumed from the latched state. Otherwise the accumulator still held buffered
// bits when the boundary was reached, so no fill() has looked at the marker yet
// and br.pos still sits at (or just before) it — the marker must be located and
// skipped here.
//
// Handling only the latched case silently corrupted every conforming file that
// uses restart intervals: the marker was never consumed, the next fill() latched
// it and returned false, and from then on readBit returned 0 forever, so every
// interval after the first decoded as padding with no error reported.
func (br *bitReader) restart() {
	br.bits = 0
	br.nbits = 0

	if br.atMarker {
		if br.marker >= 0xD0 && br.marker <= 0xD7 {
			br.pos += 2 // consume the RSTn marker
			br.atMarker = false
			br.marker = 0
		}
		return
	}

	// Scan forward for the RSTn. Any bytes in between are the encoder's pad bits
	// (T.81 E.1.1 requires padding to a byte boundary before the marker); stuffed
	// 0xFF00 sequences are skipped. A marker that is not an RSTn ends the scan and
	// is left for the caller's normal marker handling.
	for i := br.pos; i+1 < len(br.data); i++ {
		if br.data[i] != 0xFF {
			continue
		}
		next := br.data[i+1]
		if next >= 0xD0 && next <= 0xD7 {
			br.pos = i + 2
			br.atMarker = false
			br.marker = 0
			return
		}
		if next != 0x00 { // a real, non-restart marker: stop here
			return
		}
	}
}

// decodeLossless parses and decodes a lossless SOF3 JPEG, returning interleaved
// samples (one int per component per pixel, component-minor) plus geometry.
func decodeLossless(data []byte) (frame losslessFrame, samples []int, err error) {
	if len(data) < 2 || data[0] != 0xFF || data[1] != mSOI {
		return frame, nil, errGoJPEGMalformed
	}
	var huff [4]*huffTable // DC tables 0..3 (lossless uses class-0 tables)
	i := 2
	sawSOF := false
	for {
		if i+1 >= len(data) {
			return frame, nil, errGoJPEGMalformed
		}
		if data[i] != 0xFF {
			return frame, nil, errGoJPEGMalformed
		}
		marker := data[i+1]
		i += 2
		if marker == mEOI {
			return frame, nil, errGoJPEGMalformed // EOI before scan data
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			continue // standalone markers
		}
		if i+2 > len(data) {
			return frame, nil, errGoJPEGMalformed
		}
		segLen := int(data[i])<<8 | int(data[i+1])
		if segLen < 2 || i+segLen > len(data) {
			return frame, nil, errGoJPEGMalformed
		}
		seg := data[i+2 : i+segLen]

		switch marker {
		case mSOF3:
			if err := parseSOF3(seg, &frame); err != nil {
				return frame, nil, err
			}
			sawSOF = true
		case mDHT:
			if err := parseDHT(seg, &huff); err != nil {
				return frame, nil, err
			}
		case mDRI:
			if len(seg) < 2 {
				return frame, nil, errGoJPEGMalformed
			}
			frame.restartIv = int(seg[0])<<8 | int(seg[1])
		case mSOS:
			if !sawSOF {
				return frame, nil, errGoJPEGMalformed
			}
			if err := parseSOS(seg, &frame); err != nil {
				return frame, nil, err
			}
			samples, err = decodeScan(data[i+segLen:], &frame, &huff)
			return frame, samples, err
		default:
			if marker >= 0xC0 && marker <= 0xCF && marker != mDHT {
				// Some other SOF (baseline/progressive/etc.) — not ours.
				return frame, nil, errGoJPEGUnsupported
			}
			// APPn, COM, DQT, etc.: skip.
		}
		i += segLen
	}
}

func parseSOF3(seg []byte, frame *losslessFrame) error {
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
	if frame.precision < 2 || frame.precision > 16 || frame.width == 0 || frame.height == 0 {
		return errGoJPEGUnsupported
	}
	if frame.width*frame.height*nf > maxGoJPEGSamples {
		return errGoJPEGUnsupported
	}
	if len(seg) < 6+nf*3 {
		return errGoJPEGMalformed
	}
	frame.comps = make([]lcomponent, nf)
	for c := 0; c < nf; c++ {
		off := 6 + c*3
		frame.comps[c].id = seg[off]
		h := seg[off+1] >> 4
		v := seg[off+1] & 0x0F
		if h != 1 || v != 1 {
			return errGoJPEGUnsupported // subsampled lossless is not supported
		}
	}
	return nil
}

func parseDHT(seg []byte, huff *[4]*huffTable) error {
	p := 0
	for p < len(seg) {
		tc := seg[p] >> 4
		th := seg[p] & 0x0F
		p++
		if tc != 0 || th > 3 {
			return errGoJPEGUnsupported // lossless uses DC (class 0) tables only
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
		huff[th] = buildHuffTable(counts, vals)
	}
	return nil
}

func parseSOS(seg []byte, frame *losslessFrame) error {
	if len(seg) < 1 {
		return errGoJPEGMalformed
	}
	ns := int(seg[0])
	if ns != len(frame.comps) {
		return errGoJPEGUnsupported // only fully-interleaved scans are supported
	}
	if len(seg) < 1+ns*2+3 {
		return errGoJPEGMalformed
	}
	for c := 0; c < ns; c++ {
		cs := seg[1+c*2]
		td := seg[1+c*2+1] >> 4
		if td > 3 {
			return errGoJPEGMalformed // table selector must index the 4-entry table set
		}
		// Map the scan component to the frame component order.
		matched := false
		for fc := range frame.comps {
			if frame.comps[fc].id == cs {
				frame.comps[fc].dcTable = int(td)
				matched = true
				break
			}
		}
		if !matched {
			return errGoJPEGMalformed
		}
	}
	tail := seg[1+ns*2:]
	frame.predictor = int(tail[0]) // Ss
	frame.pointXfrm = int(tail[2] & 0x0F)
	if frame.predictor < 1 || frame.predictor > 7 {
		return errGoJPEGUnsupported
	}
	// Guard the default-prediction shift (1 << (precision-pointXfrm-1)) against a
	// negative or oversized shift amount from malformed headers.
	if frame.pointXfrm >= frame.precision {
		return errGoJPEGUnsupported
	}
	return nil
}

// decodeScan runs the lossless entropy decode for a fully-interleaved scan of
// 1x1-sampled components, returning component-minor interleaved samples.
func decodeScan(entropy []byte, frame *losslessFrame, huff *[4]*huffTable) ([]int, error) {
	nc := len(frame.comps)
	w, h := frame.width, frame.height
	out := make([]int, w*h*nc)
	br := &bitReader{data: entropy}

	defaultPx := 1 << (frame.precision - frame.pointXfrm - 1)
	mask := (1 << frame.precision) - 1

	tables := make([]*huffTable, nc)
	for c := range frame.comps {
		t := huff[frame.comps[c].dcTable]
		if t == nil {
			return nil, errGoJPEGMalformed
		}
		tables[c] = t
	}

	// at returns the reconstructed sample for component c at (x,y), or 0 off-grid.
	at := func(c, x, y int) int {
		if x < 0 || y < 0 {
			return 0
		}
		return out[(y*w+x)*nc+c]
	}

	restartIv := frame.restartIv
	mcu := 0 // MCU == pixel position for 1x1 sampling
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if restartIv > 0 && mcu > 0 && mcu%restartIv == 0 {
				br.restart()
			}
			restarted := restartIv > 0 && mcu%restartIv == 0
			for c := 0; c < nc; c++ {
				s, err := br.decodeHuff(tables[c])
				if err != nil {
					return nil, err
				}
				if s > 16 {
					return nil, errGoJPEGMalformed
				}
				// SSSS=16 is the special max category: the difference is -32768
				// and no additional bits are sent (T.81 lossless).
				var diff int
				if s == 16 {
					diff = -32768
				} else {
					diff = extend(br.receive(int(s)), int(s))
				}

				var px int
				switch {
				case restarted, x == 0 && y == 0:
					px = defaultPx
				case y == 0:
					px = at(c, x-1, y) // first line: predictor 1 (Ra)
				case x == 0:
					px = at(c, x, y-1) // first column: predictor 2 (Rb)
				default:
					ra := at(c, x-1, y)
					rb := at(c, x, y-1)
					rc := at(c, x-1, y-1)
					px = predict(frame.predictor, ra, rb, rc)
				}
				out[(y*w+x)*nc+c] = (px + diff) & mask
			}
			mcu++
		}
	}
	return out, nil
}

func predict(sel, ra, rb, rc int) int {
	switch sel {
	case 1:
		return ra
	case 2:
		return rb
	case 3:
		return rc
	case 4:
		return ra + rb - rc
	case 5:
		return ra + (rb-rc)>>1
	case 6:
		return rb + (ra-rc)>>1
	case 7:
		return (ra + rb) >> 1
	}
	return ra
}

// decodeLosslessInto decodes a lossless JPEG and packs it into output using the
// codec's little-endian convention: 1 byte/sample when precision <= 8, else 2.
func decodeLosslessInto(encoded, output []byte) error {
	frame, samples, err := decodeLossless(encoded)
	if err != nil {
		return err
	}
	nc := len(frame.comps)
	bytesPerSample := 1
	if frame.precision > 8 {
		bytesPerSample = 2
	}
	need := frame.width * frame.height * nc * bytesPerSample
	if need > len(output) {
		return errGoJPEGOutputSize
	}
	if bytesPerSample == 1 {
		for idx, s := range samples {
			output[idx] = byte(s)
		}
		return nil
	}
	for idx, s := range samples {
		output[idx*2] = byte(s)
		output[idx*2+1] = byte(s >> 8)
	}
	return nil
}

// gojpegBackend is the pure-Go JPEG backend. It decodes lossless SOF3 at any
// supported precision; DCT profiles fall through to errGoJPEGUnsupported (the
// caller then tries libjpeg if present). Encode is not implemented in pure Go.
type gojpegBackend struct{}

func (gojpegBackend) Name() string { return "gojpeg" }

func (gojpegBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (gojpegBackend) Decode8Context(_ context.Context, encoded []byte, output []byte) error {
	return gojpegDecodeInto(encoded, output)
}

func (gojpegBackend) Decode12(encoded []byte, output []byte) error {
	return gojpegDecodeInto(encoded, output)
}

func (gojpegBackend) Decode16(encoded []byte, output []byte) error {
	return gojpegDecodeInto(encoded, output)
}

// Encode12 produces JPEG Extended Sequential (SOF1, lossy DCT) for the DICOM
// JPEG Extended 12-bit transfer syntax — the correct encoding for that syntax.
// Grayscale only (the transcoder always encodes a single component here); a
// 3-component request falls back to the lossless encoder.
func (gojpegBackend) Encode12(raw []byte, width uint16, height uint16, samples uint16, _ int) ([]byte, error) {
	if samples == 1 {
		return encodeDCTJPEG(raw, int(width), int(height), 12, defaultDCTQuality)
	}
	return encodeLosslessJPEG(raw, int(width), int(height), int(samples), 12)
}

// Encode16 produces lossless SOF3 JPEG (predictor 1) for JPEG Lossless 16-bit,
// matching the libjpeg backend's lossless encode; it round-trips exactly.
func (gojpegBackend) Encode16(raw []byte, width uint16, height uint16, samples uint16, _ int) ([]byte, error) {
	return encodeLosslessJPEG(raw, int(width), int(height), int(samples), 16)
}

// gojpegBackendPriority is below the cgo libjpeg backend so libjpeg wins when
// both are compiled in, while gojpeg is selected automatically otherwise.
const gojpegBackendPriority = 0

func registerPureGoBackend() {
	_ = RegisterBackendWithPriority("gojpeg", func() Backend { return gojpegBackend{} }, gojpegBackendPriority)
}
