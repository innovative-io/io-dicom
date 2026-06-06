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
	if fd.numGroups != 1 {
		return nil, nil, errors.New("gojxl: multi-group frames not yet supported")
	}
	if _, err := readTOC(b, numTocEntries(fd.numGroups, fd.numDCGroups, fh.NumPasses)); err != nil {
		return nil, nil, err
	}

	// LfGlobal preamble.
	if fh.Flags != 0 {
		return nil, nil, errors.New("gojxl: frame flags (patches/splines/noise/DC) not yet supported")
	}
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

	// Channel layout: color channels + extra channels, all full-resolution.
	w, h := int(sh.Xsize), int(sh.Ysize)
	nbColor := 3
	if meta.Color.ColorSpace == csGray {
		nbColor = 1
	}
	nbExtra := len(meta.ExtraChannels)
	image := make([]modChannel, nbColor+nbExtra)
	for i := range image {
		image[i] = modChannel{w: w, h: h, pix: make([]int32, w*h)}
	}

	gh, err := readGroupHeader(b)
	if err != nil {
		return nil, nil, err
	}
	if !gh.useGlobalTree {
		return nil, nil, errors.New("gojxl: local trees not yet supported")
	}
	for _, t := range gh.transforms {
		if t.id != transformRCT {
			return nil, nil, errors.New("gojxl: Palette/Squeeze transforms not yet supported")
		}
	}

	reader := newANSSymbolReader(code, b, w)
	for ci := range image {
		decodeChannel(reader, b, tree, ctxMap, image, ci, 0, gh.wp)
	}
	if !reader.checkFinalState() {
		return nil, nil, errors.New("gojxl: modular ANS final state failed")
	}

	if !applyInverseTransforms(image, gh.transforms, 0) {
		return nil, nil, errors.New("gojxl: unsupported transform during inverse")
	}
	return image, &meta, nil
}
