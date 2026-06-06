package gojxl

import (
	_ "embed"
	"testing"
)

//go:embed testdata/gray8_8x4_lossless.jxl
var gray8x4 []byte

func TestHeaderFixture(t *testing.T) {
	h, err := ReadHeader(gray8x4)
	if err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	if h.Size.Xsize != 8 || h.Size.Ysize != 4 {
		t.Fatalf("size: got %dx%d, want 8x4", h.Size.Xsize, h.Size.Ysize)
	}
	if h.Meta.BitDepth.BitsPerSample != 8 || h.Meta.BitDepth.FloatingPoint {
		t.Fatalf("bit depth: got %d (fp=%v), want 8 int", h.Meta.BitDepth.BitsPerSample, h.Meta.BitDepth.FloatingPoint)
	}
	if ch := h.Meta.Color.Channels(); ch != 1 {
		t.Fatalf("channels: got %d, want 1 (grayscale)", ch)
	}
	if h.Meta.NumExtraChannels != 0 {
		t.Fatalf("extra channels: got %d, want 0", h.Meta.NumExtraChannels)
	}
	if h.Meta.Color.ColorSpace != csGray {
		t.Fatalf("color space: got %d, want kGray(%d)", h.Meta.Color.ColorSpace, csGray)
	}
}
