package gojxl

import "errors"

var errInvalidCfL = errors.New("gojxl: invalid color-correlation parameters")

// Chroma-from-luma (CfL) for VarDCT, ported from lib/jxl/chroma_from_luma.h and
// the apply in dec_group.cc DequantLane. The X and B (chroma) coefficients are
// predicted from the Y (luma) coefficient:
//
//	x' = YtoXRatio(xFactor) * y + x
//	b' = YtoBRatio(bFactor) * y + b
//
// where xFactor/bFactor are per-64x64-color-tile values (decoded separately) and
// the ratios are derived from the frame-global correlation parameters below.

const (
	kColorTileDim          = 64
	kColorTileDimInBlocks  = kColorTileDim / acBlockDim // 8
	kDefaultColorFactor    = 84
	kCFLFixedPointPrec     = 11
	kYToBRatio             = 1.0 // cms::kYToBRatio
)

// colorCorrelation holds the frame-global CfL parameters (ColorCorrelationMap).
type colorCorrelation struct {
	colorFactor uint32
	colorScale  float32
	baseX       float32
	baseB       float32
	ytoxDC      int32
	ytobDC      int32
}

func defaultColorCorrelation() colorCorrelation {
	return colorCorrelation{
		colorFactor: kDefaultColorFactor,
		colorScale:  1.0 / float32(kDefaultColorFactor),
		baseX:       0.0,
		baseB:       kYToBRatio,
		ytoxDC:      0,
		ytobDC:      0,
	}
}

func (c *colorCorrelation) ytoXRatio(xFactor int32) float32 {
	return c.baseX + float32(xFactor)*c.colorScale
}

func (c *colorCorrelation) ytoBRatio(bFactor int32) float32 {
	return c.baseB + float32(bFactor)*c.colorScale
}

// applyCfL maps dequantized (x, y, b) coefficients with the given per-tile
// factors to the color-correlated (x', b') outputs (Y is unchanged).
func (c *colorCorrelation) applyCfL(x, y, b float32, xFactor, bFactor int32) (float32, float32) {
	xOut := c.ytoXRatio(xFactor)*y + x
	bOut := c.ytoBRatio(bFactor)*y + b
	return xOut, bOut
}

// kColorFactorDist = U32Enc(Val(84), Val(256), BitsOffset(8,2), BitsOffset(16,258)).
var kColorFactorDist = [4]u32d{u32Val(kDefaultColorFactor), u32Val(256), u32Off(8, 2), u32Off(16, 258)}

// decodeCfLDC reads the frame-global CfL parameters (ColorCorrelationMap::DecodeDC).
// The per-tile ytox/ytob maps are coded elsewhere (the cmap modular stream).
func decodeCfLDC(b *bitReader) (colorCorrelation, error) {
	c := defaultColorCorrelation()
	if b.ReadBits(1) == 1 {
		return c, nil // all default
	}
	c.colorFactor = b.ReadU32(kColorFactorDist[0], kColorFactorDist[1], kColorFactorDist[2], kColorFactorDist[3])
	if c.colorFactor == 0 {
		return c, errInvalidCfL
	}
	c.colorScale = 1.0 / float32(c.colorFactor)
	bx, err := b.ReadF16()
	if err != nil {
		return c, err
	}
	if bx < -4.0 || bx > 4.0 {
		return c, errInvalidCfL
	}
	c.baseX = bx
	bb, err := b.ReadF16()
	if err != nil {
		return c, err
	}
	if bb < -4.0 || bb > 4.0 {
		return c, errInvalidCfL
	}
	c.baseB = bb
	c.ytoxDC = int32(b.ReadBits(8)) - 128 // + INT8_MIN
	c.ytobDC = int32(b.ReadBits(8)) - 128
	return c, nil
}
