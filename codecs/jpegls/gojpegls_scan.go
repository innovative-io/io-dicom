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
	if nf != 1 && nf != 3 {
		return errJLSUnsupported
	}
	if f.width*f.height*nf > maxJLSSamples {
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
	if f.ilv != 0 {
		return errJLSUnsupported // interleaved scans not yet supported
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
	need := f.width * f.height * nc * bps
	if need > len(output) {
		return errJLSOutputSize
	}

	d := newJLSDecoder(&f, encoded[scanOff:])
	// Non-interleaved (ILV=0): each component is a full plane, decoded in turn.
	planes := make([][]int, nc)
	for c := 0; c < nc; c++ {
		planes[c] = make([]int, f.width*f.height)
		d.decodePlane(planes[c])
	}

	// Pack component-minor interleaved, little-endian.
	for y := 0; y < f.height; y++ {
		for x := 0; x < f.width; x++ {
			for c := 0; c < nc; c++ {
				v := planes[c][y*f.width+x]
				idx := (y*f.width+x)*nc + c
				if bps == 1 {
					output[idx] = byte(v)
				} else {
					output[idx*2] = byte(v)
					output[idx*2+1] = byte(v >> 8)
				}
			}
		}
	}
	return nil
}

// decodePlane decodes one component plane (ILV=0) into plane (row-major).
func (d *jlsDecoder) decodePlane(plane []int) {
	W, H := d.f.width, d.f.height
	d.runIndex = 0
	prev := make([]int, W) // reconstructed line above (zero for first line)
	cur := make([]int, W)
	rcLeft := 0 // R[y-1][-1] for the current line

	for y := 0; y < H; y++ {
		lineRb0 := 0
		if y > 0 {
			lineRb0 = prev[0]
		}
		x := 0
		for x < W {
			// neighbors
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
				a = lineRb0 // Ra at line start = sample above (b)
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
				px += d.c[q]
			} else {
				px -= d.c[q]
			}
			if px < 0 {
				px = 0
			} else if px > d.f.maxval {
				px = d.f.maxval
			}
			cur[x] = d.decodeRegular(q, sign, px)
			x++
		}
		copy(plane[y*W:y*W+W], cur)
		rcLeft = lineRb0
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
	// k: TEMP = A + (N>>1)*nRItype; smallest k with (N<<k) >= TEMP.
	temp := d.a[q] + (d.n[q]>>1)*nRItype
	k := 0
	for (d.n[q] << k) < temp {
		k++
	}
	limit := d.limit - jlsRunJ[d.runIndex] - 1
	emerr := d.decodeValue(k, limit)

	// ComputeErrVal(temp = EMErrval + nRItype, k).
	t := emerr + nRItype
	mapFlag := t & 1
	errAbs := (t + mapFlag) / 2
	condition := 0
	if k != 0 || 2*d.nn[q] >= d.n[q] {
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
		d.nn[q]++
	}
	d.a[q] += (emerr + 1 - nRItype) >> 1
	if d.n[q] == d.f.reset {
		d.a[q] >>= 1
		d.n[q] >>= 1
		d.nn[q] >>= 1
	}
	d.n[q]++
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
	// lossless. encodeJLS errors for unsupported geometry (e.g. multi-component),
	// which is correct — better than producing an invalid stream that reports success.
	near := 0
	if nearLossless {
		near = 1
	}
	return encodeJLS(raw, int(width), int(height), int(samples), int(bitsa), near)
}

func registerPureGoBackend() {
	_ = mgr.RegisterWithPriority("gojpegls", func() Backend { return gojpeglsBackend{} }, 0)
}
