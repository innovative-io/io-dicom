//go:build openjpeg && cgo

package jpeg2000

import (
	"math/rand"
	"testing"
)

// TestEncodeDecodedByOpenJPEG proves the pure-Go encoder emits spec-compliant
// codestreams: encode with pure-Go, decode with openjpeg, require exact recovery.
func TestEncodeDecodedByOpenJPEG(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })
	if err := UseBackend("openjpeg"); err != nil {
		t.Fatalf("openjpeg backend: %v", err)
	}
	cases := []struct{ w, h, nc, prec int }{
		{16, 16, 1, 8}, {17, 37, 1, 8}, {64, 64, 1, 8},
		{100, 70, 1, 16}, {33, 33, 3, 8}, {128, 128, 1, 12},
	}
	for _, c := range cases {
		bps := 1
		if c.prec > 8 {
			bps = 2
		}
		raw := make([]byte, c.w*c.h*c.nc*bps)
		r := rand.New(rand.NewSource(int64(c.w*13 + c.h + c.nc)))
		// Use a smoother signal (gradient + noise) so it's image-like.
		for y := 0; y < c.h; y++ {
			for x := 0; x < c.w; x++ {
				for k := 0; k < c.nc; k++ {
					v := (x + y + r.Intn(8)) & ((1 << uint(c.prec)) - 1)
					i := (y*c.w+x)*c.nc + k
					if bps == 1 {
						raw[i] = byte(v)
					} else {
						raw[i*2] = byte(v)
						raw[i*2+1] = byte(v >> 8)
					}
				}
			}
		}
		stream, err := goJ2Kencode(raw, c.w, c.h, c.nc, c.prec)
		if err != nil {
			t.Fatalf("%+v encode: %v", c, err)
		}
		out := make([]byte, len(raw))
		if err := J2Kdecode(stream, uint32(len(stream)), out); err != nil {
			t.Fatalf("%+v openjpeg decode: %v", c, err)
		}
		for i := range raw {
			if raw[i] != out[i] {
				t.Fatalf("%+v: byte %d: openjpeg got %d want %d", c, i, out[i], raw[i])
			}
		}
		t.Logf("%+v: openjpeg round-trip OK (%d coded)", c, len(stream))
	}
}
