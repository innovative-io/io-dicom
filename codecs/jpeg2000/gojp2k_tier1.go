package jpeg2000

// Tier-1 EBCOT code-block decoding (ITU-T T.800 Annex D): decode each
// code-block's MQ-coded segment, bit-plane by bit-plane, through the three
// coding passes (significance propagation, magnitude refinement, cleanup) with
// context modeling, producing signed wavelet coefficients.
//
// Baseline scope: code-block style 0 (no arithmetic bypass, no reset, no
// per-pass termination, no vertically-causal/segmentation flags) — matches the
// DICOM J2K fixtures.

// Context indices (T.800).
const (
	ctxUNI = 18 // uniform
	ctxRUN = 17 // run-length
	ctxZC0 = 0  // first significance context
)

// initContexts returns the 19 MQ contexts with their initial states
// (T.800 Table D.7): UNIFORM→46, RUNLENGTH→3, the all-zero ZC context→4,
// everything else→0; all MPS=0.
func initContexts() [19]mqContext {
	var cx [19]mqContext
	cx[ctxZC0].index = 4
	cx[ctxRUN].index = 3
	cx[ctxUNI].index = 46
	return cx
}

// t1 holds the per-code-block decoding state.
type t1 struct {
	w, h    int
	sig     []bool  // significant
	sign    []bool  // true = negative
	mag     []int32 // accumulated magnitude
	visited []bool  // touched in the current bit-plane's significance pass
	refined []bool  // has been refined at least once
	orient  int
	mq      *mqDecoder
	cx      [19]mqContext
}

func (t *t1) idx(x, y int) int { return y*t.w + x }

// sigAt reports whether neighbor (x,y) is significant (out-of-bounds = false).
func (t *t1) sigAt(x, y int) bool {
	if x < 0 || y < 0 || x >= t.w || y >= t.h {
		return false
	}
	return t.sig[y*t.w+x]
}

// signAt returns -1/0/+1 sign contribution of neighbor (x,y): 0 if insignificant
// or out of bounds, -1 if significant&negative, +1 if significant&positive.
func (t *t1) signContrib(x, y int) int {
	if x < 0 || y < 0 || x >= t.w || y >= t.h || !t.sig[y*t.w+x] {
		return 0
	}
	if t.sign[y*t.w+x] {
		return -1
	}
	return 1
}

// zcContext computes the zero-coding (significance) context (T.800 Table D.1).
func (t *t1) zcContext(x, y int) int {
	h := b2i(t.sigAt(x-1, y)) + b2i(t.sigAt(x+1, y))
	v := b2i(t.sigAt(x, y-1)) + b2i(t.sigAt(x, y+1))
	d := b2i(t.sigAt(x-1, y-1)) + b2i(t.sigAt(x+1, y-1)) + b2i(t.sigAt(x-1, y+1)) + b2i(t.sigAt(x+1, y+1))

	switch t.orient {
	case bandHL:
		h, v = v, h // HL swaps horizontal/vertical roles
		fallthrough
	case bandLL, bandLH:
		switch {
		case h == 2:
			return 8
		case h == 1:
			if v >= 1 {
				return 7
			}
			if d >= 1 {
				return 6
			}
			return 5
		case v == 2:
			return 4
		case v == 1:
			return 3
		case d >= 2:
			return 2
		case d == 1:
			return 1
		}
		return 0
	default: // bandHH
		hv := h + v
		switch {
		case d >= 3:
			return 8
		case d == 2:
			if hv >= 1 {
				return 7
			}
			return 6
		case d == 1:
			if hv >= 2 {
				return 5
			}
			if hv == 1 {
				return 4
			}
			return 3
		case hv >= 2:
			return 2
		case hv == 1:
			return 1
		}
		return 0
	}
}

// scContext computes the sign-coding context and the sign XOR-bit (T.800 D.3.2).
func (t *t1) scContext(x, y int) (int, int) {
	hc := clamp1(t.signContrib(x-1, y) + t.signContrib(x+1, y))
	vc := clamp1(t.signContrib(x, y-1) + t.signContrib(x, y+1))
	// Table D.2 mapping.
	if hc == 1 {
		switch vc {
		case 1:
			return 13, 0
		case 0:
			return 12, 0
		default:
			return 11, 0
		}
	} else if hc == 0 {
		switch vc {
		case 1:
			return 10, 0
		case 0:
			return 9, 0
		default:
			return 10, 1
		}
	}
	// hc == -1
	switch vc {
	case 1:
		return 11, 1
	case 0:
		return 12, 1
	default:
		return 13, 1
	}
}

// mrContext computes the magnitude-refinement context (T.800 D.3.3).
func (t *t1) mrContext(x, y int) int {
	if t.refined[t.idx(x, y)] {
		return 16
	}
	// first refinement: 15 if any 8-neighbour significant, else 14.
	if t.sigAt(x-1, y) || t.sigAt(x+1, y) || t.sigAt(x, y-1) || t.sigAt(x, y+1) ||
		t.sigAt(x-1, y-1) || t.sigAt(x+1, y-1) || t.sigAt(x-1, y+1) || t.sigAt(x+1, y+1) {
		return 15
	}
	return 14
}

// decodeSign decodes the sign of a newly-significant coefficient at (x,y).
func (t *t1) decodeSign(x, y int) {
	ctx, xorBit := t.scContext(x, y)
	s := t.mq.decode(&t.cx[ctx]) ^ xorBit
	t.sign[t.idx(x, y)] = s == 1
}

func (t *t1) anyNeighborSignificant(x, y int) bool {
	return t.sigAt(x-1, y) || t.sigAt(x+1, y) || t.sigAt(x, y-1) || t.sigAt(x, y+1) ||
		t.sigAt(x-1, y-1) || t.sigAt(x+1, y-1) || t.sigAt(x-1, y+1) || t.sigAt(x+1, y+1)
}

// sigPropPass: significance propagation (T.800 D.3.1).
func (t *t1) sigPropPass(bp int) {
	for y0 := 0; y0 < t.h; y0 += 4 {
		for x := 0; x < t.w; x++ {
			for y := y0; y < y0+4 && y < t.h; y++ {
				i := t.idx(x, y)
				t.visited[i] = false
				if t.sig[i] || !t.anyNeighborSignificant(x, y) {
					continue
				}
				if t.mq.decode(&t.cx[t.zcContext(x, y)]) == 1 {
					t.decodeSign(x, y)
					t.sig[i] = true
					t.mag[i] |= 1 << uint(bp)
				}
				t.visited[i] = true
			}
		}
	}
}

// magRefPass: magnitude refinement (T.800 D.3.4).
func (t *t1) magRefPass(bp int) {
	for y0 := 0; y0 < t.h; y0 += 4 {
		for x := 0; x < t.w; x++ {
			for y := y0; y < y0+4 && y < t.h; y++ {
				i := t.idx(x, y)
				if !t.sig[i] || t.visited[i] {
					continue
				}
				bit := t.mq.decode(&t.cx[t.mrContext(x, y)])
				t.mag[i] |= int32(bit) << uint(bp)
				t.refined[i] = true
			}
		}
	}
}

// cleanupPass: cleanup with run-length coding (T.800 D.3.5).
func (t *t1) cleanupPass(bp int) {
	for y0 := 0; y0 < t.h; y0 += 4 {
		for x := 0; x < t.w; x++ {
			y := y0
			stripeH := 4
			if y0+4 > t.h {
				stripeH = t.h - y0
			}
			// Run-length coding: only when the full 4-row column is insignificant
			// and none was visited in SPP and none has a significant neighbour.
			useRun := stripeH == 4
			if useRun {
				for k := 0; k < 4; k++ {
					i := t.idx(x, y0+k)
					if t.sig[i] || t.visited[i] || t.anyNeighborSignificant(x, y0+k) {
						useRun = false
						break
					}
				}
			}
			if useRun {
				if t.mq.decode(&t.cx[ctxRUN]) == 0 {
					continue // no coefficient in the column becomes significant
				}
				// run length: 2 bits, uniform context, MSB first.
				r := (t.mq.decode(&t.cx[ctxUNI]) << 1) | t.mq.decode(&t.cx[ctxUNI])
				yy := y0 + r
				t.sig[t.idx(x, yy)] = true
				t.decodeSign(x, yy)
				t.mag[t.idx(x, yy)] |= 1 << uint(bp)
				y = y0 + r + 1
			}
			for ; y < y0+stripeH; y++ {
				i := t.idx(x, y)
				if t.sig[i] || t.visited[i] {
					continue
				}
				if t.mq.decode(&t.cx[t.zcContext(x, y)]) == 1 {
					t.decodeSign(x, y)
					t.sig[i] = true
					t.mag[i] |= 1 << uint(bp)
				}
			}
		}
	}
}

// decodeCodeBlock decodes one code-block's coefficients. mb is the number of
// magnitude bit-planes for the subband (Mb). Returns signed coefficients.
func decodeCodeBlock(cb *codeBlock, orient, mb int) []int32 {
	w, h := cb.w(), cb.h()
	if w <= 0 || h <= 0 || cb.npasses == 0 || len(cb.segs) == 0 {
		return make([]int32, w*h)
	}
	// Concatenate segments (baseline: no per-pass termination → typically one).
	var data []byte
	if len(cb.segs) == 1 {
		data = cb.segs[0]
	} else {
		for _, s := range cb.segs {
			data = append(data, s...)
		}
	}
	t := &t1{
		w: w, h: h, orient: orient,
		sig:     make([]bool, w*h),
		sign:    make([]bool, w*h),
		mag:     make([]int32, w*h),
		visited: make([]bool, w*h),
		refined: make([]bool, w*h),
		cx:      initContexts(),
		mq:      newMQDecoder(data, 0, len(data)),
	}

	bpStart := mb - 1 - cb.nzeroBP
	if bpStart < 0 {
		return t.mag
	}
	bp := bpStart
	passNo := 0
	lowestBP := bpStart
	// First bit-plane: cleanup only.
	t.cleanupPass(bp)
	passNo++
	lowestBP = bp
	bp--
	for passNo < cb.npasses && bp >= 0 {
		if passNo < cb.npasses {
			t.sigPropPass(bp)
			passNo++
			lowestBP = bp
		}
		if passNo < cb.npasses {
			t.magRefPass(bp)
			passNo++
			lowestBP = bp
		}
		if passNo < cb.npasses {
			t.cleanupPass(bp)
			passNo++
			lowestBP = bp
		}
		bp--
	}

	// Mid-point reconstruction: when the code-block was not decoded to bit-plane 0
	// (rate truncation), each significant coefficient's true value lies in
	// [mag, mag+2^lowestBP); reconstruct at the mid-point by setting the bit just
	// below the lowest decoded bit-plane (T.800 / openjpeg "halfb").
	half := int32(0)
	if lowestBP > 0 {
		half = 1 << uint(lowestBP-1)
	}
	out := make([]int32, w*h)
	for i := range out {
		m := t.mag[i]
		if m != 0 {
			m += half
		}
		if t.sign[i] {
			out[i] = -m
		} else {
			out[i] = m
		}
	}
	return out
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func clamp1(v int) int {
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return v
}
