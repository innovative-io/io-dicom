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
	acm     *acMetadata
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
	// DecodeGroup(ModularDC) for extra channels is a no-op with no extra
	// channels (this subset). AC metadata follows.
	if len(st.meta.ExtraChannels) != 0 {
		return st, errors.New("gojxl: VarDCT with extra channels not yet supported")
	}
	if err := decodeAcMetadata(b, st, 0); err != nil {
		return st, err
	}

	return st, errVarDCTIncomplete
}

// acMetadata holds per-block transform/quant info for a DC group.
type acMetadata struct {
	bw, bh    int             // block grid dimensions
	strategy  []acStrategyType // per-block AC strategy (top-left of each varblock)
	valid     []bool          // whether a block position is the top-left of a varblock
	quantF    []int32         // per-block raw quant field
	epf       []uint8         // per-block EPF sharpness
	ytoxMap   []int32         // per-color-tile CfL X factor
	ytobMap   []int32         // per-color-tile CfL B factor
	ctW, ctH  int             // color-tile grid dims
}

// decodeAcMetadata decodes the AC metadata for a DC group
// (ModularFrameDecoder::DecodeAcMetadata): a 4-channel modular stream giving the
// per-color-tile CfL maps, the per-block AC strategy + raw quant field, and the
// EPF sharpness field.
func decodeAcMetadata(b *bitReader, st *vardctState, groupID int) error {
	bw := int(divCeil(st.fd.xsize, acBlockDim))
	bh := int(divCeil(st.fd.ysize, acBlockDim))
	upperBound := bw * bh
	b.Refill()
	count := int(b.ReadBits(ceilLog2Nonzero(uint32(upperBound)))) + 1

	ctW := (bw + 7) >> 3
	ctH := (bh + 7) >> 3

	img := &modImage{bitdepth: int(st.meta.BitDepth.BitsPerSample)}
	img.channel = []modChannel{
		{w: ctW, h: ctH, hshift: 3, vshift: 3, pix: make([]int32, ctW*ctH)}, // ytox
		{w: ctW, h: ctH, hshift: 3, vshift: 3, pix: make([]int32, ctW*ctH)}, // ytob
		{w: count, h: 2, pix: make([]int32, count*2)},                       // ACS + QF
		{w: bw, h: bh, pix: make([]int32, bw*bh)},                           // EPF sharpness
	}

	gh, err := readGroupHeader(b)
	if err != nil {
		return err
	}
	if !gh.useGlobalTree {
		return errors.New("gojxl: AC metadata local trees not supported")
	}
	for i := range gh.transforms {
		if err := metaApplyTransform(img, &gh.transforms[i]); err != nil {
			return err
		}
	}
	streamID := 1 + 2*int(st.fd.numDCGroups) + groupID // ACMetadata stream id
	reader := newANSSymbolReader(st.code, b, maxChannelWidth(img.channel))
	// Two-pass decode order: channels with min(hshift,vshift) >= 3 first.
	for _, dcPass := range []bool{true, false} {
		for ci := range img.channel {
			ch := &img.channel[ci]
			isDC := ch.hshift >= 3 && ch.vshift >= 3
			if isDC != dcPass {
				continue
			}
			decodeChannel(reader, b, st.tree, st.ctxMap, img.channel, ci, streamID, gh.wp)
		}
	}
	if !reader.checkFinalState() {
		return errors.New("gojxl: AC metadata ANS final state failed")
	}
	for i := len(gh.transforms) - 1; i >= 0; i-- {
		if err := inverseTransform(img, gh.transforms[i], gh.wp); err != nil {
			return err
		}
	}

	base := img.nbMeta
	acsRow := img.channel[base+2].pix[0:count]     // row 0
	qfRow := img.channel[base+2].pix[count : 2*count] // row 1
	epfCh := img.channel[base+3].pix

	md := &acMetadata{
		bw: bw, bh: bh, ctW: ctW, ctH: ctH,
		strategy: make([]acStrategyType, bw*bh),
		valid:    make([]bool, bw*bh),
		quantF:   make([]int32, bw*bh),
		epf:      make([]uint8, bw*bh),
		ytoxMap:  append([]int32(nil), img.channel[base+0].pix...),
		ytobMap:  append([]int32(nil), img.channel[base+1].pix...),
	}
	num := 0
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			idx := by*bw + bx
			md.epf[idx] = uint8(epfCh[idx])
			if md.valid[idx] {
				continue // covered by an earlier multiblock strategy
			}
			if num >= count {
				return errors.New("gojxl: AC metadata corrupted (count)")
			}
			rawACS := acsRow[num]
			if rawACS < 0 || rawACS >= acNumValidStrategies {
				return errors.New("gojxl: invalid AC strategy")
			}
			t := acStrategyType(rawACS)
			cbx, cby := t.coveredBlocksX(), t.coveredBlocksY()
			if bx+cbx > bw || by+cby > bh {
				return errors.New("gojxl: AC strategy overflows block grid")
			}
			qf := qfRow[num]
			if qf < 0 {
				qf = 0
			} else if qf > kQuantMax-1 {
				qf = kQuantMax - 1
			}
			// Mark the covered blocks; the top-left carries the strategy.
			for iy := 0; iy < cby; iy++ {
				for ix := 0; ix < cbx; ix++ {
					p := (by+iy)*bw + (bx + ix)
					md.valid[p] = true
				}
			}
			md.strategy[idx] = t
			md.quantF[idx] = 1 + qf
			num++
		}
	}
	st.acm = md
	return nil
}

const kQuantMax = 256

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
