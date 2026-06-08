package gojxl

import (
	"math/rand"
	"testing"
)

// TestEncodeDecodeRoundTrip verifies the pure-Go lossless encoder is the exact
// inverse of the decoder across sizes, channel counts and bit depths. (The
// emitted streams are additionally validated byte-exact against djxl out of
// band — see the encoder docs.)
func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []struct{ w, h, nc, bd int }{
		{1, 1, 1, 8}, {8, 4, 1, 8}, {16, 16, 1, 8}, {17, 37, 1, 8},
		{8, 8, 3, 8}, {10, 7, 1, 16}, {32, 32, 3, 8}, {64, 48, 3, 16},
		{100, 100, 1, 8}, {300, 200, 3, 8}, {1024, 3, 1, 8},
	}
	for _, c := range cases {
		rng := rand.New(rand.NewSource(int64(c.w*131 + c.h*17 + c.nc*7 + c.bd)))
		bps := 1
		if c.bd > 8 {
			bps = 2
		}
		pix := make([]byte, c.w*c.h*c.nc*bps)
		maxv := 1 << uint(c.bd)
		for i := 0; i < c.w*c.h*c.nc; i++ {
			v := rng.Intn(maxv)
			if bps == 1 {
				pix[i] = byte(v)
			} else {
				pix[i*2] = byte(v)
				pix[i*2+1] = byte(v >> 8)
			}
		}
		data, err := Encode(pix, c.w, c.h, c.nc, c.bd)
		if err != nil {
			t.Errorf("%+v encode: %v", c, err)
			continue
		}
		img, err := Decode(data)
		if err != nil {
			t.Errorf("%+v decode: %v", c, err)
			continue
		}
		if img.W != c.w || img.H != c.h || img.Channels != c.nc || img.BitDepth != c.bd {
			t.Errorf("%+v: geometry got %dx%d ch=%d bd=%d", c, img.W, img.H, img.Channels, img.BitDepth)
			continue
		}
		for i := range pix {
			if img.Pixels[i] != pix[i] {
				t.Errorf("%+v: byte %d differs: got %d want %d", c, i, img.Pixels[i], pix[i])
				break
			}
		}
	}
}
