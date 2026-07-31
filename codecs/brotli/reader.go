// Package brotli implements a pure-Go Brotli decompressor (RFC 7932).
//
// It exists to support JPEG XL "JPEG bitstream reconstruction" (jbrd) boxes,
// whose variable-length data (APP/COM markers, inter-marker and tail bytes) is
// Brotli-compressed. The decoder covers the full RFC 7932 format: window
// header, stored/compressed/metadata meta-blocks, simple and complex prefix
// codes, block splitting, context modeling, the static dictionary and word
// transforms. It deliberately implements only the standard (10..24 bit) window;
// the large-window extension is not used by libjxl's encoder.
//
// The package has no third-party dependencies; the 122 KB static dictionary and
// 2 KB context-lookup table are embedded (flate-compressed for the former).
package brotli

import "errors"

var (
	errTruncated  = errors.New("brotli: truncated input")
	errCorrupt    = errors.New("brotli: corrupt stream")
	errTooLarge   = errors.New("brotli: decompressed size exceeds limit")
	errDictionary = errors.New("brotli: invalid dictionary reference")
)

// bitReader reads bits least-significant-bit first within each byte, bytes in
// stream order, matching the Brotli bit packing.
type bitReader struct {
	src    []byte
	pos    int    // index of the next unread byte in src
	bitbuf uint64 // buffered bits, valid in the low `bitcnt` positions
	bitcnt uint   // number of valid bits currently in bitbuf
	eof    bool   // set once a read ran past the end of src
}

func newBitReader(src []byte) *bitReader { return &bitReader{src: src} }

// fill loads bytes into bitbuf until it holds more than 56 bits or src is
// exhausted. Reading past the end yields zero bits and sets eof.
func (b *bitReader) fill() {
	for b.bitcnt <= 56 {
		if b.pos >= len(b.src) {
			b.eof = true
			break
		}
		b.bitbuf |= uint64(b.src[b.pos]) << b.bitcnt
		b.pos++
		b.bitcnt += 8
	}
}

// readBits consumes n bits (0..56) and returns them. Bits beyond the input are
// returned as zero with eof set.
func (b *bitReader) readBits(n uint) uint32 {
	if n == 0 {
		return 0
	}
	if b.bitcnt < n {
		b.fill()
		if b.bitcnt < n {
			// Not enough real bits; the remainder is implicit zero.
			v := uint32(b.bitbuf & ((1 << n) - 1))
			b.bitbuf = 0
			b.bitcnt = 0
			return v
		}
	}
	v := uint32(b.bitbuf & ((1 << n) - 1))
	b.bitbuf >>= n
	b.bitcnt -= n
	return v
}

// peek returns the next n bits (0..15) without consuming them, used by the
// Huffman decoder which then skips the exact code length.
func (b *bitReader) peek(n uint) uint32 {
	if n == 0 {
		return 0
	}
	if b.bitcnt < n {
		b.fill()
	}
	return uint32(b.bitbuf & ((1 << n) - 1))
}

// skip discards n already-peeked bits.
func (b *bitReader) skip(n uint) {
	if n == 0 {
		return
	}
	if b.bitcnt < n {
		b.fill()
		if b.bitcnt < n {
			b.bitbuf = 0
			b.bitcnt = 0
			return
		}
	}
	b.bitbuf >>= n
	b.bitcnt -= n
}

// alignToByte discards buffered bits up to the next byte boundary so the stream
// position becomes byte-aligned (used before stored/metadata payloads).
func (b *bitReader) alignToByte() {
	drop := b.bitcnt & 7
	b.bitbuf >>= drop
	b.bitcnt -= drop
}

// readAlignedBytes copies n bytes assuming the reader is byte-aligned.
func (b *bitReader) readAlignedBytes(n int) ([]byte, error) {
	// Check availability before allocating. n comes from the codestream (MLEN,
	// up to 2^24), so allocating first let a 4-byte input reserve 16 MiB before
	// failing the bounds check below.
	if n < 0 {
		return nil, errCorrupt
	}
	if buffered := int(b.bitcnt / 8); n > buffered && b.pos+(n-buffered) > len(b.src) {
		return nil, errTruncated
	}
	out := make([]byte, 0, n)
	for n > 0 && b.bitcnt >= 8 {
		out = append(out, byte(b.bitbuf&0xFF))
		b.bitbuf >>= 8
		b.bitcnt -= 8
		n--
	}
	if n > 0 {
		if b.pos+n > len(b.src) {
			return nil, errTruncated
		}
		out = append(out, b.src[b.pos:b.pos+n]...)
		b.pos += n
	}
	return out, nil
}
