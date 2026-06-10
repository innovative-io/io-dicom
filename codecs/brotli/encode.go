package brotli

// bitWriter writes bits least-significant-bit first, matching the Brotli bit
// packing (the inverse of bitReader).
type bitWriter struct {
	out []byte
	cur uint64 // pending bits in the low `n` positions
	n   uint
}

func (w *bitWriter) writeBits(nbits uint, val uint64) {
	w.cur |= (val & ((1 << nbits) - 1)) << w.n
	w.n += nbits
	for w.n >= 8 {
		w.out = append(w.out, byte(w.cur))
		w.cur >>= 8
		w.n -= 8
	}
}

// alignByte flushes any partial byte (zero-padded) to a byte boundary.
func (w *bitWriter) alignByte() {
	if w.n > 0 {
		w.out = append(w.out, byte(w.cur))
		w.cur = 0
		w.n = 0
	}
}

// Compress produces a valid Brotli stream for src using uncompressed
// (stored) meta-blocks. It performs no actual compression — it is meant for
// payloads that are already small or incompressible (e.g. JPEG marker data in a
// jbrd box), where a conformant stream matters more than ratio. The output is
// accepted by any RFC 7932 decoder.
func Compress(src []byte) []byte {
	w := &bitWriter{}
	w.writeBits(1, 0) // WBITS=16 (window header: a single "0" bit)

	const maxChunk = 1 << 24 // 6-nibble MLEN holds up to 2^24 bytes
	pos := 0
	for pos < len(src) {
		chunk := len(src) - pos
		if chunk > maxChunk {
			chunk = maxChunk
		}
		// MLEN must use the minimal number of nibbles (4, 5 or 6) so the most
		// significant nibble is non-zero, matching the decoder's check.
		mlenMinus1 := uint64(chunk - 1)
		nibbles := uint(4)
		for nibbles < 6 && mlenMinus1 >= (1<<(4*nibbles)) {
			nibbles++
		}
		w.writeBits(1, 0)                  // ISLAST = 0
		w.writeBits(2, uint64(nibbles-4))  // MNIBBLES selector
		w.writeBits(4*nibbles, mlenMinus1) // MLEN-1
		w.writeBits(1, 1)                  // ISUNCOMPRESSED = 1
		w.alignByte()
		w.out = append(w.out, src[pos:pos+chunk]...)
		pos += chunk
	}
	// Final empty last meta-block.
	w.writeBits(1, 1) // ISLAST = 1
	w.writeBits(1, 1) // ISLASTEMPTY = 1
	w.alignByte()
	return w.out
}
