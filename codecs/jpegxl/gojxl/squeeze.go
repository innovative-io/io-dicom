package gojxl

import "errors"

// Inverse Squeeze transform (libjxl modular/transform/squeeze.cc). Squeeze is a
// reversible Haar-like wavelet split used for progressive/responsive modular;
// MetaSqueeze halves dimensions and inserts residual channels, InvSqueeze
// recombines them.

const maxFirstPreviewSize = 8

// defaultSqueezeParams replicates DefaultSqueezeParameters for the current
// channel layout (used when a Squeeze transform carries no explicit params).
func defaultSqueezeParams(img *modImage) []squeezeParam {
	nbMeta := img.nbMeta
	nbChannels := len(img.channel) - nbMeta
	var params []squeezeParam
	w := img.channel[nbMeta].w
	h := img.channel[nbMeta].h
	wide := w > h

	if nbChannels > 2 && img.channel[nbMeta+1].w == w && img.channel[nbMeta+1].h == h {
		params = append(params, squeezeParam{horizontal: true, inPlace: false, beginC: uint32(nbMeta + 1), numC: 2})
		params = append(params, squeezeParam{horizontal: false, inPlace: false, beginC: uint32(nbMeta + 1), numC: 2})
	}
	base := squeezeParam{beginC: uint32(nbMeta), numC: uint32(nbChannels), inPlace: true}
	if !wide {
		if h > maxFirstPreviewSize {
			p := base
			p.horizontal = false
			params = append(params, p)
			h = (h + 1) / 2
		}
	}
	for w > maxFirstPreviewSize || h > maxFirstPreviewSize {
		if w > maxFirstPreviewSize {
			p := base
			p.horizontal = true
			params = append(params, p)
			w = (w + 1) / 2
		}
		if h > maxFirstPreviewSize {
			p := base
			p.horizontal = false
			params = append(params, p)
			h = (h + 1) / 2
		}
	}
	return params
}

// metaSqueeze sets up the channel layout for a Squeeze transform (MetaSqueeze).
func metaSqueeze(img *modImage, params []squeezeParam) error {
	for _, p := range params {
		beginc := int(p.beginC)
		endc := int(p.beginC) + int(p.numC) - 1
		if beginc < 0 || endc >= len(img.channel) || endc < beginc {
			return errors.New("gojxl: invalid squeeze channel range")
		}
		if beginc < img.nbMeta {
			if endc >= img.nbMeta || !p.inPlace {
				return errors.New("gojxl: invalid squeeze meta range")
			}
			img.nbMeta += int(p.numC)
		}
		offset := len(img.channel)
		if p.inPlace {
			offset = endc + 1
		}
		for c := beginc; c <= endc; c++ {
			ch := &img.channel[c]
			w, h := ch.w, ch.h
			if w == 0 || h == 0 {
				return errors.New("gojxl: squeezing empty channel")
			}
			var rw, rh int
			if p.horizontal {
				ch.w = (w + 1) / 2
				if ch.hshift >= 0 {
					ch.hshift++
				}
				rw, rh = w-(w+1)/2, h
			} else {
				ch.h = (h + 1) / 2
				if ch.vshift >= 0 {
					ch.vshift++
				}
				rw, rh = w, h-(h+1)/2
			}
			ch.pix = make([]int32, ch.w*ch.h) // shrink (re-decoded; values filled later)
			placeholder := modChannel{w: rw, h: rh, hshift: ch.hshift, vshift: ch.vshift, pix: make([]int32, rw*rh)}
			ins := offset + (c - beginc)
			img.channel = append(img.channel[:ins], append([]modChannel{placeholder}, img.channel[ins:]...)...)
		}
	}
	return nil
}

func smoothTendency(B, a, n int64) int64 {
	var diff int64
	if B >= a && a >= n {
		diff = (4*B - 3*n - a + 6) / 12
		if diff-(diff&1) > 2*(B-a) {
			diff = 2*(B-a) + 1
		}
		if diff+(diff&1) > 2*(a-n) {
			diff = 2 * (a - n)
		}
	} else if B <= a && a <= n {
		diff = (4*B - 3*n - a - 6) / 12
		if diff+(diff&1) < 2*(B-a) {
			diff = 2*(B-a) - 1
		}
		if diff-(diff&1) < 2*(a-n) {
			diff = 2 * (a - n)
		}
	}
	return diff
}

// invHSqueeze undoes a horizontal squeeze: channel[c] (avg) + channel[rc]
// (residual) -> channel[c] doubled in width.
func invHSqueeze(channel []modChannel, c, rc int) {
	chin := channel[c]
	res := channel[rc]
	outW := chin.w + res.w
	out := make([]int32, outW*chin.h)
	if res.w > 0 && res.h > 0 {
		for y := 0; y < chin.h; y++ {
			avgRow := chin.pix[y*chin.w:]
			resRow := res.pix[y*res.w:]
			outRow := out[y*outW:]
			for x := 0; x < res.w; x++ {
				avg := int64(avgRow[x])
				nextAvg := avg
				if x+1 < chin.w {
					nextAvg = int64(avgRow[x+1])
				}
				left := avg
				if x > 0 {
					left = int64(outRow[(x<<1)-1])
				}
				tendency := smoothTendency(left, avg, nextAvg)
				diff := int64(resRow[x]) + tendency
				A := avg + diff/2
				outRow[x<<1] = int32(A)
				outRow[(x<<1)+1] = int32(A - diff)
			}
			if outW&1 != 0 {
				outRow[outW-1] = avgRow[chin.w-1]
			}
		}
	}
	channel[c] = modChannel{w: outW, h: chin.h, hshift: chin.hshift - 1, vshift: chin.vshift, pix: out}
}

// invVSqueeze undoes a vertical squeeze.
func invVSqueeze(channel []modChannel, c, rc int) {
	chin := channel[c]
	res := channel[rc]
	outH := chin.h + res.h
	w := chin.w
	out := make([]int32, w*outH)
	if res.w > 0 && res.h > 0 {
		for y := 0; y < res.h; y++ {
			avgRow := chin.pix[y*w:]
			nextAvgY := y
			if y+1 < chin.h {
				nextAvgY = y + 1
			}
			navgRow := chin.pix[nextAvgY*w:]
			resRow := res.pix[y*res.w:]
			outRow := out[(y<<1)*w:]
			noutRow := out[((y<<1)+1)*w:]
			var poutRow []int32
			if y > 0 {
				poutRow = out[((y<<1)-1)*w:]
			} else {
				poutRow = avgRow
			}
			for x := 0; x < w; x++ {
				avg := int64(avgRow[x])
				nextAvg := int64(navgRow[x])
				top := int64(poutRow[x])
				tendency := smoothTendency(top, avg, nextAvg)
				diff := int64(resRow[x]) + tendency
				o := avg + diff/2
				outRow[x] = int32(o)
				noutRow[x] = int32(o - diff)
			}
		}
		if outH&1 != 0 {
			y := chin.h - 1
			copy(out[(y<<1)*w:(y<<1)*w+w], chin.pix[y*w:y*w+w])
		}
	}
	channel[c] = modChannel{w: w, h: outH, hshift: chin.hshift, vshift: chin.vshift - 1, pix: out}
}

// invSqueeze undoes all squeeze params in reverse order (InvSqueeze).
func invSqueeze(img *modImage, params []squeezeParam) error {
	for i := len(params) - 1; i >= 0; i-- {
		p := params[i]
		beginc := int(p.beginC)
		endc := int(p.beginC) + int(p.numC) - 1
		offset := len(img.channel) + beginc - endc - 1
		if p.inPlace {
			offset = endc + 1
		}
		if beginc < img.nbMeta {
			img.nbMeta -= int(p.numC)
		}
		for c := beginc; c <= endc; c++ {
			rc := offset + c - beginc
			if rc >= len(img.channel) {
				return errors.New("gojxl: squeeze residual out of range")
			}
			if img.channel[c].w < img.channel[rc].w || img.channel[c].h < img.channel[rc].h {
				return errors.New("gojxl: corrupted squeeze")
			}
			if p.horizontal {
				invHSqueeze(img.channel, c, rc)
			} else {
				invVSqueeze(img.channel, c, rc)
			}
		}
		// Erase the residual channels [offset, offset+num_c).
		img.channel = append(img.channel[:offset], img.channel[offset+int(p.numC):]...)
	}
	return nil
}
