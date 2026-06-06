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

	quantLib      [kNumQuantTables]*quantEncoding
	numHistograms int
	usedACS       uint32
	coeffOrders   map[int][3][]int
	acCode        *ansCode
	acCtxMap      []uint8
	acCoeffs      [3][]int32
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

	// ----- ACGlobal / HfGlobal -----
	if err := decodeHfGlobal(b, st); err != nil {
		return st, err
	}

	// ----- AC group: quantized coefficient decode -----
	if st.fd.numGroups != 1 {
		return st, errors.New("gojxl: multi-AC-group VarDCT not yet supported")
	}
	if err := decodeACGroup(b, st); err != nil {
		return st, err
	}

	return st, nil
}

// DecodeVarDCT decodes a lossy VarDCT JPEG XL frame to an 8-bit sRGB image.
// Restricted to the currently-supported subset (XYB, single group/pass, all
// DCT8x8 blocks). Loop filters (Gaborish/EPF) are not yet applied, so block
// edges differ slightly from a conforming decoder; interiors match.
func DecodeVarDCT(data []byte) (*Image, error) {
	st, err := decodeVarDCTFrame(data)
	if err != nil {
		return nil, err
	}
	return reconstructVarDCT(st)
}

// decodeACGroup decodes the quantized AC coefficients of one AC group
// (dec_group.cc DecodeGroupImpl / DecodeACVarBlock). Restricted to the single
// group / single pass / no-chroma-subsampling subset. Coefficients are stored
// per channel as bw*bh blocks of 64 (DCT8x8 etc.; the LLF/DC slot stays 0 here
// and is filled from the DC image during reconstruction).
func decodeACGroup(b *bitReader, st *vardctState) error {
	bw, bh := st.acm.bw, st.acm.bh
	// AC coefficient blocks per channel (index 0=X,1=Y,2=B), bw*bh blocks of 64.
	coeffs := [3][]int32{
		make([]int32, bw*bh*64),
		make([]int32, bw*bh*64),
		make([]int32, bw*bh*64),
	}
	// Per-channel non-zero count grids for prediction.
	nzeros := [3][]int32{
		make([]int32, bw*bh),
		make([]int32, bw*bh),
		make([]int32, bw*bh),
	}

	// Single histogram set -> no histogram selector bits, ctx_offset 0.
	reader := newANSSymbolReader(st.acCode, b, 0)

	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			idx := by*bw + bx
			if !st.acm.valid[idx] {
				continue
			}
			acs := st.acm.strategy[idx]
			ord := int(kStrategyOrder[acs])
			coveredBlocks := acs.numBlocks()
			log2cov := acs.log2CoveredBlocks()
			size := coveredBlocks * 64
			qf := st.acm.quantF[idx]

			// Channel decode order is Y, X, B = {1, 0, 2}.
			for _, c := range [3]int{1, 0, 2} {
				var rowTop []int32
				if by > 0 {
					rowTop = nzeros[c][(by-1)*bw : by*bw]
				}
				row := nzeros[c][by*bw : (by+1)*bw]
				predicted := int(predictFromTopAndLeft(rowTop, row, bx, 32))

				blockCtx := st.blockCtx.blockContext(0, uint32(qf), ord, c)
				nzeroCtx := st.blockCtx.nonZeroContext(predicted, blockCtx)
				nz := int(reader.readHybridUint(nzeroCtx, b, st.acCtxMap))
				if nz > size-coveredBlocks {
					return errors.New("gojxl: AC nzeros too large")
				}
				// Record nzeros for the covered region (prediction of neighbors).
				val := int32((nz + coveredBlocks - 1) >> uint(log2cov))
				for yy := 0; yy < acs.coveredBlocksY(); yy++ {
					for xx := 0; xx < acs.coveredBlocksX(); xx++ {
						nzeros[c][(by+yy)*bw+(bx+xx)] = val
					}
				}

				histoOffset := st.blockCtx.zeroDensityContextsOffset(blockCtx)
				prev := 0
				if nz <= size/16 {
					prev = 1
				}
				order := st.coeffOrders[ord][c]
				block := coeffs[c][idx*64 : idx*64+size]
				for k := coveredBlocks; k < size && nz != 0; k++ {
					ctx := histoOffset + zeroDensityContext(nz, k, coveredBlocks, log2cov, prev)
					u := reader.readHybridUint(ctx, b, st.acCtxMap)
					block[order[k]] += unpackSigned32(u)
					if u != 0 {
						prev = 1
					} else {
						prev = 0
					}
					nz -= prev
				}
				if nz != 0 {
					return errors.New("gojxl: AC nzeros not exhausted at block end")
				}
			}
		}
	}
	if !reader.checkFinalState() {
		return errors.New("gojxl: AC group ANS final state failed")
	}
	st.acCoeffs = coeffs
	return nil
}

// kOrderEnc = U32Enc(Val(0x5F), Val(0x13), Val(0), Bits(13)) (frame_header.h).
var kOrderEnc = [4]u32d{u32Val(0x5F), u32Val(0x13), u32Val(0), u32Bits(kNumOrders)}

// decodeHfGlobal parses the AC global section: the AC dequant matrices, the
// histogram count, the coefficient orders, and the AC entropy histograms.
func decodeHfGlobal(b *bitReader, st *vardctState) error {
	// AC DequantMatrices. all_default -> use the built-in default library.
	allDefault := b.ReadBits(1) == 1
	if !allDefault {
		return errors.New("gojxl: non-default AC dequant matrices not yet supported")
	}
	st.quantLib = buildDefaultQuantLibrary()

	// Number of histogram sets.
	numHistoBits := ceilLog2Nonzero(st.fd.numGroups)
	st.numHistograms = 1 + int(b.ReadBits(numHistoBits))

	// used_acs: the set of AC strategies actually present in the frame.
	var usedACS uint32
	for _, s := range st.acm.strategy {
		// Only the valid (top-left) blocks carry a real strategy; default 0
		// (DCT) for the covered ones is harmless since DCT is always present
		// when any block is DCT, but restrict to valid entries to be exact.
		usedACS |= 1 << uint(s)
	}
	st.usedACS = usedACS

	// Single pass only for this subset.
	if st.fh.NumPasses != 1 {
		return errors.New("gojxl: multi-pass VarDCT not yet supported")
	}
	orders, err := decodeCoeffOrders(b, st.usedACS)
	if err != nil {
		return err
	}
	st.coeffOrders = orders

	// AC histograms.
	numContexts := st.numHistograms * st.blockCtx.numACContexts()
	st.acCode, st.acCtxMap, err = decodeHistograms(b, numContexts, false)
	if err != nil {
		return err
	}
	return nil
}

// decodeCoeffOrders reads the coefficient orders for all used AC strategies
// (DecodeCoeffOrders). Returns a map keyed by [orderClass][channel] -> order.
func decodeCoeffOrders(b *bitReader, usedACS uint32) (map[int][3][]int, error) {
	usedOrders := b.ReadU32(kOrderEnc[0], kOrderEnc[1], kOrderEnc[2], kOrderEnc[3])

	var reader *ansSymbolReader
	var ctxMap []uint8
	if usedOrders != 0 {
		code, cm, err := decodeHistograms(b, kPermutationContexts, false)
		if err != nil {
			return nil, err
		}
		ctxMap = cm
		reader = newANSSymbolReader(code, b, 0)
	}

	acsMask := uint32(0)
	for o := 0; o < acNumValidStrategies; o++ {
		if usedACS&(1<<uint(o)) == 0 {
			continue
		}
		acsMask |= 1 << kStrategyOrder[o]
	}

	orders := map[int][3][]int{}
	computed := uint32(0)
	for o := 0; o < acNumValidStrategies; o++ {
		ord := int(kStrategyOrder[o])
		if computed&(1<<uint(ord)) != 0 {
			continue
		}
		computed |= 1 << uint(ord)
		t := acStrategyType(o)
		used := acsMask&(1<<uint(ord)) != 0

		if usedOrders&(1<<uint(ord)) == 0 {
			// Natural order (only needed if actually used).
			if used {
				natural := naturalCoeffOrder(t.coveredBlocksX(), t.coveredBlocksY())
				var triplet [3][]int
				for c := 0; c < 3; c++ {
					triplet[c] = natural
				}
				orders[ord] = triplet
			}
		} else {
			var triplet [3][]int
			for c := 0; c < 3; c++ {
				order, err := decodeCoeffOrder(t, reader, b, ctxMap)
				if err != nil {
					return nil, err
				}
				if used {
					triplet[c] = order
				}
			}
			if used {
				orders[ord] = triplet
			}
		}
	}
	if usedOrders != 0 && !reader.checkFinalState() {
		return nil, errors.New("gojxl: coeff-order ANS final state failed")
	}
	return orders, nil
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
