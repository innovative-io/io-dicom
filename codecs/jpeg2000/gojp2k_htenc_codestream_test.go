package jpeg2000

import (
	"math/rand"
	"testing"
)

// TestHTEncodeRoundTripSelf encodes raw images with the pure-Go HTJ2K encoder
// and decodes them with the pure-Go decoder, requiring exact recovery
// (lossless reversible 5/3 + HT Cleanup pass).
func TestHTEncodeRoundTripSelf(t *testing.T) {
	cases := []struct {
		w, h, nc, prec int
	}{
		{16, 16, 1, 8}, {17, 37, 1, 8}, {64, 64, 1, 8}, {100, 70, 1, 16},
		{33, 33, 3, 8}, {7, 100, 1, 8}, {128, 128, 1, 12},
		{1, 1, 1, 8}, {3, 5, 1, 8}, {200, 13, 1, 10}, {64, 200, 3, 8},
	}
	for _, c := range cases {
		bps := 1
		if c.prec > 8 {
			bps = 2
		}
		raw := make([]byte, c.w*c.h*c.nc*bps)
		r := rand.New(rand.NewSource(int64(c.w*7 + c.h*3 + c.nc)))
		maxv := 1 << uint(c.prec)
		for i := 0; i < c.w*c.h*c.nc; i++ {
			v := r.Intn(maxv)
			if bps == 1 {
				raw[i] = byte(v)
			} else {
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
		}
		stream, err := goHTJ2Kencode(raw, c.w, c.h, c.nc, c.prec)
		if err != nil {
			t.Fatalf("%+v encode: %v", c, err)
		}
		out := make([]byte, len(raw))
		if err := goJ2Kdecode(stream, out); err != nil {
			t.Fatalf("%+v decode: %v (stream %d bytes)", c, err, len(stream))
		}
		for i := range raw {
			if raw[i] != out[i] {
				t.Fatalf("%+v: byte %d differs: got %d want %d", c, i, out[i], raw[i])
			}
		}
		t.Logf("%+v: HT round-trip OK (%d raw → %d coded)", c, len(raw), len(stream))
	}
}

// TestHTEncodeGradient exercises smooth + structured content (not just noise),
// which stresses zero/insignificant blocks and small-magnitude subbands.
func TestHTEncodeGradient(t *testing.T) {
	w, h := 96, 72
	raw := make([]byte, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			raw[y*w+x] = byte((x*3 + y*2) & 0xFF)
		}
	}
	stream, err := goHTJ2Kencode(raw, w, h, 1, 8)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out := make([]byte, len(raw))
	if err := goJ2Kdecode(stream, out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := range raw {
		if raw[i] != out[i] {
			t.Fatalf("byte %d: got %d want %d", i, out[i], raw[i])
		}
	}
}
