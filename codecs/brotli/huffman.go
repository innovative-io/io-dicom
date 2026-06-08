package brotli

// huffTable is a canonical (DEFLATE-style) prefix-code decoder. Brotli packs
// prefix-code bits least-significant-first, so the first transmitted bit is the
// most-significant bit of the canonical code; this is the same convention as
// DEFLATE and is handled by indexing a flat lookup table with bit-reversed
// codes.
type huffTable struct {
	maxLen uint     // longest code length (0 => single-symbol, zero-bit code)
	single int      // symbol returned for a zero-bit code
	table  []uint32 // [maxLen-bit index] -> (length<<16)|symbol
}

// reverseBits reverses the low n bits of x.
func reverseBits(x uint32, n uint) uint32 {
	var r uint32
	for i := uint(0); i < n; i++ {
		r = (r << 1) | (x & 1)
		x >>= 1
	}
	return r
}

// buildHuff builds a decoder from per-symbol code lengths (0 = unused). Returns
// nil only if the length set is empty/invalid.
func buildHuff(lengths []uint8) *huffTable {
	maxLen := 0
	var count [16]int
	used := 0
	last := 0
	for sym, l := range lengths {
		if l > 15 {
			return nil
		}
		if l != 0 {
			used++
			last = sym
			count[l]++
			if int(l) > maxLen {
				maxLen = int(l)
			}
		}
	}
	if used == 0 {
		return nil
	}
	if used == 1 {
		// A single symbol is coded with zero bits.
		return &huffTable{maxLen: 0, single: last}
	}
	// Canonical first code per length (most-significant-bit first).
	var nextCode [16]uint32
	code := uint32(0)
	count[0] = 0
	for bits := 1; bits <= maxLen; bits++ {
		code = (code + uint32(count[bits-1])) << 1
		nextCode[bits] = code
	}
	size := 1 << uint(maxLen)
	t := make([]uint32, size)
	for sym, l := range lengths {
		if l == 0 {
			continue
		}
		c := nextCode[l]
		nextCode[l]++
		rev := reverseBits(c, uint(l))
		step := 1 << uint(l)
		entry := (uint32(l) << 16) | uint32(sym)
		for f := int(rev); f < size; f += step {
			t[f] = entry
		}
	}
	return &huffTable{maxLen: uint(maxLen), table: t}
}

// decode reads one symbol from b.
func (h *huffTable) decode(b *bitReader) int {
	if h.maxLen == 0 {
		return h.single
	}
	idx := b.peek(h.maxLen)
	e := h.table[idx]
	b.skip(uint(e >> 16))
	return int(e & 0xFFFF)
}

// readHuffmanCode reads a prefix-code description (simple or complex) for an
// alphabet of the given size and returns its decoder.
func readHuffmanCode(b *bitReader, alphabetSize int) (*huffTable, error) {
	hskip := b.readBits(2)
	if hskip == 1 {
		return readSimpleHuffman(b, alphabetSize)
	}
	return readComplexHuffman(b, alphabetSize, int(hskip))
}

func bitsForSize(n int) uint {
	bits := uint(0)
	for (1 << bits) < n {
		bits++
	}
	return bits
}

func readSimpleHuffman(b *bitReader, alphabetSize int) (*huffTable, error) {
	nsym := int(b.readBits(2)) + 1
	abits := bitsForSize(alphabetSize)
	var syms [4]int
	for i := 0; i < nsym; i++ {
		s := int(b.readBits(abits))
		if s >= alphabetSize {
			return nil, errCorrupt
		}
		syms[i] = s
	}
	lengths := make([]uint8, alphabetSize)
	switch nsym {
	case 1:
		return &huffTable{maxLen: 0, single: syms[0]}, nil
	case 2:
		lengths[syms[0]] = 1
		lengths[syms[1]] = 1
	case 3:
		lengths[syms[0]] = 1
		lengths[syms[1]] = 2
		lengths[syms[2]] = 2
	case 4:
		if b.readBits(1) == 1 {
			lengths[syms[0]] = 1
			lengths[syms[1]] = 2
			lengths[syms[2]] = 3
			lengths[syms[3]] = 3
		} else {
			lengths[syms[0]] = 2
			lengths[syms[1]] = 2
			lengths[syms[2]] = 2
			lengths[syms[3]] = 2
		}
	}
	h := buildHuff(lengths)
	if h == nil {
		return nil, errCorrupt
	}
	return h, nil
}

// kCodeLengthCodeOrder is the transmission order of the 18 code-length-code
// lengths.
var kCodeLengthCodeOrder = [18]int{1, 2, 3, 4, 0, 5, 17, 6, 16, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// Fixed prefix code used to read each code-length-code length (value 0..5) from
// a 4-bit window. Index = next 4 bits; tables give the consumed length and
// decoded value.
var kCodeLengthPrefixLength = [16]uint{2, 2, 2, 3, 2, 2, 2, 4, 2, 2, 2, 3, 2, 2, 2, 4}
var kCodeLengthPrefixValue = [16]uint8{0, 4, 3, 2, 0, 4, 3, 1, 0, 4, 3, 2, 0, 4, 3, 5}

const initialRepeatedCodeLength = 8
const codeLengthRepeatCode = 16

func readComplexHuffman(b *bitReader, alphabetSize, hskip int) (*huffTable, error) {
	// Read the lengths of the 18-symbol code-length code.
	var clcl [18]uint8
	space := 32
	numCodes := 0
	for i := hskip; i < 18; i++ {
		p := b.peek(4)
		ln := kCodeLengthPrefixLength[p]
		v := kCodeLengthPrefixValue[p]
		b.skip(ln)
		clcl[kCodeLengthCodeOrder[i]] = v
		if v != 0 {
			numCodes++
			space -= 32 >> v
			if space <= 0 {
				break
			}
		}
	}
	if numCodes == 0 {
		return nil, errCorrupt
	}
	clHuff := buildHuff(clcl[:])
	if clHuff == nil {
		return nil, errCorrupt
	}

	// Read the symbol code lengths, honoring repeat codes 16/17.
	lengths := make([]uint8, alphabetSize)
	var repeat, repeatCodeLen, prevCodeLen uint32 = 0, 0, initialRepeatedCodeLength
	hspace := uint32(32768)
	sym := 0
	for sym < alphabetSize && hspace > 0 {
		codeLen := clHuff.decode(b)
		if codeLen < codeLengthRepeatCode {
			repeat = 0
			lengths[sym] = uint8(codeLen)
			sym++
			if codeLen != 0 {
				prevCodeLen = uint32(codeLen)
				hspace -= 32768 >> uint(codeLen)
			}
			continue
		}
		extraBits := uint(codeLen) - 14 // 16->2, 17->3
		var newLen uint32
		if codeLen == 17 {
			newLen = 0
		} else {
			newLen = prevCodeLen
		}
		if repeatCodeLen != newLen {
			repeat = 0
			repeatCodeLen = newLen
		}
		oldRepeat := repeat
		if repeat > 0 {
			repeat -= 2
			repeat <<= extraBits
		}
		repeat += b.readBits(extraBits) + 3
		delta := repeat - oldRepeat
		if sym+int(delta) > alphabetSize {
			return nil, errCorrupt
		}
		if repeatCodeLen != 0 {
			for i := uint32(0); i < delta; i++ {
				lengths[sym+int(i)] = uint8(repeatCodeLen)
			}
			hspace -= delta << (15 - repeatCodeLen)
		}
		sym += int(delta)
	}
	h := buildHuff(lengths)
	if h == nil {
		return nil, errCorrupt
	}
	return h, nil
}

// decodeVarLenUint8 reads the RFC 7932 variable-length 8-bit value used for
// block-type and tree counts.
func decodeVarLenUint8(b *bitReader) int {
	if b.readBits(1) == 0 {
		return 0
	}
	nbits := b.readBits(3)
	return int((1 << nbits) + b.readBits(uint(nbits)))
}
