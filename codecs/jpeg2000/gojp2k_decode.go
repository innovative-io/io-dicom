package jpeg2000

// Top-level pure-Go JPEG 2000 decode: parse → tier-2 → tier-1 → inverse DWT →
// DC level shift → output samples. Baseline scope (single tile, single layer,
// LRCP, default precincts, 5/3 reversible or 9/7 irreversible-not-yet) covers the
// common DICOM JPEG 2000 codestreams; anything outside it returns
// errJ2KUnsupported so the caller can fall back to the native backend.

import "encoding/binary"

// decodeTileComponent decodes one tile-component into spatial samples.
func decodeTileComponent(cs *j2kCodestream, frame []byte, tc *tileComp) ([]int32, error) {
	if tc.style.transform != 1 {
		return nil, errJ2KUnsupported // only 5/3 reversible for now
	}
	quant := tc.quant
	if quant.style != 0 {
		return nil, errJ2KUnsupported // reversible quantization only
	}

	// Decode each code-block to coefficients, indexed by subband expIdx.
	maxExp := 0
	for _, r := range tc.resolutions {
		for _, sb := range r.subbands {
			if sb.expIdx+1 > maxExp {
				maxExp = sb.expIdx + 1
			}
		}
	}
	band := make([][]int32, maxExp)

	for ri := range tc.resolutions {
		res := &tc.resolutions[ri]
		for si := range res.subbands {
			sb := &res.subbands[si]
			sw, sh := sb.w(), sb.h()
			buf := make([]int32, sw*sh)
			// Mb: number of magnitude bit-planes for this subband (reversible).
			exp := 0
			if sb.expIdx < len(quant.exponents) {
				exp = quant.exponents[sb.expIdx]
			}
			mb := quant.guardBits + exp - 1
			for bi := range sb.blocks {
				cb := &sb.blocks[bi]
				coeffs := decodeCodeBlock(cb, sb.orient, mb)
				// Place code-block coefficients into the subband buffer.
				cw := cb.w()
				for yy := 0; yy < cb.h(); yy++ {
					for xx := 0; xx < cw; xx++ {
						gx := cb.x0 - sb.x0 + xx
						gy := cb.y0 - sb.y0 + yy
						buf[gy*sw+gx] = coeffs[yy*cw+xx]
					}
				}
			}
			band[sb.expIdx] = buf
		}
	}

	samples := idwt53(tc, band)

	// DC level shift for unsigned components.
	ci := cs.comps[tc.comp]
	if !ci.signed {
		shift := int32(1) << uint(ci.precision-1)
		for i := range samples {
			samples[i] += shift
		}
	}
	return samples, nil
}

// goJ2Kdecode decodes a raw JPEG 2000 codestream into out using the codec's
// little-endian sample convention (2 bytes/sample when precision > 8).
func goJ2Kdecode(frame, out []byte) error {
	cs, err := parseCodestream(frame)
	if err != nil {
		return err
	}
	if cs.numTilesX()*cs.numTilesY() != 1 || len(cs.tileParts) != 1 {
		return errJ2KUnsupported // single tile only
	}
	nc := cs.numComps()
	prec := cs.comps[0].precision
	bps := 1
	if prec > 8 {
		bps = 2
	}
	w := cs.xsiz - cs.xOsiz
	h := cs.ysiz - cs.yOsiz
	need := w * h * nc * bps
	if need > len(out) {
		return errJ2KMalformed
	}

	tp := cs.tileParts[0]
	tcs := make([]*tileComp, nc)
	for c := 0; c < nc; c++ {
		tc, err := cs.buildTileComponent(0, c)
		if err != nil {
			return err
		}
		tcs[c] = tc
	}
	// Tier-2 fills code-block segments for all components in one pass.
	if err := decodeTileTier2(cs, tcs, frame, tp.dataStart, tp.dataEnd); err != nil {
		return err
	}

	planes := make([][]int32, nc)
	for c := 0; c < nc; c++ {
		s, err := decodeTileComponent(cs, frame, tcs[c])
		if err != nil {
			return err
		}
		planes[c] = s
	}

	// Pack component-interleaved, little-endian.
	signed := cs.comps[0].signed
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < nc; c++ {
				v := planes[c][y*w+x]
				idx := (y*w+x)*nc + c
				if bps == 1 {
					out[idx] = byte(v)
				} else {
					if signed {
						binary.LittleEndian.PutUint16(out[idx*2:], uint16(int16(v)))
					} else {
						binary.LittleEndian.PutUint16(out[idx*2:], uint16(v))
					}
				}
			}
		}
	}
	return nil
}
