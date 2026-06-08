package gojxl

import "errors"

// Header is the parsed codestream header: dimensions plus image metadata.
type Header struct {
	Size SizeHeader
	Meta ImageMetadata
}

// ReadHeader parses the JPEG XL signature, SizeHeader and ImageMetadata from a
// full file (raw codestream or "JXL " container). The ICC blob (when
// want_icc) and frame data are not consumed.
func ReadHeader(data []byte) (Header, error) {
	cs, err := codestream(data)
	if err != nil {
		return Header{}, err
	}
	if len(cs) < 2 || cs[0] != 0xFF || cs[1] != 0x0A {
		return Header{}, errors.New("gojxl: bad codestream signature")
	}
	b := newBitReader(cs[2:])
	sh, err := readSizeHeader(b)
	if err != nil {
		return Header{}, err
	}
	meta, err := readImageMetadata(b)
	if err != nil {
		return Header{}, err
	}
	return Header{Size: sh, Meta: meta}, nil
}

// SizeHeader holds the image dimensions (headers.cc SizeHeader::VisitFields).
type SizeHeader struct {
	Xsize uint32
	Ysize uint32
}

// kRatios are the aspect-ratio shortcuts (headers.cc kRatios): numerator,
// denominator. Index is (ratio-1).
var kRatioNum = [7]uint32{1, 12, 4, 3, 16, 5, 2}
var kRatioDen = [7]uint32{1, 10, 3, 2, 9, 4, 1}

func readSizeHeader(b *bitReader) (SizeHeader, error) {
	var sh SizeHeader
	small := b.ReadBool()

	var ysize uint32
	if small {
		ysize = (b.ReadBits(5) + 1) * 8
	} else {
		ysize = b.ReadU32(u32Off(9, 1), u32Off(13, 1), u32Off(18, 1), u32Off(30, 1))
	}
	sh.Ysize = ysize

	ratio := b.ReadBits(3)
	if ratio == 0 {
		if small {
			sh.Xsize = (b.ReadBits(5) + 1) * 8
		} else {
			sh.Xsize = b.ReadU32(u32Off(9, 1), u32Off(13, 1), u32Off(18, 1), u32Off(30, 1))
		}
	} else {
		sh.Xsize = uint32(uint64(ysize) * uint64(kRatioNum[ratio-1]) / uint64(kRatioDen[ratio-1]))
	}
	if !b.ok() {
		return sh, errTruncated
	}
	if sh.Xsize == 0 || sh.Ysize == 0 {
		return sh, errors.New("gojxl: zero image dimension")
	}
	return sh, nil
}

// ---------------------------------------------------------------------------
// Container / signature handling.
// ---------------------------------------------------------------------------

// codestream extracts the raw JPEG XL codestream from data, unwrapping the
// ISO-BMFF ("JXL ") container when present. A raw codestream begins with the
// two-byte signature 0xFF 0x0A.
func codestream(data []byte) ([]byte, error) {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0x0A {
		return data, nil // raw codestream
	}
	// ISO-BMFF container signature box: 0x0000000C 'JXL ' 0x0D0A870A.
	if len(data) >= 12 && data[0] == 0 && data[1] == 0 && data[2] == 0 && data[3] == 0x0C &&
		data[4] == 'J' && data[5] == 'X' && data[6] == 'L' && data[7] == ' ' &&
		data[8] == 0x0D && data[9] == 0x0A && data[10] == 0x87 && data[11] == 0x0A {
		return containerCodestream(data)
	}
	return nil, errors.New("gojxl: not a JPEG XL stream (bad signature)")
}

// containerCodestream concatenates the codestream from a "JXL " ISO-BMFF file:
// the whole `jxlc` box, or the ordered sequence of `jxlp` partial-codestream
// boxes (each prefixed by a 4-byte index whose high bit marks the last).
func containerCodestream(data []byte) ([]byte, error) {
	var out []byte
	gotJxlc := false
	pos := 0
	for pos+8 <= len(data) {
		size := int(be32(data, pos))
		hdr := 8
		boxEnd := 0
		switch {
		case size == 1:
			// 64-bit largesize.
			if pos+16 > len(data) {
				return nil, errTruncated
			}
			large := be64(data, pos+8)
			hdr = 16
			if large == 0 {
				boxEnd = len(data)
			} else {
				boxEnd = pos + int(large)
			}
		case size == 0:
			boxEnd = len(data) // extends to EOF
		default:
			boxEnd = pos + size
		}
		if boxEnd < pos+hdr || boxEnd > len(data) {
			return nil, errTruncated
		}
		typ := string(data[pos+4 : pos+8])
		body := data[pos+hdr : boxEnd]
		switch typ {
		case "jxlc":
			out = append(out, body...)
			gotJxlc = true
		case "jxlp":
			if len(body) < 4 {
				return nil, errTruncated
			}
			out = append(out, body[4:]...) // skip the 4-byte sequence index
		}
		pos = boxEnd
	}
	if len(out) == 0 {
		return nil, errors.New("gojxl: container has no codestream box")
	}
	_ = gotJxlc
	return out, nil
}

func be32(b []byte, o int) uint32 {
	return uint32(b[o])<<24 | uint32(b[o+1])<<16 | uint32(b[o+2])<<8 | uint32(b[o+3])
}

func be64(b []byte, o int) uint64 {
	return uint64(be32(b, o))<<32 | uint64(be32(b, o+4))
}
