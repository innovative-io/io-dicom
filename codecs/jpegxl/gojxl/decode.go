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

// Decode decodes a lossless Modular JPEG XL image (the subset gojxl supports:
// single frame, Modular encoding, RCT/Palette/Squeeze transforms, single- and
// multi-group). It returns an error for inputs outside that subset (VarDCT,
// animation, ICC, non-identity orientation, ...), so a caller can fall back to
// another backend.
func Decode(data []byte) (*Image, error) {
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
