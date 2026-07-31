package gojxl

import (
	"errors"

	"github.com/innovative-io/io-dicom/codecs/brotli"
)

// This file ports libjxl's JPEGData serialization (lib/jxl/jpeg/jpeg_data.cc
// VisitFields and dec_jpeg_data.cc DecodeJPEGData) used by the JPEG XL
// "JPEG bitstream reconstruction" (jbrd) box. Together with the codestream's
// JPEG-mode VarDCT frame it allows bit-exact reconstruction of the original
// JPEG for transfer syntax 1.2.840.10008.1.2.4.111.
//
// Only the read direction is implemented. The jbrd box is a bit-coded Fields
// header followed (after byte alignment) by a single Brotli stream carrying the
// unknown APP markers, COM markers, inter-marker data and trailing bytes.

const (
	jpegHuffmanMaxBitLength = 16
	jpegHuffmanAlphabetSize = 256
	jpegDCAlphabetSize      = 12
	jpegMaxNumPasses        = 11 // kMaxNumPasses
)

// App marker types (AppMarkerType). kUnknown markers are stored verbatim in the
// Brotli stream; the others are reconstructed from fixed tags + the JXL ICC.
const (
	appMarkerUnknown = 0
	appMarkerICC     = 1
	appMarkerExif    = 2
	appMarkerXMP     = 3
)

type jpegQuantTable struct {
	values    [64]int32 // filled later from the codestream, not the jbrd fields
	precision uint32
	index     uint32
	isLast    bool
}

type jpegHuffmanCode struct {
	counts [jpegHuffmanMaxBitLength + 1]uint32
	values [jpegHuffmanAlphabetSize + 1]uint32
	slotID int
	isLast bool
}

type jpegComponentScanInfo struct {
	compIdx  uint32
	dcTblIdx uint32
	acTblIdx uint32
}

type jpegExtraZeroRun struct {
	blockIdx        uint32
	numExtraZeroRun uint32
}

type jpegScanInfo struct {
	numComponents  uint32
	Ss, Se, Ah, Al uint32
	components     [4]jpegComponentScanInfo
	lastNeededPass uint32
	resetPoints    []uint32
	extraZeroRuns  []jpegExtraZeroRun
}

type jpegComponent struct {
	id            uint32
	hSampFactor   int
	vSampFactor   int
	quantIdx      uint32
	widthInBlocks uint32
	heightInBlks  uint32
	coeffs        []int16 // filled later from the codestream
}

// jpegData mirrors libjxl jpeg::JPEGData (the reconstruction metadata).
type jpegData struct {
	width, height   int
	restartInterval uint32
	appData         [][]byte
	appMarkerType   []uint32
	comData         [][]byte
	quant           []jpegQuantTable
	huffmanCode     []jpegHuffmanCode
	components      []jpegComponent
	scanInfo        []jpegScanInfo
	markerOrder     []uint8
	interMarkerData [][]byte
	tailData        []byte
	hasZeroPadding  bool
	paddingBits     []bool
}

// extractJBRDBox returns the body of the `jbrd` (JPEG reconstruction data) box
// from a JPEG XL ISO-BMFF container, or nil if the file is a raw codestream or
// has no such box.
func extractJBRDBox(data []byte) []byte {
	if len(data) < 12 || !(data[0] == 0 && data[3] == 0x0C && data[4] == 'J' &&
		data[5] == 'X' && data[6] == 'L' && data[7] == ' ') {
		return nil
	}
	pos := 0
	for pos+8 <= len(data) {
		size := int(be32(data, pos))
		hdr := 8
		boxEnd := 0
		switch {
		case size == 1:
			if pos+16 > len(data) {
				return nil
			}
			large := be64(data, pos+8)
			hdr = 16
			if large == 0 {
				boxEnd = len(data)
			} else {
				boxEnd = pos + int(large)
			}
		case size == 0:
			boxEnd = len(data)
		default:
			boxEnd = pos + size
		}
		if boxEnd < pos+hdr || boxEnd > len(data) {
			return nil
		}
		if string(data[pos+4:pos+8]) == "jbrd" {
			return data[pos+hdr : boxEnd]
		}
		pos = boxEnd
	}
	return nil
}

// decodeJPEGData parses a jbrd box body into a jpegData structure.
func decodeJPEGData(data []byte) (*jpegData, error) {
	b := newBitReader(data)
	jd := &jpegData{}
	if err := jd.visitFields(b); err != nil {
		return nil, err
	}
	if err := b.JumpToByteBoundary(); err != nil {
		return nil, err
	}
	if !b.ok() {
		return nil, errors.New("gojxl: jbrd field data truncated")
	}
	consumed := b.bitsConsumed() / 8

	// The remainder of the box is one Brotli stream holding, in order: the
	// bytes of each unknown APP marker, each COM marker, each inter-marker
	// block, then the tail data. Decode it whole and split by known sizes.
	var need int
	for i, m := range jd.appData {
		if jd.appMarkerType[i] == appMarkerUnknown {
			need += len(m)
		}
	}
	for _, m := range jd.comData {
		need += len(m)
	}
	for _, m := range jd.interMarkerData {
		need += len(m)
	}
	need += len(jd.tailData)

	if consumed > len(data) {
		return nil, errTruncated
	}
	var blob []byte
	if need > 0 {
		var err error
		// The exact output size is known here, so bound the decode to it rather
		// than discovering an oversized stream after materialising it. Without a
		// bound this was an OOM reachable from any received instance carrying a
		// jbrd box.
		blob, err = brotli.DecompressBounded(data[consumed:], need, need)
		if err != nil {
			return nil, err
		}
		if len(blob) != need {
			return nil, errors.New("gojxl: jbrd brotli output size mismatch")
		}
	}
	off := 0
	take := func(n int) []byte { s := blob[off : off+n]; off += n; return s }
	for i := range jd.appData {
		if jd.appMarkerType[i] == appMarkerUnknown {
			copy(jd.appData[i], take(len(jd.appData[i])))
		}
	}
	for i := range jd.comData {
		copy(jd.comData[i], take(len(jd.comData[i])))
	}
	for i := range jd.interMarkerData {
		jd.interMarkerData[i] = append(jd.interMarkerData[i][:0], take(len(jd.interMarkerData[i]))...)
	}
	copy(jd.tailData, take(len(jd.tailData)))
	return jd, nil
}

// visitFields ports JPEGData::VisitFields (read direction only).
func (jd *jpegData) visitFields(b *bitReader) error {
	isGray := b.ReadBool()
	numColor := 3
	if isGray {
		numColor = 1
	}

	// Marker order: 6 bits per marker (value + 0xC0), until EOI (0xD9).
	var numApp, numCom, numScans, numInter int
	hasDRI := false
	for {
		marker := uint8(b.ReadBits(6)) + 0xC0
		jd.markerOrder = append(jd.markerOrder, marker)
		switch {
		case marker&0xF0 == 0xE0:
			numApp++
		case marker == 0xFE:
			numCom++
		case marker == 0xDA:
			numScans++
		case marker == 0xFF:
			numInter++ // fake marker signaling inter-marker data
		case marker == 0xDD:
			hasDRI = true
		}
		if marker == 0xD9 {
			break
		}
		if len(jd.markerOrder) > 16384 {
			return errors.New("gojxl: jbrd too many markers")
		}
	}

	jd.appData = make([][]byte, numApp)
	jd.appMarkerType = make([]uint32, numApp)
	jd.comData = make([][]byte, numCom)
	jd.scanInfo = make([]jpegScanInfo, numScans)

	for i := 0; i < numApp; i++ {
		jd.appMarkerType[i] = b.ReadU32(u32Val(0), u32Val(1), u32Off(1, 2), u32Off(2, 4))
		if jd.appMarkerType[i] > appMarkerXMP {
			return errors.New("gojxl: jbrd unknown app marker type")
		}
		ln := b.ReadBits(16)
		jd.appData[i] = make([]byte, ln+1)
		if len(jd.appData[i]) < 3 {
			return errors.New("gojxl: jbrd invalid app marker size")
		}
	}
	for i := 0; i < numCom; i++ {
		ln := b.ReadBits(16)
		jd.comData[i] = make([]byte, ln+1)
		if len(jd.comData[i]) < 3 {
			return errors.New("gojxl: jbrd invalid com marker size")
		}
	}

	numQuant := b.ReadU32(u32Val(1), u32Val(2), u32Val(3), u32Val(4))
	if numQuant == 4 {
		return errors.New("gojxl: jbrd invalid number of quant tables")
	}
	jd.quant = make([]jpegQuantTable, numQuant)
	for i := range jd.quant {
		jd.quant[i].precision = b.ReadBits(1)
		jd.quant[i].index = b.ReadBits(2)
		jd.quant[i].isLast = b.ReadBool()
	}

	// Component layout.
	componentType := b.ReadBits(2) // 0=gray,1=YCbCr,2=RGB,3=custom
	var numComp uint32
	switch componentType {
	case 0:
		numComp = 1
	case 3:
		numComp = b.ReadU32(u32Val(1), u32Val(2), u32Val(3), u32Val(4))
		if numComp != 1 && numComp != 3 {
			return errors.New("gojxl: jbrd invalid number of components")
		}
	default:
		numComp = 3
	}
	_ = numColor
	jd.components = make([]jpegComponent, numComp)
	switch componentType {
	case 3:
		for i := range jd.components {
			jd.components[i].id = b.ReadBits(8)
		}
	case 0:
		jd.components[0].id = 1
	case 2:
		jd.components[0].id, jd.components[1].id, jd.components[2].id = 'R', 'G', 'B'
	default:
		jd.components[0].id, jd.components[1].id, jd.components[2].id = 1, 2, 3
	}
	for i := range jd.components {
		jd.components[i].quantIdx = b.ReadBits(2)
		if jd.components[i].quantIdx >= numQuant {
			return errors.New("gojxl: jbrd invalid component quant table")
		}
	}

	numHuff := b.ReadU32(u32Val(4), u32Off(3, 2), u32Off(4, 10), u32Off(6, 26))
	jd.huffmanCode = make([]jpegHuffmanCode, numHuff)
	for hi := range jd.huffmanCode {
		hc := &jd.huffmanCode[hi]
		isAC := b.ReadBool()
		id := b.ReadBits(2)
		hc.slotID = int(boolToU32(isAC)<<4) | int(id)
		hc.isLast = b.ReadBool()
		var numSymbols uint32
		for i := 0; i <= 16; i++ {
			hc.counts[i] = b.ReadU32(u32Val(0), u32Val(1), u32Off(3, 2), u32Bits(8))
			numSymbols += hc.counts[i]
		}
		if numSymbols < 1 {
			return errors.New("gojxl: jbrd empty huffman table")
		}
		if int(numSymbols) > len(hc.values) {
			return errors.New("gojxl: jbrd huffman code too large")
		}
		for i := uint32(0); i < numSymbols; i++ {
			hc.values[i] = b.ReadU32(u32Bits(2), u32Off(2, 4), u32Off(4, 8), u32Off(8, 1))
		}
		if hc.values[numSymbols-1] != jpegHuffmanAlphabetSize {
			return errors.New("gojxl: jbrd missing EOI symbol")
		}
	}

	for si := range jd.scanInfo {
		sc := &jd.scanInfo[si]
		sc.numComponents = b.ReadU32(u32Val(1), u32Val(2), u32Val(3), u32Val(4))
		if sc.numComponents >= 4 {
			return errors.New("gojxl: jbrd invalid scan component count")
		}
		sc.Ss = b.ReadBits(6)
		sc.Se = b.ReadBits(6)
		sc.Al = b.ReadBits(4)
		sc.Ah = b.ReadBits(4)
		for i := uint32(0); i < sc.numComponents; i++ {
			sc.components[i].compIdx = b.ReadBits(2)
			if sc.components[i].compIdx >= numComp {
				return errors.New("gojxl: jbrd invalid scan component idx")
			}
			sc.components[i].acTblIdx = b.ReadBits(2)
			sc.components[i].dcTblIdx = b.ReadBits(2)
		}
		sc.lastNeededPass = b.ReadU32(u32Val(0), u32Val(1), u32Val(2), u32Off(3, 3))
	}

	if hasDRI {
		jd.restartInterval = b.ReadBits(16)
	}

	for si := range jd.scanInfo {
		sc := &jd.scanInfo[si]
		numReset := b.ReadU32(u32Val(0), u32Off(2, 1), u32Off(4, 4), u32Off(16, 20))
		sc.resetPoints = make([]uint32, numReset)
		lastBlock := -1
		for k := range sc.resetPoints {
			delta := b.ReadU32(u32Val(0), u32Off(3, 1), u32Off(5, 9), u32Off(28, 41))
			bi := int(delta) + lastBlock + 1
			if uint32(bi) >= 3<<26 {
				return errors.New("gojxl: jbrd invalid reset block id")
			}
			sc.resetPoints[k] = uint32(bi)
			lastBlock = bi
		}
		numEZR := b.ReadU32(u32Val(0), u32Off(2, 1), u32Off(4, 4), u32Off(16, 20))
		sc.extraZeroRuns = make([]jpegExtraZeroRun, numEZR)
		lastBlock = -1
		for k := range sc.extraZeroRuns {
			sc.extraZeroRuns[k].numExtraZeroRun = b.ReadU32(u32Val(1), u32Off(2, 2), u32Off(4, 5), u32Off(8, 20))
			delta := b.ReadU32(u32Val(0), u32Off(3, 1), u32Off(5, 9), u32Off(28, 41))
			bi := int(delta) + lastBlock + 1
			if uint32(bi) > 3<<26 {
				return errors.New("gojxl: jbrd invalid extra-zero-run block id")
			}
			sc.extraZeroRuns[k].blockIdx = uint32(bi)
			lastBlock = bi
		}
	}

	jd.interMarkerData = make([][]byte, numInter)
	for i := 0; i < numInter; i++ {
		ln := b.ReadBits(16)
		jd.interMarkerData[i] = make([]byte, ln)
	}

	tailLen := b.ReadU32(u32Val(0), u32Off(8, 1), u32Off(16, 257), u32Off(22, 65793))
	jd.tailData = make([]byte, tailLen)

	jd.hasZeroPadding = b.ReadBool()
	if jd.hasZeroPadding {
		nbit := b.ReadBits(24)
		jd.paddingBits = make([]bool, nbit)
		for i := range jd.paddingBits {
			jd.paddingBits[i] = b.ReadBool()
		}
	}
	if !b.ok() {
		return errTruncated
	}
	return nil
}

func boolToU32(v bool) uint32 {
	if v {
		return 1
	}
	return 0
}
