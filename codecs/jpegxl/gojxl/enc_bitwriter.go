package gojxl

// Bit writer and field-coder writers — the inverse of bitreader.go, for the
// pure-Go lossless Modular encoder. Bits are written LSB-first within each byte.

type bitWriter struct {
	data  []byte
	cur   uint64
	nbits int
}

func newBitWriter() *bitWriter { return &bitWriter{} }

// WriteBits appends the low n bits of v (LSB first; 0 <= n <= 56).
func (w *bitWriter) WriteBits(v uint64, n int) {
	if n == 0 {
		return
	}
	w.cur |= (v & ((1 << uint(n)) - 1)) << uint(w.nbits)
	w.nbits += n
	for w.nbits >= 8 {
		w.data = append(w.data, byte(w.cur))
		w.cur >>= 8
		w.nbits -= 8
	}
}

func (w *bitWriter) WriteBool(b bool) {
	if b {
		w.WriteBits(1, 1)
	} else {
		w.WriteBits(0, 1)
	}
}

// ZeroPadToByte flushes to the next byte boundary with zero bits.
func (w *bitWriter) ZeroPadToByte() {
	if w.nbits > 0 {
		w.data = append(w.data, byte(w.cur))
		w.cur = 0
		w.nbits = 0
	}
}

// Bytes returns the written bytes, padding the final partial byte with zeros.
func (w *bitWriter) Bytes() []byte {
	w.ZeroPadToByte()
	return w.data
}

// WriteU32 encodes value with the cheapest of the four distributions that can
// represent it (inverse of bitReader.ReadU32).
func (w *bitWriter) WriteU32(value uint32, d0, d1, d2, d3 u32d) {
	dists := [4]u32d{d0, d1, d2, d3}
	for sel := 0; sel < 4; sel++ {
		d := dists[sel]
		if value < d.offset {
			continue
		}
		extra := value - d.offset
		if d.bits >= 32 || extra < (uint32(1)<<uint(d.bits)) {
			w.WriteBits(uint64(sel), 2)
			w.WriteBits(uint64(extra), int(d.bits))
			return
		}
	}
	panic("gojxl: value not representable by U32 distributions")
}

// WriteU64 encodes value (inverse of bitReader.ReadU64).
func (w *bitWriter) WriteU64(value uint64) {
	switch {
	case value == 0:
		w.WriteBits(0, 2)
	case value <= 16:
		w.WriteBits(1, 2)
		w.WriteBits(value-1, 4)
	case value <= 272:
		w.WriteBits(2, 2)
		w.WriteBits(value-17, 8)
	default:
		w.WriteBits(3, 2)
		w.WriteBits(value&0xFFF, 12)
		value >>= 12
		shift := uint(12)
		for value != 0 {
			w.WriteBits(1, 1) // continuation
			if shift == 60 {
				w.WriteBits(value&0xF, 4)
				value = 0
				break
			}
			w.WriteBits(value&0xFF, 8)
			value >>= 8
			shift += 8
		}
		w.WriteBits(0, 1) // end
	}
}

// WriteEnum encodes an enum value (inverse of bitReader.ReadEnum):
// U32(Val(0), Val(1), BitsOffset(4, 2), BitsOffset(6, 18)).
func (w *bitWriter) WriteEnum(value uint32) {
	w.WriteU32(value, u32Val(0), u32Val(1), u32Off(4, 2), u32Off(6, 18))
}
