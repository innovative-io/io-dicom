package jpeg2000

// Tier-1 EBCOT code-block encoding (ITU-T T.800 Annex D), the inverse of the
// tier-1 decoder. For each bit-plane from the most significant used plane down to
// 0, it runs the three coding passes (significance propagation, magnitude
// refinement, cleanup), emitting each binary decision through the MQ encoder with
// the same context model and iteration order the decoder uses — so the produced
// segment decodes back to the original coefficients.

// t1enc holds the per-code-block encoding state. It reuses the decoder's context
// helpers (zcContext, scContext, mrContext, etc.) via an embedded t1 view.
type t1enc struct {
	t1
	absmag []int32 // |coefficient| per sample
	enc    *mqEncoder
}

func (e *t1enc) bit(i, bp int) int { return int((e.absmag[i] >> uint(bp)) & 1) }

func (e *t1enc) encodeSign(x, y int) {
	ctx, xorBit := e.scContext(x, y)
	s := 0
	if e.sign[e.idx(x, y)] {
		s = 1
	}
	e.enc.encode(&e.cx[ctx], s^xorBit)
}

func (e *t1enc) sigPropPassEnc(bp int) {
	for y0 := 0; y0 < e.h; y0 += 4 {
		for x := 0; x < e.w; x++ {
			for y := y0; y < y0+4 && y < e.h; y++ {
				i := e.idx(x, y)
				e.visited[i] = false
				if e.sig[i] || !e.anyNeighborSignificant(x, y) {
					continue
				}
				b := e.bit(i, bp)
				e.enc.encode(&e.cx[e.zcContext(x, y)], b)
				if b == 1 {
					e.encodeSign(x, y)
					e.sig[i] = true
				}
				e.visited[i] = true
			}
		}
	}
}

func (e *t1enc) magRefPassEnc(bp int) {
	for y0 := 0; y0 < e.h; y0 += 4 {
		for x := 0; x < e.w; x++ {
			for y := y0; y < y0+4 && y < e.h; y++ {
				i := e.idx(x, y)
				if !e.sig[i] || e.visited[i] {
					continue
				}
				e.enc.encode(&e.cx[e.mrContext(x, y)], e.bit(i, bp))
				e.refined[i] = true
			}
		}
	}
}

func (e *t1enc) cleanupPassEnc(bp int) {
	for y0 := 0; y0 < e.h; y0 += 4 {
		for x := 0; x < e.w; x++ {
			y := y0
			stripeH := 4
			if y0+4 > e.h {
				stripeH = e.h - y0
			}
			useRun := stripeH == 4
			if useRun {
				for k := 0; k < 4; k++ {
					i := e.idx(x, y0+k)
					if e.sig[i] || e.visited[i] || e.anyNeighborSignificant(x, y0+k) {
						useRun = false
						break
					}
				}
			}
			if useRun {
				first := -1
				for k := 0; k < 4; k++ {
					if e.bit(e.idx(x, y0+k), bp) == 1 {
						first = k
						break
					}
				}
				if first < 0 {
					e.enc.encode(&e.cx[ctxRUN], 0)
					continue
				}
				e.enc.encode(&e.cx[ctxRUN], 1)
				e.enc.encode(&e.cx[ctxUNI], (first>>1)&1)
				e.enc.encode(&e.cx[ctxUNI], first&1)
				yy := y0 + first
				e.sig[e.idx(x, yy)] = true
				e.encodeSign(x, yy)
				y = y0 + first + 1
			}
			for ; y < y0+stripeH; y++ {
				i := e.idx(x, y)
				if e.sig[i] || e.visited[i] {
					continue
				}
				b := e.bit(i, bp)
				e.enc.encode(&e.cx[e.zcContext(x, y)], b)
				if b == 1 {
					e.encodeSign(x, y)
					e.sig[i] = true
				}
			}
		}
	}
}

// encodeCodeBlock encodes signed coefficients (row-major over w×h) into one
// MQ-coded segment. mb is the subband's magnitude bit-plane count (Mb). Returns
// the coded bytes, the number of coding passes, and the number of leading
// all-zero (insignificant) bit-planes. An all-zero block returns npasses 0.
func encodeCodeBlock(coeffs []int32, w, h, orient, mb int) (data []byte, npasses, nzeroBP int) {
	n := w * h
	absmag := make([]int32, n)
	sign := make([]bool, n)
	maxBit := -1
	for i, c := range coeffs {
		m := c
		if m < 0 {
			m = -m
			sign[i] = true
		}
		absmag[i] = m
		for b := mb - 1; b > maxBit; b-- {
			if (m>>uint(b))&1 == 1 {
				maxBit = b
				break
			}
		}
	}
	if maxBit < 0 {
		return nil, 0, 0 // insignificant block
	}
	nzeroBP = mb - 1 - maxBit

	e := &t1enc{
		t1: t1{
			w: w, h: h, orient: orient,
			sig:     make([]bool, n),
			sign:    sign,
			visited: make([]bool, n),
			refined: make([]bool, n),
			cx:      initContexts(),
		},
		absmag: absmag,
		enc:    newMQEncoder(),
	}

	bp := maxBit
	e.cleanupPassEnc(bp)
	npasses = 1
	bp--
	for ; bp >= 0; bp-- {
		e.sigPropPassEnc(bp)
		e.magRefPassEnc(bp)
		e.cleanupPassEnc(bp)
		npasses += 3
	}
	return e.enc.flush(), npasses, nzeroBP
}
