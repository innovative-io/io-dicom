package gojxl

import "errors"

var errVarDCTIncomplete = errors.New("gojxl: VarDCT decode not yet complete")

// vardctState carries the parsed VarDCT global decoder state between sections as
// the integration is built out.
type vardctState struct {
	sh      SizeHeader
	meta    *ImageMetadata
	fh      FrameHeader
	fd      frameDimensions
	quant   *quantizer
	blockCtx *blockCtxMap
	cmap    colorCorrelation
	tree    []treeNode
	code    *ansCode
	ctxMap  []uint8
	dc      *dcChannels
}

// decodeVarDCTFrame decodes a lossy VarDCT frame. It is being built
// incrementally; until the full pipeline is wired it parses the global sections
// and returns errVarDCTIncomplete.
func decodeVarDCTFrame(data []byte) (*vardctState, error) {
	cs, err := codestream(data)
	if err != nil {
		return nil, err
	}
	if len(cs) < 2 || cs[0] != 0xFF || cs[1] != 0x0A {
		return nil, errors.New("gojxl: bad codestream signature")
	}
	b := newBitReader(cs[2:])
	st := &vardctState{}
	if st.sh, err = readSizeHeader(b); err != nil {
		return nil, err
	}
	meta, err := readImageMetadata(b)
	if err != nil {
		return nil, err
	}
	st.meta = &meta
	if meta.Color.WantICC {
		return nil, errors.New("gojxl: ICC profiles not yet supported")
	}
	readTransformData(b, meta.XYBEncoded)
	if err := b.JumpToByteBoundary(); err != nil {
		return nil, err
	}
	if st.fh, err = readFrameHeader(b, &meta); err != nil {
		return nil, err
	}
	if st.fh.Encoding != frameVarDCT {
		return nil, errors.New("gojxl: not a VarDCT frame")
	}
	st.fd = computeFrameDimensions(st.sh.Xsize, st.sh.Ysize, st.fh.GroupSizeShift, st.fh.Upsampling)
	if _, err = readTOC(b, numTocEntries(st.fd.numGroups, st.fd.numDCGroups, st.fh.NumPasses)); err != nil {
		return nil, err
	}
	if st.fh.Flags != 0 {
		return nil, errors.New("gojxl: frame flags (patches/splines/noise/DC) not yet supported")
	}

	// ----- LfGlobal: DequantMatrices DC, then VarDCT global DC info -----
	readDequantMatricesDC(b)
	st.quant = decodeQuantizer(b)
	if st.blockCtx, err = decodeBlockCtxMap(b); err != nil {
		return nil, err
	}
	if st.cmap, err = decodeCfLDC(b); err != nil {
		return nil, err
	}

	// ----- LfGlobal: modular global info (tree + histograms) -----
	hasTree := b.ReadBits(1) == 1
	if !hasTree {
		return nil, errors.New("gojxl: VarDCT frame without a global tree not yet supported")
	}
	if st.tree, err = decodeTree(b, 1<<20); err != nil {
		return nil, err
	}
	if st.code, st.ctxMap, err = decodeHistograms(b, (len(st.tree)+1)/2, false); err != nil {
		return nil, err
	}

	// ----- DC group: VarDCT DC (LF) image -----
	// Single DC group only (16-bit-friendly small images) for now.
	if st.fd.numDCGroups != 1 {
		return st, errors.New("gojxl: multi-DC-group VarDCT not yet supported")
	}
	if err := decodeVarDCTDCImage(b, st, 0); err != nil {
		return st, err
	}

	return st, errVarDCTIncomplete
}

// dcChannels holds the decoded (still-quantized) LF/DC planes in [Y, X, B] order.
type dcChannels struct {
	w, h           int
	extraPrecision int
	y, x, bch      []int32
}

// decodeVarDCTDCImage decodes the LF/DC image for a DC group: a 3-channel
// modular sub-stream at 1/8 resolution (ModularFrameDecoder::DecodeVarDCTDC),
// using the global tree/histograms. The result is left quantized; dequant
// happens during reconstruction.
func decodeVarDCTDCImage(b *bitReader, st *vardctState, groupID int) error {
	dcW := int(divCeil(st.fd.xsize, acBlockDim))
	dcH := int(divCeil(st.fd.ysize, acBlockDim))

	b.Refill()
	extraPrecision := int(b.ReadBits(2))

	// 3 channels at DC resolution, decoded as a modular image. Channel storage
	// order matches libjxl: index 0 = Y (c=1), 1 = X (c=0), 2 = B (c=2).
	img := &modImage{bitdepth: int(st.meta.BitDepth.BitsPerSample)}
	for i := 0; i < 3; i++ {
		img.channel = append(img.channel, modChannel{w: dcW, h: dcH, pix: make([]int32, dcW*dcH)})
	}

	gh, err := readGroupHeader(b)
	if err != nil {
		return err
	}
	if !gh.useGlobalTree {
		return errors.New("gojxl: VarDCT DC local trees not supported")
	}
	for i := range gh.transforms {
		if err := metaApplyTransform(img, &gh.transforms[i]); err != nil {
			return err
		}
	}
	// VarDCTDC stream id = 1 + group_id.
	streamID := 1 + groupID
	reader := newANSSymbolReader(st.code, b, maxChannelWidth(img.channel))
	for ci := range img.channel {
		decodeChannel(reader, b, st.tree, st.ctxMap, img.channel, ci, streamID, gh.wp)
	}
	if !reader.checkFinalState() {
		return errors.New("gojxl: VarDCT DC ANS final state failed")
	}
	for i := len(gh.transforms) - 1; i >= 0; i-- {
		if err := inverseTransform(img, gh.transforms[i], gh.wp); err != nil {
			return err
		}
	}

	base := img.nbMeta
	st.dc = &dcChannels{
		w: dcW, h: dcH, extraPrecision: extraPrecision,
		y:   img.channel[base+0].pix,
		x:   img.channel[base+1].pix,
		bch: img.channel[base+2].pix,
	}
	return nil
}
