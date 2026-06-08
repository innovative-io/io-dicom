package gojxl

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDecodeInterleaved checks the exported Decode produces correctly
// interleaved samples (channel-interleaved, LE for >8-bit).
func TestDecodeInterleaved(t *testing.T) {
	t.Run("rgb8", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join("testdata", "rgb8_8x4_lossless.jxl"))
		if err != nil {
			t.Skipf("fixture unavailable: %v", err)
		}
		img, err := Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		if img.W != 8 || img.H != 4 || img.Channels != 3 || img.BitDepth != 8 {
			t.Fatalf("geometry: %dx%d ch=%d bd=%d", img.W, img.H, img.Channels, img.BitDepth)
		}
		if len(img.Pixels) != 8*4*3 {
			t.Fatalf("pixel bytes: %d", len(img.Pixels))
		}
		for i := 0; i < 32; i++ {
			for c := 0; c < 3; c++ {
				want := byte((i*3 + c*40) & 0xFF)
				if got := img.Pixels[i*3+c]; got != want {
					t.Fatalf("px %d ch %d: got %d want %d", i, c, got, want)
				}
			}
		}
	})

	t.Run("gray16", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join("testdata", "gray16_8x4_lossless.jxl"))
		if err != nil {
			t.Skipf("fixture unavailable: %v", err)
		}
		img, err := Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		if img.BitDepth != 16 || len(img.Pixels) != 8*4*2 {
			t.Fatalf("16-bit: bd=%d bytes=%d", img.BitDepth, len(img.Pixels))
		}
		for i := 0; i < 32; i++ {
			want := uint16((i * 1000) & 0xFFFF)
			got := uint16(img.Pixels[i*2]) | uint16(img.Pixels[i*2+1])<<8 // little-endian
			if got != want {
				t.Fatalf("px %d: got %d want %d", i, got, want)
			}
		}
	})
}
