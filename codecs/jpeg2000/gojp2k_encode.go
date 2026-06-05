package jpeg2000

// Pure-Go JPEG 2000 encoder: DC level shift → forward 5/3 DWT → tier-1 EBCOT
// code-block encode → tier-2 packet assembly → codestream markers. Baseline
// scope mirrors the decoder: single tile, single layer, LRCP progression,
// default maximal precincts, 5/3 reversible (lossless), no MCT.

import "encoding/binary"

// ---------------------------------------------------------------------------
// bioWriter: packet-header bit writer with JPEG 2000 bit-stuffing (inverse of
// bioReader / openjpeg opj_bio encoder).
// ---------------------------------------------------------------------------

type bioWriter struct {
	out []byte
	buf uint32
	ct  int
}

func newBioWriter() *bioWriter { return &bioWriter{ct: 8} }

func (b *bioWriter) byteout() {
	b.buf = (b.buf << 8) & 0xFFFF
	if b.buf == 0xFF00 {
		b.ct = 7
	} else {
		b.ct = 8
	}
	b.out = append(b.out, byte(b.buf>>8))
}

func (b *bioWriter) putBit(bit int) {
	if b.ct == 0 {
		b.byteout()
	}
	b.ct--
	b.buf |= uint32(bit) << uint(b.ct)
}

func (b *bioWriter) putBits(v, n int) {
	for i := n - 1; i >= 0; i-- {
		b.putBit((v >> uint(i)) & 1)
	}
}

// flush byte-aligns the header (FLUSH). Returns the header bytes.
func (b *bioWriter) flush() []byte {
	b.byteout()
	if b.ct == 7 {
		b.byteout()
	}
	return b.out
}

// ---------------------------------------------------------------------------
// tagTreeEnc: tag-tree coding (inverse of tagTree.decode), driven by the known
// leaf values.
// ---------------------------------------------------------------------------

type tagTreeEnc struct {
	*tagTree
	nodeVal []int // true min value per node
}

func newTagTreeEnc(w, h int, leafVals []int) *tagTreeEnc {
	t := newTagTree(w, h)
	nv := make([]int, len(t.value))
	for m := 0; m < h; m++ {
		for n := 0; n < w; n++ {
			nv[t.off[0]+m*w+n] = leafVals[m*w+n]
		}
	}
	// Each coarser node is the min over its (up to four) finer children.
	for level := 1; level < t.nlevels; level++ {
		for mm := 0; mm < t.levelH[level]; mm++ {
			for nn := 0; nn < t.levelW[level]; nn++ {
				min := 1 << 30
				for dm := 0; dm < 2; dm++ {
					for dn := 0; dn < 2; dn++ {
						cm, cn := 2*mm+dm, 2*nn+dn
						if cm < t.levelH[level-1] && cn < t.levelW[level-1] {
							if v := nv[t.off[level-1]+cm*t.levelW[level-1]+cn]; v < min {
								min = v
							}
						}
					}
				}
				nv[t.off[level]+mm*t.levelW[level]+nn] = min
			}
		}
	}
	return &tagTreeEnc{t, nv}
}

func (t *tagTreeEnc) encode(bw *bioWriter, m, n, threshold int) {
	minVal := 0
	for level := t.nlevels - 1; level >= 0; level-- {
		mm := m >> level
		nn := n >> level
		idx := t.off[level] + mm*t.levelW[level] + nn
		if t.value[idx] < minVal {
			t.value[idx] = minVal
		}
		for t.value[idx] < threshold && !t.fixed[idx] {
			if t.value[idx] == t.nodeVal[idx] {
				bw.putBit(1)
				t.fixed[idx] = true
			} else {
				bw.putBit(0)
				t.value[idx]++
			}
		}
		minVal = t.value[idx]
	}
}

// writeNumPasses is the inverse of readNumPasses (T.800 Table B.4).
func writeNumPasses(bw *bioWriter, p int) {
	if p == 1 {
		bw.putBit(0)
		return
	}
	bw.putBit(1)
	if p == 2 {
		bw.putBit(0)
		return
	}
	bw.putBit(1)
	if p <= 5 {
		bw.putBits(p-3, 2)
		return
	}
	bw.putBits(3, 2)
	if p <= 36 {
		bw.putBits(p-6, 5)
		return
	}
	bw.putBits(31, 5)
	bw.putBits(p-37, 7)
}

func bitLen(v int) int {
	n := 0
	for v > 0 {
		n++
		v >>= 1
	}
	return n
}

// ---------------------------------------------------------------------------
// codestream assembly
// ---------------------------------------------------------------------------

const encGuardBits = 2

// maxJ2KComponents caps the component count the pure-Go encoder accepts. DICOM
// pixel data uses 1 (monochrome) or 3 (colour); this is a generous upper bound
// that rejects absurd values while never constraining real images.
const maxJ2KComponents = 256

// goJ2Kencode encodes raw little-endian samples (1 byte/sample for prec≤8, else
// 2) into a lossless 5/3 JPEG 2000 codestream.
func goJ2Kencode(raw []byte, w, h, nc, prec int) ([]byte, error) {
	if w <= 0 || h <= 0 || nc <= 0 || prec <= 0 || prec > 16 || nc > maxJ2KComponents {
		return nil, errJ2KUnsupported
	}
	bps := 1
	if prec > 8 {
		bps = 2
	}
	// Require the raw payload to be exactly w·h·nc·bps bytes (no missing or extra
	// bytes). Compute the size overflow-safe: before each multiply, a
	// division-based guard ensures the running product never exceeds the codec's
	// payload cap, so a hostile w/h/nc (any factor) can never overflow int64.
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

	// Cap decomposition levels so no dimension is decomposed below 1 sample.
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

	// Pass 1: forward DWT every component and find the per-subband peak bit-plane
	// across all components (the QCD is shared, so all components must agree on Mb).
	tcs := make([]*tileComp, nc)
	bands := make([][][]int32, nc)
	numSub := 1 + 3*NL
	exps := make([]int, numSub)
	shift := int32(1) << uint(prec-1)
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
			samples[i] = v - shift // DC level shift (unsigned input)
		}
		band := fdwt53(tc, samples)
		for ri := range tc.resolutions {
			for si := range tc.resolutions[ri].subbands {
				sb := &tc.resolutions[ri].subbands[si]
				for _, v := range band[sb.expIdx] {
					if v < 0 {
						v = -v
					}
					if b := bitLen(int(v)) - 1; b > exps[sb.expIdx] {
						exps[sb.expIdx] = b
					}
				}
			}
		}
		tcs[c] = tc
		bands[c] = band
	}

	// Pass 2: encode each code-block with the shared per-subband Mb.
	for c := 0; c < nc; c++ {
		tc := tcs[c]
		for ri := range tc.resolutions {
			res := &tc.resolutions[ri]
			for si := range res.subbands {
				sb := &res.subbands[si]
				coeffs := bands[c][sb.expIdx]
				mb := encGuardBits + exps[sb.expIdx] - 1
				sw := sb.w()
				for bi := range sb.blocks {
					cb := &sb.blocks[bi]
					cw, ch := cb.w(), cb.h()
					bc := make([]int32, cw*ch)
					for yy := 0; yy < ch; yy++ {
						for xx := 0; xx < cw; xx++ {
							gx := cb.x0 - sb.x0 + xx
							gy := cb.y0 - sb.y0 + yy
							bc[yy*cw+xx] = coeffs[gy*sw+gx]
						}
					}
					data, np, nz := encodeCodeBlock(bc, cw, ch, sb.orient, mb)
					cb.npasses = np
					cb.nzeroBP = nz
					if np > 0 {
						cb.segs = [][]byte{data}
					}
				}
			}
		}
	}

	packets := encodePackets(cs, tcs, NL)

	return assembleCodestream(cs, w, h, nc, prec, NL, exps, packets), nil
}

// encodePackets builds the LRCP packet stream (single layer): for each
// resolution, for each component, one packet (header + body).
func encodePackets(cs *j2kCodestream, tcs []*tileComp, NL int) []byte {
	var out []byte
	for r := 0; r <= NL; r++ {
		for c := 0; c < len(tcs); c++ {
			tc := tcs[c]
			if r >= len(tc.resolutions) {
				continue
			}
			out = append(out, encodeOnePacket(&tc.resolutions[r])...)
		}
	}
	return out
}

func encodeOnePacket(res *resolution) []byte {
	// Does any code-block contribute?
	nonEmpty := false
	for si := range res.subbands {
		for bi := range res.subbands[si].blocks {
			if res.subbands[si].blocks[bi].npasses > 0 {
				nonEmpty = true
			}
		}
	}
	bw := newBioWriter()
	if !nonEmpty {
		bw.putBit(0)
		return bw.flush()
	}
	bw.putBit(1)

	// Per-subband tag-trees (inclusion + zero-bitplane).
	type sbTrees struct{ incl, zbp *tagTreeEnc }
	trees := make([]sbTrees, len(res.subbands))
	for si := range res.subbands {
		sb := &res.subbands[si]
		inclVals := make([]int, sb.cbCols*sb.cbRows)
		zbpVals := make([]int, sb.cbCols*sb.cbRows)
		for i := range sb.blocks {
			if sb.blocks[i].npasses > 0 {
				inclVals[i] = 0 // included in layer 0
			} else {
				inclVals[i] = 1 // not in layer 0 (single layer ⇒ never)
			}
			zbpVals[i] = sb.blocks[i].nzeroBP
		}
		trees[si] = sbTrees{
			incl: newTagTreeEnc(sb.cbCols, sb.cbRows, inclVals),
			zbp:  newTagTreeEnc(sb.cbCols, sb.cbRows, zbpVals),
		}
	}

	var body []byte
	for si := range res.subbands {
		sb := &res.subbands[si]
		for by := 0; by < sb.cbRows; by++ {
			for bx := 0; bx < sb.cbCols; bx++ {
				cb := &sb.blocks[by*sb.cbCols+bx]
				if cb.lblock == 0 {
					cb.lblock = 3
				}
				included := cb.npasses > 0
				// Inclusion (first inclusion ⇒ tag-tree; threshold = layer+1 = 1).
				trees[si].incl.encode(bw, by, bx, 1)
				if !included {
					continue
				}
				// Zero bit-plane count (tag-tree, resolve fully).
				trees[si].zbp.encode(bw, by, bx, 1<<30)
				writeNumPasses(bw, cb.npasses)
				// Lblock increment so the length fits, then a terminating 0.
				passBits := floorLog2(cb.npasses)
				need := bitLen(len(cb.segs[0]))
				for cb.lblock+passBits < need {
					bw.putBit(1)
					cb.lblock++
				}
				bw.putBit(0)
				bw.putBits(len(cb.segs[0]), cb.lblock+passBits)
				body = append(body, cb.segs[0]...)
			}
		}
	}
	header := bw.flush()
	return append(header, body...)
}

func assembleCodestream(cs *j2kCodestream, w, h, nc, prec, NL int, exps []int, packets []byte) []byte {
	var out []byte
	put16 := func(v int) { out = append(out, byte(v>>8), byte(v)) }
	put32 := func(v int) { out = append(out, byte(v>>24), byte(v>>16), byte(v>>8), byte(v)) }

	// SOC
	out = append(out, 0xFF, 0x4F)
	// SIZ
	out = append(out, 0xFF, 0x51)
	put16(38 + 3*nc) // Lsiz
	put16(0)         // Rsiz
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
	out = append(out, 0x00) // code-block style
	out = append(out, 0x01) // transform 5/3
	// QCD (reversible / no quantization)
	out = append(out, 0xFF, 0x5C)
	put16(3 + len(exps))                     // Lqcd
	out = append(out, byte(encGuardBits<<5)) // Sqcd: guard bits, style 0
	for _, e := range exps {
		out = append(out, byte(e<<3)) // exponent in high 5 bits
	}
	// SOT
	sodAndData := 2 + len(packets) // SOD marker + packet data
	psot := 12 + sodAndData        // SOT segment (12) + SOD + data
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
