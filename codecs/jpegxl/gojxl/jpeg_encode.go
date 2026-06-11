package gojxl

import (
	"errors"
	"math/bits"
)

// This file ports libjxl's JPEG bitstream serializer (dec_jpeg_data_writer.cc)
// for the baseline-sequential case: it turns a populated jpegData (markers,
// Huffman/quant tables, scan info and quantized DCT coefficients) back into a
// byte-exact JPEG file.

const kJpegPrecision = 8

// kJpegNaturalOrder maps a zigzag position to the natural (row-major) index of
// the 8x8 coefficient block.
var kJpegNaturalOrder = [64]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}

// huffEncTable is a built JPEG Huffman encoding table (symbol -> code/length).
type huffEncTable struct {
	depth       [256]uint8
	code        [256]uint32
	initialized bool
}

// buildHuffEncTable assigns canonical JPEG Huffman codes from a JPEGHuffmanCode
// (BuildHuffmanCodeTable).
func buildHuffEncTable(hc *jpegHuffmanCode, t *huffEncTable) error {
	var size [257]int
	p := 0
	for l := 1; l <= jpegHuffmanMaxBitLength; l++ {
		n := int(hc.counts[l])
		if p+n > jpegHuffmanAlphabetSize+1 {
			return errors.New("gojxl: invalid huffman counts")
		}
		for ; n > 0; n-- {
			size[p] = l
			p++
		}
	}
	if p == 0 {
		return nil
	}
	lastP := p - 1
	size[lastP] = 0

	var huffCode [257]uint32
	code := uint32(0)
	si := size[0]
	p = 0
	for size[p] != 0 {
		for size[p] == si {
			huffCode[p] = code
			code++
			p++
		}
		code <<= 1
		si++
	}
	for p = 0; p < lastP; p++ {
		v := hc.values[p]
		t.depth[v] = uint8(size[p])
		t.code[v] = huffCode[p]
	}
	return nil
}

// jpegBitWriter writes JPEG entropy data MSB-first with 0xFF byte stuffing.
type jpegBitWriter struct {
	out []byte
	acc uint64
	cnt uint // number of valid bits buffered in the low `cnt` positions of acc
}

func (w *jpegBitWriter) writeBits(n uint, code uint32) {
	if n == 0 {
		return
	}
	w.acc = (w.acc << n) | uint64(code&((1<<n)-1))
	w.cnt += n
	for w.cnt >= 8 {
		w.cnt -= 8
		b := byte(w.acc >> w.cnt)
		w.out = append(w.out, b)
		if b == 0xFF {
			w.out = append(w.out, 0x00)
		}
	}
}

func (w *jpegBitWriter) writeSymbol(sym int, t *huffEncTable) {
	w.writeBits(uint(t.depth[sym]), t.code[sym])
}

func (w *jpegBitWriter) writeSymbolBits(sym int, t *huffEncTable, nbits uint, val uint32) {
	w.writeBits(nbits+uint(t.depth[sym]), val|(t.code[sym]<<nbits))
}

// padToByte flushes to a byte boundary, consuming explicit padding bits when the
// JPEG used non-trivial padding, otherwise padding with 1 bits (standard JPEG).
func (w *jpegBitWriter) padToByte(pad []bool, padPos *int) error {
	rem := w.cnt & 7
	if rem == 0 {
		return nil
	}
	npad := 8 - rem
	var v uint32
	if pad == nil {
		v = (1 << npad) - 1
	} else {
		for i := uint(0); i < npad; i++ {
			v <<= 1
			if *padPos >= len(pad) {
				return errors.New("gojxl: not enough padding bits")
			}
			if pad[*padPos] {
				v |= 1
			}
			*padPos++
		}
	}
	w.writeBits(npad, v)
	return nil
}

// encodeBlockSequential Huffman-encodes one 8x8 block (EncodeDCTBlockSequential).
func (w *jpegBitWriter) encodeBlockSequential(coeffs []int16, dcHuff, acHuff *huffEncTable, numZeroRuns int, lastDC *int32) {
	diff := int32(coeffs[0]) - *lastDC
	*lastDC = int32(coeffs[0])
	dcNbits := 0
	if diff != 0 {
		m := diff
		if m < 0 {
			m = -m
		}
		dcNbits = bits.Len32(uint32(m))
	}
	w.writeSymbol(dcNbits, dcHuff)
	if dcNbits > 0 {
		v := diff
		if v < 0 {
			v--
		}
		w.writeBits(uint(dcNbits), uint32(v)&((1<<uint(dcNbits))-1))
	}

	r := 0
	for i := 1; i < 64; i++ {
		t := int32(coeffs[kJpegNaturalOrder[i]])
		if t == 0 {
			r++
			continue
		}
		for r > 15 {
			w.writeSymbol(0xF0, acHuff)
			r -= 16
		}
		m := t
		if m < 0 {
			m = -m
		}
		acNbits := bits.Len32(uint32(m))
		sym := (r << 4) | acNbits
		v := t
		if v < 0 {
			v--
		}
		w.writeSymbolBits(sym, acHuff, uint(acNbits), uint32(v)&((1<<uint(acNbits))-1))
		r = 0
	}
	for i := 0; i < numZeroRuns; i++ {
		w.writeSymbol(0xF0, acHuff)
		r -= 16
	}
	if r > 0 {
		w.writeSymbol(0, acHuff)
	}
}

// jpegSerializer holds the running marker/table state.
type jpegSerializer struct {
	jd        *jpegData
	out       []byte
	dcTables  [4]huffEncTable
	acTables  [4]huffEncTable
	dqtIndex  int
	dhtIndex  int
	appIndex  int
	comIndex  int
	dataIndex int
	scanIndex int
	seenDRI   bool
	isProg    bool
	padBits   []bool
	padPos    int
}

func be16(v int) (byte, byte) { return byte(v >> 8), byte(v & 0xFF) }

// EncodeJPEG serializes a jpegData (as produced by DecodeJPEGFromJXL) into the
// original JPEG file bytes. Only baseline-sequential scans are supported.
func EncodeJPEG(jd *jpegData) ([]byte, error) {
	s := &jpegSerializer{jd: jd}
	if jd.hasZeroPadding {
		s.padBits = jd.paddingBits
	}
	if len(jd.markerOrder) == 0 {
		return nil, errors.New("gojxl: empty marker order")
	}
	s.out = append(s.out, 0xFF, 0xD8) // SOI
	for _, m := range jd.markerOrder {
		if err := s.serializeSection(m); err != nil {
			return nil, err
		}
	}
	return s.out, nil
}

func (s *jpegSerializer) serializeSection(marker uint8) error {
	switch {
	case marker == 0xC0 || marker == 0xC1 || marker == 0xC2 || marker == 0xC9 || marker == 0xCA:
		if marker <= 0xC2 {
			s.isProg = marker == 0xC2
		}
		return s.encodeSOF(marker)
	case marker == 0xC4:
		return s.encodeDHT()
	case marker >= 0xD0 && marker <= 0xD7:
		s.out = append(s.out, 0xFF, marker)
		return nil
	case marker == 0xD9:
		s.out = append(s.out, 0xFF, 0xD9)
		s.out = append(s.out, s.jd.tailData...)
		return nil
	case marker == 0xDA:
		return s.encodeScan()
	case marker == 0xDB:
		return s.encodeDQT()
	case marker == 0xDD:
		s.seenDRI = true
		hi, lo := be16(int(s.jd.restartInterval))
		s.out = append(s.out, 0xFF, 0xDD, 0, 4, hi, lo)
		return nil
	case marker >= 0xE0 && marker <= 0xEF:
		if s.appIndex >= len(s.jd.appData) {
			return errors.New("gojxl: app marker overflow")
		}
		s.out = append(s.out, 0xFF)
		s.out = append(s.out, s.jd.appData[s.appIndex]...)
		s.appIndex++
		return nil
	case marker == 0xFE:
		if s.comIndex >= len(s.jd.comData) {
			return errors.New("gojxl: com marker overflow")
		}
		s.out = append(s.out, 0xFF)
		s.out = append(s.out, s.jd.comData[s.comIndex]...)
		s.comIndex++
		return nil
	case marker == 0xFF:
		if s.dataIndex >= len(s.jd.interMarkerData) {
			return errors.New("gojxl: inter-marker overflow")
		}
		s.out = append(s.out, s.jd.interMarkerData[s.dataIndex]...)
		s.dataIndex++
		return nil
	}
	return errors.New("gojxl: unknown marker in order")
}

func (s *jpegSerializer) encodeSOF(marker uint8) error {
	jd := s.jd
	n := len(jd.components)
	markerLen := 8 + 3*n
	hi, lo := be16(markerLen)
	hh, hl := be16(jd.height)
	wh, wl := be16(jd.width)
	s.out = append(s.out, 0xFF, marker, hi, lo, kJpegPrecision, hh, hl, wh, wl, byte(n))
	for i := 0; i < n; i++ {
		c := jd.components[i]
		if int(c.quantIdx) >= len(jd.quant) {
			return errors.New("gojxl: bad quant idx")
		}
		s.out = append(s.out, byte(c.id), byte((c.hSampFactor<<4)|c.vSampFactor), byte(jd.quant[c.quantIdx].index))
	}
	return nil
}

func (s *jpegSerializer) encodeDQT() error {
	jd := s.jd
	markerLen := 2
	for i := s.dqtIndex; i < len(jd.quant); i++ {
		markerLen += 1 + 64
		if jd.quant[i].precision != 0 {
			markerLen += 64
		}
		if jd.quant[i].isLast {
			break
		}
	}
	hi, lo := be16(markerLen)
	s.out = append(s.out, 0xFF, 0xDB, hi, lo)
	for {
		if s.dqtIndex >= len(jd.quant) {
			return errors.New("gojxl: dqt overflow")
		}
		q := &jd.quant[s.dqtIndex]
		s.dqtIndex++
		s.out = append(s.out, byte((q.precision<<4)|q.index))
		for i := 0; i < 64; i++ {
			val := q.values[kJpegNaturalOrder[i]]
			if q.precision != 0 {
				s.out = append(s.out, byte(val>>8))
			}
			s.out = append(s.out, byte(val&0xFF))
		}
		if q.isLast {
			break
		}
	}
	return nil
}

func (s *jpegSerializer) encodeDHT() error {
	jd := s.jd
	markerLen := 2
	for i := s.dhtIndex; i < len(jd.huffmanCode); i++ {
		hc := &jd.huffmanCode[i]
		markerLen += jpegHuffmanMaxBitLength
		for j := 0; j < len(hc.counts); j++ {
			markerLen += int(hc.counts[j])
		}
		if hc.isLast {
			break
		}
	}
	hi, lo := be16(markerLen)
	s.out = append(s.out, 0xFF, 0xC4, hi, lo)
	for {
		if s.dhtIndex >= len(jd.huffmanCode) {
			return errors.New("gojxl: dht overflow")
		}
		hc := &jd.huffmanCode[s.dhtIndex]
		s.dhtIndex++
		index := hc.slotID
		var t *huffEncTable
		if index&0x10 != 0 {
			t = &s.acTables[index-0x10]
		} else {
			t = &s.dcTables[index]
		}
		if err := buildHuffEncTable(hc, t); err != nil {
			return err
		}
		t.initialized = true

		total := 0
		maxLen := 0
		for i := 0; i < len(hc.counts); i++ {
			if hc.counts[i] != 0 {
				maxLen = i
			}
			total += int(hc.counts[i])
		}
		total--
		s.out = append(s.out, byte(hc.slotID))
		for i := 1; i <= jpegHuffmanMaxBitLength; i++ {
			if i == maxLen {
				s.out = append(s.out, byte(hc.counts[i]-1))
			} else {
				s.out = append(s.out, byte(hc.counts[i]))
			}
		}
		for i := 0; i < total; i++ {
			s.out = append(s.out, byte(hc.values[i]))
		}
		if hc.isLast {
			break
		}
	}
	return nil
}

func (s *jpegSerializer) encodeScan() error {
	jd := s.jd
	if s.scanIndex >= len(jd.scanInfo) {
		return errors.New("gojxl: scan overflow")
	}
	scan := &jd.scanInfo[s.scanIndex]
	s.scanIndex++

	Ss, Se, Al := int(scan.Ss), int(scan.Se), int(scan.Al)
	if s.isProg || Ss != 0 || Se != 63 || Al != 0 || scan.Ah != 0 {
		return errors.New("gojxl: only baseline-sequential scans are supported")
	}

	// SOS marker.
	nScans := int(scan.numComponents)
	markerLen := 6 + 2*nScans
	hi, lo := be16(markerLen)
	s.out = append(s.out, 0xFF, 0xDA, hi, lo, byte(nScans))
	for i := 0; i < nScans; i++ {
		si := scan.components[i]
		if int(si.compIdx) >= len(jd.components) {
			return errors.New("gojxl: bad scan comp idx")
		}
		s.out = append(s.out, byte(jd.components[si.compIdx].id), byte((si.dcTblIdx<<4)+si.acTblIdx))
	}
	s.out = append(s.out, byte(scan.Ss), byte(scan.Se), byte((scan.Ah<<4)|scan.Al))

	// Entropy-coded data.
	restartInterval := 0
	if s.seenDRI {
		restartInterval = int(s.jd.restartInterval)
	}
	mcusPerRow, mcuRows := jd.calculateMcuSize(scan)
	isInterleaved := scan.numComponents > 1

	w := &jpegBitWriter{}
	var lastDC [4]int32
	blockScanIndex := 0
	restartsToGo := restartInterval
	nextRestart := 0
	// reset points and extra-zero-runs.
	nextResetPos := 0
	nextReset := -1
	if len(scan.resetPoints) > 0 {
		nextReset = int(scan.resetPoints[0])
		nextResetPos = 1
	}
	ezrPos := 0
	nextEZR := -1
	if len(scan.extraZeroRuns) > 0 {
		nextEZR = int(scan.extraZeroRuns[0].blockIdx)
	}

	for my := 0; my < mcuRows; my++ {
		for mx := 0; mx < mcusPerRow; mx++ {
			if restartInterval > 0 && restartsToGo == 0 {
				if err := w.padToByte(s.padBits, &s.padPos); err != nil {
					return err
				}
				w.out = append(w.out, 0xFF, byte(0xD0+nextRestart))
				nextRestart = (nextRestart + 1) & 7
				restartsToGo = restartInterval
				lastDC = [4]int32{}
			}
			for i := 0; i < nScans; i++ {
				si := scan.components[i]
				c := &jd.components[si.compIdx]
				dcHuff := &s.dcTables[si.dcTblIdx]
				acHuff := &s.acTables[si.acTblIdx]
				if !dcHuff.initialized || !acHuff.initialized {
					return errors.New("gojxl: huffman table used before defined")
				}
				nby, nbx := 1, 1
				if isInterleaved {
					nby, nbx = c.vSampFactor, c.hSampFactor
				}
				for iy := 0; iy < nby; iy++ {
					for ix := 0; ix < nbx; ix++ {
						by := my*nby + iy
						bx := mx*nbx + ix
						blockIdx := by*int(c.widthInBlocks) + bx
						if blockScanIndex == nextReset {
							// Sequential mode has no buffered EOB run to flush.
							if nextResetPos < len(scan.resetPoints) {
								nextReset = int(scan.resetPoints[nextResetPos])
								nextResetPos++
							} else {
								nextReset = -1
							}
						}
						numZeroRuns := 0
						if blockScanIndex == nextEZR {
							numZeroRuns = int(scan.extraZeroRuns[ezrPos].numExtraZeroRun)
							ezrPos++
							if ezrPos < len(scan.extraZeroRuns) {
								nextEZR = int(scan.extraZeroRuns[ezrPos].blockIdx)
							} else {
								nextEZR = -1
							}
						}
						off := blockIdx << 6
						if off+64 > len(c.coeffs) {
							return errors.New("gojxl: block index out of range")
						}
						w.encodeBlockSequential(c.coeffs[off:off+64], dcHuff, acHuff, numZeroRuns, &lastDC[si.compIdx])
						blockScanIndex++
					}
				}
			}
			restartsToGo--
		}
	}
	if err := w.padToByte(s.padBits, &s.padPos); err != nil {
		return err
	}
	s.out = append(s.out, w.out...)
	return nil
}

// calculateMcuSize ports JPEGData::CalculateMcuSize.
func (jd *jpegData) calculateMcuSize(scan *jpegScanInfo) (mcusPerRow, mcuRows int) {
	isInterleaved := scan.numComponents > 1
	base := &jd.components[scan.components[0].compIdx]
	hGroup, vGroup := 1, 1
	if !isInterleaved {
		hGroup = base.hSampFactor
		vGroup = base.vSampFactor
	}
	maxH, maxV := 1, 1
	for i := range jd.components {
		if jd.components[i].hSampFactor > maxH {
			maxH = jd.components[i].hSampFactor
		}
		if jd.components[i].vSampFactor > maxV {
			maxV = jd.components[i].vSampFactor
		}
	}
	mcusPerRow = divCeilInt(jd.width*hGroup, 8*maxH)
	mcuRows = divCeilInt(jd.height*vGroup, 8*maxV)
	return
}
