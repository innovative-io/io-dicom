package jpeg2000

// Pure-Go HTJ2K (ISO/IEC 15444-15) HT block decoder, ported from openjph's
// scalar reference (ojph_block_decoder32.cpp + ojph_block_common.cpp). It decodes
// the mandatory HT Cleanup pass and, for multi-pass code-blocks, the SigProp and
// MagRef refinement passes, into sign-magnitude "decoded_data" words.
//
// The three sub-bitstreams of the cleanup pass are read by three machines:
//   - MEL   : forward adaptive run-length for quad significance.
//   - VLC   : backward variable-length codes for quad significance/EMB and u.
//   - MagSgn: forward magnitude+sign bits.
// openjph reads these with architecture-aligned 32-bit loads; the bit content is
// independent of buffer alignment, so this port streams byte-by-byte instead,
// which is equivalent and simpler.

import "math/bits"

// ---------------------------------------------------------------------------
// MEL: forward run decoder. Bits are accumulated MSB-first in tmp.
// ---------------------------------------------------------------------------

type htMEL struct {
	data    []byte
	pos     int // index of next byte to read
	size    int // bytes remaining in MEL+VLC segment for MEL reads
	tmp     uint64
	bits    int
	unstuff bool
	k       int
	numRuns int
	runs    uint64
}

var htMelExp = [13]int{0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 4, 5}

func (m *htMEL) read() {
	if m.bits > 32 {
		return
	}
	var val uint32 = 0xFFFFFFFF
	if m.size > 4 {
		val = uint32(m.data[m.pos]) | uint32(m.data[m.pos+1])<<8 |
			uint32(m.data[m.pos+2])<<16 | uint32(m.data[m.pos+3])<<24
		m.pos += 4
		m.size -= 4
	} else if m.size > 0 {
		i := 0
		for m.size > 1 {
			v := uint32(m.data[m.pos])
			m.pos++
			mask := ^(uint32(0xFF) << uint(i))
			val = (val & mask) | (v << uint(i))
			m.size--
			i += 8
		}
		v := uint32(m.data[m.pos])
		m.pos++
		v |= 0xF // MEL and VLC segments can overlap
		mask := ^(uint32(0xFF) << uint(i))
		val = (val & mask) | (v << uint(i))
		m.size--
	}

	nbits := 32 - b2i(m.unstuff)
	t := val & 0xFF
	unstuff := (val & 0xFF) == 0xFF
	nbits -= b2i(unstuff)
	t = t << uint(8-b2i(unstuff))

	t |= (val >> 8) & 0xFF
	unstuff = ((val >> 8) & 0xFF) == 0xFF
	nbits -= b2i(unstuff)
	t = t << uint(8-b2i(unstuff))

	t |= (val >> 16) & 0xFF
	unstuff = ((val >> 16) & 0xFF) == 0xFF
	nbits -= b2i(unstuff)
	t = t << uint(8-b2i(unstuff))

	t |= (val >> 24) & 0xFF
	m.unstuff = ((val >> 24) & 0xFF) == 0xFF

	m.tmp |= uint64(t) << uint(64-nbits-m.bits)
	m.bits += nbits
}

func (m *htMEL) decode() {
	if m.bits < 6 {
		m.read()
	}
	for m.bits >= 6 && m.numRuns < 8 {
		eval := htMelExp[m.k]
		var run int
		if m.tmp&(1<<63) != 0 {
			run = (1 << uint(eval)) - 1
			if m.k+1 < 12 {
				m.k++
			} else {
				m.k = 12
			}
			m.tmp <<= 1
			m.bits--
			run <<= 1
		} else {
			run = int(m.tmp>>uint(63-eval)) & ((1 << uint(eval)) - 1)
			if m.k-1 > 0 {
				m.k--
			} else {
				m.k = 0
			}
			m.tmp <<= uint(eval + 1)
			m.bits -= eval + 1
			run = (run << 1) + 1
		}
		pos := m.numRuns * 7
		m.runs &= ^(uint64(0x3F) << uint(pos))
		m.runs |= uint64(run) << uint(pos)
		m.numRuns++
	}
}

func (m *htMEL) getRun() int {
	if m.numRuns == 0 {
		m.decode()
	}
	t := int(m.runs & 0x7F)
	m.runs >>= 7
	m.numRuns--
	return t
}

func newHTMEL(data []byte, lcup, scup int) *htMEL {
	m := &htMEL{data: data, pos: lcup - scup, size: scup - 1}
	num := 4 - (m.pos & 0x3)
	for i := 0; i < num; i++ {
		var d uint64 = 0xFF
		if m.size > 0 {
			d = uint64(m.data[m.pos])
		}
		if m.size == 1 {
			d |= 0xF
		}
		if m.size > 0 {
			m.pos++
		}
		m.size--
		dBits := 8 - b2i(m.unstuff)
		m.tmp = (m.tmp << uint(dBits)) | d
		m.bits += dBits
		m.unstuff = (d & 0xFF) == 0xFF
	}
	m.tmp <<= uint(64 - m.bits)
	return m
}

// ---------------------------------------------------------------------------
// VLC: backward reader. Bits accumulate LSB-first in tmp.
// ---------------------------------------------------------------------------

type htRev struct {
	data    []byte
	pos     int // index of next byte to read (moving backward)
	size    int
	tmp     uint64
	bits    int
	unstuff bool
}

func (r *htRev) read() {
	if r.bits > 32 {
		return
	}
	// Gather up to 4 bytes moving backward; first read = highest address.
	var val uint32
	if r.size > 3 {
		b0 := uint32(r.data[r.pos])
		b1 := uint32(r.data[r.pos-1])
		b2 := uint32(r.data[r.pos-2])
		b3 := uint32(r.data[r.pos-3])
		val = b3 | b2<<8 | b1<<16 | b0<<24
		r.pos -= 4
		r.size -= 4
	} else if r.size > 0 {
		i := 24
		for r.size > 0 {
			v := uint32(r.data[r.pos])
			r.pos--
			val |= v << uint(i)
			r.size--
			i -= 8
		}
	}

	tmp := val >> 24
	nbits := 8 - b2i(r.unstuff && ((val>>24)&0x7F) == 0x7F)
	unstuff := (val >> 24) > 0x8F

	tmp |= ((val >> 16) & 0xFF) << uint(nbits)
	nbits += 8 - b2i(unstuff && ((val>>16)&0x7F) == 0x7F)
	unstuff = ((val >> 16) & 0xFF) > 0x8F

	tmp |= ((val >> 8) & 0xFF) << uint(nbits)
	nbits += 8 - b2i(unstuff && ((val>>8)&0x7F) == 0x7F)
	unstuff = ((val >> 8) & 0xFF) > 0x8F

	tmp |= (val & 0xFF) << uint(nbits)
	nbits += 8 - b2i(unstuff && (val&0x7F) == 0x7F)
	unstuff = (val & 0xFF) > 0x8F

	r.tmp |= uint64(tmp) << uint(r.bits)
	r.bits += nbits
	r.unstuff = unstuff
}

func (r *htRev) fetch() uint32 {
	if r.bits < 32 {
		r.read()
		if r.bits < 32 {
			r.read()
		}
	}
	return uint32(r.tmp)
}

func (r *htRev) advance(n uint32) uint32 {
	r.tmp >>= n
	r.bits -= int(n)
	return uint32(r.tmp)
}

func newHTRev(data []byte, lcup, scup int) *htRev {
	r := &htRev{data: data, pos: lcup - 2, size: scup - 2}
	d := uint32(r.data[r.pos])
	r.pos--
	r.tmp = uint64(d >> 4)
	r.bits = 4 - b2i((r.tmp&7) == 7)
	r.unstuff = (d | 0xF) > 0x8F

	num := 1 + (r.pos & 0x3)
	tnum := num
	if r.size < tnum {
		tnum = r.size
	}
	for i := 0; i < tnum; i++ {
		d := uint64(r.data[r.pos])
		r.pos--
		dBits := 8 - b2i(r.unstuff && (d&0x7F) == 0x7F)
		r.tmp |= d << uint(r.bits)
		r.bits += dBits
		r.unstuff = d > 0x8F
	}
	r.size -= tnum
	r.read()
	return r
}

// ---------------------------------------------------------------------------
// MagSgn: forward reader, feeds 0xFF when exhausted.
// ---------------------------------------------------------------------------

type htFrwd struct {
	data    []byte
	pos     int
	size    int
	tmp     uint64
	bits    int
	unstuff bool
	fillFF  bool // feed 0xFF when exhausted (MagSgn); else feed 0 (SigProp)
}

func (f *htFrwd) read() {
	var fillWord uint32
	if f.fillFF {
		fillWord = 0xFFFFFFFF
	}
	var val uint32
	if f.size > 3 {
		val = uint32(f.data[f.pos]) | uint32(f.data[f.pos+1])<<8 |
			uint32(f.data[f.pos+2])<<16 | uint32(f.data[f.pos+3])<<24
		f.pos += 4
		f.size -= 4
	} else if f.size > 0 {
		i := 0
		val = fillWord
		for f.size > 0 {
			v := uint32(f.data[f.pos])
			f.pos++
			mask := ^(uint32(0xFF) << uint(i))
			val = (val & mask) | (v << uint(i))
			f.size--
			i += 8
		}
	} else {
		val = fillWord
	}

	nbits := 8 - b2i(f.unstuff)
	t := val & 0xFF
	unstuff := (val & 0xFF) == 0xFF

	t |= ((val >> 8) & 0xFF) << uint(nbits)
	nbits += 8 - b2i(unstuff)
	unstuff = ((val >> 8) & 0xFF) == 0xFF

	t |= ((val >> 16) & 0xFF) << uint(nbits)
	nbits += 8 - b2i(unstuff)
	unstuff = ((val >> 16) & 0xFF) == 0xFF

	t |= ((val >> 24) & 0xFF) << uint(nbits)
	nbits += 8 - b2i(unstuff)
	f.unstuff = ((val >> 24) & 0xFF) == 0xFF

	f.tmp |= uint64(t) << uint(f.bits)
	f.bits += nbits
}

func (f *htFrwd) fetch() uint32 {
	if f.bits < 32 {
		f.read()
		if f.bits < 32 {
			f.read()
		}
	}
	return uint32(f.tmp)
}

func (f *htFrwd) advance(n uint32) {
	f.tmp >>= n
	f.bits -= int(n)
}

func newHTFrwd(data []byte, start, size int, fillFF bool) *htFrwd {
	f := &htFrwd{data: data, pos: start, size: size, fillFF: fillFF}
	var fillByte uint64
	if fillFF {
		fillByte = 0xFF
	}
	num := 4 - (f.pos & 0x3)
	for i := 0; i < num; i++ {
		d := fillByte
		if f.size > 0 {
			d = uint64(f.data[f.pos])
			f.pos++
		}
		f.size--
		f.tmp |= d << uint(f.bits)
		f.bits += 8 - b2i(f.unstuff)
		f.unstuff = (d & 0xFF) == 0xFF
	}
	f.read()
	return f
}

// htRev (backward reader) is reused for MagRef (MRP). newHTRevMRP initialises it
// over the refinement segment [lcup, lcup+len2), growing backward, with the
// MRP-specific start (no 12-bit length prefix, initial unstuff=true, fill 0).
func newHTRevMRP(data []byte, lcup, len2 int) *htRev {
	r := &htRev{data: data, pos: lcup + len2 - 1, size: len2, unstuff: true}
	num := 1 + (r.pos & 0x3)
	for i := 0; i < num; i++ {
		var d uint64
		if r.size > 0 {
			d = uint64(r.data[r.pos])
		}
		if r.size > 0 {
			r.pos--
		}
		r.size--
		dBits := 8 - b2i(r.unstuff && (d&0x7F) == 0x7F)
		r.tmp |= d << uint(r.bits)
		r.bits += dBits
		r.unstuff = d > 0x8F
	}
	r.read()
	return r
}

// decodeHTBlock decodes an HT code-block: the mandatory Cleanup pass plus, when
// numPasses>1, the SigProp refinement pass, and when numPasses>2, the MagRef
// refinement pass. coded holds the cleanup segment (lengths1) followed by the
// refinement segment (lengths2). missingMSBs is the number of all-zero MSB
// bit-planes. Returns width*height packed sign-magnitude words (sign in bit 31),
// or ok=false on a malformed block.
func decodeHTBlock(coded []byte, lengths1, lengths2, numPasses, missingMSBs, width, height int, stripeCausal bool) ([]uint32, bool) {
	if missingMSBs > 30 || lengths1 < 2 {
		return nil, false
	}
	lcup := lengths1
	p := 30 - missingMSBs
	mmsbp2 := missingMSBs + 2

	// Pad so the byte-oriented readers can over-read up to a few bytes.
	total := lengths1 + lengths2
	if total > len(coded) {
		return nil, false
	}
	buf := make([]byte, total+16)
	copy(buf, coded[:total])

	scup := (int(buf[lcup-1]) << 4) + (int(buf[lcup-2]) & 0xF)
	if scup < 2 || scup > lcup || scup > 4079 {
		return nil, false
	}

	sstr := ((width + 2) + 7) &^ 7 // scratch stride in 16-bit entries
	scratch := make([]uint16, sstr*(height/2+2))

	// ----- step 1: decode VLC + MEL into scratch (rho/u_off/e_1/e_k + u_q) ----
	mel := newHTMEL(buf, lcup, scup)
	vlc := newHTRev(buf, lcup, scup)
	run := mel.getRun()
	cQ := uint32(0)

	// initial quad row
	{
		spBase := 0
		x := 0
		for x < width {
			vlcVal := vlc.fetch()
			t0 := uint32(htVLCTbl0[cQ+(vlcVal&0x7F)])
			if cQ == 0 {
				run -= 2
				if run != -1 {
					t0 = 0
				}
				if run < 0 {
					run = mel.getRun()
				}
			}
			scratch[spBase+0] = uint16(t0)
			x += 2
			cQ = ((t0 & 0x10) << 3) | ((t0 & 0xE0) << 2)
			vlcVal = vlc.advance(t0 & 0x7)

			t1 := uint32(htVLCTbl0[cQ+(vlcVal&0x7F)])
			if cQ == 0 && x < width {
				run -= 2
				if run != -1 {
					t1 = 0
				}
				if run < 0 {
					run = mel.getRun()
				}
			}
			if x >= width {
				t1 = 0
			}
			scratch[spBase+2] = uint16(t1)
			x += 2
			cQ = ((t1 & 0x10) << 3) | ((t1 & 0xE0) << 2)
			vlcVal = vlc.advance(t1 & 0x7)

			uvlcMode := ((t0 & 0x8) << 3) | ((t1 & 0x8) << 4)
			if uvlcMode == 0xc0 {
				run -= 2
				if run == -1 {
					uvlcMode += 0x40
				}
				if run < 0 {
					run = mel.getRun()
				}
			}
			uvlcEntry := uint32(htUVLCTbl0[uvlcMode+(vlcVal&0x3F)])
			vlcVal = vlc.advance(uvlcEntry & 0x7)
			uvlcEntry >>= 3
			ln := uvlcEntry & 0xF
			tmp := vlcVal & ((1 << ln) - 1)
			vlc.advance(ln) // consume quad-pair suffix bits; result unused
			uvlcEntry >>= 4
			ln = uvlcEntry & 0x7
			uvlcEntry >>= 3
			scratch[spBase+1] = uint16(1 + (uvlcEntry & 7) + (tmp &^ (0xFF << ln)))
			scratch[spBase+3] = uint16(1 + (uvlcEntry >> 3) + (tmp >> ln))
			spBase += 4
		}
		scratch[spBase+0] = 0
		scratch[spBase+1] = 0
	}

	// non-initial quad rows
	for y := 2; y < height; y += 2 {
		cQ = 0
		spBase := (y >> 1) * sstr
		x := 0
		for x < width {
			cQ |= (uint32(scratch[spBase+0-sstr]) & 0xA0) << 2
			cQ |= (uint32(scratch[spBase+2-sstr]) & 0x20) << 4

			vlcVal := vlc.fetch()
			t0 := uint32(htVLCTbl1[cQ+(vlcVal&0x7F)])
			if cQ == 0 {
				run -= 2
				if run != -1 {
					t0 = 0
				}
				if run < 0 {
					run = mel.getRun()
				}
			}
			scratch[spBase+0] = uint16(t0)
			x += 2
			cQ = ((t0 & 0x40) << 2) | ((t0 & 0x80) << 1)
			cQ |= uint32(scratch[spBase+0-sstr]) & 0x80
			cQ |= (uint32(scratch[spBase+2-sstr]) & 0xA0) << 2
			cQ |= (uint32(scratch[spBase+4-sstr]) & 0x20) << 4
			vlcVal = vlc.advance(t0 & 0x7)

			t1 := uint32(htVLCTbl1[cQ+(vlcVal&0x7F)])
			if cQ == 0 && x < width {
				run -= 2
				if run != -1 {
					t1 = 0
				}
				if run < 0 {
					run = mel.getRun()
				}
			}
			if x >= width {
				t1 = 0
			}
			scratch[spBase+2] = uint16(t1)
			x += 2
			cQ = ((t1 & 0x40) << 2) | ((t1 & 0x80) << 1)
			cQ |= uint32(scratch[spBase+2-sstr]) & 0x80
			vlcVal = vlc.advance(t1 & 0x7)

			uvlcMode := ((t0 & 0x8) << 3) | ((t1 & 0x8) << 4)
			uvlcEntry := uint32(htUVLCTbl1[uvlcMode+(vlcVal&0x3F)])
			vlcVal = vlc.advance(uvlcEntry & 0x7)
			uvlcEntry >>= 3
			ln := uvlcEntry & 0xF
			tmp := vlcVal & ((1 << ln) - 1)
			vlc.advance(ln) // consume quad-pair suffix bits; result unused
			uvlcEntry >>= 4
			ln = uvlcEntry & 0x7
			uvlcEntry >>= 3
			scratch[spBase+1] = uint16((uvlcEntry & 7) + (tmp &^ (0xFF << ln)))
			scratch[spBase+3] = uint16((uvlcEntry >> 3) + (tmp >> ln))
			spBase += 4
		}
		scratch[spBase+0] = 0
		scratch[spBase+1] = 0
	}

	// ----- step 2: decode MagSgn into decoded_data -----
	// Quads write two rows at a time, and the SigProp pass writes 4-row stripes,
	// so round the row count up to a multiple of 4 to absorb writes past the last
	// real row of an odd-sized block.
	stride := width
	dh := (height + 3) &^ 3
	decoded := make([]uint32, stride*dh)
	magsgn := newHTFrwd(buf, 0, lcup-scup, true)
	vnScratch := make([]uint32, width+8)

	decodeQuadSample := func(inf, uQ uint32, bit int, ms *htFrwd) (uint32, uint32) {
		var val, vn uint32
		if inf&(1<<uint(4+bit)) != 0 {
			msVal := ms.fetch()
			mN := uQ - ((inf >> uint(12+bit)) & 1)
			ms.advance(mN)
			val = msVal << 31
			vn = msVal & ((1 << mN) - 1)
			vn |= ((inf >> uint(8+bit)) & 1) << mN
			vn |= 1
			val |= (vn + 2) << uint(p-1)
		}
		return val, vn
	}

	// initial sample row pair
	{
		sp := 0
		vp := 0
		dp := 0
		prevVn := uint32(0)
		x := 0
		for x < width {
			inf := uint32(scratch[sp])
			uQ := uint32(scratch[sp+1])
			if uQ > uint32(mmsbp2) {
				return nil, false
			}
			val, _ := decodeQuadSample(inf, uQ, 0, magsgn)
			decoded[dp] = val
			val, vn := decodeQuadSample(inf, uQ, 1, magsgn)
			decoded[dp+stride] = val
			vnScratch[vp] = prevVn | vn
			prevVn = 0
			dp++
			x++
			if x >= width {
				vp++
				break
			}
			val, _ = decodeQuadSample(inf, uQ, 2, magsgn)
			decoded[dp] = val
			val, vn = decodeQuadSample(inf, uQ, 3, magsgn)
			decoded[dp+stride] = val
			prevVn = vn
			dp++
			x++
			sp += 2
			vp++
		}
		vnScratch[vp] = prevVn
	}

	// non-initial sample row pairs
	for y := 2; y < height; y += 2 {
		spBase := (y >> 1) * sstr
		sp := spBase
		vp := 0
		dp := y * stride
		prevVn := uint32(0)
		x := 0
		for x < width {
			inf := uint32(scratch[sp])
			uq := uint32(scratch[sp+1])
			gamma := inf & 0xF0
			gamma &= gamma - 0x10
			emax := vnScratch[vp] | vnScratch[vp+1]
			emax = 31 - uint32(bits.LeadingZeros32(emax|2))
			kappa := uint32(1)
			if gamma != 0 {
				kappa = emax
			}
			uQ := uq + kappa
			if uQ > uint32(mmsbp2) {
				return nil, false
			}
			val, _ := decodeQuadSample(inf, uQ, 0, magsgn)
			decoded[dp] = val
			val, vn := decodeQuadSample(inf, uQ, 1, magsgn)
			decoded[dp+stride] = val
			vnScratch[vp] = prevVn | vn
			prevVn = 0
			dp++
			x++
			if x >= width {
				vp++
				break
			}
			val, _ = decodeQuadSample(inf, uQ, 2, magsgn)
			decoded[dp] = val
			val, vn = decodeQuadSample(inf, uQ, 3, magsgn)
			decoded[dp+stride] = val
			prevVn = vn
			dp++
			x++
			sp += 2
			vp++
		}
		vnScratch[vp] = prevVn
	}

	if numPasses <= 1 {
		return decoded, true
	}

	// ----- refinement passes (SigProp, then MagRef) -----
	// Re-arrange quad significance (rho, bits 4..7 of each inf word) into column
	// significance: each nibble holds one column of 4 rows, 8 columns per word.
	mstr := (width + 3) >> 2
	mstr = ((mstr + 2) + 7) &^ 7
	sigmaRows := (height+3)/4 + 2
	sigma := make([]uint16, mstr*sigmaRows+2)
	for y := 0; y < height; y += 4 {
		spBase := (y >> 1) * sstr
		dpBase := (y >> 2) * mstr
		di := 0
		for x := 0; x < width; x += 4 {
			sp := spBase + x
			s0 := uint32(scratch[sp])
			s2 := uint32(scratch[sp+2])
			s0b := uint32(scratch[sp+sstr])
			s2b := uint32(scratch[sp+2+sstr])
			t0 := ((s0 & 0x30) >> 4) | ((s0 & 0xC0) >> 2)
			t0 |= ((s2 & 0x30) << 4) | ((s2 & 0xC0) << 6)
			t1 := ((s0b & 0x30) >> 2) | (s0b & 0xC0)
			t1 |= ((s2b & 0x30) << 6) | ((s2b & 0xC0) << 8)
			sigma[dpBase+di] = uint16(t0 | t1)
			di++
		}
		sigma[dpBase+di] = 0
	}

	// SigProp pass.
	{
		prevRowSig := make([]uint16, (width+15)/4+8)
		sigprop := newHTFrwd(buf, lcup, lengths2, false)
		for y := 0; y < height; y += 4 {
			pattern := uint32(0xFFFF)
			if height-y < 4 {
				pattern = 0x7777
				if height-y < 3 {
					pattern = 0x3333
					if height-y < 2 {
						pattern = 0x1111
					}
				}
			}
			prev := uint32(0)
			curSigBase := (y >> 2) * mstr
			for x := 0; x < width; x += 4 {
				k := x >> 2
				curSig := curSigBase + k
				s := x + 4 - width
				if s < 0 {
					s = 0
				}
				pattern >>= uint(s * 4)

				ps := ld32(prevRowSig, k)
				ns := ld32(sigma, curSig+mstr)
				u := (ps & 0x88888888) >> 3
				if !stripeCausal {
					u |= (ns & 0x11111111) << 3
				}
				csig := ld32(sigma, curSig)
				mbr := csig
				mbr |= (csig & 0x77777777) << 1
				mbr |= (csig & 0xEEEEEEEE) >> 1
				mbr |= u
				t := mbr
				mbr |= t << 4
				mbr |= t >> 4
				mbr |= prev >> 12
				mbr &= pattern
				mbr &^= csig

				newSig := mbr
				if newSig != 0 {
					cwd := sigprop.fetch()
					cnt := uint32(0)
					colMask := uint32(0xF)
					invSig := ^csig & pattern
					for i := 0; i < 16; i += 4 {
						if colMask&newSig != 0 {
							sampleMask := uint32(0x1111) & colMask
							if newSig&sampleMask != 0 {
								newSig &^= sampleMask
								if cwd&1 != 0 {
									newSig |= (uint32(0x33) << uint(i)) & invSig
								}
								cwd >>= 1
								cnt++
							}
							sampleMask <<= 1
							if newSig&sampleMask != 0 {
								newSig &^= sampleMask
								if cwd&1 != 0 {
									newSig |= (uint32(0x76) << uint(i)) & invSig
								}
								cwd >>= 1
								cnt++
							}
							sampleMask <<= 1
							if newSig&sampleMask != 0 {
								newSig &^= sampleMask
								if cwd&1 != 0 {
									newSig |= (uint32(0xEC) << uint(i)) & invSig
								}
								cwd >>= 1
								cnt++
							}
							sampleMask <<= 1
							if newSig&sampleMask != 0 {
								newSig &^= sampleMask
								if cwd&1 != 0 {
									newSig |= (uint32(0xC8) << uint(i)) & invSig
								}
								cwd >>= 1
								cnt++
							}
						}
						colMask <<= 4
					}
					if newSig != 0 {
						dpx := y*stride + x
						val := uint32(3) << uint(p-2)
						colMask = 0xF
						for i := 0; i < 4; i++ {
							if colMask&newSig != 0 {
								sampleMask := uint32(0x1111) & colMask
								if newSig&sampleMask != 0 {
									decoded[dpx] = (cwd << 31) | val
									cwd >>= 1
									cnt++
								}
								sampleMask += sampleMask
								if newSig&sampleMask != 0 {
									decoded[dpx+stride] = (cwd << 31) | val
									cwd >>= 1
									cnt++
								}
								sampleMask += sampleMask
								if newSig&sampleMask != 0 {
									decoded[dpx+2*stride] = (cwd << 31) | val
									cwd >>= 1
									cnt++
								}
								sampleMask += sampleMask
								if newSig&sampleMask != 0 {
									decoded[dpx+3*stride] = (cwd << 31) | val
									cwd >>= 1
									cnt++
								}
							}
							dpx++
							colMask <<= 4
						}
					}
					sigprop.advance(cnt)
				}

				newSig |= csig
				prevRowSig[k] = uint16(newSig)
				t = newSig
				newSig |= (t & 0x7777) << 1
				newSig |= (t & 0xEEEE) >> 1
				prev = newSig | u
				prev &= 0xF000
			}
		}
	}

	// MagRef pass.
	if numPasses > 2 {
		magref := newHTRevMRP(buf, lcup, lengths2)
		half := uint32(1) << uint(p-2)
		for y := 0; y < height; y += 4 {
			curSigBase := (y >> 2) * mstr
			dppBase := y * stride
			csi := 0
			for i := 0; i < width; i += 8 {
				cwd := magref.fetch()
				sig := ld32(sigma, curSigBase+csi)
				csi += 2
				if sig != 0 {
					colMask := uint32(0xF)
					for j := 0; j < 8; j++ {
						if sig&colMask != 0 {
							dp := dppBase + i + j
							sampleMask := uint32(0x11111111) & colMask
							for k := 0; k < 4; k++ {
								if sig&sampleMask != 0 {
									sym := cwd & 1
									sym = (1 - sym) << uint(p-1)
									sym |= half
									decoded[dp] ^= sym
									cwd >>= 1
								}
								sampleMask += sampleMask
								dp += stride
							}
						}
						colMask <<= 4
					}
				}
				magref.advance(uint32(bits.OnesCount32(sig)))
			}
		}
	}

	return decoded, true
}

// ld32 reads two consecutive little-endian uint16 entries as one uint32, matching
// openjph's *(ui32*) reads over the 16-bit significance scratch arrays.
func ld32(s []uint16, i int) uint32 {
	return uint32(s[i]) | uint32(s[i+1])<<16
}
