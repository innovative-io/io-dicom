package gojxl

// assembleJPEGVarDCT writes a JPEG-mode VarDCT codestream: non-XYB YCbCr, RAW
// (JPEG) dequant table, chroma-from-luma disabled (cmap=0), all DCT-8 blocks.
// It is the encode counterpart of decodeVarDCTFrame + populateJPEGCoefficients
// for the 4:4:4 case.
func assembleJPEGVarDCT(w, h, bw, bh int, gray bool, cs [3]uint32, csHShift, csVShift [3]int,
	dcMod [3][]int32, acCoeff [3][][]int32, qt []int32, denom float32) ([]byte, error) {

	fd := computeFrameDimensions(uint32(w), uint32(h), 1, 1)
	numGroups := int(fd.numGroups)

	// Natural DCT-8 coefficient order (used_orders = 0); this is the JXL scan
	// order, not the identity, and must match the decoder.
	order := naturalCoeffOrder(1, 1)

	// Per-DC-group tokens. Each DC group covers a dcGroupDim-blocks (2048-pixel)
	// region of the luma DC grid and is an independent pair of modular
	// sub-streams (DC image + AC metadata), Gradient-predicted within the group.
	// All DC groups + the quant table share one global histogram.
	dcStore := [3]int{1, 0, 2}
	dcGroupDim := int(fd.groupDim)
	ndgx := divCeilInt(bw, dcGroupDim)
	numDCGroups := int(fd.numDCGroups)
	type dcGroupTok struct {
		dcTk, acmTk []encTokenized
		count       int
		acmAlphabet int
	}
	dcGroups := make([]dcGroupTok, numDCGroups)
	modAlphabet := 0
	var modTkGroups [][]encTokenized // every modular stream, for the shared histogram
	for g := 0; g < numDCGroups; g++ {
		gx, gy := g%ndgx, g/ndgx
		bx0, by0 := gx*dcGroupDim, gy*dcGroupDim
		gbw := minInt(dcGroupDim, bw-bx0)
		gbh := minInt(dcGroupDim, bh-by0)

		var dcTokens []encToken
		for p := 0; p < 3; p++ {
			jc := dcStore[p]
			hs, vs := csHShift[jc], csVShift[jc]
			cbw := bw >> uint(hs)
			pbx0, pby0 := bx0>>uint(hs), by0>>uint(vs)
			cgbw := divCeilInt(gbw, 1<<uint(hs))
			cgbh := divCeilInt(gbh, 1<<uint(vs))
			dcTokens = gradientTokens(dcMod[p], cbw, pbx0, pby0, cgbw, cgbh, dcTokens)
		}
		gctW, gctH := (gbw+7)>>3, (gbh+7)>>3
		count := gbw * gbh
		var acmTokens []encToken
		acmTokens = gradientTokens(make([]int32, gctW*gctH), gctW, 0, 0, gctW, gctH, acmTokens) // ytox = 0
		acmTokens = gradientTokens(make([]int32, gctW*gctH), gctW, 0, 0, gctW, gctH, acmTokens) // ytob = 0
		acmTokens = gradientTokens(make([]int32, count*2), count, 0, 0, count, 2, acmTokens)    // ACS=0, QF=0
		acmTokens = gradientTokens(make([]int32, gbw*gbh), gbw, 0, 0, gbw, gbh, acmTokens)      // EPF = 0

		dcTk, dcMax := tokenizeAll(dcTokens)
		acmTk, acmMax := tokenizeAll(acmTokens)
		dcGroups[g] = dcGroupTok{dcTk: dcTk, acmTk: acmTk, count: count, acmAlphabet: acmMax + 1}
		modAlphabet = maxInt(modAlphabet, dcMax)
		// The shared global modular histogram is sized from the DC-image tokens
		// only. The AC-metadata tokens (tens of thousands of zeros) get their own
		// local degenerate histogram instead (see writeDCGroup), so they neither
		// skew the DC histogram nor pay for it.
		modTkGroups = append(modTkGroups, dcTk)
	}
	var qtTokens []encToken
	for c := 0; c < 3; c++ {
		qtTokens = gradientTokens(qt[c*64:c*64+64], 8, 0, 0, 8, 8, qtTokens)
	}
	qtTk, qtMax := tokenizeAll(qtTokens)
	modAlphabet = maxInt(modAlphabet, qtMax) + 1
	modTkGroups = append(modTkGroups, qtTk)

	perGroupAC, acMax, _ := acTokensByGroupJPEGCtx(fd, bw, bh, acCoeff, order, csHShift, csVShift)
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
	modRev, modFreq := writeANSRealHeader(lf, modAlphabet, 1, modTkGroups...)

	// DC group sections: DC image + AC metadata, one per DC group.
	writeDCGroup := func(dcw *bitWriter, g int) {
		dcw.WriteBits(0, 2) // extra_precision = 0
		writeModularGroupHeader(dcw)
		encodeANSData(dcw, ansEncState{tokens: dcGroups[g].dcTk, revMap: modRev, freqs: modFreq})
		dcw.WriteBits(uint64(dcGroups[g].count-1), ceilLog2Nonzero(uint32(dcGroups[g].count)))
		// AC metadata is all zeros; give it a local tree + its own (degenerate)
		// histogram so its tens of thousands of zero tokens cost almost nothing and
		// don't share — and skew — the DC image's global histogram.
		writeModularGroupHeaderLocal(dcw)
		encodeANSStream(dcw, maTreeTokens, numTreeContexts)
		acmRev, acmFreq := writeANSRealHeader(dcw, dcGroups[g].acmAlphabet, 1, dcGroups[g].acmTk)
		encodeANSData(dcw, ansEncState{tokens: dcGroups[g].acmTk, revMap: acmRev, freqs: acmFreq})
	}

	// HfGlobal: RAW dequant matrices + coeff orders + clustered AC histograms.
	writeHfGlobal := func(hf *bitWriter) (ctxToCluster []uint8, clRev [][][]uint16, clFreq [][]uint16) {
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
		return writeACEntropyHeader(hf, perGroupAC, acAlphabet, acCtxCount)
	}

	if single {
		writeDCGroup(lf, 0)
		ctxToCluster, clRev, clFreq := writeHfGlobal(lf)
		encodeACDataMulti(lf, perGroupAC[0], ctxToCluster, clRev, clFreq)
		return finishJPEGVarDCT(w, h, gray, cs, [][]byte{lf.Bytes()}, fd)
	}

	// Section order: LfGlobal, DC groups, HfGlobal, AC groups.
	sections := make([][]byte, 0, 2+numDCGroups+numGroups)
	sections = append(sections, lf.Bytes())
	for g := 0; g < numDCGroups; g++ {
		dcw := newBitWriter()
		writeDCGroup(dcw, g)
		sections = append(sections, dcw.Bytes())
	}
	hf := newBitWriter()
	ctxToCluster, clRev, clFreq := writeHfGlobal(hf)
	sections = append(sections, hf.Bytes())
	for g := 0; g < numGroups; g++ {
		gw := newBitWriter()
		encodeACDataMulti(gw, perGroupAC[g], ctxToCluster, clRev, clFreq)
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
