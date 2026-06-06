package jpegls

import "context"

// parseMarkers walks the JPEG-LS marker segments and returns the frame metadata
// plus the offset where the entropy-coded scan data begins.
func parseJLS(data []byte) (jlsFrame, int, error) {
	var f jlsFrame
	if len(data) < 2 || data[0] != 0xFF || data[1] != jlsSOI {
		return f, 0, errJLSMalformed
	}
	i := 2
	sawSOF := false
	for {
		if i+1 >= len(data) || data[i] != 0xFF {
			return f, 0, errJLSMalformed
		}
		marker := data[i+1]
		i += 2
		if marker == jlsEOI {
			return f, 0, errJLSMalformed
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			continue
		}
		if i+2 > len(data) {
			return f, 0, errJLSMalformed
		}
		segLen := int(data[i])<<8 | int(data[i+1])
		if segLen < 2 || i+segLen > len(data) {
			return f, 0, errJLSMalformed
		}
		seg := data[i+2 : i+segLen]

		switch marker {
		case jlsSOF55:
			if err := parseJLSFrame(seg, &f); err != nil {
				return f, 0, err
			}
			sawSOF = true
		case jlsLSE:
			if err := parseJLSPreset(seg, &f); err != nil {
				return f, 0, err
			}
		case jlsSOS:
			if !sawSOF {
				return f, 0, errJLSMalformed
			}
			if err := parseJLSScan(seg, &f); err != nil {
				return f, 0, err
			}
			f.computeDefaultParams()
			return f, i + segLen, nil
		default:
			// APPn, COM, etc.: skip.
		}
		i += segLen
	}
}

func parseJLSFrame(seg []byte, f *jlsFrame) error {
	if len(seg) < 6 {
		return errJLSMalformed
	}
	f.precision = int(seg[0])
	f.height = int(seg[1])<<8 | int(seg[2])
	f.width = int(seg[3])<<8 | int(seg[4])
	nf := int(seg[5])
	if f.precision < 2 || f.precision > 16 || f.width == 0 || f.height == 0 {
		return errJLSUnsupported
	}
	if nf < 1 || nf > 255 {
		return errJLSUnsupported
	}
	if int64(f.width)*int64(f.height)*int64(nf) > maxJLSSamples {
		return errJLSUnsupported
	}
	if len(seg) < 6+nf*3 {
		return errJLSMalformed
	}
	f.comps = make([]jlsComponent, nf)
	for c := 0; c < nf; c++ {
		off := 6 + c*3
		f.comps[c].id = seg[off]
		f.comps[c].h = int(seg[off+1] >> 4)
		f.comps[c].v = int(seg[off+1] & 0x0F)
	}
	return nil
}

func parseJLSPreset(seg []byte, f *jlsFrame) error {
	if len(seg) < 1 {
		return errJLSMalformed
	}
	switch seg[0] {
	case 1: // preset coding parameters: ID + MAXVAL + T1 + T2 + T3 + RESET
		if len(seg) < 11 {
			return errJLSMalformed
		}
		f.maxval = int(seg[1])<<8 | int(seg[2])
		f.t1 = int(seg[3])<<8 | int(seg[4])
		f.t2 = int(seg[5])<<8 | int(seg[6])
		f.t3 = int(seg[7])<<8 | int(seg[8])
		f.reset = int(seg[9])<<8 | int(seg[10])
	default:
		return errJLSUnsupported // mapping tables / other LSE types not supported
	}
	return nil
}

func parseJLSScan(seg []byte, f *jlsFrame) error {
	if len(seg) < 1 {
		return errJLSMalformed
	}
	ns := int(seg[0])
	if ns != len(f.comps) || len(seg) < 1+ns*2+3 {
		return errJLSUnsupported
	}
	tail := seg[1+ns*2:]
	f.near = int(tail[0])
	f.ilv = int(tail[1])
	if f.ilv != 0 && f.ilv != 1 && f.ilv != 2 {
		return errJLSUnsupported // ILV must be 0 (none), 1 (line) or 2 (sample)
	}
	// Non-interleaved multi-component is encoded as separate single-component
	// scans (one SOS each), which this single-scan reader does not assemble; only
	// single-component ILV=0 is supported.
	if f.ilv == 0 && len(f.comps) != 1 {
		return errJLSUnsupported
	}
	// Sample-interleaved coding is defined only for 3-component (triplet) or
	// 4-component (quad) pixels (CharLS do_line(triplet/quad)).
	if f.ilv == 2 && len(f.comps) != 3 && len(f.comps) != 4 {
		return errJLSUnsupported
	}
	// tail[2] is the point-transform field (Al). The decoder does not apply a
	// point transform, so a non-zero value would silently yield wrong pixels —
	// reject it as unsupported.
	if tail[2] != 0 {
		return errJLSUnsupported
	}
	return nil
}

const maxJLSSamples = 1 << 28

// decodeJLSInto decodes a JPEG-LS payload into output using the codec's
// little-endian convention (2 bytes/sample when precision > 8).
func decodeJLSInto(encoded, output []byte) error {
	f, scanOff, err := parseJLS(encoded)
	if err != nil {
		return err
	}
	nc := len(f.comps)
	bps := 1
	if f.precision > 8 {
		bps = 2
	}
	// int64 to avoid overflow on 32-bit builds (nc up to 255).
	need := int64(f.width) * int64(f.height) * int64(nc) * int64(bps)
	if need > int64(len(output)) {
		return errJLSOutputSize
	}

	d := newJLSDecoder(&f, encoded[scanOff:])
	switch f.ilv {
	case 1:
		d.decodeLineInterleaved(output, nc, bps)
	case 2:
		d.decodeSampleInterleaved(output, nc, bps)
	default:
		// Non-interleaved (ILV=0): each component is a full plane, decoded in
		// turn and written directly into the pixel-interleaved (component-minor),
		// little-endian output one line at a time. Streaming per line avoids a
		// W*H intermediate int plane (8 bytes/sample), which on large images was
		// the dominant allocation and could OOM the process.
		for c := 0; c < nc; c++ {
			d.decodePlaneInto(output, c, nc, bps)
		}
	}
	return nil
}

// decodeScalarLine decodes one scan line of a single component into cur, using
// prev as the reconstructed line above and rcLeft as Rc at the start of the line
// (R[y-1][-1]). It returns the rcLeft to pass for the next line. The regular/run
// context state and d.runIndex are shared via the decoder; line-interleaved
// scans save/restore d.runIndex per component around this call.
func (d *jlsDecoder) decodeScalarLine(cur, prev []int, y, rcLeft int) int {
	W := d.f.width
	// The callers zero-initialize prev for row 0, so reading prev[x] yields the
	// correct all-zero neighbors there; the per-pixel y==0 special-cases are thus
	// redundant and omitted, keeping the hot inner loop branch-free on row index.
	lineRb0 := prev[0]
	x := 0
	for x < W {
		// neighbors
		b := prev[x]
		var dd int
		if x+1 < W {
			dd = prev[x+1]
		} else {
			dd = b
		}
		var a, c int
		if x == 0 {
			a = lineRb0 // Ra at line start = sample above (b)
			c = rcLeft
		} else {
			a = cur[x-1]
			c = prev[x-1]
		}

		q1 := d.quantize(dd - b)
		q2 := d.quantize(b - c)
		q3 := d.quantize(c - a)

		if q1 == 0 && q2 == 0 && q3 == 0 {
			x = d.decodeRun(cur, prev, y, a, x, W)
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
			px += d.ctx[q].c
		} else {
			px -= d.ctx[q].c
		}
		if px < 0 {
			px = 0
		} else if px > d.f.maxval {
			px = d.f.maxval
		}
		cur[x] = d.decodeRegular(q, sign, px)
		x++
	}
	return lineRb0
}

// putSample writes a reconstructed sample into the codec's little-endian output
// at sample index idx (bps bytes per sample).
func putSample(out []byte, idx, v, bps int) {
	if bps == 1 {
		out[idx] = byte(v)
		return
	}
	out[idx*2] = byte(v)
	out[idx*2+1] = byte(v >> 8)
}

// decodeLineInterleaved decodes an ILV=1 (line-interleaved) multi-component scan
// directly into the pixel-interleaved output. Each component keeps its own
// reconstructed lines and run index; the regular/run context statistics are
// shared across components (CharLS decode_lines, interleave_mode::line).
func (d *jlsDecoder) decodeLineInterleaved(out []byte, nc, bps int) {
	W, H := d.f.width, d.f.height
	prev := make([][]int, nc)
	cur := make([][]int, nc)
	rcLeft := make([]int, nc)
	runIdx := make([]int, nc)
	for c := 0; c < nc; c++ {
		prev[c] = make([]int, W)
		cur[c] = make([]int, W)
	}
	for y := 0; y < H; y++ {
		for c := 0; c < nc; c++ {
			d.runIndex = runIdx[c]
			rcLeft[c] = d.decodeScalarLine(cur[c], prev[c], y, rcLeft[c])
			runIdx[c] = d.runIndex
		}
		base := y * W * nc
		for x := 0; x < W; x++ {
			for c := 0; c < nc; c++ {
				putSample(out, base+x*nc+c, cur[c][x], bps)
			}
		}
		for c := 0; c < nc; c++ {
			prev[c], cur[c] = cur[c], prev[c]
		}
	}
}

// decodeSampleInterleaved decodes an ILV=2 (sample-interleaved) multi-component
// scan directly into the pixel-interleaved output. All components share a single
// run index and the regular/run context statistics; a pixel enters run mode only
// when every component's context is 0, and a run reconstructs whole pixels
// (CharLS decode_lines + do_line(triplet/quad), interleave_mode::sample).
func (d *jlsDecoder) decodeSampleInterleaved(out []byte, nc, bps int) {
	W, H := d.f.width, d.f.height
	prev := make([][]int, nc)
	cur := make([][]int, nc)
	rcLeft := make([]int, nc)
	for c := 0; c < nc; c++ {
		prev[c] = make([]int, W)
		cur[c] = make([]int, W)
	}
	ra := make([]int, nc)
	rb := make([]int, nc)
	rc := make([]int, nc)
	rd := make([]int, nc)
	qs := make([]int, nc)
	sgn := make([]int, nc)
	lineRb0 := make([]int, nc)
	d.runIndex = 0

	for y := 0; y < H; y++ {
		for c := 0; c < nc; c++ {
			lineRb0[c] = 0
			if y > 0 {
				lineRb0[c] = prev[c][0]
			}
		}
		x := 0
		for x < W {
			allZero := true
			for c := 0; c < nc; c++ {
				var a, b, cc, dd int
				if y == 0 {
					b, cc, dd = 0, 0, 0
				} else {
					b = prev[c][x]
					if x+1 < W {
						dd = prev[c][x+1]
					} else {
						dd = b
					}
				}
				if x == 0 {
					a = lineRb0[c]
					cc = rcLeft[c]
				} else {
					a = cur[c][x-1]
					if y > 0 {
						cc = prev[c][x-1]
					}
				}
				ra[c], rb[c], rc[c], rd[c] = a, b, cc, dd
				q := 81*d.quantize(dd-b) + 9*d.quantize(b-cc) + d.quantize(cc-a)
				s := 1
				if q < 0 {
					q = -q
					s = -1
				}
				qs[c], sgn[c] = q, s
				if q != 0 {
					allZero = false
				}
			}
			if allZero {
				x = d.decodeSampleRun(cur, prev, ra, y, x, nc)
				continue
			}
			for c := 0; c < nc; c++ {
				px := predict(ra[c], rb[c], rc[c])
				if sgn[c] > 0 {
					px += d.ctx[qs[c]].c
				} else {
					px -= d.ctx[qs[c]].c
				}
				if px < 0 {
					px = 0
				} else if px > d.f.maxval {
					px = d.f.maxval
				}
				cur[c][x] = d.decodeRegular(qs[c], sgn[c], px)
			}
			x++
		}
		base := y * W * nc
		for x := 0; x < W; x++ {
			for c := 0; c < nc; c++ {
				putSample(out, base+x*nc+c, cur[c][x], bps)
			}
		}
		for c := 0; c < nc; c++ {
			rcLeft[c] = lineRb0[c]
			prev[c], cur[c] = cur[c], prev[c]
		}
	}
}

// decodeSampleRun handles run mode for a sample-interleaved scan starting at
// pixel x. A run reconstructs whole pixels equal to the run pixel ra; the
// interrupting pixel uses run context 0 for every component with sign(rb-ra)
// (CharLS decode_run_pixels + decode_run_interruption_pixel for triplet/quad).
func (d *jlsDecoder) decodeSampleRun(cur, prev [][]int, ra []int, y, x, nc int) int {
	W := d.f.width
	remaining := W - x
	index := 0
	for d.br.readBit() == 1 {
		block := 1 << jlsRunJ[d.runIndex]
		count := block
		if count > remaining-index {
			count = remaining - index
		}
		index += count
		if count == block && d.runIndex < 31 {
			d.runIndex++
		}
		if index == remaining {
			break
		}
	}
	interrupted := index != remaining
	if interrupted && jlsRunJ[d.runIndex] > 0 {
		index += d.br.readBits(jlsRunJ[d.runIndex])
	}
	if index > remaining {
		index = remaining
		interrupted = false
	}
	for i := 0; i < index; i++ {
		for c := 0; c < nc; c++ {
			cur[c][x+i] = ra[c]
		}
	}
	x += index
	if interrupted && x < W {
		for c := 0; c < nc; c++ {
			rbc := 0
			if y > 0 {
				rbc = prev[c][x]
			}
			errval := d.decodeRIError(365, 0)
			if rbc < ra[c] {
				errval = -errval
			}
			cur[c][x] = d.computeRecon(rbc, errval)
		}
		x++
		if d.runIndex > 0 {
			d.runIndex--
		}
	}
	return x
}

// decodePlaneInto decodes one component plane (ILV=0) directly into the
// pixel-interleaved output as component c of nc, one reconstructed line at a
// time. Only the two O(W) line buffers are held, so peak memory is independent
// of image height (no W*H int plane).
func (d *jlsDecoder) decodePlaneInto(output []byte, c, nc, bps int) {
	W, H := d.f.width, d.f.height
	d.runIndex = 0
	prev := make([]int, W) // reconstructed line above (zero for first line)
	cur := make([]int, W)
	rcLeft := 0 // R[y-1][-1] for the current line

	for y := 0; y < H; y++ {
		rcLeft = d.decodeScalarLine(cur, prev, y, rcLeft)
		row := y * W
		for x := 0; x < W; x++ {
			putSample(output, (row+x)*nc+c, cur[x], bps)
		}
		prev, cur = cur, prev
	}
}

// decodeRun handles run mode starting at column x, returning the next column
// (CharLS DecodeRunPixels + DecodeRIPixel).
func (d *jlsDecoder) decodeRun(cur, prev []int, y, ra, x, W int) int {
	runVal := ra
	remaining := W - x
	index := 0
	for d.br.readBit() == 1 {
		block := 1 << jlsRunJ[d.runIndex]
		count := block
		if count > remaining-index {
			count = remaining - index
		}
		index += count
		if count == block && d.runIndex < 31 {
			d.runIndex++
		}
		if index == remaining {
			break
		}
	}
	interrupted := index != remaining
	if interrupted && jlsRunJ[d.runIndex] > 0 {
		index += d.br.readBits(jlsRunJ[d.runIndex])
	}
	if index > remaining { // guard malformed/misaligned streams
		index = remaining
		interrupted = false
	}
	for i := 0; i < index; i++ {
		cur[x+i] = runVal
	}
	x += index
	if interrupted && x < W {
		rb := 0
		if y > 0 {
			rb = prev[x]
		}
		cur[x] = d.decodeRunInterruption(runVal, rb)
		x++
		if d.runIndex > 0 {
			d.runIndex--
		}
	}
	return x
}

// decodeRunInterruption decodes the sample that ended a run (CharLS
// DecodeRIPixel + DecodeRIError; run contexts 365 = nRItype 0, 366 = nRItype 1).
func (d *jlsDecoder) decodeRunInterruption(ra, rb int) int {
	if abs(ra-rb) <= d.f.near {
		errval := d.decodeRIError(366, 1)
		return d.computeRecon(ra, errval)
	}
	errval := d.decodeRIError(365, 0)
	if rb < ra {
		errval = -errval
	}
	return d.computeRecon(rb, errval)
}

// decodeRIError decodes a run-interruption error for context q with nRItype.
func (d *jlsDecoder) decodeRIError(q, nRItype int) int {
	cs := &d.ctx[q]
	// k: TEMP = A + (N>>1)*nRItype; smallest k with (N<<k) >= TEMP.
	temp := cs.a + (cs.n>>1)*nRItype
	k := 0
	for (cs.n << k) < temp {
		k++
	}
	limit := d.limit - jlsRunJ[d.runIndex] - 1
	emerr := d.decodeValue(k, limit)

	// ComputeErrVal(temp = EMErrval + nRItype, k).
	t := emerr + nRItype
	mapFlag := t & 1
	errAbs := (t + mapFlag) / 2
	condition := 0
	if k != 0 || 2*cs.nn >= cs.n {
		condition = 1
	}
	var errval int
	if condition == mapFlag {
		errval = -errAbs
	} else {
		errval = errAbs
	}

	// UpdateVariables(errval, EMErrval).
	if errval < 0 {
		cs.nn++
	}
	cs.a += (emerr + 1 - nRItype) >> 1
	if cs.n == d.f.reset {
		cs.a >>= 1
		cs.n >>= 1
		cs.nn >>= 1
	}
	cs.n++
	return errval
}

// Backend wiring -----------------------------------------------------------

type gojpeglsBackend struct{}

func (gojpeglsBackend) Name() string { return "gojpegls" }

func (gojpeglsBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (gojpeglsBackend) Decode(encoded []byte, output []byte) error {
	return decodeJLSInto(encoded, output)
}

func (gojpeglsBackend) DecodeContext(_ context.Context, encoded []byte, output []byte) error {
	return decodeJLSInto(encoded, output)
}

func (gojpeglsBackend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, nearLossless bool) ([]byte, error) {
	// NEAR=1 matches the charls backend's near-lossless setting; NEAR=0 is
	// lossless. Multi-component input is encoded line-interleaved (ILV=1).
	// encodeJLS errors for unsupported geometry rather than producing an invalid
	// stream that reports success.
	near := 0
	if nearLossless {
		near = 1
	}
	return encodeJLS(raw, int(width), int(height), int(samples), int(bitsa), near)
}

func registerPureGoBackend() {
	_ = mgr.RegisterWithPriority("gojpegls", func() Backend { return gojpeglsBackend{} }, 0)
}
