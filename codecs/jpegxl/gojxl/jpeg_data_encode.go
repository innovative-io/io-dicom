package gojxl

import "github.com/innovative-io/io-dicom/codecs/brotli"

// encodeJPEGData serializes a jpegData into a jbrd box body: the bit-coded
// Fields header (the inverse of visitFields) followed, after byte alignment, by
// a Brotli stream of the unknown-APP / COM / inter-marker / tail bytes. It is
// the inverse of decodeJPEGData.
func encodeJPEGData(jd *jpegData) ([]byte, error) {
	w := newBitWriter()
	if err := writeJPEGFields(jd, w); err != nil {
		return nil, err
	}
	w.ZeroPadToByte()
	out := w.Bytes()

	var blob []byte
	for i, m := range jd.appData {
		if i < len(jd.appMarkerType) && jd.appMarkerType[i] != appMarkerUnknown {
			continue
		}
		blob = append(blob, m...)
	}
	for _, m := range jd.comData {
		blob = append(blob, m...)
	}
	for _, m := range jd.interMarkerData {
		blob = append(blob, m...)
	}
	blob = append(blob, jd.tailData...)
	out = append(out, brotli.Compress(blob)...)
	return out, nil
}

// writeJPEGFields is the write direction of JPEGData::VisitFields.
func writeJPEGFields(jd *jpegData, w *bitWriter) error {
	w.WriteBool(len(jd.components) == 1)

	hasDRI := false
	for _, m := range jd.markerOrder {
		w.WriteBits(uint64(m-0xC0), 6)
		if m == 0xDD {
			hasDRI = true
		}
	}

	for i := range jd.appData {
		t := uint32(appMarkerUnknown)
		if i < len(jd.appMarkerType) {
			t = jd.appMarkerType[i]
		}
		w.WriteU32(t, u32Val(0), u32Val(1), u32Off(1, 2), u32Off(2, 4))
		w.WriteBits(uint64(len(jd.appData[i])-1), 16)
	}
	for _, m := range jd.comData {
		w.WriteBits(uint64(len(m)-1), 16)
	}

	w.WriteU32(uint32(len(jd.quant)), u32Val(1), u32Val(2), u32Val(3), u32Val(4))
	for i := range jd.quant {
		w.WriteBits(uint64(jd.quant[i].precision), 1)
		w.WriteBits(uint64(jd.quant[i].index), 2)
		w.WriteBool(jd.quant[i].isLast)
	}

	// Component layout: classify into gray / YCbCr / RGB / custom.
	ct := jpegComponentType(jd.components)
	w.WriteBits(uint64(ct), 2)
	if ct == 3 {
		w.WriteU32(uint32(len(jd.components)), u32Val(1), u32Val(2), u32Val(3), u32Val(4))
		for i := range jd.components {
			w.WriteBits(uint64(jd.components[i].id), 8)
		}
	}
	for i := range jd.components {
		w.WriteBits(uint64(jd.components[i].quantIdx), 2)
	}

	w.WriteU32(uint32(len(jd.huffmanCode)), u32Val(4), u32Off(3, 2), u32Off(4, 10), u32Off(6, 26))
	for hi := range jd.huffmanCode {
		hc := &jd.huffmanCode[hi]
		isAC := (hc.slotID >> 4) != 0
		w.WriteBool(isAC)
		w.WriteBits(uint64(hc.slotID&0xF), 2)
		w.WriteBool(hc.isLast)
		var numSymbols uint32
		for i := 0; i <= 16; i++ {
			w.WriteU32(hc.counts[i], u32Val(0), u32Val(1), u32Off(3, 2), u32Bits(8))
			numSymbols += hc.counts[i]
		}
		for i := uint32(0); i < numSymbols; i++ {
			w.WriteU32(hc.values[i], u32Bits(2), u32Off(2, 4), u32Off(4, 8), u32Off(8, 1))
		}
	}

	for si := range jd.scanInfo {
		sc := &jd.scanInfo[si]
		w.WriteU32(sc.numComponents, u32Val(1), u32Val(2), u32Val(3), u32Val(4))
		w.WriteBits(uint64(sc.Ss), 6)
		w.WriteBits(uint64(sc.Se), 6)
		w.WriteBits(uint64(sc.Al), 4)
		w.WriteBits(uint64(sc.Ah), 4)
		for i := uint32(0); i < sc.numComponents; i++ {
			w.WriteBits(uint64(sc.components[i].compIdx), 2)
			w.WriteBits(uint64(sc.components[i].acTblIdx), 2)
			w.WriteBits(uint64(sc.components[i].dcTblIdx), 2)
		}
		w.WriteU32(sc.lastNeededPass, u32Val(0), u32Val(1), u32Val(2), u32Off(3, 3))
	}

	if hasDRI {
		w.WriteBits(uint64(jd.restartInterval), 16)
	}

	for si := range jd.scanInfo {
		sc := &jd.scanInfo[si]
		w.WriteU32(uint32(len(sc.resetPoints)), u32Val(0), u32Off(2, 1), u32Off(4, 4), u32Off(16, 20))
		last := -1
		for _, bi := range sc.resetPoints {
			delta := int(bi) - last - 1
			w.WriteU32(uint32(delta), u32Val(0), u32Off(3, 1), u32Off(5, 9), u32Off(28, 41))
			last = int(bi)
		}
		w.WriteU32(uint32(len(sc.extraZeroRuns)), u32Val(0), u32Off(2, 1), u32Off(4, 4), u32Off(16, 20))
		last = -1
		for _, ez := range sc.extraZeroRuns {
			w.WriteU32(ez.numExtraZeroRun, u32Val(1), u32Off(2, 2), u32Off(4, 5), u32Off(8, 20))
			delta := int(ez.blockIdx) - last - 1
			w.WriteU32(uint32(delta), u32Val(0), u32Off(3, 1), u32Off(5, 9), u32Off(28, 41))
			last = int(ez.blockIdx)
		}
	}

	for _, m := range jd.interMarkerData {
		w.WriteBits(uint64(len(m)), 16)
	}

	w.WriteU32(uint32(len(jd.tailData)), u32Val(0), u32Off(8, 1), u32Off(16, 257), u32Off(22, 65793))

	w.WriteBool(jd.hasZeroPadding)
	if jd.hasZeroPadding {
		w.WriteBits(uint64(len(jd.paddingBits)), 24)
		for _, b := range jd.paddingBits {
			w.WriteBool(b)
		}
	}
	return nil
}

// jpegComponentType classifies the components the same way VisitFields reads it.
func jpegComponentType(comps []jpegComponent) int {
	if len(comps) == 1 && comps[0].id == 1 {
		return 0 // gray
	}
	if len(comps) == 3 && comps[0].id == 1 && comps[1].id == 2 && comps[2].id == 3 {
		return 1 // YCbCr
	}
	if len(comps) == 3 && comps[0].id == 'R' && comps[1].id == 'G' && comps[2].id == 'B' {
		return 2 // RGB
	}
	return 3 // custom
}
