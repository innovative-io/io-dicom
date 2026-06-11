package brotli

// Decompress decodes a complete Brotli stream and returns the original bytes.
// If sizeHint > 0 it is used to pre-size the output buffer.
func Decompress(src []byte, sizeHint int) ([]byte, error) {
	d := &decoder{b: newBitReader(src)}
	if sizeHint > 0 {
		d.out = make([]byte, 0, sizeHint)
	}
	if err := d.run(); err != nil {
		return nil, err
	}
	return d.out, nil
}

type decoder struct {
	b           *bitReader
	out         []byte
	maxBackward int
	p1, p2      int    // previous two output bytes (literal context)
	ring        [4]int // distance ring buffer, ring[0] = most recent
}

func (d *decoder) run() error {
	lgwin, err := readWindowBits(d.b)
	if err != nil {
		return err
	}
	d.maxBackward = (1 << uint(lgwin)) - 16
	// Distance ring buffer, most-recent first. Brotli seeds it so the implicit
	// "last distance" is 4 (RFC 7932 §4 / reference dist-cache {16,15,11,4} with
	// the write index at 0 makes slot 3 the most recent).
	d.ring = [4]int{4, 11, 15, 16}

	for {
		mlen, isLast, isUncompressed, isMetadata, err := readMetaBlockLength(d.b)
		if err != nil {
			return err
		}
		if isMetadata {
			// Metadata payload produces no output; skip its bytes.
			d.b.alignToByte()
			if _, err := d.b.readAlignedBytes(mlen); err != nil {
				return err
			}
			if isLast {
				break
			}
			continue
		}
		if mlen == 0 {
			if isLast {
				break
			}
			continue
		}
		if isUncompressed {
			d.b.alignToByte()
			raw, err := d.b.readAlignedBytes(mlen)
			if err != nil {
				return err
			}
			d.out = append(d.out, raw...)
			if len(raw) >= 2 {
				d.p1 = int(raw[len(raw)-1])
				d.p2 = int(raw[len(raw)-2])
			} else if len(raw) == 1 {
				d.p2 = d.p1
				d.p1 = int(raw[0])
			}
			if isLast {
				break
			}
			continue
		}
		if err := d.decodeMetaBlock(mlen); err != nil {
			return err
		}
		if isLast {
			break
		}
	}
	return nil
}

func readWindowBits(b *bitReader) (int, error) {
	if b.readBits(1) == 0 {
		return 16, nil
	}
	n := b.readBits(3)
	if n != 0 {
		return 17 + int(n), nil
	}
	m := b.readBits(3)
	if m != 0 {
		return 8 + int(m), nil
	}
	return 17, nil
}

func readMetaBlockLength(b *bitReader) (mlen int, isLast, isUncompressed, isMetadata bool, err error) {
	isLast = b.readBits(1) == 1
	if isLast && b.readBits(1) == 1 {
		// ISLASTEMPTY: empty final meta-block.
		return 0, true, false, false, nil
	}
	sel := b.readBits(2)
	if sel == 3 {
		// Metadata meta-block.
		if b.readBits(1) != 0 { // reserved bit must be zero
			return 0, false, false, false, errCorrupt
		}
		numBytes := int(b.readBits(2))
		if numBytes == 0 {
			return 0, false, false, true, nil
		}
		v := 0
		for i := 0; i < numBytes; i++ {
			db := int(b.readBits(8))
			if i == numBytes-1 && numBytes > 1 && db == 0 {
				return 0, false, false, false, errCorrupt
			}
			v |= db << (8 * i)
		}
		return v + 1, false, false, true, nil
	}
	numNibbles := int(sel) + 4
	v := 0
	for i := 0; i < numNibbles; i++ {
		nb := int(b.readBits(4))
		if i == numNibbles-1 && numNibbles > 4 && nb == 0 {
			return 0, false, false, false, errCorrupt
		}
		v |= nb << (4 * i)
	}
	mlen = v + 1
	if !isLast {
		isUncompressed = b.readBits(1) == 1
	}
	return mlen, isLast, isUncompressed, false, nil
}

// blockSplit tracks block-type/length state for one of the three categories
// (literals, insert-and-copy, distance).
type blockSplit struct {
	numTypes int
	typeHuff *huffTable
	lenHuff  *huffTable
	rb       [2]int // [previous, current] block type
	length   int
}

func readBlockSplit(b *bitReader, numTypes int) (*blockSplit, error) {
	bs := &blockSplit{numTypes: numTypes, rb: [2]int{1, 0}}
	if numTypes >= 2 {
		th, err := readHuffmanCode(b, numTypes+2)
		if err != nil {
			return nil, err
		}
		lh, err := readHuffmanCode(b, 26)
		if err != nil {
			return nil, err
		}
		bs.typeHuff = th
		bs.lenHuff = lh
		s := lh.decode(b)
		bs.length = int(blockLenOffset[s] + b.readBits(blockLenExtra[s]))
	} else {
		bs.length = 1 << 30
	}
	return bs, nil
}

func (bs *blockSplit) cur() int { return bs.rb[1] }

func (bs *blockSplit) advance(b *bitReader) {
	sym := bs.typeHuff.decode(b)
	var t int
	switch {
	case sym == 0:
		t = bs.rb[0]
	case sym == 1:
		t = bs.rb[1] + 1
	default:
		t = sym - 2
	}
	if t >= bs.numTypes {
		t -= bs.numTypes
	}
	bs.rb[0] = bs.rb[1]
	bs.rb[1] = t
	s := bs.lenHuff.decode(b)
	bs.length = int(blockLenOffset[s] + b.readBits(blockLenExtra[s]))
}

func (d *decoder) decodeMetaBlock(mlen int) error {
	b := d.b

	litSplit, err := readBlockSplit(b, decodeVarLenUint8(b)+1)
	if err != nil {
		return err
	}
	icSplit, err := readBlockSplit(b, decodeVarLenUint8(b)+1)
	if err != nil {
		return err
	}
	distSplit, err := readBlockSplit(b, decodeVarLenUint8(b)+1)
	if err != nil {
		return err
	}

	npostfix := int(b.readBits(2))
	ndirect := int(b.readBits(4)) << uint(npostfix)

	cmode := make([]int, litSplit.numTypes)
	for i := range cmode {
		cmode[i] = int(b.readBits(2))
	}

	numTreesL, litCmap, err := readContextMap(b, 64*litSplit.numTypes)
	if err != nil {
		return err
	}
	numTreesD, distCmap, err := readContextMap(b, 4*distSplit.numTypes)
	if err != nil {
		return err
	}

	litHuff := make([]*huffTable, numTreesL)
	for i := range litHuff {
		if litHuff[i], err = readHuffmanCode(b, 256); err != nil {
			return err
		}
	}
	icHuff := make([]*huffTable, icSplit.numTypes)
	for i := range icHuff {
		if icHuff[i], err = readHuffmanCode(b, 704); err != nil {
			return err
		}
	}
	distAlphabet := 16 + ndirect + (48 << uint(npostfix))
	distHuff := make([]*huffTable, numTreesD)
	for i := range distHuff {
		if distHuff[i], err = readHuffmanCode(b, distAlphabet); err != nil {
			return err
		}
	}

	dict := dictionary()
	produced := 0
	for produced < mlen {
		if icSplit.length == 0 {
			icSplit.advance(b)
		}
		icSplit.length--
		cmdSym := icHuff[icSplit.cur()].decode(b)
		if cmdSym >= len(kCmdLut) {
			return errCorrupt
		}
		e := kCmdLut[cmdSym]
		insertLen := int(e.insOffset + b.readBits(e.insExtra))
		copyLen := int(e.cpOffset + b.readBits(e.cpExtra))

		for j := 0; j < insertLen; j++ {
			if litSplit.length == 0 {
				litSplit.advance(b)
			}
			litSplit.length--
			lt := litSplit.cur()
			ctx := context(cmode[lt], d.p1, d.p2)
			ht := litCmap[lt*64+ctx]
			lit := byte(litHuff[ht].decode(b))
			d.out = append(d.out, lit)
			d.p2 = d.p1
			d.p1 = int(lit)
		}
		produced += insertLen
		if produced >= mlen {
			break
		}

		var distance int
		explicit := false // explicit distance code (eligible to update the ring)
		if e.dist0 {
			distance = d.ring[0]
		} else {
			if distSplit.length == 0 {
				distSplit.advance(b)
			}
			distSplit.length--
			dt := distSplit.cur()
			dctx := copyLen - 2
			if dctx > 3 {
				dctx = 3
			}
			dh := distCmap[dt*4+dctx]
			distSym := distHuff[dh].decode(b)
			distance, explicit = decodeDistance(b, distSym, npostfix, ndirect, d.ring)
		}

		maxDistance := len(d.out)
		if maxDistance > d.maxBackward {
			maxDistance = d.maxBackward
		}
		if distance <= maxDistance {
			if distance <= 0 {
				return errCorrupt
			}
			// Only genuine backward references update the distance ring; a
			// dictionary reference (distance > maxDistance) does not.
			if explicit {
				d.ring[3] = d.ring[2]
				d.ring[2] = d.ring[1]
				d.ring[1] = d.ring[0]
				d.ring[0] = distance
			}
			src := len(d.out) - distance
			for j := 0; j < copyLen; j++ {
				bb := d.out[src+j]
				d.out = append(d.out, bb)
				d.p2 = d.p1
				d.p1 = int(bb)
			}
			produced += copyLen
		} else {
			wlen := copyLen
			if wlen < 4 || wlen > 24 {
				return errDictionary
			}
			distMinus := distance - maxDistance - 1
			sb := uint(dictSizeBitsByLength[wlen])
			numWords := 1 << sb
			index := distMinus & (numWords - 1)
			tID := distMinus >> sb
			wordOff := int(dictOffsetByLength[wlen]) + index*wlen
			if wordOff < 0 || wordOff+wlen > len(dict) {
				return errDictionary
			}
			before := len(d.out)
			d.out, err = applyTransform(d.out, tID, dict[wordOff:wordOff+wlen])
			if err != nil {
				return err
			}
			added := len(d.out) - before
			produced += added
			if added >= 2 {
				d.p1 = int(d.out[len(d.out)-1])
				d.p2 = int(d.out[len(d.out)-2])
			} else if added == 1 {
				d.p2 = d.p1
				d.p1 = int(d.out[len(d.out)-1])
			}
		}
	}
	return nil
}

// readContextMap decodes a context map of the given size, returning the number
// of distinct htrees and the per-context htree indices.
func readContextMap(b *bitReader, size int) (int, []uint8, error) {
	numHtrees := decodeVarLenUint8(b) + 1
	cmap := make([]uint8, size)
	if numHtrees <= 1 {
		return numHtrees, cmap, nil
	}
	rleMax := 0
	if b.readBits(1) == 1 {
		rleMax = int(b.readBits(4)) + 1
	}
	huff, err := readHuffmanCode(b, numHtrees+rleMax)
	if err != nil {
		return 0, nil, err
	}
	i := 0
	for i < size {
		sym := huff.decode(b)
		switch {
		case sym == 0:
			cmap[i] = 0
			i++
		case sym <= rleMax:
			run := (1 << uint(sym)) + int(b.readBits(uint(sym)))
			for k := 0; k < run && i < size; k++ {
				cmap[i] = 0
				i++
			}
		default:
			cmap[i] = uint8(sym - rleMax)
			i++
		}
	}
	if b.readBits(1) == 1 {
		inverseMTF(cmap)
	}
	return numHtrees, cmap, nil
}

func inverseMTF(v []uint8) {
	var mtf [256]uint8
	for i := 0; i < 256; i++ {
		mtf[i] = uint8(i)
	}
	for i := range v {
		idx := v[i]
		val := mtf[idx]
		v[i] = val
		copy(mtf[1:int(idx)+1], mtf[0:int(idx)])
		mtf[0] = val
	}
}

// decodeDistance turns a distance symbol into a distance value. The boolean
// reports whether the distance ring buffer should be updated (false only for
// the implicit last-distance code 0).
func decodeDistance(b *bitReader, distSym, npostfix, ndirect int, ring [4]int) (int, bool) {
	if distSym < 16 {
		sc := distanceShortCode[distSym]
		return ring[sc.ring] + sc.offset, distSym != 0
	}
	if distSym < 16+ndirect {
		return distSym - 15, true
	}
	postfixMask := (1 << uint(npostfix)) - 1
	v := distSym - ndirect - 16
	ndistbits := uint(1 + (v >> uint(npostfix+1)))
	extra := int(b.readBits(ndistbits))
	hcode := v >> uint(npostfix)
	lcode := v & postfixMask
	offset := ((2 + (hcode & 1)) << ndistbits) - 4
	return ((offset + extra) << uint(npostfix)) + lcode + ndirect + 1, true
}
