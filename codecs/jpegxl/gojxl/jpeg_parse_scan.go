package gojxl

import "errors"

// jpegHuffDecoder is a canonical JPEG Huffman decoder (CCITT T.81 Annex F.2.2.3).
type jpegHuffDecoder struct {
	minCode [17]int32
	maxCode [17]int32 // -1 when no codes of that length
	valPtr  [17]int
	values  []uint8
}

func buildJpegHuffDecoder(counts [17]uint32, values []uint8) *jpegHuffDecoder {
	d := &jpegHuffDecoder{values: values}
	code := int32(0)
	k := 0
	for l := 1; l <= 16; l++ {
		if counts[l] > 0 {
			d.valPtr[l] = k
			d.minCode[l] = code
			code += int32(counts[l])
			d.maxCode[l] = code - 1
			k += int(counts[l])
		} else {
			d.maxCode[l] = -1
		}
		code <<= 1
	}
	return d
}

// jpegBitReader reads the entropy-coded segment MSB-first, unescaping 0xFF/0x00
// and stopping at the next marker. Ports libjxl's BitReaderState.
type jpegBitReader struct {
	data          []byte
	pos           int
	val           uint64
	bitsLeft      int
	nextMarkerPos int
}

func newJPEGBitReader(data []byte, pos int) *jpegBitReader {
	br := &jpegBitReader{data: data}
	br.reset(pos)
	return br
}

func (br *jpegBitReader) reset(pos int) {
	br.pos = pos
	br.val = 0
	br.bitsLeft = 0
	br.nextMarkerPos = len(br.data) - 2
	br.fill()
}

func (br *jpegBitReader) nextByte() uint8 {
	if br.pos >= br.nextMarkerPos {
		br.pos++
		return 0
	}
	c := br.data[br.pos]
	br.pos++
	if c == 0xFF {
		esc := br.data[br.pos]
		if esc == 0 {
			br.pos++
		} else {
			// Start of the next marker.
			br.nextMarkerPos = br.pos - 1
		}
	}
	return c
}

func (br *jpegBitReader) fill() {
	if br.bitsLeft <= 16 {
		for br.bitsLeft <= 56 {
			br.val <<= 8
			br.val |= uint64(br.nextByte())
			br.bitsLeft += 8
		}
	}
}

func (br *jpegBitReader) readBits(n int) int {
	br.fill()
	v := (br.val >> uint(br.bitsLeft-n)) & ((1 << uint(n)) - 1)
	br.bitsLeft -= n
	return int(v)
}

func (br *jpegBitReader) readBit() int { return br.readBits(1) }

// readSymbol decodes one Huffman symbol bit-by-bit.
func (br *jpegBitReader) readSymbol(d *jpegHuffDecoder) (int, error) {
	code := int32(0)
	for l := 1; l <= 16; l++ {
		code = (code << 1) | int32(br.readBit())
		if d.maxCode[l] >= 0 && code <= d.maxCode[l] {
			return int(d.values[d.valPtr[l]+int(code-d.minCode[l])]), nil
		}
	}
	return 0, errors.New("gojxl: invalid Huffman code in scan")
}

// finishStream records padding bits and rewinds unused bytes, leaving p.pos at
// the next marker.
func (br *jpegBitReader) finishStream(jd *jpegData) (int, error) {
	npad := br.bitsLeft & 7
	if npad > 0 {
		mask := uint64(1)<<uint(npad) - 1
		pad := (br.val >> uint(br.bitsLeft-npad)) & mask
		if pad != mask {
			jd.hasZeroPadding = true
		}
		for i := npad - 1; i >= 0; i-- {
			jd.paddingBits = append(jd.paddingBits, (pad>>uint(i))&1 == 1)
		}
	}
	unused := br.bitsLeft >> 3
	for unused > 0 {
		unused--
		br.pos--
		if br.pos < br.nextMarkerPos && br.data[br.pos] == 0 && br.data[br.pos-1] == 0xFF {
			br.pos--
		}
	}
	if br.pos > br.nextMarkerPos {
		return 0, errors.New("gojxl: unexpected end of scan")
	}
	return br.pos, nil
}

func huffExtend(x, s int) int {
	if x >= 1<<uint(s-1) {
		return x
	}
	return x - (1 << uint(s)) + 1
}

// processScan reads the SOS header and the entropy-coded scan, filling component
// coefficients plus reset points / extra-zero-runs / padding bits.
func (p *jpegParser) processScan(dcDec, acDec *[4]*jpegHuffDecoder) error {
	jd := p.jd
	p.u16() // marker length
	nComps := p.u8()
	if nComps < 1 || nComps > len(jd.components) {
		return errors.New("gojxl: invalid scan component count")
	}
	var sc jpegScanInfo
	sc.numComponents = uint32(nComps)
	for i := 0; i < nComps; i++ {
		id := uint32(p.u8())
		found := false
		for j := range jd.components {
			if jd.components[j].id == id {
				sc.components[i].compIdx = uint32(j)
				found = true
			}
		}
		if !found {
			return errors.New("gojxl: scan references unknown component")
		}
		t := p.u8()
		sc.components[i].dcTblIdx = uint32(t >> 4)
		sc.components[i].acTblIdx = uint32(t & 0xF)
	}
	sc.Ss = uint32(p.u8())
	sc.Se = uint32(p.u8())
	ah := p.u8()
	sc.Ah = uint32(ah >> 4)
	sc.Al = uint32(ah & 0xF)
	if sc.Ss != 0 || sc.Se != 63 || sc.Ah != 0 || sc.Al != 0 {
		return errors.New("gojxl: only baseline-sequential scans are supported")
	}

	maxH, maxV := 1, 1
	for i := range jd.components {
		if jd.components[i].hSampFactor > maxH {
			maxH = jd.components[i].hSampFactor
		}
		if jd.components[i].vSampFactor > maxV {
			maxV = jd.components[i].vSampFactor
		}
	}
	interleaved := nComps > 1
	mcuRows := divCeilInt(jd.height, maxV*8)
	mcusPerRow := divCeilInt(jd.width, maxH*8)
	if !interleaved {
		c := &jd.components[sc.components[0].compIdx]
		mcusPerRow = divCeilInt(jd.width*c.hSampFactor, 8*maxH)
		mcuRows = divCeilInt(jd.height*c.vSampFactor, 8*maxV)
	}

	br := newJPEGBitReader(p.data, p.pos)
	var lastDC [4]int32
	restartsToGo := int(jd.restartInterval)
	nextRestart := 0
	blockScanIndex := 0

	for my := 0; my < mcuRows; my++ {
		for mx := 0; mx < mcusPerRow; mx++ {
			if jd.restartInterval > 0 && restartsToGo == 0 {
				pos, err := br.finishStream(jd)
				if err != nil {
					return err
				}
				expected := uint8(0xD0 + nextRestart)
				if pos+1 >= len(p.data) || p.data[pos] != 0xFF || p.data[pos+1] != expected {
					return errors.New("gojxl: missing restart marker")
				}
				br.reset(pos + 2)
				nextRestart = (nextRestart + 1) & 7
				restartsToGo = int(jd.restartInterval)
				lastDC = [4]int32{}
			}
			if jd.restartInterval > 0 {
				restartsToGo--
			}
			for i := 0; i < nComps; i++ {
				si := sc.components[i]
				c := &jd.components[si.compIdx]
				dc := dcDec[si.dcTblIdx]
				ac := acDec[si.acTblIdx]
				if dc == nil || ac == nil {
					return errors.New("gojxl: scan uses undefined Huffman table")
				}
				nby, nbx := 1, 1
				if interleaved {
					nby, nbx = c.vSampFactor, c.hSampFactor
				}
				for iy := 0; iy < nby; iy++ {
					for ix := 0; ix < nbx; ix++ {
						by := my*nby + iy
						bx := mx*nbx + ix
						blockIdx := by*int(c.widthInBlocks) + bx
						coeffs := c.coeffs[blockIdx*64 : blockIdx*64+64]
						nzr, err := p.decodeBlock(br, dc, ac, &lastDC[si.compIdx], coeffs)
						if err != nil {
							return err
						}
						if nzr > 0 {
							sc.extraZeroRuns = append(sc.extraZeroRuns, jpegExtraZeroRun{
								blockIdx: uint32(blockScanIndex), numExtraZeroRun: uint32(nzr)})
						}
						blockScanIndex++
					}
				}
			}
		}
	}
	pos, err := br.finishStream(jd)
	if err != nil {
		return err
	}
	p.pos = pos
	jd.scanInfo = append(jd.scanInfo, sc)
	return nil
}

// decodeBlock decodes one baseline 8x8 block. Returns the number of trailing
// ZRL runs (extra zero runs) for bit-exact reconstruction.
func (p *jpegParser) decodeBlock(br *jpegBitReader, dc, ac *jpegHuffDecoder, lastDC *int32, coeffs []int16) (int, error) {
	s, err := br.readSymbol(dc)
	if err != nil {
		return 0, err
	}
	if s >= jpegDCAlphabetSize {
		return 0, errors.New("gojxl: invalid DC symbol")
	}
	diff := 0
	if s > 0 {
		diff = huffExtend(br.readBits(s), s)
	}
	coeff := diff + int(*lastDC)
	coeffs[0] = int16(coeff)
	*lastDC = int32(coeff)

	numZeroRuns := 0
	for k := 1; k <= 63; k++ {
		sr, err := br.readSymbol(ac)
		if err != nil {
			return 0, err
		}
		if sr >= jpegHuffmanAlphabetSize {
			return 0, errors.New("gojxl: invalid AC symbol")
		}
		r := sr >> 4
		ss := sr & 15
		if ss > 0 {
			k += r
			if k > 63 {
				return 0, errors.New("gojxl: AC coefficient out of band")
			}
			coeffs[kJpegNaturalOrder[k]] = int16(huffExtend(br.readBits(ss), ss))
			numZeroRuns = 0
		} else if r == 15 {
			k += 15
			numZeroRuns++
		} else {
			// EOB (baseline: run length 1).
			break
		}
	}
	return numZeroRuns, nil
}
