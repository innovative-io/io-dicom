package gojxl

// Brotli-style prefix (Huffman) code decoder, ported from libjxl dec_huffman.cc
// and huffman_table.cc. Used when ANSCode.usePrefixCode is set.

const (
	huffmanTableBits = 8
	prefixMaxBits    = 15
)

// huffmanCode is one multi-level decoding-table entry.
type huffmanCode struct {
	bits  uint8  // number of bits used for this symbol (or table+root bits)
	value uint16 // symbol value, or 2nd-level table offset
}

var kCodeLengthCodeOrder = [18]uint8{1, 2, 3, 4, 0, 5, 17, 6, 16, 7, 8, 9, 10, 11, 12, 13, 14, 15}

const kCodeLengthRepeatCode = 16

// getNextKey returns reverse(reverse(key,len)+1, len).
func getNextKey(key, length int) int {
	step := 1 << uint(length-1)
	for key&step != 0 {
		step >>= 1
	}
	return (key & (step - 1)) + step
}

// replicateValue stores code at table[end-step], table[end-2*step], ... down to 0.
func replicateValue(table []huffmanCode, step, end int, code huffmanCode) {
	for {
		end -= step
		table[end] = code
		if end <= 0 {
			break
		}
	}
}

func nextTableBitSize(count *[16]uint16, length, rootBits int) int {
	left := 1 << uint(length-rootBits)
	for length < prefixMaxBits {
		if left <= int(count[length]) {
			break
		}
		left -= int(count[length])
		length++
		left <<= 1
	}
	return length - rootBits
}

// buildHuffmanTable fills rootTable from code lengths; returns total table size.
func buildHuffmanTable(rootTable []huffmanCode, rootBits int, codeLengths []uint8, count *[16]uint16) uint32 {
	var offset [prefixMaxBits + 1]uint16
	maxLength := 1
	if len(codeLengths) > (1 << prefixMaxBits) {
		return 0
	}
	sorted := make([]uint16, len(codeLengths))
	{
		var sum uint16
		for l := 1; l <= prefixMaxBits; l++ {
			offset[l] = sum
			if count[l] != 0 {
				sum += count[l]
				maxLength = l
			}
		}
	}
	for sym := 0; sym < len(codeLengths); sym++ {
		if codeLengths[sym] != 0 {
			sorted[offset[codeLengths[sym]]] = uint16(sym)
			offset[codeLengths[sym]]++
		}
	}

	tableOff := 0 // index of current sub-table within rootTable
	tableBits := rootBits
	tableSize := 1 << uint(tableBits)
	totalSize := tableSize

	// Special case: only one symbol.
	if offset[prefixMaxBits] == 1 {
		code := huffmanCode{bits: 0, value: sorted[0]}
		for key := 0; key < totalSize; key++ {
			rootTable[key] = code
		}
		return uint32(totalSize)
	}

	if tableBits > maxLength {
		tableBits = maxLength
		tableSize = 1 << uint(tableBits)
	}

	key := 0
	symbol := 0
	codeBits := 1
	step := 2
	for {
		for count[codeBits] != 0 {
			code := huffmanCode{bits: uint8(codeBits), value: sorted[symbol]}
			symbol++
			replicateValue(rootTable[tableOff+key:], step, tableSize, code)
			key = getNextKey(key, codeBits)
			count[codeBits]--
		}
		step <<= 1
		codeBits++
		if codeBits > tableBits {
			break
		}
	}

	// Replicate the partial root table to fill it.
	for totalSize != tableSize {
		copy(rootTable[tableOff+tableSize:tableOff+2*tableSize], rootTable[tableOff:tableOff+tableSize])
		tableSize <<= 1
	}

	// Second-level tables.
	mask := totalSize - 1
	low := -1
	length := rootBits + 1
	step = 2
	for ; length <= maxLength; length, step = length+1, step<<1 {
		for count[length] != 0 {
			if (key & mask) != low {
				tableOff += tableSize
				tableBits = nextTableBitSize(count, length, rootBits)
				tableSize = 1 << uint(tableBits)
				totalSize += tableSize
				low = key & mask
				rootTable[low].bits = uint8(tableBits + rootBits)
				rootTable[low].value = uint16(tableOff - low)
			}
			code := huffmanCode{bits: uint8(length - rootBits), value: sorted[symbol]}
			symbol++
			replicateValue(rootTable[tableOff+(key>>uint(rootBits)):], step, tableSize, code)
			key = getNextKey(key, length)
			count[length]--
		}
	}
	return uint32(totalSize)
}

// huffmanDecodingData holds a built prefix-code table.
type huffmanDecodingData struct {
	table []huffmanCode
}

func floorLog2Nonzero(x uint32) int {
	n := -1
	for x != 0 {
		n++
		x >>= 1
	}
	return n
}

func readSimpleCode(alphabetSize int, br *bitReader, table []huffmanCode) bool {
	maxBits := 0
	if alphabetSize > 1 {
		maxBits = floorLog2Nonzero(uint32(alphabetSize-1)) + 1
	}
	numSymbols := int(br.ReadBits(2)) + 1
	var symbols [4]uint16
	for i := 0; i < numSymbols; i++ {
		s := uint16(br.ReadBits(maxBits))
		if int(s) >= alphabetSize {
			return false
		}
		symbols[i] = s
	}
	for i := 0; i < numSymbols-1; i++ {
		for j := i + 1; j < numSymbols; j++ {
			if symbols[i] == symbols[j] {
				return false
			}
		}
	}
	if numSymbols == 4 {
		numSymbols += int(br.ReadBits(1))
	}
	swap := func(i, j int) { symbols[i], symbols[j] = symbols[j], symbols[i] }

	tableSize := 1
	switch numSymbols {
	case 1:
		table[0] = huffmanCode{0, symbols[0]}
	case 2:
		if symbols[0] > symbols[1] {
			swap(0, 1)
		}
		table[0] = huffmanCode{1, symbols[0]}
		table[1] = huffmanCode{1, symbols[1]}
		tableSize = 2
	case 3:
		if symbols[1] > symbols[2] {
			swap(1, 2)
		}
		table[0] = huffmanCode{1, symbols[0]}
		table[2] = huffmanCode{1, symbols[0]}
		table[1] = huffmanCode{2, symbols[1]}
		table[3] = huffmanCode{2, symbols[2]}
		tableSize = 4
	case 4:
		for i := 0; i < 3; i++ {
			for j := i + 1; j < 4; j++ {
				if symbols[i] > symbols[j] {
					swap(i, j)
				}
			}
		}
		table[0] = huffmanCode{2, symbols[0]}
		table[2] = huffmanCode{2, symbols[1]}
		table[1] = huffmanCode{2, symbols[2]}
		table[3] = huffmanCode{2, symbols[3]}
		tableSize = 4
	case 5:
		if symbols[2] > symbols[3] {
			swap(2, 3)
		}
		table[0] = huffmanCode{1, symbols[0]}
		table[1] = huffmanCode{2, symbols[1]}
		table[2] = huffmanCode{1, symbols[0]}
		table[3] = huffmanCode{3, symbols[2]}
		table[4] = huffmanCode{1, symbols[0]}
		table[5] = huffmanCode{2, symbols[1]}
		table[6] = huffmanCode{1, symbols[0]}
		table[7] = huffmanCode{3, symbols[3]}
		tableSize = 8
	default:
		return false
	}
	goalSize := 1 << huffmanTableBits
	for tableSize != goalSize {
		copy(table[tableSize:2*tableSize], table[:tableSize])
		tableSize <<= 1
	}
	return true
}

// staticHuffForCodeLenLengths is the fixed prefix code for code-length code
// lengths (dec_huffman.cc `huff[16]`).
var staticHuffForCodeLenLengths = [16]huffmanCode{
	{2, 0}, {2, 4}, {2, 3}, {3, 2}, {2, 0}, {2, 4}, {2, 3}, {4, 1},
	{2, 0}, {2, 4}, {2, 3}, {3, 2}, {2, 0}, {2, 4}, {2, 3}, {4, 5},
}

func readHuffmanCodeLengths(codeLengthCodeLengths []uint8, numSymbols int, codeLengths []uint8, br *bitReader) bool {
	symbol := 0
	prevCodeLen := uint8(8) // kDefaultCodeLength
	repeat := 0
	repeatCodeLen := uint8(0)
	space := 32768

	var counts [16]uint16
	for i := 0; i < 18; i++ {
		counts[codeLengthCodeLengths[i]]++
	}
	table := make([]huffmanCode, 32)
	if buildHuffmanTable(table, 5, codeLengthCodeLengths, &counts) == 0 {
		return false
	}

	for symbol < numSymbols && space > 0 {
		br.Refill()
		p := table[br.PeekBits(5)]
		br.Consume(int(p.bits))
		codeLen := uint8(p.value)
		if codeLen < kCodeLengthRepeatCode {
			repeat = 0
			codeLengths[symbol] = codeLen
			symbol++
			if codeLen != 0 {
				prevCodeLen = codeLen
				space -= 32768 >> uint(codeLen)
			}
		} else {
			extraBits := int(codeLen) - 14
			var newLen uint8
			if codeLen == kCodeLengthRepeatCode {
				newLen = prevCodeLen
			}
			if repeatCodeLen != newLen {
				repeat = 0
				repeatCodeLen = newLen
			}
			oldRepeat := repeat
			if repeat > 0 {
				repeat -= 2
				repeat <<= uint(extraBits)
			}
			repeat += int(br.ReadBits(extraBits)) + 3
			repeatDelta := repeat - oldRepeat
			if symbol+repeatDelta > numSymbols {
				return false
			}
			for k := 0; k < repeatDelta; k++ {
				codeLengths[symbol+k] = repeatCodeLen
			}
			symbol += repeatDelta
			if repeatCodeLen != 0 {
				space -= repeatDelta << uint(15-repeatCodeLen)
			}
		}
	}
	if space != 0 {
		return false
	}
	for ; symbol < numSymbols; symbol++ {
		codeLengths[symbol] = 0
	}
	return true
}

// ReadFromBitStream builds the prefix code for one histogram.
func (h *huffmanDecodingData) ReadFromBitStream(alphabetSize int, br *bitReader) bool {
	if alphabetSize > (1 << prefixMaxBits) {
		return false
	}
	simpleCodeOrSkip := br.ReadBits(2)
	if simpleCodeOrSkip == 1 {
		h.table = make([]huffmanCode, 1<<huffmanTableBits)
		return readSimpleCode(alphabetSize, br, h.table)
	}

	codeLengths := make([]uint8, alphabetSize)
	var codeLengthCodeLengths [18]uint8
	space := 32
	numCodes := 0
	for i := int(simpleCodeOrSkip); i < 18 && space > 0; i++ {
		idx := kCodeLengthCodeOrder[i]
		br.Refill()
		p := staticHuffForCodeLenLengths[br.PeekBits(4)]
		br.Consume(int(p.bits))
		v := uint8(p.value)
		codeLengthCodeLengths[idx] = v
		if v != 0 {
			space -= 32 >> uint(v)
			numCodes++
		}
	}
	ok := (numCodes == 1 || space == 0) &&
		readHuffmanCodeLengths(codeLengthCodeLengths[:], alphabetSize, codeLengths, br)
	if !ok {
		return false
	}
	var counts [16]uint16
	for i := 0; i < alphabetSize; i++ {
		counts[codeLengths[i]]++
	}
	h.table = make([]huffmanCode, alphabetSize+376)
	size := buildHuffmanTable(h.table, huffmanTableBits, codeLengths, &counts)
	if size == 0 {
		return false
	}
	h.table = h.table[:size]
	return true
}

// ReadSymbol decodes one prefix-coded symbol.
func (h *huffmanDecodingData) ReadSymbol(br *bitReader) uint16 {
	idx := br.PeekBits(huffmanTableBits)
	e := h.table[idx]
	nBits := int(e.bits)
	if nBits > huffmanTableBits {
		br.Consume(huffmanTableBits)
		nBits -= huffmanTableBits
		base := int(idx) + int(e.value)
		e = h.table[base+int(br.PeekBits(nBits))]
	}
	br.Consume(int(e.bits))
	return e.value
}
