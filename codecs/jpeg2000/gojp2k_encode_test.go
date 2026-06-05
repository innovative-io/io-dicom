package jpeg2000

import (
	"math/rand"
	"testing"
)

// TestEncodeRoundTripSelf encodes raw images and decodes them with the pure-Go
// decoder, requiring exact recovery (lossless 5/3).
func TestEncodeRoundTripSelf(t *testing.T) {
	cases := []struct {
		w, h, nc, prec int
	}{
		{16, 16, 1, 8}, {17, 37, 1, 8}, {64, 64, 1, 8}, {100, 70, 1, 16},
		{33, 33, 3, 8}, {7, 100, 1, 8}, {128, 128, 1, 12},
	}
	for _, c := range cases {
		bps := 1
		if c.prec > 8 {
			bps = 2
		}
		raw := make([]byte, c.w*c.h*c.nc*bps)
		r := rand.New(rand.NewSource(int64(c.w*7 + c.h)))
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
		stream, err := goJ2Kencode(raw, c.w, c.h, c.nc, c.prec)
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
		t.Logf("%+v: round-trip OK (%d raw → %d coded)", c, len(raw), len(stream))
	}
}

// TestEncodeInputValidation checks that goJ2Kencode rejects malformed parameters
// and non-exact payload sizes instead of silently accepting them.
func TestEncodeInputValidation(t *testing.T) {
	good := make([]byte, 4*4*1*1) // 4×4, 1 comp, 8-bit
	cases := []struct {
		name           string
		raw            []byte
		w, h, nc, prec int
	}{
		{"ok-exact", good, 4, 4, 1, 8},
		{"too-few-bytes", good[:len(good)-1], 4, 4, 1, 8},
		{"too-many-bytes", append(append([]byte{}, good...), 0), 4, 4, 1, 8},
		{"zero-width", good, 0, 4, 1, 8},
		{"zero-prec", good, 4, 4, 1, 0},
		{"prec-too-large", good, 4, 4, 1, 17},
		{"too-many-components", good, 4, 4, maxJ2KComponents + 1, 8},
		{"huge-height-overflow", good, 4, 1 << 40, 1, 8},
		{"huge-width-overflow", good, 1 << 40, 4, 1, 8},
	}
	for _, c := range cases {
		_, err := goJ2Kencode(c.raw, c.w, c.h, c.nc, c.prec)
		if c.name == "ok-exact" {
			if err != nil {
				t.Errorf("%s: unexpected error: %v", c.name, err)
			}
		} else if err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}
}
