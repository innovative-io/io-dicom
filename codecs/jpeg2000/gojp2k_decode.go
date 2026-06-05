package jpeg2000

// Top-level pure-Go JPEG 2000 decode: parse → tier-2 → tier-1 → inverse DWT →
// DC level shift → output samples. Baseline scope (single tile, single layer,
// LRCP, default precincts, 5/3 reversible and 9/7 irreversible) covers the
// common DICOM JPEG 2000 codestreams; anything outside it returns
// errJ2KUnsupported so the caller can fall back to the native backend.

import (
	"encoding/binary"
	"math"
)

// decodeTileComponent decodes one tile-component into spatial samples.
func decodeTileComponent(cs *j2kCodestream, frame []byte, tc *tileComp) ([]int32, error) {
	transform := tc.style.transform
	if transform != 0 && transform != 1 {
		return nil, errJ2KUnsupported // 5/3 reversible or 9/7 irreversible only
	}
	quant := tc.quant
	// quant.style: 0 = no quantization (5/3), 1 = scalar derived, 2 = scalar
	// expounded. 5/3 must be unquantized; 9/7 must carry scalar quantization.
	if transform == 1 && quant.style != 0 {
		return nil, errJ2KUnsupported
	}
	if transform == 0 && quant.style == 0 {
		return nil, errJ2KUnsupported
	}
	// HT (High-Throughput, ISO 15444-15) code-blocks: only the reversible (5/3)
	// Cleanup path is implemented. Defer lossy HT to the native backend.
	if tc.style.htCodeblocks && transform != 1 {
		return nil, errJ2KUnsupported
	}
	prec := cs.comps[tc.comp].precision

	// bandQuant returns the (exponent, mantissa) for a subband. For expounded
	// (and the no-quantization layout) each subband carries its own entry indexed
	// by expIdx; for derived quantization a single entry is signalled and the
	// per-subband exponent is recovered from the decomposition level (T.800 E.5):
	// εb = ε0 − NL + nb, with nb = NL−r+1 for resolution r>0 and nb = NL for LL.
	bandQuant := func(r, expIdx int) (expn, mant int) {
		if quant.style == 1 {
			if len(quant.exponents) > 0 {
				expn = quant.exponents[0]
			}
			if len(quant.mantissas) > 0 {
				mant = quant.mantissas[0]
			}
			if r > 0 {
				expn = expn - r + 1
			}
			return
		}
		if expIdx < len(quant.exponents) {
			expn = quant.exponents[expIdx]
		}
		if expIdx < len(quant.mantissas) {
			mant = quant.mantissas[expIdx]
		}
		return
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
	band := make([][]int32, maxExp)     // 5/3 integer coefficients
	band97 := make([][]float32, maxExp) // 9/7 dequantized float coefficients

	for ri := range tc.resolutions {
		res := &tc.resolutions[ri]
		for si := range res.subbands {
			sb := &res.subbands[si]
			sw, sh := sb.w(), sb.h()
			expn, mant := bandQuant(ri, sb.expIdx)
			// Mb: number of magnitude bit-planes for this subband.
			mb := quant.guardBits + expn - 1
			// Δb = (1 + μb/2^11) · 2^(Rb − εb). Normally Rb = precision + log2 gain,
			// but to match openjpeg's inverse 9/7 (which scales the high-pass by 2/K
			// rather than 1/K) the sub-band gain is forced to 0 here; the extra
			// factor of two per high-pass lifting level supplies the gain instead.
			var stepsize float32
			if transform == 0 {
				rb := prec // gain forced to 0 (see two_invK in gojp2k_dwt97.go)
				stepsize = float32((1.0 + float64(mant)/2048.0) * math.Exp2(float64(rb-expn)))
			}
			ibuf := band[sb.expIdx]
			var fbuf []float32
			if transform == 0 {
				fbuf = make([]float32, sw*sh)
			} else {
				ibuf = make([]int32, sw*sh)
			}
			for bi := range sb.blocks {
				cb := &sb.blocks[bi]
				if tc.style.htCodeblocks {
					// HT Cleanup pass (reversible). Single-pass blocks only for now;
					// refuse multi-pass (SigProp/MagRef) so the caller can fall back.
					if cb.npasses == 0 || len(cb.segs) == 0 {
						continue // empty/insignificant block stays zero
					}
					if cb.npasses != 1 {
						return nil, errJ2KUnsupported
					}
					seg := cb.segs[0]
					decoded, ok := decodeHTCleanup(seg, len(seg), cb.nzeroBP, cb.w(), cb.h())
					if !ok {
						return nil, errJ2KMalformed
					}
					shift := uint(31 - mb)
					cw := cb.w()
					for yy := 0; yy < cb.h(); yy++ {
						for xx := 0; xx < cw; xx++ {
							gx := cb.x0 - sb.x0 + xx
							gy := cb.y0 - sb.y0 + yy
							v := decoded[yy*cw+xx]
							m := int32((v & 0x7FFFFFFF) >> shift)
							if v&0x80000000 != 0 {
								m = -m
							}
							ibuf[gy*sw+gx] = m
						}
					}
					continue
				}
				coeffs, lowestBP := decodeCodeBlock(cb, sb.orient, mb)
				// Mid-point of the dequantization interval (T.800 E.1.1). The
				// reversible path keeps integer coefficients, so the mid-point only
				// contributes when the block was rate-truncated (lowestBP>0). The
				// irreversible path is float, so the mid-point is always added —
				// including the half-step (2^-1 = 0.5) when decoded to bit-plane 0.
				var imid int32
				if lowestBP > 0 {
					imid = int32(1) << uint(lowestBP-1)
				}
				fmid := float32(math.Exp2(float64(lowestBP - 1)))
				// Place code-block coefficients into the subband buffer.
				cw := cb.w()
				for yy := 0; yy < cb.h(); yy++ {
					for xx := 0; xx < cw; xx++ {
						gx := cb.x0 - sb.x0 + xx
						gy := cb.y0 - sb.y0 + yy
						c := coeffs[yy*cw+xx]
						if transform == 0 {
							var fv float32
							switch {
							case c > 0:
								fv = (float32(c) + fmid) * stepsize
							case c < 0:
								fv = (float32(c) - fmid) * stepsize
							}
							fbuf[gy*sw+gx] = fv
						} else {
							switch {
							case c > 0:
								c += imid
							case c < 0:
								c -= imid
							}
							ibuf[gy*sw+gx] = c
						}
					}
				}
			}
			if transform == 0 {
				band97[sb.expIdx] = fbuf
			} else {
				band[sb.expIdx] = ibuf
			}
		}
	}

	var samples []int32
	if transform == 0 {
		fs := idwt97(tc, band97)
		samples = make([]int32, len(fs))
		for i, v := range fs {
			samples[i] = int32(math.Round(float64(v)))
		}
	} else {
		samples = idwt53(tc, band)
	}

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
	if w <= 0 || h <= 0 || nc <= 0 {
		return errJ2KMalformed
	}
	// Validate the required output size without overflowing int on hostile SIZ
	// values: accumulate in int64 and reject as soon as the running product
	// exceeds the (already bounded) output buffer. Each factor multiplies a value
	// already capped at len(out), so no intermediate can overflow int64.
	outLen := int64(len(out))
	need := int64(w)
	for _, f := range []int{h, nc, bps} {
		if need > outLen {
			return errJ2KMalformed
		}
		need *= int64(f)
	}
	if need > outLen {
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
