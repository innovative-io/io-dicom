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

	return st, errVarDCTIncomplete
}
