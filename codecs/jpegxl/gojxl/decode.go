package gojxl

import "errors"

// Image is a decoded JPEG XL image: interleaved samples plus geometry.
type Image struct {
	W, H     int
	Channels int // color channels + extra channels, in decode order
	BitDepth int
	// Pixels are interleaved samples, row-major, channel-interleaved. Samples
	// wider than 8 bits occupy 2 little-endian bytes each.
	Pixels []byte
}

// Decode decodes a single-frame JPEG XL image. It supports the lossless Modular
// subset (RCT/Palette/Squeeze, single- and multi-group) and the lossy VarDCT
// subset for XYB RGB/grayscale: the full common transform set, multi-group,
// multi-DC-group, permuted TOC, local/global trees, one-or-more AC histogram
// sets, multiple passes, CfL, Gaborish and EPF, at any size. It returns an error
// for inputs outside those subsets (VarDCT with DCT128/256, non-XYB color, extra
// channels, animation, ICC, non-identity orientation, ...), so a caller can fall
// back to another backend.
func Decode(data []byte) (*Image, error) {
	// Dispatch on the frame encoding: VarDCT (lossy) vs Modular (lossless).
	if enc, err := peekFrameEncoding(data); err == nil && enc == frameVarDCT {
		return DecodeVarDCT(data)
	}
	channels, meta, err := decodeModularFrame(data)
	if err != nil {
		return nil, err
	}
	if meta.Orientation != 1 {
		return nil, errors.New("gojxl: non-identity orientation not yet supported")
	}
	if len(channels) == 0 {
		return nil, errors.New("gojxl: no channels decoded")
	}
	w, h := channels[0].w, channels[0].h
	nc := len(channels)
	bd := int(meta.BitDepth.BitsPerSample)
	bps := 1
	if bd > 8 {
		bps = 2
	}
	for c := 1; c < nc; c++ {
		if channels[c].w != w || channels[c].h != h {
			return nil, errors.New("gojxl: channels have mismatched dimensions")
		}
	}
	pix := make([]byte, w*h*nc*bps)
	for i := 0; i < w*h; i++ {
		for c := 0; c < nc; c++ {
			v := uint32(channels[c].pix[i])
			idx := (i*nc + c) * bps
			pix[idx] = byte(v)
			if bps == 2 {
				pix[idx+1] = byte(v >> 8)
			}
		}
	}
	return &Image{W: w, H: h, Channels: nc, BitDepth: bd, Pixels: pix}, nil
}

// peekFrameEncoding parses the codestream headers up to the frame header and
// reports the frame's encoding (frameVarDCT or frameModular) so Decode can
// dispatch to the right decoder. It does not consume or validate the frame body.
func peekFrameEncoding(data []byte) (encoding uint32, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("gojxl: malformed header")
		}
	}()
	cs, err := codestream(data)
	if err != nil {
		return 0, err
	}
	if len(cs) < 2 || cs[0] != 0xFF || cs[1] != 0x0A {
		return 0, errors.New("gojxl: bad codestream signature")
	}
	b := newBitReader(cs[2:])
	sh, err := readSizeHeader(b)
	if err != nil {
		return 0, err
	}
	_ = sh
	meta, err := readImageMetadata(b)
	if err != nil {
		return 0, err
	}
	readTransformData(b, meta.XYBEncoded)
	if err := b.JumpToByteBoundary(); err != nil {
		return 0, err
	}
	fh, err := readFrameHeader(b, &meta)
	if err != nil {
		return 0, err
	}
	return fh.Encoding, nil
}
