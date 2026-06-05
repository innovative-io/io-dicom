package jpeg2000

// HTJ2K (ISO 15444-15, T.814) HT Cleanup-pass block ENCODER.
//
// This is a faithful pure-Go port of openjph's ojph_encode_codeblock32 and its
// three bit machines (MEL, VLC, MagSgn). It is the inverse of decodeHTBlock in
// gojp2k_ht.go: encoding a sign-magnitude code-block and feeding the result back
// to decodeHTBlock reproduces the input exactly (round-trip validated in tests),
// and the emitted stream is byte-compatible with openjph's encoder.
//
// Layout of the produced cleanup segment (matching decodeHTBlock's expectations):
//
//	[ MagSgn bytes (forward) ][ MEL bytes (forward) ][ VLC bytes (forward) ]
//
// with the Scup interface-locator word written into the last 1.5 bytes:
// Scup = lenMEL + lenVLC.
//
// Only single-pass (Cleanup-only, num_passes == 1) is emitted, matching openjph.

import "math/bits"

// --------------------------------------------------------------------------
// MEL encoder (forward adaptive run-length, MSB-first byte packing with
// 0xFF -> 7-bit bit-stuffing).
// --------------------------------------------------------------------------

type htMelEnc struct {
	buf           []byte
	pos           int
	remainingBits int
	tmp           int
	run           int
	k             int
	threshold     int
}

var htMelEncExp = [13]int{0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 4, 5}

func newHTMelEnc(buf []byte) *htMelEnc {
	return &htMelEnc{buf: buf, remainingBits: 8, threshold: 1}
}

func (m *htMelEnc) emitBit(v int) {
	m.tmp = (m.tmp << 1) + v
	m.remainingBits--
	if m.remainingBits == 0 {
		m.buf[m.pos] = byte(m.tmp)
		m.pos++
		if m.tmp == 0xFF {
			m.remainingBits = 7
		} else {
			m.remainingBits = 8
		}
		m.tmp = 0
	}
}

func (m *htMelEnc) encode(bit bool) {
	if !bit {
		m.run++
		if m.run >= m.threshold {
			m.emitBit(1)
			m.run = 0
			m.k = min(12, m.k+1)
			m.threshold = 1 << uint(htMelEncExp[m.k])
		}
	} else {
		m.emitBit(0)
		t := htMelEncExp[m.k]
		for t > 0 {
			t--
			m.emitBit((m.run >> uint(t)) & 1)
		}
		m.run = 0
		m.k = max(0, m.k-1)
		m.threshold = 1 << uint(htMelEncExp[m.k])
	}
}

// --------------------------------------------------------------------------
// VLC encoder (BACKWARD writer, growing down from the end of its buffer,
// with 0x8F/0x7F bit-stuffing).
// --------------------------------------------------------------------------

type htVlcEnc struct {
	vbuf        []byte
	end         int // index that models openjph's buf[0] (last byte of region)
	pos         int
	usedBits    int
	tmp         int
	lastGreater bool // last emitted byte was > 0x8F
}

func newHTVlcEnc(buf []byte) *htVlcEnc {
	v := &htVlcEnc{vbuf: buf, end: len(buf) - 1, pos: 1, usedBits: 4, tmp: 0xF, lastGreater: true}
	v.vbuf[v.end] = 0xFF
	return v
}

func (v *htVlcEnc) encode(cwd, cwdLen int) {
	for cwdLen > 0 {
		avail := 8 - b2i(v.lastGreater) - v.usedBits
		t := min(avail, cwdLen)
		v.tmp |= (cwd & ((1 << uint(t)) - 1)) << uint(v.usedBits)
		v.usedBits += t
		avail -= t
		cwdLen -= t
		cwd >>= uint(t)
		if avail == 0 {
			if v.lastGreater && v.tmp != 0x7F {
				v.lastGreater = false
				continue // one empty bit remaining
			}
			v.vbuf[v.end-v.pos] = byte(v.tmp)
			v.pos++
			v.lastGreater = v.tmp > 0x8F
			v.tmp = 0
			v.usedBits = 0
		}
	}
}

// terminateMelVlc flushes and fuses the trailing MEL and VLC bytes, exactly as
// openjph's terminate_mel_vlc.
func terminateMelVlc(m *htMelEnc, v *htVlcEnc) {
	if m.run > 0 {
		m.emitBit(1)
	}
	m.tmp = m.tmp << uint(m.remainingBits)
	melMask := (0xFF << uint(m.remainingBits)) & 0xFF
	vlcMask := 0xFF >> uint(8-v.usedBits)
	if (melMask | vlcMask) == 0 {
		return
	}
	fuse := m.tmp | v.tmp
	if ((fuse^m.tmp)&melMask|(fuse^v.tmp)&vlcMask) == 0 && fuse != 0xFF && v.pos > 1 {
		m.buf[m.pos] = byte(fuse)
		m.pos++
	} else {
		m.buf[m.pos] = byte(m.tmp) // m.tmp cannot be 0xFF here
		m.pos++
		v.vbuf[v.end-v.pos] = byte(v.tmp)
		v.pos++
	}
}

// --------------------------------------------------------------------------
// MagSgn encoder (forward magnitude+sign packing with 0xFF -> 7-bit stuffing).
// --------------------------------------------------------------------------

type htMsEnc struct {
	buf      []byte
	pos      int
	maxBits  int
	usedBits int
	tmp      uint32
}

func newHTMsEnc(buf []byte) *htMsEnc {
	return &htMsEnc{buf: buf, maxBits: 8}
}

func (m *htMsEnc) encode(cwd uint32, cwdLen int) {
	for cwdLen > 0 {
		t := min(m.maxBits-m.usedBits, cwdLen)
		m.tmp |= (cwd & ((1 << uint(t)) - 1)) << uint(m.usedBits)
		m.usedBits += t
		cwd >>= uint(t)
		cwdLen -= t
		if m.usedBits >= m.maxBits {
			m.buf[m.pos] = byte(m.tmp)
			m.pos++
			if m.tmp == 0xFF {
				m.maxBits = 7
			} else {
				m.maxBits = 8
			}
			m.tmp = 0
			m.usedBits = 0
		}
	}
}

func (m *htMsEnc) terminate() {
	if m.usedBits != 0 {
		t := m.maxBits - m.usedBits // unused bits
		m.tmp |= (0xFF & ((1 << uint(t)) - 1)) << uint(m.usedBits)
		m.usedBits += t
		if m.tmp != 0xFF {
			m.buf[m.pos] = byte(m.tmp)
			m.pos++
		}
	} else if m.maxBits == 7 {
		m.pos--
	}
}

// --------------------------------------------------------------------------
// Cleanup-pass code-block encoder (inverse of decodeHTBlock).
// --------------------------------------------------------------------------

// encodeHTCleanup encodes one code-block's coefficients (sign-magnitude uint32
// words: bit31 = sign, bits0..30 = magnitude) into a single HT Cleanup segment.
// buf is laid out row-major with the given stride; width/height are the real
// code-block dimensions. Returns the cleanup segment bytes.
func encodeHTCleanup(buf []uint32, missingMSBs, width, height, stride int) []byte {
	msBuf := make([]byte, width*height*5+64)
	melBuf := make([]byte, width*height+512)
	vlcBuf := make([]byte, width*height*2+512)
	mel := newHTMelEnc(melBuf)
	vlc := newHTVlcEnc(vlcBuf)
	ms := newHTMsEnc(msBuf)

	p := uint(30 - missingMSBs)

	// e_val: per-column-pair running E (max of two samples); cx_val: significance
	// context bits. Two sentinels (start + end) like openjph's 513-byte arrays.
	eVal := make([]int, width/2+8)
	cxVal := make([]int, width/2+8)

	// readSample reconstructs (significant, e_q, v_n) from a sign-magnitude word,
	// mirroring the openjph block: val = 2t; val >>= p; val &= ~1; if val { ... }.
	readSample := func(t uint32) (bool, int, uint32) {
		val := t + t
		val >>= p
		val &^= 1
		if val == 0 {
			return false, 0, 0
		}
		val-- // 2mu - 1
		eqv := 32 - bits.LeadingZeros32(val)
		val-- // 2(mu-1)
		return true, eqv, val + (t >> 31)
	}

	var eqmax [2]int
	var eq [8]int
	var rho [2]int
	var s [8]uint32

	// ---------------- initial row of quads (y == 0) ----------------
	li, ci := 0, 0
	eVal[0] = 0
	cxVal[0] = 0
	cq0 := 0
	sp := 0
	for x := 0; x < width; x += 4 {
		// quad 0 (columns x, x+1)
		if sig, e, sv := readSample(buf[sp]); sig {
			rho[0] |= 1
			eq[0] = e
			eqmax[0] = e
			s[0] = sv
		}
		var t uint32
		if height > 1 {
			t = buf[sp+stride]
		}
		sp++
		if sig, e, sv := readSample(t); sig {
			rho[0] |= 2
			eq[1] = e
			eqmax[0] = max(eqmax[0], e)
			s[1] = sv
		}
		if x+1 < width {
			if sig, e, sv := readSample(buf[sp]); sig {
				rho[0] |= 4
				eq[2] = e
				eqmax[0] = max(eqmax[0], e)
				s[2] = sv
			}
			t = 0
			if height > 1 {
				t = buf[sp+stride]
			}
			sp++
			if sig, e, sv := readSample(t); sig {
				rho[0] |= 8
				eq[3] = e
				eqmax[0] = max(eqmax[0], e)
				s[3] = sv
			}
		}

		uq0v := max(eqmax[0], 1) // kappa_q = 1
		uQ0 := uq0v - 1
		uQ1 := 0

		eps0 := 0
		if uQ0 > 0 {
			eps0 |= b2i(eq[0] == eqmax[0])
			eps0 |= b2i(eq[1] == eqmax[0]) << 1
			eps0 |= b2i(eq[2] == eqmax[0]) << 2
			eps0 |= b2i(eq[3] == eqmax[0]) << 3
		}
		eVal[li] = max(eVal[li], eq[1])
		li++
		eVal[li] = eq[3]
		cxVal[ci] = cxVal[ci] | ((rho[0] & 2) >> 1)
		ci++
		cxVal[ci] = (rho[0] & 8) >> 3

		tuple0 := int(htEncVLCTbl0[(cq0<<8)+(rho[0]<<4)+eps0])
		vlc.encode(tuple0>>8, (tuple0>>4)&7)
		if cq0 == 0 {
			mel.encode(rho[0] != 0)
		}
		emitMagSgn(ms, rho[0], uq0v, tuple0, s[0], s[1], s[2], s[3])

		if x+2 < width {
			// quad 1 (columns x+2, x+3)
			if sig, e, sv := readSample(buf[sp]); sig {
				rho[1] |= 1
				eq[4] = e
				eqmax[1] = e
				s[4] = sv
			}
			t = 0
			if height > 1 {
				t = buf[sp+stride]
			}
			sp++
			if sig, e, sv := readSample(t); sig {
				rho[1] |= 2
				eq[5] = e
				eqmax[1] = max(eqmax[1], e)
				s[5] = sv
			}
			if x+3 < width {
				if sig, e, sv := readSample(buf[sp]); sig {
					rho[1] |= 4
					eq[6] = e
					eqmax[1] = max(eqmax[1], e)
					s[6] = sv
				}
				t = 0
				if height > 1 {
					t = buf[sp+stride]
				}
				sp++
				if sig, e, sv := readSample(t); sig {
					rho[1] |= 8
					eq[7] = e
					eqmax[1] = max(eqmax[1], e)
					s[7] = sv
				}
			}

			cq1 := (rho[0] >> 1) | (rho[0] & 1)
			uq1v := max(eqmax[1], 1)
			uQ1 = uq1v - 1

			eps1 := 0
			if uQ1 > 0 {
				eps1 |= b2i(eq[4] == eqmax[1])
				eps1 |= b2i(eq[5] == eqmax[1]) << 1
				eps1 |= b2i(eq[6] == eqmax[1]) << 2
				eps1 |= b2i(eq[7] == eqmax[1]) << 3
			}
			eVal[li] = max(eVal[li], eq[5])
			li++
			eVal[li] = eq[7]
			cxVal[ci] = cxVal[ci] | ((rho[1] & 2) >> 1)
			ci++
			cxVal[ci] = (rho[1] & 8) >> 3

			tuple1 := int(htEncVLCTbl0[(cq1<<8)+(rho[1]<<4)+eps1])
			vlc.encode(tuple1>>8, (tuple1>>4)&7)
			if cq1 == 0 {
				mel.encode(rho[1] != 0)
			}
			emitMagSgn(ms, rho[1], uq1v, tuple1, s[4], s[5], s[6], s[7])
		}

		// u-value coding (initial row: MEL-assisted, with u>2 shortcuts).
		if uQ0 > 0 && uQ1 > 0 {
			mel.encode(min(uQ0, uQ1) > 2)
		}
		if uQ0 > 2 && uQ1 > 2 {
			a, b := htUVLCEncTbl[uQ0-2], htUVLCEncTbl[uQ1-2]
			vlc.encode(int(a.pre), int(a.preLen))
			vlc.encode(int(b.pre), int(b.preLen))
			vlc.encode(int(a.suf), int(a.sufLen))
			vlc.encode(int(b.suf), int(b.sufLen))
		} else if uQ0 > 2 && uQ1 > 0 {
			a := htUVLCEncTbl[uQ0]
			vlc.encode(int(a.pre), int(a.preLen))
			vlc.encode(uQ1-1, 1)
			vlc.encode(int(a.suf), int(a.sufLen))
		} else {
			a, b := htUVLCEncTbl[uQ0], htUVLCEncTbl[uQ1]
			vlc.encode(int(a.pre), int(a.preLen))
			vlc.encode(int(b.pre), int(b.preLen))
			vlc.encode(int(a.suf), int(a.sufLen))
			vlc.encode(int(b.suf), int(b.sufLen))
		}

		// prepare for next iteration
		cq0 = (rho[1] >> 1) | (rho[1] & 1)
		s = [8]uint32{}
		eq = [8]int{}
		rho[0], rho[1] = 0, 0
		eqmax[0], eqmax[1] = 0, 0
	}
	eVal[li+1] = 0

	// ---------------- non-initial rows of quads ----------------
	for y := 2; y < height; y += 2 {
		li = 0
		maxE := max(eVal[0], eVal[1]) - 1
		eVal[0] = 0
		ci = 0
		cq0 = cxVal[0] + (cxVal[1] << 2)
		cxVal[0] = 0

		sp = y * stride
		for x := 0; x < width; x += 4 {
			// quad 0
			if sig, e, sv := readSample(buf[sp]); sig {
				rho[0] |= 1
				eq[0] = e
				eqmax[0] = e
				s[0] = sv
			}
			var t uint32
			if y+1 < height {
				t = buf[sp+stride]
			}
			sp++
			if sig, e, sv := readSample(t); sig {
				rho[0] |= 2
				eq[1] = e
				eqmax[0] = max(eqmax[0], e)
				s[1] = sv
			}
			if x+1 < width {
				if sig, e, sv := readSample(buf[sp]); sig {
					rho[0] |= 4
					eq[2] = e
					eqmax[0] = max(eqmax[0], e)
					s[2] = sv
				}
				t = 0
				if y+1 < height {
					t = buf[sp+stride]
				}
				sp++
				if sig, e, sv := readSample(t); sig {
					rho[0] |= 8
					eq[3] = e
					eqmax[0] = max(eqmax[0], e)
					s[3] = sv
				}
			}

			kappa := 1
			if rho[0]&(rho[0]-1) != 0 {
				kappa = max(1, maxE)
			}
			uq0v := max(eqmax[0], kappa)
			uQ0 := uq0v - kappa
			uQ1 := 0

			eps0 := 0
			if uQ0 > 0 {
				eps0 |= b2i(eq[0] == eqmax[0])
				eps0 |= b2i(eq[1] == eqmax[0]) << 1
				eps0 |= b2i(eq[2] == eqmax[0]) << 2
				eps0 |= b2i(eq[3] == eqmax[0]) << 3
			}
			eVal[li] = max(eVal[li], eq[1])
			li++
			maxE = max(eVal[li], eVal[li+1]) - 1
			eVal[li] = eq[3]
			cxVal[ci] = cxVal[ci] | ((rho[0] & 2) >> 1)
			ci++
			cq1 := cxVal[ci] + (cxVal[ci+1] << 2)
			cxVal[ci] = (rho[0] & 8) >> 3

			tuple0 := int(htEncVLCTbl1[(cq0<<8)+(rho[0]<<4)+eps0])
			vlc.encode(tuple0>>8, (tuple0>>4)&7)
			if cq0 == 0 {
				mel.encode(rho[0] != 0)
			}
			emitMagSgn(ms, rho[0], uq0v, tuple0, s[0], s[1], s[2], s[3])

			if x+2 < width {
				// quad 1
				if sig, e, sv := readSample(buf[sp]); sig {
					rho[1] |= 1
					eq[4] = e
					eqmax[1] = e
					s[4] = sv
				}
				t = 0
				if y+1 < height {
					t = buf[sp+stride]
				}
				sp++
				if sig, e, sv := readSample(t); sig {
					rho[1] |= 2
					eq[5] = e
					eqmax[1] = max(eqmax[1], e)
					s[5] = sv
				}
				if x+3 < width {
					if sig, e, sv := readSample(buf[sp]); sig {
						rho[1] |= 4
						eq[6] = e
						eqmax[1] = max(eqmax[1], e)
						s[6] = sv
					}
					t = 0
					if y+1 < height {
						t = buf[sp+stride]
					}
					sp++
					if sig, e, sv := readSample(t); sig {
						rho[1] |= 8
						eq[7] = e
						eqmax[1] = max(eqmax[1], e)
						s[7] = sv
					}
				}

				kappa = 1
				if rho[1]&(rho[1]-1) != 0 {
					kappa = max(1, maxE)
				}
				cq1 |= ((rho[0] & 4) >> 1) | ((rho[0] & 8) >> 2)
				uq1v := max(eqmax[1], kappa)
				uQ1 = uq1v - kappa

				eps1 := 0
				if uQ1 > 0 {
					eps1 |= b2i(eq[4] == eqmax[1])
					eps1 |= b2i(eq[5] == eqmax[1]) << 1
					eps1 |= b2i(eq[6] == eqmax[1]) << 2
					eps1 |= b2i(eq[7] == eqmax[1]) << 3
				}
				eVal[li] = max(eVal[li], eq[5])
				li++
				maxE = max(eVal[li], eVal[li+1]) - 1
				eVal[li] = eq[7]
				cxVal[ci] = cxVal[ci] | ((rho[1] & 2) >> 1)
				ci++
				cq0 = cxVal[ci] + (cxVal[ci+1] << 2)
				cxVal[ci] = (rho[1] & 8) >> 3

				tuple1 := int(htEncVLCTbl1[(cq1<<8)+(rho[1]<<4)+eps1])
				vlc.encode(tuple1>>8, (tuple1>>4)&7)
				if cq1 == 0 {
					mel.encode(rho[1] != 0)
				}
				emitMagSgn(ms, rho[1], uq1v, tuple1, s[4], s[5], s[6], s[7])
			}

			// u-value coding (non-initial rows: plain uvlc).
			a, b := htUVLCEncTbl[uQ0], htUVLCEncTbl[uQ1]
			vlc.encode(int(a.pre), int(a.preLen))
			vlc.encode(int(b.pre), int(b.preLen))
			vlc.encode(int(a.suf), int(a.sufLen))
			vlc.encode(int(b.suf), int(b.sufLen))

			// prepare for next iteration
			cq0 |= ((rho[1] & 4) >> 1) | ((rho[1] & 8) >> 2)
			s = [8]uint32{}
			eq = [8]int{}
			rho[0], rho[1] = 0, 0
			eqmax[0], eqmax[1] = 0, 0
		}
	}

	terminateMelVlc(mel, vlc)
	ms.terminate()

	length := mel.pos + vlc.pos + ms.pos
	out := make([]byte, length)
	copy(out[0:], ms.buf[:ms.pos])
	copy(out[ms.pos:], mel.buf[:mel.pos])
	copy(out[ms.pos+mel.pos:], vlc.vbuf[vlc.end-vlc.pos+1:vlc.end+1])

	// interface-locator word (Scup = lenMEL + lenVLC)
	numBytes := mel.pos + vlc.pos
	out[length-1] = byte(numBytes >> 4)
	out[length-2] = (out[length-2] & 0xF0) | byte(numBytes&0xF)
	return out
}

// emitMagSgn writes the four MagSgn codewords of one quad given its rho, Uq,
// and the VLC tuple holding the EMB bits in its low nibble.
func emitMagSgn(ms *htMsEnc, rho, uq, tuple int, s0, s1, s2, s3 uint32) {
	if rho&1 != 0 {
		m := uq - (tuple & 1)
		ms.encode(s0&((1<<uint(m))-1), m)
	}
	if rho&2 != 0 {
		m := uq - ((tuple & 2) >> 1)
		ms.encode(s1&((1<<uint(m))-1), m)
	}
	if rho&4 != 0 {
		m := uq - ((tuple & 4) >> 2)
		ms.encode(s2&((1<<uint(m))-1), m)
	}
	if rho&8 != 0 {
		m := uq - ((tuple & 8) >> 3)
		ms.encode(s3&((1<<uint(m))-1), m)
	}
}
