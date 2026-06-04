package jpeg

import (
	"bytes"
	"math/rand"
	"testing"
)

// TestGoJPEGLosslessEncodeRoundTrip encodes synthetic images with the pure-Go
// lossless encoder and decodes them back with the pure-Go decoder, requiring an
// exact match (lossless). Covers 8/12/16-bit and 1/3 components.
func TestGoJPEGLosslessEncodeRoundTrip(t *testing.T) {
	cases := []struct {
		name             string
		w, h, samples, p int
	}{
		{"gray8", 37, 19, 1, 8},
		{"gray12", 64, 48, 1, 12},
		{"gray16", 50, 50, 1, 16},
		{"rgb8", 20, 16, 3, 8},
		{"rgb16", 24, 24, 3, 16},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bps := 1
			if tc.p > 8 {
				bps = 2
			}
			maxv := (1 << tc.p) - 1
			raw := make([]byte, tc.w*tc.h*tc.samples*bps)
			r := rand.New(rand.NewSource(int64(tc.p)))
			for i := 0; i < tc.w*tc.h*tc.samples; i++ {
				// smooth gradient + a little noise, clamped to precision
				v := (i*7 + r.Intn(16)) % (maxv + 1)
				if bps == 1 {
					raw[i] = byte(v)
				} else {
					raw[i*2] = byte(v)
					raw[i*2+1] = byte(v >> 8)
				}
			}

			enc, err := encodeLosslessJPEG(raw, tc.w, tc.h, tc.samples, tc.p)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			// Decode geometry sanity.
			f, samples, derr := decodeLossless(enc)
			if derr != nil {
				t.Fatalf("decode: %v", derr)
			}
			if f.width != tc.w || f.height != tc.h || len(f.comps) != tc.samples || f.precision != tc.p {
				t.Fatalf("geometry mismatch: %dx%d c%d p%d", f.width, f.height, len(f.comps), f.precision)
			}

			out := make([]byte, len(raw))
			if err := decodeLosslessInto(enc, out); err != nil {
				t.Fatalf("decodeInto: %v", err)
			}
			if !bytes.Equal(out, raw) {
				t.Fatalf("round trip not lossless (%d samples)", len(samples))
			}
		})
	}
}
