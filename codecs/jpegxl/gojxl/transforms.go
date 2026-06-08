package gojxl

import "errors"

// Inverse Modular transforms (libjxl modular/transform/*): RCT and Palette.
// Squeeze is added as needed.

// modImage is the Modular image during decode: an ordered channel list plus the
// number of leading meta channels (e.g. a palette) that are not output.
type modImage struct {
	channel  []modChannel
	nbMeta   int
	bitdepth int
}

// metaApplyTransform updates the channel layout for a transform before decode
// (the MetaApply step). RCT leaves the layout unchanged; Palette replaces the
// palettized channels with [palette, index]; Squeeze halves dimensions and
// inserts residual channels. For Squeeze, default parameters are resolved in
// place so the inverse step uses the same set.
func metaApplyTransform(img *modImage, t *transform) error {
	switch t.id {
	case transformRCT:
		return nil
	case transformPalette:
		return metaPalette(img, *t)
	case transformSqueeze:
		if len(t.squeezes) == 0 {
			t.squeezes = defaultSqueezeParams(img)
		}
		return metaSqueeze(img, t.squeezes)
	default:
		return errors.New("gojxl: unknown transform")
	}
}

func metaPalette(img *modImage, t transform) error {
	beginC := int(t.beginC)
	nb := int(t.numC)
	endC := beginC + nb - 1
	if endC >= len(img.channel) {
		return errors.New("gojxl: palette channel range out of bounds")
	}
	if beginC >= img.nbMeta {
		img.nbMeta++
	} else {
		img.nbMeta += 2 - nb
	}
	// Erase channels (beginC+1 .. endC].
	img.channel = append(img.channel[:beginC+1], img.channel[endC+1:]...)
	// Insert the palette meta channel at the front: width = nb_colors+nb_deltas,
	// height = nb (one row per palettized channel).
	pal := modChannel{
		w: int(t.nbColors + t.nbDeltas), h: nb,
		hshift: -1, vshift: -1,
		pix: make([]int32, int(t.nbColors+t.nbDeltas)*nb),
	}
	img.channel = append([]modChannel{pal}, img.channel...)
	return nil
}

// inverseTransform undoes one transform after decode.
func inverseTransform(img *modImage, t transform, wpH wpHeader) error {
	switch t.id {
	case transformRCT:
		invRCT(img.channel, int(t.beginC), t.rctType)
		return nil
	case transformPalette:
		return invPalette(img, t, wpH)
	case transformSqueeze:
		return invSqueeze(img, t.squeezes)
	default:
		return errors.New("gojxl: unknown transform")
	}
}

// invRCT applies the inverse reversible color transform in place over channels
// [beginC, beginC+2] (rct.cc InvRCT + InvRCTRow).
func invRCT(channel []modChannel, beginC int, rctType uint32) {
	if rctType == 0 {
		return
	}
	permutation := int(rctType / 7)
	custom := int(rctType % 7)
	m := beginC
	w := channel[m].w
	h := channel[m].h
	c0 := channel[m].pix
	c1 := channel[m+1].pix
	c2 := channel[m+2].pix
	o0 := m + permutation%3
	o1 := m + (permutation+1+permutation/3)%3
	o2 := m + (permutation+2-permutation/3)%3
	out0 := channel[o0].pix
	out1 := channel[o1].pix
	out2 := channel[o2].pix

	second := custom >> 1
	third := custom & 1
	for i := 0; i < w*h; i++ {
		if custom == 6 { // YCoCg
			Y := c0[i]
			Co := c1[i]
			Cg := c2[i]
			tmp := Y - (Cg >> 1)
			G := Cg + tmp
			B := tmp - (Co >> 1)
			R := B + Co
			out0[i] = R
			out1[i] = G
			out2[i] = B
			continue
		}
		first := c0[i]
		sec := c1[i]
		thr := c2[i]
		if third != 0 {
			thr += first
		}
		if second == 1 {
			sec += first
		} else if second == 2 {
			sec += (first + thr) >> 1
		}
		out0[i] = first
		out1[i] = sec
		out2[i] = thr
	}
}

// invPalette expands the index channel back into nb color channels using the
// palette meta channel (palette.cc InvPalette, simple case: nb_deltas==0 and
// predictor==Zero with indices within the palette).
func invPalette(img *modImage, t transform, wpH wpHeader) error {
	if len(img.channel) < 1 {
		return errors.New("gojxl: palette transform without palette")
	}
	nb := img.channel[0].h
	c0 := int(t.beginC) + 1
	if c0 >= len(img.channel) {
		return errors.New("gojxl: palette index channel out of range")
	}
	idxCh := img.channel[c0]
	w, h := idxCh.w, idxCh.h
	palette := img.channel[0]
	palW := palette.w
	bitDepth := img.bitdepth
	if bitDepth > 24 {
		bitDepth = 24
	}
	if t.nbDeltas != 0 || t.predictor != predZero {
		return errors.New("gojxl: delta-palette not yet supported")
	}

	// Output color channels.
	outs := make([]modChannel, nb)
	for c := 0; c < nb; c++ {
		outs[c] = modChannel{w: w, h: h, hshift: idxCh.hshift, vshift: idxCh.vshift, pix: make([]int32, w*h)}
	}
	for i := 0; i < w*h; i++ {
		index := int(idxCh.pix[i])
		for c := 0; c < nb; c++ {
			outs[c].pix[i] = getPaletteValue(palette.pix, index, c, palW, bitDepth)
		}
	}
	// Replace [palette(0), index(c0)] region: remove palette (front) and the
	// index channel, insert the nb output channels at c0-1 (= beginC).
	rest := append([]modChannel{}, img.channel[1:c0]...)
	rest = append(rest, outs...)
	rest = append(rest, img.channel[c0+1:]...)
	img.channel = rest
	if c0 >= img.nbMeta {
		img.nbMeta--
	} else {
		img.nbMeta -= 2 - nb
	}
	return nil
}

// getPaletteValue returns palette entry (index, channel c). Only the in-range
// case (0 <= index < palette_size) is implemented; delta/cube indices error via
// the caller's nb_deltas guard.
func getPaletteValue(palette []int32, index, c, paletteSize, bitDepth int) int32 {
	if index >= 0 && index < paletteSize {
		return palette[c*paletteSize+index]
	}
	// Out-of-range (implicit cube/delta palettes) not yet supported; clamp.
	if index < 0 {
		index = 0
	} else if index >= paletteSize {
		index = paletteSize - 1
	}
	return palette[c*paletteSize+index]
}
