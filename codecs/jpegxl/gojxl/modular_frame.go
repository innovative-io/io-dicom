package gojxl

import "errors"

// decodeModularFrame decodes a single-frame, single-group lossless Modular
// JPEG XL image into its channels. Scope: no patches/splines/noise, global
// tree, RCT-only transforms (Palette/Squeeze pending). It is the integration
// of stages 1-3 used to validate against djxl.
func decodeModularFrame(data []byte) ([]modChannel, *ImageMetadata, error) {
	cs, err := codestream(data)
	if err != nil {
		return nil, nil, err
	}
	if len(cs) < 2 || cs[0] != 0xFF || cs[1] != 0x0A {
		return nil, nil, errors.New("gojxl: bad codestream signature")
	}
	b := newBitReader(cs[2:])
	sh, err := readSizeHeader(b)
	if err != nil {
		return nil, nil, err
	}
	meta, err := readImageMetadata(b)
	if err != nil {
		return nil, nil, err
	}
	if meta.Color.WantICC {
		return nil, nil, errors.New("gojxl: ICC profiles not yet supported")
	}
	readTransformData(b, meta.XYBEncoded)
	if err := b.JumpToByteBoundary(); err != nil {
		return nil, nil, err
	}
	fh, err := readFrameHeader(b, &meta)
	if err != nil {
		return nil, nil, err
	}
	if fh.Encoding != frameModular {
		return nil, nil, errors.New("gojxl: VarDCT frames not yet supported")
	}
	fd := computeFrameDimensions(sh.Xsize, sh.Ysize, fh.GroupSizeShift, fh.Upsampling)
	tocSizes, err := readTOC(b, numTocEntries(fd.numGroups, fd.numDCGroups, fh.NumPasses))
	if err != nil {
		return nil, nil, err
	}
	if fh.Flags != 0 {
		return nil, nil, errors.New("gojxl: frame flags (patches/splines/noise/DC) not yet supported")
	}

	// Byte offset of the section data (section 0 = LfGlobal) within cs[2:].
	frameStart := b.bitsConsumed() / 8
	body := cs[2:]

	// ----- LfGlobal (section 0) -----
	readDequantMatricesDC(b)
	hasTree := b.ReadBits(1) == 1
	if !hasTree {
		return nil, nil, errors.New("gojxl: frames without a global tree not yet supported")
	}
	tree, err := decodeTree(b, 1<<20)
	if err != nil {
		return nil, nil, err
	}
	code, ctxMap, err := decodeHistograms(b, (len(tree)+1)/2, false)
	if err != nil {
		return nil, nil, err
	}

	// Channel layout: color + extra channels, full-resolution.
	w, h := int(sh.Xsize), int(sh.Ysize)
	nbColor := 3
	if meta.Color.ColorSpace == csGray {
		nbColor = 1
	}
	nbExtra := len(meta.ExtraChannels)
	img := &modImage{bitdepth: int(meta.BitDepth.BitsPerSample)}
	for i := 0; i < nbColor+nbExtra; i++ {
		img.channel = append(img.channel, modChannel{w: w, h: h, pix: make([]int32, w*h)})
	}

	gh, err := readGroupHeader(b)
	if err != nil {
		return nil, nil, err
	}
	if !gh.useGlobalTree {
		return nil, nil, errors.New("gojxl: local trees not yet supported")
	}
	for i := range gh.transforms {
		if err := metaApplyTransform(img, &gh.transforms[i]); err != nil {
			return nil, nil, err
		}
	}

	groupDim := int(fd.groupDim)
	// First non-meta channel that is too big for the LfGlobal section; channels
	// before it are decoded globally, the rest per group.
	beginc := img.nbMeta
	for beginc < len(img.channel) {
		c := &img.channel[beginc]
		if c.w > groupDim || c.h > groupDim {
			break
		}
		beginc++
	}

	if fd.numGroups == 1 {
		// Everything fits in one group: decode all channels from LfGlobal.
		reader := newANSSymbolReader(code, b, maxChannelWidth(img.channel))
		for ci := range img.channel {
			decodeChannel(reader, b, tree, ctxMap, img.channel, ci, 0, gh.wp)
		}
		if !reader.checkFinalState() {
			return nil, nil, errors.New("gojxl: modular ANS final state failed")
		}
	} else {
		// Decode the small (global) channels from LfGlobal, then each big
		// channel per group.
		if beginc > img.nbMeta {
			reader := newANSSymbolReader(code, b, maxChannelWidth(img.channel[:beginc]))
			for ci := 0; ci < beginc; ci++ {
				decodeChannel(reader, b, tree, ctxMap, img.channel, ci, 0, gh.wp)
			}
			if !reader.checkFinalState() {
				return nil, nil, errors.New("gojxl: LfGlobal ANS final state failed")
			}
		}
		// Section byte ranges within body.
		offsets := make([]int, len(tocSizes)+1)
		offsets[0] = frameStart
		for i, s := range tocSizes {
			offsets[i+1] = offsets[i] + int(s)
		}
		for g := uint32(0); g < fd.numGroups; g++ {
			gx := int(g % fd.xsizeGroups)
			gy := int(g / fd.xsizeGroups)
			rect := groupRect{x: gx * groupDim, y: gy * groupDim}
			rect.w = minInt(groupDim, w-rect.x)
			rect.h = minInt(groupDim, h-rect.y)
			si := acGroupIndex(0, g, fd.numGroups, fd.numDCGroups)
			if int(si)+1 >= len(offsets) || offsets[si+1] > len(body) {
				return nil, nil, errTruncated
			}
			section := body[offsets[si]:offsets[si+1]]
			groupID := 1 + 3*int(fd.numDCGroups) + kDequantMatricesNum + int(fd.numGroups)*0 + int(g)
			if err := decodeModularGroup(section, img, beginc, tree, code, ctxMap, rect, groupID, groupDim); err != nil {
				return nil, nil, err
			}
		}
	}

	// Undo the global transforms in reverse order.
	for i := len(gh.transforms) - 1; i >= 0; i-- {
		if err := inverseTransform(img, gh.transforms[i], gh.wp); err != nil {
			return nil, nil, err
		}
	}
	return img.channel[img.nbMeta:], &meta, nil
}

const kDequantMatricesNum = 17

type groupRect struct{ x, y, w, h int }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxChannelWidth(channels []modChannel) int {
	m := 0
	for i := range channels {
		if channels[i].w > m {
			m = channels[i].w
		}
	}
	return m
}

// decodeModularGroup decodes one group's big channels (index >= beginc),
// restricted to rect, and places them into img. Assumes no per-channel shift
// (hshift==vshift==0), i.e. no Squeeze across groups.
func decodeModularGroup(section []byte, img *modImage, beginc int, tree []treeNode,
	code *ansCode, ctxMap []uint8, rect groupRect, groupID, groupDim int) error {
	gb := newBitReader(section)
	ggh, err := readGroupHeader(gb)
	if err != nil {
		return err
	}
	if !ggh.useGlobalTree {
		return errors.New("gojxl: per-group local trees not yet supported")
	}

	gi := &modImage{bitdepth: img.bitdepth}
	var fullIdx []int
	for c := beginc; c < len(img.channel); c++ {
		fc := &img.channel[c]
		if fc.hshift != 0 || fc.vshift != 0 {
			return errors.New("gojxl: multi-group with channel shifts (Squeeze) not yet supported")
		}
		rw := minInt(rect.w, fc.w-rect.x)
		rh := minInt(rect.h, fc.h-rect.y)
		if rw <= 0 || rh <= 0 {
			continue
		}
		gi.channel = append(gi.channel, modChannel{w: rw, h: rh, pix: make([]int32, rw*rh)})
		fullIdx = append(fullIdx, c)
	}
	if len(gi.channel) == 0 {
		return nil
	}
	nbBase := len(gi.channel)

	// Per-group transforms (MetaApply on the group sub-image).
	for i := range ggh.transforms {
		if err := metaApplyTransform(gi, &ggh.transforms[i]); err != nil {
			return err
		}
	}

	reader := newANSSymbolReader(code, gb, maxChannelWidth(gi.channel))
	for ci := range gi.channel {
		decodeChannel(reader, gb, tree, ctxMap, gi.channel, ci, groupID, ggh.wp)
	}
	if !reader.checkFinalState() {
		return errors.New("gojxl: group ANS final state failed")
	}

	// Undo per-group transforms.
	for i := len(ggh.transforms) - 1; i >= 0; i-- {
		if err := inverseTransform(gi, ggh.transforms[i], ggh.wp); err != nil {
			return err
		}
	}
	if len(gi.channel)-gi.nbMeta != nbBase {
		return errors.New("gojxl: group channel count changed after inverse transforms")
	}

	// Place the output channels into the full image.
	for k, c := range fullIdx {
		gc := &gi.channel[gi.nbMeta+k]
		fc := &img.channel[c]
		for y := 0; y < gc.h; y++ {
			copy(fc.pix[(rect.y+y)*fc.w+rect.x:(rect.y+y)*fc.w+rect.x+gc.w], gc.pix[y*gc.w:(y+1)*gc.w])
		}
	}
	return nil
}
