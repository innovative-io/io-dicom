package gojxl

import "errors"

// ColorSpace values (cms/color_encoding_cms.h ColorSpace).
const (
	csRGB     = 0
	csGray    = 1
	csXYB     = 2
	csUnknown = 3
)

// ExtraChannel type values (image_metadata.h ExtraChannel).
const (
	ecAlpha     = 0
	ecSpotColor = 2
	ecCFA       = 5
)

// BitDepth (image_metadata.cc BitDepth::VisitFields).
type BitDepth struct {
	FloatingPoint bool
	BitsPerSample uint32
	ExpBits       uint32
}

func readBitDepth(b *bitReader) (BitDepth, error) {
	var bd BitDepth
	bd.FloatingPoint = b.ReadBool()
	if !bd.FloatingPoint {
		bd.BitsPerSample = b.ReadU32(u32Val(8), u32Val(10), u32Val(12), u32Off(6, 1))
		if bd.BitsPerSample > 31 {
			return bd, errors.New("gojxl: invalid bits_per_sample")
		}
	} else {
		bd.BitsPerSample = b.ReadU32(u32Val(32), u32Val(16), u32Val(24), u32Off(6, 1))
		bd.ExpBits = b.ReadBits(4) + 1
		if bd.ExpBits < 2 || bd.ExpBits > 8 {
			return bd, errors.New("gojxl: invalid exponent_bits_per_sample")
		}
	}
	return bd, nil
}

// CustomTransferFunction (color_encoding_internal.cc).
type CustomTransferFunction struct {
	HaveGamma        bool
	Gamma            uint32
	TransferFunction uint32
}

// ColorEncoding (color_encoding_internal.cc ColorEncoding::VisitFields).
type ColorEncoding struct {
	WantICC         bool
	ColorSpace      uint32
	WhitePoint      uint32
	Primaries       uint32
	TF              CustomTransferFunction
	RenderingIntent uint32
}

func (c *ColorEncoding) hasPrimaries() bool {
	return c.ColorSpace != csGray && c.ColorSpace != csXYB
}
func (c *ColorEncoding) implicitWhitePoint() bool { return c.ColorSpace == csXYB }

// Channels returns the number of color channels (1 for gray, else 3).
func (c *ColorEncoding) Channels() int {
	if c.ColorSpace == csGray {
		return 1
	}
	return 3
}

func defaultColorEncoding() ColorEncoding {
	return ColorEncoding{
		ColorSpace:      csRGB,
		WhitePoint:      1,                                            // kD65
		Primaries:       1,                                            // kSRGB
		TF:              CustomTransferFunction{TransferFunction: 13}, // kSRGB
		RenderingIntent: 1,                                            // kRelative
	}
}

// readCustomxy consumes one (x, y) chromaticity pair (Customxy::VisitFields).
func readCustomxy(b *bitReader) {
	b.ReadU32(u32Bits(19), u32Off(19, 524288), u32Off(20, 1048576), u32Off(21, 2097152))
	b.ReadU32(u32Bits(19), u32Off(19, 524288), u32Off(20, 1048576), u32Off(21, 2097152))
}

func readColorEncoding(b *bitReader) (ColorEncoding, error) {
	if b.ReadBool() { // all_default
		return defaultColorEncoding(), nil
	}
	var c ColorEncoding
	c.WantICC = b.ReadBool()
	c.ColorSpace = b.ReadEnum()

	if !c.WantICC {
		if !c.implicitWhitePoint() {
			c.WhitePoint = b.ReadEnum()
			if c.WhitePoint == 2 { // kCustom
				readCustomxy(b)
			}
		} else {
			c.WhitePoint = 1 // kD65 (set implicitly)
		}
		if c.hasPrimaries() {
			c.Primaries = b.ReadEnum()
			if c.Primaries == 2 { // kCustom
				readCustomxy(b) // red
				readCustomxy(b) // green
				readCustomxy(b) // blue
			}
		}
		// CustomTransferFunction: SetImplicit() is true only for XYB.
		if c.ColorSpace != csXYB {
			c.TF.HaveGamma = b.ReadBool()
			if c.TF.HaveGamma {
				c.TF.Gamma = b.ReadBits(24)
			} else {
				c.TF.TransferFunction = b.ReadEnum()
			}
		}
		c.RenderingIntent = b.ReadEnum()
	}
	if !b.ok() {
		return c, errTruncated
	}
	return c, nil
}

// ExtraChannelInfo (image_metadata.cc ExtraChannelInfo::VisitFields).
type ExtraChannelInfo struct {
	Type            uint32
	BitDepth        BitDepth
	DimShift        uint32
	Name            string
	AlphaAssociated bool
}

func visitNameString(b *bitReader) string {
	n := b.ReadU32(u32Val(0), u32Bits(4), u32Off(5, 16), u32Off(10, 48))
	if n == 0 {
		return ""
	}
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte(b.ReadBits(8))
	}
	return string(buf)
}

func readExtraChannelInfo(b *bitReader) (ExtraChannelInfo, error) {
	if b.ReadBool() { // all_default
		return ExtraChannelInfo{Type: ecAlpha, BitDepth: BitDepth{BitsPerSample: 8}}, nil
	}
	var e ExtraChannelInfo
	e.Type = b.ReadEnum()
	bd, err := readBitDepth(b)
	if err != nil {
		return e, err
	}
	e.BitDepth = bd
	e.DimShift = b.ReadU32(u32Val(0), u32Val(3), u32Val(4), u32Off(3, 1))
	e.Name = visitNameString(b)
	switch e.Type {
	case ecAlpha:
		e.AlphaAssociated = b.ReadBool()
	case ecSpotColor:
		for i := 0; i < 4; i++ {
			b.ReadF16()
		}
	case ecCFA:
		b.ReadU32(u32Val(1), u32Bits(2), u32Off(4, 3), u32Off(8, 19))
	}
	if !b.ok() {
		return e, errTruncated
	}
	return e, nil
}

// ToneMapping (image_metadata.cc) — consumed for stream position only.
func readToneMapping(b *bitReader) {
	if b.ReadBool() { // all_default
		return
	}
	b.ReadF16()  // intensity_target
	b.ReadF16()  // min_nits
	b.ReadBool() // relative_to_max_display
	b.ReadF16()  // linear_below
}

// previewHeader / animationHeader consume the nested headers (rare in DICOM).
func readPreviewHeader(b *bitReader) {
	div8 := b.ReadBool()
	if div8 {
		b.ReadU32(u32Val(16), u32Val(32), u32Off(5, 1), u32Off(9, 33))
	} else {
		b.ReadU32(u32Off(6, 1), u32Off(8, 65), u32Off(10, 321), u32Off(12, 1345))
	}
	ratio := b.ReadBits(3)
	if ratio == 0 {
		if div8 {
			b.ReadU32(u32Val(16), u32Val(32), u32Off(5, 1), u32Off(9, 33))
		} else {
			b.ReadU32(u32Off(6, 1), u32Off(8, 65), u32Off(10, 321), u32Off(12, 1345))
		}
	}
}

func readAnimationHeader(b *bitReader) {
	b.ReadU32(u32Val(100), u32Val(1000), u32Off(10, 1), u32Off(30, 1)) // tps_numerator
	b.ReadU32(u32Val(1), u32Val(1001), u32Off(8, 1), u32Off(10, 1))    // tps_denominator
	b.ReadU32(u32Val(0), u32Bits(3), u32Bits(16), u32Bits(32))         // num_loops
	b.ReadBool()                                                       // have_timecodes
}

// skipExtensions consumes a BeginExtensions/EndExtensions block (fields.cc).
func skipExtensions(b *bitReader) error {
	extensions := b.ReadU64()
	if extensions == 0 {
		return nil
	}
	var total uint64
	for rem := extensions; rem != 0; rem &= rem - 1 {
		total += b.ReadU64()
	}
	for i := uint64(0); i < total; i++ {
		b.ReadBits(1)
	}
	if !b.ok() {
		return errTruncated
	}
	return nil
}

// readOpsinInverseMatrix consumes an OpsinInverseMatrix bundle (9+3+4 F16s).
func readOpsinInverseMatrix(b *bitReader) {
	if b.ReadBool() { // all_default
		return
	}
	for i := 0; i < 16; i++ {
		b.ReadF16()
	}
}

// readTransformData consumes the CustomTransformData bundle that follows
// ImageMetadata in the codestream (image_metadata.cc).
func readTransformData(b *bitReader, xybEncoded bool) {
	if b.ReadBool() { // all_default
		return
	}
	if xybEncoded {
		readOpsinInverseMatrix(b)
	}
	mask := b.ReadBits(3)
	if mask&0x1 != 0 {
		for i := 0; i < 15; i++ {
			b.ReadF16()
		}
	}
	if mask&0x2 != 0 {
		for i := 0; i < 55; i++ {
			b.ReadF16()
		}
	}
	if mask&0x4 != 0 {
		for i := 0; i < 210; i++ {
			b.ReadF16()
		}
	}
}

// ImageMetadata (image_metadata.cc ImageMetadata::VisitFields).
type ImageMetadata struct {
	Orientation      uint32
	BitDepth         BitDepth
	Modular16bit     bool
	NumExtraChannels uint32
	ExtraChannels    []ExtraChannelInfo
	XYBEncoded       bool
	Color            ColorEncoding
	HavePreview      bool
	HaveAnimation    bool
	HaveIntrinsic    bool
}

func readImageMetadata(b *bitReader) (ImageMetadata, error) {
	var m ImageMetadata
	if b.ReadBool() { // all_default
		m.Orientation = 1
		m.BitDepth = BitDepth{BitsPerSample: 8}
		m.Modular16bit = true
		m.XYBEncoded = true
		m.Color = defaultColorEncoding()
		return m, nil
	}

	extraFields := b.ReadBool()
	if extraFields {
		m.Orientation = b.ReadBits(3) + 1
		m.HaveIntrinsic = b.ReadBool()
		if m.HaveIntrinsic {
			if _, err := readSizeHeader(b); err != nil {
				return m, err
			}
		}
		m.HavePreview = b.ReadBool()
		if m.HavePreview {
			readPreviewHeader(b)
		}
		m.HaveAnimation = b.ReadBool()
		if m.HaveAnimation {
			readAnimationHeader(b)
		}
	} else {
		m.Orientation = 1
	}

	bd, err := readBitDepth(b)
	if err != nil {
		return m, err
	}
	m.BitDepth = bd
	m.Modular16bit = b.ReadBool()
	m.NumExtraChannels = b.ReadU32(u32Val(0), u32Val(1), u32Off(4, 2), u32Off(12, 1))
	for i := uint32(0); i < m.NumExtraChannels; i++ {
		eci, err := readExtraChannelInfo(b)
		if err != nil {
			return m, err
		}
		m.ExtraChannels = append(m.ExtraChannels, eci)
	}

	m.XYBEncoded = b.ReadBool()
	c, err := readColorEncoding(b)
	if err != nil {
		return m, err
	}
	m.Color = c
	if extraFields {
		readToneMapping(b)
	}
	if err := skipExtensions(b); err != nil {
		return m, err
	}
	if !b.ok() {
		return m, errTruncated
	}
	return m, nil
}
