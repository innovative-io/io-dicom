package jpeg2000

// Pure-Go HTJ2K (ISO 15444-15) lossless encoder: DC level shift → forward 5/3
// DWT → HT Cleanup-pass block coding (encodeHTCleanup) → tier-2 packet assembly
// → HTJ2K codestream markers (SIZ Rsiz=HT, CAP, COD with HT code-block style).
//
// This is the HT counterpart of goJ2Kencode. It reuses the same DWT, tile/
// subband geometry, and tier-2 packet machinery; only the tier-1 block coding,
// coefficient packing, QCD exponents, and a few markers differ. Reversible
// (lossless) only, mirroring openjph's reversible HT (single Cleanup pass with
// missing_msbs = K_max-1 per significant block).

import "encoding/binary"

// HTJ2Kdecode decodes a JPEG 2000 / HTJ2K codestream into output using the
// pure-Go decoder directly, independent of the registered JPEG 2000 backend.
// This lets the JPIP codec decode HTJ2K without being affected by jpeg2000
// backend selection. A panic on malformed input is converted to an error.
func HTJ2Kdecode(encoded []byte, output []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errJ2KMalformed
		}
	}()
	return goJ2Kdecode(stripJP2(encoded), output)
}

// HTJ2Kencode encodes raw interleaved samples into a lossless HTJ2K (ISO
// 15444-15) codestream. It is the pure-Go replacement for the openjph-based
// JPIP/HTJ2K encoder. A panic on malformed input is converted to an error so it
// is safe on hostile data.
func HTJ2Kencode(raw []byte, width, height, samples, bitsa int) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			data, err = nil, errJ2KMalformed
		}
	}()
	return goHTJ2Kencode(raw, width, height, samples, bitsa)
}

// goHTJ2Kencode encodes raw interleaved samples into a single-tile, single-layer
// LRCP HTJ2K codestream (reversible 5/3, no MCT).
func goHTJ2Kencode(raw []byte, w, h, nc, prec int) ([]byte, error) {
	if w <= 0 || h <= 0 || nc <= 0 || prec <= 0 || prec > 16 || nc > maxJ2KComponents {
		return nil, errJ2KUnsupported
	}
	bps := 1
	if prec > 8 {
		bps = 2
	}
	// Exact-size requirement, computed overflow-safe (see goJ2Kencode).
	need := int64(w)
	for _, f := range []int{h, nc, bps} {
		ff := int64(f)
		if ff <= 0 || need > maxCodecPayloadBytes/ff {
			return nil, errInvalidJ2KPayload
		}
		need *= ff
	}
	if int64(len(raw)) != need {
		return nil, errInvalidJ2KPayload
	}

	NL := 5
	minDim := w
	if h < minDim {
		minDim = h
	}
	for NL > 0 && (minDim>>uint(NL)) < 1 {
		NL--
	}

	cs := &j2kCodestream{
		xsiz: w, ysiz: h, xtsiz: w, ytsiz: h,
		comps: make([]componentInfo, nc),
		cod: codingStyle{
			progression: 0, numLayers: 1, mct: 0,
			decompLevels: NL, cbW: 64, cbH: 64, transform: 1,
			cbStyle: 0x40, htCodeblocks: true,
		},
	}
	for c := 0; c < nc; c++ {
		cs.comps[c] = componentInfo{precision: prec, dx: 1, dy: 1}
	}
	cs.cod.precinctW = make([]int, NL+1)
	cs.cod.precinctH = make([]int, NL+1)
	for i := range cs.cod.precinctW {
		cs.cod.precinctW[i] = 1 << 15
		cs.cod.precinctH[i] = 1 << 15
	}

	// Pass 1: forward DWT every component; find each subband's peak bit-plane.
	tcs := make([]*tileComp, nc)
	bands := make([][][]int32, nc)
	numSub := 1 + 3*NL
	expsRaw := make([]int, numSub)
	shiftLvl := int32(1) << uint(prec-1)
	for c := 0; c < nc; c++ {
		tc, err := cs.buildTileComponent(0, c)
		if err != nil {
			return nil, err
		}
		samples := make([]int32, w*h)
		for i := 0; i < w*h; i++ {
			var v int32
			if bps == 1 {
				v = int32(raw[i*nc+c])
			} else {
				idx := (i*nc + c) * 2
				v = int32(binary.LittleEndian.Uint16(raw[idx:]))
			}
			samples[i] = v - shiftLvl // DC level shift
		}
		band := fdwt53(tc, samples)
		for ri := range tc.resolutions {
			for si := range tc.resolutions[ri].subbands {
				sb := &tc.resolutions[ri].subbands[si]
				for _, v := range band[sb.expIdx] {
					if v < 0 {
						v = -v
					}
					if b := bitLen(int(v)) - 1; b > expsRaw[sb.expIdx] {
						expsRaw[sb.expIdx] = b
					}
				}
			}
		}
		tcs[c] = tc
		bands[c] = band
	}

	// Signaled exponents: K_max = exp + guardBits - 1 must be >= 2 so that
	// missing_msbs = K_max-1 >= 1 (openjph's invariant). With guardBits=2 this
	// means exp >= 1. Bumping the exponent only changes the binade alignment;
	// because encode and decode use the same K_max, reconstruction stays exact.
	exps := make([]int, numSub)
	for i := range exps {
		exps[i] = max(expsRaw[i], 1)
	}

	// Pass 2: HT Cleanup-pass encode each significant code-block.
	for c := 0; c < nc; c++ {
		tc := tcs[c]
		for ri := range tc.resolutions {
			res := &tc.resolutions[ri]
			for si := range res.subbands {
				sb := &res.subbands[si]
				coeffs := bands[c][sb.expIdx]
				kmax := exps[sb.expIdx] + encGuardBits - 1
				shift := uint(31 - kmax)
				sw := sb.w()
				for bi := range sb.blocks {
					cb := &sb.blocks[bi]
					cw, ch := cb.w(), cb.h()
					buf := make([]uint32, cw*ch)
					any := false
					for yy := 0; yy < ch; yy++ {
						for xx := 0; xx < cw; xx++ {
							gx := cb.x0 - sb.x0 + xx
							gy := cb.y0 - sb.y0 + yy
							v := coeffs[gy*sw+gx]
							if v == 0 {
								continue
							}
							any = true
							var sign uint32
							mag := v
							if v < 0 {
								sign = 0x80000000
								mag = -v
							}
							buf[yy*cw+xx] = sign | (uint32(mag) << shift)
						}
					}
					if !any {
						cb.npasses = 0
						continue
					}
					data := encodeHTCleanup(buf, kmax-1, cw, ch, cw)
					cb.npasses = 1
					cb.nzeroBP = kmax - 1
					cb.htLen1 = len(data)
					cb.htLen2 = 0
					cb.segs = [][]byte{data}
				}
			}
		}
	}

	packets := encodePackets(cs, tcs, NL)
	return assembleHTCodestream(cs, w, h, nc, prec, NL, exps, packets), nil
}

func assembleHTCodestream(cs *j2kCodestream, w, h, nc, prec, NL int, exps []int, packets []byte) []byte {
	var out []byte
	put16 := func(v int) { out = append(out, byte(v>>8), byte(v)) }
	put32 := func(v int) { out = append(out, byte(v>>24), byte(v>>16), byte(v>>8), byte(v)) }

	// SOC
	out = append(out, 0xFF, 0x4F)
	// SIZ (Rsiz signals HTJ2K capability)
	out = append(out, 0xFF, 0x51)
	put16(38 + 3*nc) // Lsiz
	put16(0x4000)    // Rsiz = RSIZ_HT_FLAG
	put32(w)         // Xsiz
	put32(h)         // Ysiz
	put32(0)         // XOsiz
	put32(0)         // YOsiz
	put32(w)         // XTsiz
	put32(h)         // YTsiz
	put32(0)         // XTOsiz
	put32(0)         // YTOsiz
	put16(nc)        // Csiz
	for c := 0; c < nc; c++ {
		out = append(out, byte(prec-1)) // Ssiz (unsigned)
		out = append(out, 1, 1)         // XRsiz, YRsiz
	}
	// CAP (extended capabilities: HTJ2K, ISO 15444-15)
	maxExp := 0
	for _, e := range exps {
		if e > maxExp {
			maxExp = e
		}
	}
	magB := maxExp + encGuardBits - 1 // get_MAGB for reversible = max K_max
	var bp int
	switch {
	case magB <= 8:
		bp = 0
	case magB < 28:
		bp = magB - 8
	default:
		bp = 13 + (magB >> 2)
	}
	out = append(out, 0xFF, 0x50)
	put16(8)          // Lcap
	put32(0x00020000) // Pcap: bit 15 (Part 15 / HTJ2K)
	put16(bp & 0x1F)  // Ccap[0]: reversible ⇒ bit5 clear, low 5 bits = Bp
	// COD
	out = append(out, 0xFF, 0x52)
	put16(12)               // Lcod
	out = append(out, 0x00) // Scod
	out = append(out, 0x00) // progression LRCP
	put16(1)                // num layers
	out = append(out, 0x00) // MCT
	out = append(out, byte(NL))
	out = append(out, 0x04) // cbW exp-2 (64 ⇒ 4)
	out = append(out, 0x04) // cbH exp-2
	out = append(out, 0x40) // code-block style: HT mode
	out = append(out, 0x01) // transform 5/3
	// QCD (reversible / no quantization)
	out = append(out, 0xFF, 0x5C)
	put16(3 + len(exps))                     // Lqcd
	out = append(out, byte(encGuardBits<<5)) // Sqcd: guard bits, style 0
	for _, e := range exps {
		out = append(out, byte(e<<3)) // exponent in high 5 bits
	}
	// SOT
	sodAndData := 2 + len(packets)
	psot := 12 + sodAndData
	out = append(out, 0xFF, 0x90)
	put16(10)               // Lsot
	put16(0)                // Isot
	put32(psot)             // Psot
	out = append(out, 0x00) // TPsot
	out = append(out, 0x01) // TNsot
	// SOD + packets
	out = append(out, 0xFF, 0x93)
	out = append(out, packets...)
	// EOC
	out = append(out, 0xFF, 0xD9)
	return out
}
