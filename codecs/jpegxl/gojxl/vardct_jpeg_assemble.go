package gojxl

// assembleJPEGVarDCT writes a JPEG-mode VarDCT codestream: non-XYB YCbCr, RAW
// (JPEG) dequant table, chroma-from-luma disabled (cmap=0), all DCT-8 blocks.
// It is the encode counterpart of decodeVarDCTFrame + populateJPEGCoefficients
// for the 4:4:4 case.
func assembleJPEGVarDCT(w, h, bw, bh int, gray bool, cs [3]uint32,
	dcMod [3][]int32, acCoeff [3][][]int32, qt []int32, denom float32) ([]byte, error) {

	fd := computeFrameDimensions(uint32(w), uint32(h), 1, 1)
	numGroups := int(fd.numGroups)

	// Natural DCT-8 coefficient order (used_orders = 0); this is the JXL scan
	// order, not the identity, and must match the decoder.
	order := naturalCoeffOrder(1, 1)

	// Modular sub-streams (DC image, AC metadata, quant table) share one flat
	// histogram, so tokenize them together to size the alphabet.
	var dcTokens []encToken
	for c := 0; c < 3; c++ {
		dcTokens = gradientTokens(dcMod[c], bw, 0, 0, bw, bh, dcTokens)
	}
	ctW, ctH := (bw+7)>>3, (bh+7)>>3
	count := bw * bh
	var acmTokens []encToken
	acmTokens = gradientTokens(make([]int32, ctW*ctH), ctW, 0, 0, ctW, ctH, acmTokens)   // ytox = 0
	acmTokens = gradientTokens(make([]int32, ctW*ctH), ctW, 0, 0, ctW, ctH, acmTokens)   // ytob = 0
	acmTokens = gradientTokens(make([]int32, count*2), count, 0, 0, count, 2, acmTokens) // ACS=0, QF=0
	acmTokens = gradientTokens(make([]int32, bw*bh), bw, 0, 0, bw, bh, acmTokens)        // EPF = 0
	var qtTokens []encToken
	for c := 0; c < 3; c++ {
		qtTokens = gradientTokens(qt[c*64:c*64+64], 8, 0, 0, 8, 8, qtTokens)
	}

	dcTk, dcMax := tokenizeAll(dcTokens)
	acmTk, acmMax := tokenizeAll(acmTokens)
	qtTk, qtMax := tokenizeAll(qtTokens)
	modAlphabet := maxInt(maxInt(dcMax, acmMax), qtMax) + 1

	perGroupAC, acMax := acTokensByGroup(fd, bw, bh, acCoeff, order)
	acAlphabet := acMax + 1
	acCtxCount := defaultBlockCtxMap().numACContexts()

	single := numGroups == 1

	// LfGlobal.
	lf := newBitWriter()
	lf.WriteBool(true) // DequantMatrices DC all_default
	lf.WriteU32(kJpegEncGlobalScale, qGlobalScaleDist[0], qGlobalScaleDist[1], qGlobalScaleDist[2], qGlobalScaleDist[3])
	lf.WriteU32(1, qQuantDCDist[0], qQuantDCDist[1], qQuantDCDist[2], qQuantDCDist[3]) // quant_dc = 1
	lf.WriteBits(1, 1)                                                                 // block context map all_default
	// CfL: a custom JPEG-compatible map (base correlations 0, default color
	// factor). The all-default CfL has base_correlation_b != 0 and is rejected by
	// conforming decoders in JPEG mode.
	lf.WriteBits(0, 1) // CfL all_default = false
	lf.WriteU32(kDefaultColorFactor, kColorFactorDist[0], kColorFactorDist[1], kColorFactorDist[2], kColorFactorDist[3])
	lf.WriteBits(uint64(floatToF16(0)), 16) // base_correlation_x = 0
	lf.WriteBits(uint64(floatToF16(0)), 16) // base_correlation_b = 0
	lf.WriteBits(128, 8)                    // ytox_dc = 0
	lf.WriteBits(128, 8)                    // ytob_dc = 0
	lf.WriteBits(1, 1)                      // has_tree
	encodeANSStream(lf, maTreeTokens, numTreeContexts)
	modRev, modFreq := writeANSFlatHeader(lf, modAlphabet, 1)

	// DC group: DC image + AC metadata.
	dcw := lf
	if !single {
		dcw = newBitWriter()
	}
	dcw.WriteBits(0, 2) // extra_precision = 0
	writeModularGroupHeader(dcw)
	encodeANSData(dcw, ansEncState{tokens: dcTk, revMap: modRev, freqs: modFreq})
	dcw.WriteBits(uint64(count-1), ceilLog2Nonzero(uint32(bw*bh)))
	writeModularGroupHeader(dcw)
	encodeANSData(dcw, ansEncState{tokens: acmTk, revMap: modRev, freqs: modFreq})

	// HfGlobal: RAW dequant matrices + coeff orders + AC histogram.
	hf := lf
	if !single {
		hf = newBitWriter()
	}
	hf.WriteBits(0, 1)                          // AC dequant matrices all_default = false
	hf.WriteBits(quantModeRAW, 3)               // table 0 mode = RAW
	hf.WriteBits(uint64(floatToF16(denom)), 16) // qtable_den (F16)
	writeModularGroupHeader(hf)
	encodeANSData(hf, ansEncState{tokens: qtTk, revMap: modRev, freqs: modFreq})
	for i := 1; i < kNumQuantTables; i++ {
		hf.WriteBits(quantModeLibrary, 3) // tables 1..16 = Library (predefined 0, no extra bits)
	}
	hf.WriteBits(0, ceilLog2Nonzero(fd.numGroups))                         // num_histograms - 1 = 0
	hf.WriteU32(0, kOrderEnc[0], kOrderEnc[1], kOrderEnc[2], kOrderEnc[3]) // used_orders = 0
	acRev, acFreq := writeANSFlatHeader(hf, acAlphabet, acCtxCount)

	if single {
		encodeANSData(lf, ansEncState{tokens: perGroupAC[0], revMap: acRev, freqs: acFreq})
		return finishJPEGVarDCT(w, h, gray, cs, [][]byte{lf.Bytes()}, fd)
	}
	sections := make([][]byte, 0, 3+numGroups)
	sections = append(sections, lf.Bytes(), dcw.Bytes(), hf.Bytes())
	for g := 0; g < numGroups; g++ {
		gw := newBitWriter()
		encodeANSData(gw, ansEncState{tokens: perGroupAC[g], revMap: acRev, freqs: acFreq})
		sections = append(sections, gw.Bytes())
	}
	return finishJPEGVarDCT(w, h, gray, cs, sections, fd)
}

// finishJPEGVarDCT writes the non-XYB headers + TOC and assembles the codestream.
func finishJPEGVarDCT(w, h int, gray bool, cs [3]uint32, sections [][]byte, fd frameDimensions) ([]byte, error) {
	m := newBitWriter()
	writeSizeHeader(m, w, h)
	writeJPEGImageMetadata(m, gray)
	m.WriteBool(true) // transform_data all_default
	m.ZeroPadToByte()
	writeJPEGFrameHeader(m, cs)
	m.WriteBits(0, 1) // TOC: no permutation
	m.ZeroPadToByte()
	for _, sec := range sections {
		writeTocSize(m, len(sec))
	}
	m.ZeroPadToByte()
	return assemble(m.Bytes(), sections), nil
}

// wrapJXLContainer builds an ISO-BMFF JPEG XL file from a jbrd box body and a
// raw codestream.
func wrapJXLContainer(jbrd, codestream []byte) []byte {
	out := []byte{
		0x00, 0x00, 0x00, 0x0C, 'J', 'X', 'L', ' ', 0x0D, 0x0A, 0x87, 0x0A, // signature box
		0x00, 0x00, 0x00, 0x14, 'f', 't', 'y', 'p', 'j', 'x', 'l', ' ', 0, 0, 0, 0, 'j', 'x', 'l', ' ', // ftyp
	}
	out = appendJXLBox(out, "jbrd", jbrd)
	out = appendJXLBox(out, "jxlc", codestream)
	return out
}

func appendJXLBox(out []byte, typ string, body []byte) []byte {
	size := 8 + len(body)
	out = append(out, byte(size>>24), byte(size>>16), byte(size>>8), byte(size))
	out = append(out, typ...)
	return append(out, body...)
}
