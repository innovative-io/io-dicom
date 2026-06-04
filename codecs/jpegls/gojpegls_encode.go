package jpegls

// Pure-Go JPEG-LS lossless encoder — the inverse of the decoder in gojpegls.go.
// It lets CGO_ENABLED=0 builds produce valid lossless JPEG-LS (e.g. for
// transcoding) instead of passing raw bytes through. Single-component
// (grayscale), 2–16 bit, NEAR=0.

// jlsBitWriter writes MSB-first bits with JPEG-LS bit stuffing: after a 0xFF
// byte is emitted, the next byte carries a stuffed 0 in its MSB (7 data bits).
type jlsBitWriter struct {
	out    []byte
	cur    byte
	nbits  uint
	lastFF bool
}

func (w *jlsBitWriter) writeBit(bit int) {
	w.cur = (w.cur << 1) | byte(bit&1)
	w.nbits++
	limit := uint(8)
	if w.lastFF {
		limit = 7
	}
	if w.nbits == limit {
		w.out = append(w.out, w.cur)
		w.lastFF = w.cur == 0xFF
		w.cur = 0
		w.nbits = 0
	}
}

func (w *jlsBitWriter) writeBits(val uint32, cnt uint) {
	for i := int(cnt) - 1; i >= 0; i-- {
		w.writeBit(int((val >> uint(i)) & 1))
	}
}

func (w *jlsBitWriter) writeZeros(n int) {
	for i := 0; i < n; i++ {
		w.writeBit(0)
	}
}

func (w *jlsBitWriter) flush() {
	// JPEG-LS pads the final partial byte with 0 bits (unlike baseline JPEG's
	// 1-bit padding); 1-padding makes a conforming decoder read a phantom sample.
	for w.nbits != 0 {
		w.writeBit(0)
	}
}

// encodeValue is the inverse of decodeValue: a limited-length Golomb code.
func (d *jlsDecoder) encodeValue(w *jlsBitWriter, merr, k, limit int) {
	high := merr >> uint(k)
	if high < limit-(d.qbpp+1) {
		w.writeZeros(high)
		w.writeBit(1)
		if k > 0 {
			w.writeBits(uint32(merr&((1<<uint(k))-1)), uint(k))
		}
		return
	}
	w.writeZeros(limit - (d.qbpp + 1))
	w.writeBit(1)
	w.writeBits(uint32(merr-1), uint(d.qbpp))
}

// encodeRegular encodes one regular-mode sample (inverse of decodeRegular).
func (d *jlsDecoder) encodeRegular(w *jlsBitWriter, q, sign, px, sample int) int {
	k := 0
	for (d.n[q] << k) < d.a[q] {
		k++
	}
	// errval such that the decoder reconstructs an in-tolerance sample from px.
	// For near-lossless the prediction error is quantized (CharLS quantize), then
	// reduced modulo RANGE consistently with the decoder.
	errval := d.quantizeError(sign * (sample - px))
	half := d.range_ / 2
	if errval < -half {
		errval += d.range_
	} else if errval >= d.range_-half {
		errval -= d.range_
	}
	// inverse of the k==0 bias flip; applies in pure-lossless mode only (NEAR==0),
	// matching the decoder / CharLS get_error_correction(near_lossless).
	mapped := errval
	if k == 0 && d.f.near == 0 && 2*d.b[q]+d.n[q]-1 < 0 {
		mapped = -errval - 1
	}
	var merr int
	if mapped >= 0 {
		merr = 2 * mapped
	} else {
		merr = -2*mapped - 1
	}
	d.encodeValue(w, merr, k, d.limit)
	d.updateRegular(q, errval)
	// Return the value the decoder will reconstruct, so the encoder feeds
	// reconstructed (not original) neighbors — required for near-lossless.
	return d.computeRecon(px, sign*errval)
}

// quantizeError quantizes a signed prediction error for the configured NEAR
// (CharLS quantize). For NEAR==0 it is the identity.
func (d *jlsDecoder) quantizeError(e int) int {
	near := d.f.near
	if near == 0 {
		return e
	}
	if e > 0 {
		return (e + near) / (2*near + 1)
	}
	return -((near - e) / (2*near + 1))
}

// encodeRunInterruption encodes the sample that ended a run (inverse of
// decodeRunInterruption).
func (d *jlsDecoder) encodeRunInterruption(w *jlsBitWriter, ra, rb, sample int) int {
	var q, nRItype, px, sign int
	if abs(ra-rb) <= d.f.near {
		q, nRItype, px, sign = 366, 1, ra, 1
	} else {
		q, nRItype, px = 365, 0, rb
		if rb >= ra {
			sign = 1
		} else {
			sign = -1
		}
	}
	errval := d.quantizeError(sign * (sample - px))
	half := d.range_ / 2
	if errval < -half {
		errval += d.range_
	} else if errval >= d.range_-half {
		errval -= d.range_
	}

	temp := d.a[q] + (d.n[q]>>1)*nRItype
	k := 0
	for (d.n[q] << k) < temp {
		k++
	}
	// inverse of ComputeErrVal: find EMErrval whose decode yields errval.
	mapFlagBase := 0
	if k != 0 || 2*d.nn[q] >= d.n[q] {
		mapFlagBase = 1
	}
	// errval = (condition==map) ? -errAbs : errAbs, with map=(EMErrval+nRItype)&1.
	// Invert: choose errAbs and parity so the relation holds.
	errAbs := errval
	neg := 0
	if errval < 0 {
		errAbs = -errval
		neg = 1
	}
	// condition(==mapFlagBase) == map => negative. So map = neg==1 ? mapFlagBase : 1-mapFlagBase
	var mapFlag int
	if neg == 1 {
		mapFlag = mapFlagBase
	} else {
		mapFlag = 1 - mapFlagBase
	}
	t := 2*errAbs - mapFlag // since errAbs = (t+map)/2  => t = 2*errAbs - map
	merr := t - nRItype
	limit := d.limit - jlsRunJ[d.runIndex] - 1
	d.encodeValue(w, merr, k, limit)

	// context update (identical to decode)
	if errval < 0 {
		d.nn[q]++
	}
	emerr := t
	d.a[q] += (emerr + 1 - nRItype) >> 1
	if d.n[q] == d.f.reset {
		d.a[q] >>= 1
		d.n[q] >>= 1
		d.nn[q] >>= 1
	}
	d.n[q]++
	// The decoder reconstructs the interrupting sample from px and the signed
	// error; return it so the encoder feeds reconstructed neighbors.
	return d.computeRecon(px, sign*errval)
}

// encodePlane encodes one component plane using regular + run mode. Each encoded
// sample's slot in plane is overwritten with the value the decoder reconstructs,
// so subsequent samples use reconstructed neighbors (required for near-lossless;
// a no-op for NEAR==0 where reconstructed == original).
func (d *jlsDecoder) encodePlane(w *jlsBitWriter, plane []int) {
	W, H := d.f.width, d.f.height
	d.runIndex = 0
	prev := make([]int, W)
	rcLeft := 0
	for y := 0; y < H; y++ {
		lineRb0 := 0
		if y > 0 {
			lineRb0 = prev[0]
		}
		cur := plane[y*W : y*W+W]
		x := 0
		for x < W {
			var a, b, c, dd int
			if y == 0 {
				b, c, dd = 0, 0, 0
			} else {
				b = prev[x]
				if x+1 < W {
					dd = prev[x+1]
				} else {
					dd = b
				}
			}
			if x == 0 {
				a = lineRb0
				c = rcLeft
			} else {
				a = cur[x-1]
				if y > 0 {
					c = prev[x-1]
				}
			}

			q1 := d.quantize(dd - b)
			q2 := d.quantize(b - c)
			q3 := d.quantize(c - a)

			if q1 == 0 && q2 == 0 && q3 == 0 {
				x = d.encodeRun(w, cur, prev, y, a, x, W)
				continue
			}
			q := 81*q1 + 9*q2 + q3
			sign := 1
			if q < 0 {
				q = -q
				sign = -1
			}
			px := predict(a, b, c)
			if sign > 0 {
				px += d.c[q]
			} else {
				px -= d.c[q]
			}
			if px < 0 {
				px = 0
			} else if px > d.f.maxval {
				px = d.f.maxval
			}
			cur[x] = d.encodeRegular(w, q, sign, px, cur[x])
			x++
		}
		rcLeft = lineRb0
		copy(prev, cur)
	}
}

// encodeRun encodes a run starting at x (inverse of decodeRun).
func (d *jlsDecoder) encodeRun(w *jlsBitWriter, cur, prev []int, y, ra, x, W int) int {
	runVal := ra
	// measure run length: samples within NEAR of runVal up to end of line. Every
	// run sample reconstructs to runVal, so write that back as the neighbor value.
	runLen := 0
	for x+runLen < W && abs(cur[x+runLen]-runVal) <= d.f.near {
		runLen++
	}
	for i := 0; i < runLen; i++ {
		cur[x+i] = runVal
	}
	remaining := W - x
	reachedEOL := x+runLen == W
	r := runLen
	for r >= (1 << jlsRunJ[d.runIndex]) {
		w.writeBit(1)
		r -= 1 << jlsRunJ[d.runIndex]
		if d.runIndex < 31 {
			d.runIndex++
		}
	}
	if reachedEOL {
		// the run reached end of line; if a partial block remains, emit a 1.
		if r > 0 {
			w.writeBit(1)
		}
		return x + runLen
	}
	// interrupted: 0 bit + remainder in J[runIndex] bits, then RI sample.
	w.writeBit(0)
	if jlsRunJ[d.runIndex] > 0 {
		w.writeBits(uint32(r), uint(jlsRunJ[d.runIndex]))
	}
	_ = remaining
	pos := x + runLen
	rb := 0
	if y > 0 {
		rb = prev[pos]
	}
	cur[pos] = d.encodeRunInterruption(w, runVal, rb, cur[pos])
	if d.runIndex > 0 {
		d.runIndex--
	}
	return pos + 1
}

// encodeJLS encodes raw samples (LE if precision>8) as JPEG-LS. NEAR==0 yields
// lossless output; NEAR>0 yields near-lossless output with the given error bound.
func encodeJLS(raw []byte, width, height, samples, precision, near int) ([]byte, error) {
	// Single-component only: multi-component ILV=0 is not validated against the
	// reference and charls rejects it, so it is left to charls.
	if width <= 0 || height <= 0 || samples != 1 || precision < 2 || precision > 16 {
		return nil, errJLSUnsupported
	}
	// NEAR is written as a single byte in the SOS scan header and must not exceed
	// MAXVAL (T.87 C.2.4.1.1). Reject out-of-range values rather than truncating
	// them into a non-conformant stream.
	maxval := (1 << precision) - 1
	if near < 0 || near > 255 || near > maxval {
		return nil, errJLSUnsupported
	}
	bps := 1
	if precision > 8 {
		bps = 2
	}
	if width*height*samples*bps != len(raw) {
		return nil, errJLSUnsupported
	}
	f := jlsFrame{precision: precision, width: width, height: height, near: near, ilv: 0}
	f.comps = make([]jlsComponent, samples)
	for c := range f.comps {
		f.comps[c] = jlsComponent{id: byte(c + 1), h: 1, v: 1}
	}
	f.computeDefaultParams()

	// header
	out := []byte{0xFF, jlsSOI}
	sof := []byte{byte(precision), byte(height >> 8), byte(height), byte(width >> 8), byte(width), byte(samples)}
	for c := 0; c < samples; c++ {
		sof = append(sof, byte(c+1), 0x11, 0x00)
	}
	out = appendJLSMarker(out, jlsSOF55, sof)
	sos := []byte{byte(samples)}
	for c := 0; c < samples; c++ {
		sos = append(sos, byte(c+1), 0x00)
	}
	sos = append(sos, byte(near), 0x00, 0x00) // NEAR, ILV=0, point transform=0
	out = appendJLSMarker(out, jlsSOS, sos)

	// planes
	planes := make([][]int, samples)
	for c := 0; c < samples; c++ {
		planes[c] = make([]int, width*height)
	}
	for px := 0; px < width*height; px++ {
		for c := 0; c < samples; c++ {
			idx := px*samples + c
			if bps == 1 {
				planes[c][px] = int(raw[idx])
			} else {
				planes[c][px] = int(raw[idx*2]) | int(raw[idx*2+1])<<8
			}
		}
	}

	// samples == 1 here (multi-component is rejected above), so this encodes the
	// single component's plane; encodePlane resets only the run index, mirroring
	// the decoder's decodePlane.
	d := newJLSDecoder(&f, nil)
	w := &jlsBitWriter{out: out}
	for c := 0; c < samples; c++ {
		d.encodePlane(w, planes[c])
	}
	w.flush()
	out = w.out
	out = append(out, 0xFF, jlsEOI)
	return out, nil
}

func appendJLSMarker(out []byte, marker byte, payload []byte) []byte {
	out = append(out, 0xFF, marker)
	n := len(payload) + 2
	out = append(out, byte(n>>8), byte(n))
	return append(out, payload...)
}
