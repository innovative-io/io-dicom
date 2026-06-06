// Package gojxl is a pure-Go JPEG XL (ISO/IEC 18181) decoder.
//
// It is being built decoder-first toward full parity with libjxl, in validated
// stages: container + headers, then the ANS/prefix entropy core, then Modular
// (lossless) frames, then VarDCT (lossy) frames, then JPEG-bitstream
// reconstruction (.111). This file provides the low-level bit reader and field
// coders (U32/U64/F16/Enum), matching libjxl's lib/jxl/dec_bit_reader.h and
// fields.cc exactly.
package gojxl

import "errors"

// errTruncated is returned when the stream ends before a required field.
var errTruncated = errors.New("gojxl: truncated bitstream")

// bitReader reads bits LSB-first within each byte, bytes in stream order — the
// JPEG XL convention. Reads past the end yield zero bits and set overread.
type bitReader struct {
	data     []byte
	pos      int    // index of next byte to load into buf
	buf      uint64 // bit buffer; the next bit to return is buf&1
	nbits    int    // number of valid bits currently in buf
	overread bool   // a read consumed bits past the end of data
}

func newBitReader(data []byte) *bitReader { return &bitReader{data: data} }

func (b *bitReader) refill() {
	for b.nbits <= 56 {
		if b.pos >= len(b.data) {
			return
		}
		b.buf |= uint64(b.data[b.pos]) << uint(b.nbits)
		b.pos++
		b.nbits += 8
	}
}

// ReadBits returns the next n bits (0 <= n <= 32), first bit in the LSB.
func (b *bitReader) ReadBits(n int) uint32 {
	if n == 0 {
		return 0
	}
	b.refill()
	if n <= b.nbits {
		v := uint32(b.buf & ((1 << uint(n)) - 1))
		b.buf >>= uint(n)
		b.nbits -= n
		return v
	}
	// Past end: return the available low bits, pad the rest with zero.
	v := uint32(b.buf & ((1 << uint(b.nbits)) - 1))
	b.buf = 0
	b.nbits = 0
	b.overread = true
	return v
}

// ReadBool reads a single bit as a boolean.
func (b *bitReader) ReadBool() bool { return b.ReadBits(1) != 0 }

// bitsConsumed reports the total number of bits read so far.
func (b *bitReader) bitsConsumed() int { return b.pos*8 - b.nbits }

// JumpToByteBoundary consumes zero-padding bits up to the next byte boundary.
// Per the spec the padding must be zero; a non-zero pad is an error.
func (b *bitReader) JumpToByteBoundary() error {
	rem := b.bitsConsumed() & 7
	if rem == 0 {
		return nil
	}
	if b.ReadBits(8-rem) != 0 {
		return errors.New("gojxl: non-zero padding bits")
	}
	return nil
}

// ok reports whether all reads so far stayed within the available data.
func (b *bitReader) ok() bool { return !b.overread }

// ---------------------------------------------------------------------------
// Field coders (libjxl fields.cc)
// ---------------------------------------------------------------------------

// u32d is one branch of a U32 distribution. A 2-bit selector chooses one of
// four; the decoded value is ReadBits(bits) + offset. Val(k) = {k, 0};
// Bits(n) = {0, n}; BitsOffset(n, o) = {o, n}.
type u32d struct {
	offset uint32
	bits   uint8
}

func u32Val(k uint32) u32d          { return u32d{offset: k, bits: 0} }
func u32Bits(n uint8) u32d          { return u32d{offset: 0, bits: n} }
func u32Off(n uint8, o uint32) u32d { return u32d{offset: o, bits: n} }

// ReadU32 decodes a U32Coder value from four distributions.
func (b *bitReader) ReadU32(d0, d1, d2, d3 u32d) uint32 {
	sel := b.ReadBits(2)
	d := [4]u32d{d0, d1, d2, d3}[sel]
	return b.ReadBits(int(d.bits)) + d.offset
}

// ReadU64 decodes a U64Coder value (fields.cc U64Coder::Read).
func (b *bitReader) ReadU64() uint64 {
	sel := b.ReadBits(2)
	switch sel {
	case 0:
		return 0
	case 1:
		return 1 + uint64(b.ReadBits(4))
	case 2:
		return 17 + uint64(b.ReadBits(8))
	}
	// selector 3: varint — first 12 bits, then 8-bit groups gated by a
	// continuation bit, last group 4 bits.
	result := uint64(b.ReadBits(12))
	shift := uint(12)
	for b.ReadBits(1) != 0 {
		if shift == 60 {
			result |= uint64(b.ReadBits(4)) << shift
			break
		}
		result |= uint64(b.ReadBits(8)) << shift
		shift += 8
	}
	return result
}

// ReadEnum decodes an Enum value (fields.h VisitorBase::Enum):
// U32(Val(0), Val(1), BitsOffset(4, 2), BitsOffset(6, 18)).
func (b *bitReader) ReadEnum() uint32 {
	return b.ReadU32(u32Val(0), u32Val(1), u32Off(4, 2), u32Off(6, 18))
}

// ReadF16 decodes an IEEE-754 half-precision float as float32 (fields.cc
// F16Coder::Read).
func (b *bitReader) ReadF16() (float32, error) {
	bits := b.ReadBits(16)
	sign := bits >> 15
	biasedExp := (bits >> 10) & 0x1F
	mantissa := bits & 0x3FF
	if biasedExp == 31 {
		return 0, errors.New("gojxl: f16 out of range (inf/nan)")
	}
	var f float32
	if biasedExp == 0 {
		// Subnormal.
		f = float32(mantissa) * (1.0 / (1 << 24))
	} else {
		// Normalized: reconstruct via integer exponent.
		exp := int(biasedExp) - 15
		frac := float32(mantissa)/1024.0 + 1.0
		f = frac * pow2(exp)
	}
	if sign != 0 {
		f = -f
	}
	return f, nil
}

func pow2(e int) float32 {
	f := float32(1)
	if e >= 0 {
		for i := 0; i < e; i++ {
			f *= 2
		}
	} else {
		for i := 0; i < -e; i++ {
			f /= 2
		}
	}
	return f
}
