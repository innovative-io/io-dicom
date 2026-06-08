package gojxl

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHeaderTable validates SizeHeader + ImageMetadata parsing against a set of
// cjxl-encoded reference files with known properties (raw codestream and
// ISO-BMFF container, grayscale/RGB/RGBA, 8- and 16-bit, small and large).
func TestHeaderTable(t *testing.T) {
	cases := []struct {
		file       string
		w, h       uint32
		bits       uint32
		channels   int
		extra      uint32
		colorSpace uint32
	}{
		{"gray8_8x4_lossless.jxl", 8, 4, 8, 1, 0, csGray},
		{"gray16_8x4_lossless.jxl", 8, 4, 16, 1, 0, csGray},
		{"rgb8_8x4_lossless.jxl", 8, 4, 8, 3, 0, csRGB},
		{"rgb16_6x6_lossless.jxl", 6, 6, 16, 3, 0, csRGB},
		{"rgba8_8x4_lossless.jxl", 8, 4, 8, 3, 1, csRGB},
		{"gray8_300x200_lossless.jxl", 300, 200, 8, 1, 0, csGray},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", c.file))
			if err != nil {
				t.Skipf("fixture unavailable: %v", err)
			}
			h, err := ReadHeader(data)
			if err != nil {
				t.Fatalf("ReadHeader: %v", err)
			}
			if h.Size.Xsize != c.w || h.Size.Ysize != c.h {
				t.Errorf("size: got %dx%d want %dx%d", h.Size.Xsize, h.Size.Ysize, c.w, c.h)
			}
			if h.Meta.BitDepth.BitsPerSample != c.bits {
				t.Errorf("bits: got %d want %d", h.Meta.BitDepth.BitsPerSample, c.bits)
			}
			if got := h.Meta.Color.Channels(); got != c.channels {
				t.Errorf("channels: got %d want %d", got, c.channels)
			}
			if h.Meta.NumExtraChannels != c.extra {
				t.Errorf("extra channels: got %d want %d", h.Meta.NumExtraChannels, c.extra)
			}
			if h.Meta.Color.ColorSpace != c.colorSpace {
				t.Errorf("color space: got %d want %d", h.Meta.Color.ColorSpace, c.colorSpace)
			}
		})
	}
}
