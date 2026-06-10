package gojxl

import (
	"errors"
	"fmt"
)

// This file parses a baseline-sequential JPEG file into the jpegData structure
// (markers, quant/Huffman tables, scan info and dequantized... no — quantized
// DCT coefficients, plus the bit-exact-reconstruction metadata: padding bits,
// reset points and extra-zero-runs). It is the inverse of jpeg_encode.go and a
// port of libjxl's enc_jpeg_data_reader.cc (kReadAll, non-progressive subset).
// ParseJPEG(src) followed by EncodeJPEG reproduces src byte-for-byte.

// jpegParser walks the JPEG byte stream.
type jpegParser struct {
	data []byte
	pos  int
	jd   *jpegData
}

func (p *jpegParser) u8() int { v := int(p.data[p.pos]); p.pos++; return v }
func (p *jpegParser) u16() int {
	v := int(p.data[p.pos])<<8 | int(p.data[p.pos+1])
	p.pos += 2
	return v
}

// ParseJPEG parses a baseline JPEG into a jpegData. Progressive JPEGs and
// 12/16-bit precision are rejected.
func ParseJPEG(data []byte) (*jpegData, error) {
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, errors.New("gojxl: not a JPEG (missing SOI)")
	}
	p := &jpegParser{data: data, pos: 2, jd: &jpegData{}}
	jd := p.jd
	var dcDec, acDec [4]*jpegHuffDecoder
	foundSOF := false
	for {
		// Skip any non-marker bytes between segments (inter-marker data).
		skipped := p.findNextMarker()
		if skipped > 0 {
			jd.markerOrder = append(jd.markerOrder, 0xFF)
			jd.interMarkerData = append(jd.interMarkerData, append([]byte(nil), data[p.pos-skipped:p.pos]...))
		}
		if p.pos+1 >= len(data) || data[p.pos] != 0xFF {
			return nil, errTruncated
		}
		marker := uint8(data[p.pos+1])
		p.pos += 2
		var err error
		switch {
		case marker == 0xC0 || marker == 0xC1:
			err = p.processSOF()
			foundSOF = true
		case marker == 0xC2:
			return nil, errors.New("gojxl: progressive JPEG not supported")
		case marker == 0xC4:
			err = p.processDHT(&dcDec, &acDec)
		case marker >= 0xD0 && marker <= 0xD7:
			// RST markers carry no data here.
		case marker == 0xD9:
			// EOI.
		case marker == 0xDA:
			err = p.processScan(&dcDec, &acDec)
		case marker == 0xDB:
			err = p.processDQT()
		case marker == 0xDD:
			err = p.processDRI()
		case marker >= 0xE0 && marker <= 0xEF:
			err = p.processMarkerSegment(&jd.appData)
		case marker == 0xFE:
			err = p.processMarkerSegment(&jd.comData)
		default:
			return nil, fmt.Errorf("gojxl: unsupported JPEG marker 0x%02x", marker)
		}
		if err != nil {
			return nil, err
		}
		jd.markerOrder = append(jd.markerOrder, marker)
		if marker == 0xD9 {
			break
		}
	}
	if !foundSOF {
		return nil, errors.New("gojxl: missing SOF marker")
	}
	if p.pos < len(data) {
		jd.tailData = append([]byte(nil), data[p.pos:]...)
	}
	if err := p.fixupIndexes(); err != nil {
		return nil, err
	}
	if len(jd.huffmanCode) == 0 {
		return nil, errors.New("gojxl: no Huffman tables")
	}
	// All APP markers are stored verbatim (type "unknown"); ICC/Exif/XMP are not
	// extracted into the JXL color profile by this encoder.
	jd.appMarkerType = make([]uint32, len(jd.appData))
	return jd, nil
}

// kIsValidMarker[i]==1 means (0xC0+i) is a valid marker.
var kIsValidMarker = [64]uint8{
	1, 1, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1,
	1, 1, 0, 1, 1, 1, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0,
}

func (p *jpegParser) findNextMarker() int {
	n := 0
	d := p.data
	for p.pos+1 < len(d) && (d[p.pos] != 0xFF || d[p.pos+1] < 0xC0 || kIsValidMarker[d[p.pos+1]-0xC0] == 0) {
		p.pos++
		n++
	}
	return n
}

func (p *jpegParser) processSOF() error {
	jd := p.jd
	if jd.width != 0 {
		return errors.New("gojxl: duplicate SOF")
	}
	p.u16() // marker length
	precision := p.u8()
	if precision != 8 {
		return fmt.Errorf("gojxl: unsupported JPEG precision %d", precision)
	}
	jd.height = p.u16()
	jd.width = p.u16()
	nc := p.u8()
	if nc < 1 || nc > 4 {
		return errors.New("gojxl: invalid component count")
	}
	jd.components = make([]jpegComponent, nc)
	maxH, maxV := 1, 1
	for i := 0; i < nc; i++ {
		jd.components[i].id = uint32(p.u8())
		f := p.u8()
		h, v := f>>4, f&0xF
		if h < 1 || h > 4 || v < 1 || v > 4 {
			return errors.New("gojxl: invalid sampling factor")
		}
		jd.components[i].hSampFactor = h
		jd.components[i].vSampFactor = v
		jd.components[i].quantIdx = uint32(p.u8())
		if h > maxH {
			maxH = h
		}
		if v > maxV {
			maxV = v
		}
	}
	mcuRows := divCeilInt(jd.height, maxV*8)
	mcuCols := divCeilInt(jd.width, maxH*8)
	for i := range jd.components {
		c := &jd.components[i]
		if maxH%c.hSampFactor != 0 || maxV%c.vSampFactor != 0 {
			return errors.New("gojxl: non-integral subsampling")
		}
		c.widthInBlocks = uint32(mcuCols * c.hSampFactor)
		c.heightInBlks = uint32(mcuRows * c.vSampFactor)
		c.coeffs = make([]int16, int(c.widthInBlocks)*int(c.heightInBlks)*64)
	}
	return nil
}

func (p *jpegParser) processDQT() error {
	end := p.pos + p.u16() - 2
	for p.pos < end && len(p.jd.quant) < 4 {
		pi := p.u8()
		prec, idx := pi>>4, pi&0xF
		if prec > 1 || idx > 3 {
			return errors.New("gojxl: invalid DQT")
		}
		var t jpegQuantTable
		t.precision = uint32(prec)
		t.index = uint32(idx)
		for i := 0; i < 64; i++ {
			var v int
			if prec != 0 {
				v = p.u16()
			} else {
				v = p.u8()
			}
			if v < 1 || v > 65535 {
				return errors.New("gojxl: invalid quant value")
			}
			t.values[kJpegNaturalOrder[i]] = int32(v)
		}
		t.isLast = p.pos == end
		p.jd.quant = append(p.jd.quant, t)
	}
	return nil
}

func (p *jpegParser) processDRI() error {
	p.u16() // length
	p.jd.restartInterval = uint32(p.u16())
	return nil
}

// processMarkerSegment stores an APP/COM segment as [marker, len_hi, len_lo,
// payload...] matching the jbrd layout.
func (p *jpegParser) processMarkerSegment(dst *[][]byte) error {
	markerLen := p.u16()
	if markerLen < 2 || p.pos+markerLen-2 > len(p.data) {
		return errors.New("gojxl: invalid marker length")
	}
	seg := append([]byte(nil), p.data[p.pos-3:p.pos-3+markerLen+1]...)
	p.pos += markerLen - 2
	*dst = append(*dst, seg)
	return nil
}

func (p *jpegParser) processDHT(dcDec, acDec *[4]*jpegHuffDecoder) error {
	end := p.pos + p.u16() - 2
	for p.pos < end {
		var hc jpegHuffmanCode
		hc.slotID = p.u8()
		isAC := hc.slotID&0x10 != 0
		idx := hc.slotID & 0xF
		if idx > 3 {
			return errors.New("gojxl: invalid Huffman slot")
		}
		total := 0
		maxDepth := 1
		for i := 1; i <= 16; i++ {
			c := p.u8()
			if c != 0 {
				maxDepth = i
			}
			hc.counts[i] = uint32(c)
			total += c
		}
		if total > 256 {
			return errors.New("gojxl: invalid Huffman code")
		}
		vals := make([]uint8, total)
		for i := 0; i < total; i++ {
			hc.values[i] = uint32(p.u8())
			vals[i] = uint8(hc.values[i])
		}
		// Build the decoder from the real table (before the sentinel).
		dec := buildJpegHuffDecoder(hc.counts, vals)
		if isAC {
			acDec[idx] = dec
		} else {
			dcDec[idx] = dec
		}
		// Append the all-ones sentinel symbol (256), matching jbrd / the writer.
		hc.counts[maxDepth]++
		hc.values[total] = jpegHuffmanAlphabetSize
		hc.isLast = p.pos == end
		p.jd.huffmanCode = append(p.jd.huffmanCode, hc)
	}
	return nil
}

func (p *jpegParser) fixupIndexes() error {
	for i := range p.jd.components {
		c := &p.jd.components[i]
		found := false
		for j := range p.jd.quant {
			if p.jd.quant[j].index == c.quantIdx {
				c.quantIdx = uint32(j)
				found = true
				break
			}
		}
		if !found {
			return errors.New("gojxl: quant table index not found")
		}
	}
	return nil
}
