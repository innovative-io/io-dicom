package jpeg2000

import (
	"math/rand"
	"testing"
)

// canonicalize encodes coeffs, decodes them back, and returns the decoded
// (canonical, reconstruction-midpoint) code-block plus the encoded bytes.
func canonicalize(t *testing.T, coeffs []uint32, missingMSBs, w, h int) ([]uint32, []byte) {
	t.Helper()
	enc := encodeHTCleanup(coeffs, missingMSBs, w, h, w)
	dec, ok := decodeHTBlock(enc, len(enc), 0, 1, missingMSBs, w, h, false)
	if !ok {
		t.Fatalf("decode failed for %dx%d", w, h)
	}
	return dec, enc
}

// TestHTEncodeRoundTrip verifies the HT Cleanup encoder is the exact inverse of
// decodeHTBlock: encoding a code-block then decoding it yields a canonical form
// that is a fixed point (re-encoding reproduces identical bytes, re-decoding
// reproduces identical coefficients).
func TestHTEncodeRoundTrip(t *testing.T) {
	sizes := []struct{ w, h int }{
		{4, 4}, {8, 8}, {16, 16}, {32, 32}, {64, 64},
		{1, 1}, {3, 5}, {7, 1}, {17, 37}, {31, 33}, {5, 9}, {2, 2}, {64, 4}, {4, 64},
	}
	for _, mmsb := range []int{0, 1, 3} {
		for _, sz := range sizes {
			for seed := int64(0); seed < 8; seed++ {
				rng := rand.New(rand.NewSource(seed*1000 + int64(sz.w*131+sz.h*7+mmsb)))
				n := sz.w * sz.h
				coeffs := make([]uint32, n)
				for i := range coeffs {
					if rng.Intn(3) == 0 { // ~1/3 significant
						mag := uint32(rng.Intn(1 << uint(6))) // up to 6-bit magnitudes
						sign := uint32(rng.Intn(2)) << 31
						coeffs[i] = sign | mag
					}
				}

				c1, b1 := canonicalize(t, coeffs, mmsb, sz.w, sz.h)
				// Fixed point: encode the canonical form, must reproduce bytes.
				b2 := encodeHTCleanup(c1, mmsb, sz.w, sz.h, sz.w)
				if len(b1) != len(b2) {
					t.Fatalf("mmsb=%d %dx%d seed=%d: re-encode length %d != %d",
						mmsb, sz.w, sz.h, seed, len(b2), len(b1))
				}
				for i := range b1 {
					if b1[i] != b2[i] {
						t.Fatalf("mmsb=%d %dx%d seed=%d: re-encode byte %d: %02x != %02x",
							mmsb, sz.w, sz.h, seed, i, b2[i], b1[i])
					}
				}
				// Re-decode must reproduce the canonical coefficients.
				c2, ok := decodeHTBlock(b2, len(b2), 0, 1, mmsb, sz.w, sz.h, false)
				if !ok {
					t.Fatalf("mmsb=%d %dx%d seed=%d: re-decode failed", mmsb, sz.w, sz.h, seed)
				}
				stride := sz.w
				for y := 0; y < sz.h; y++ {
					for x := 0; x < sz.w; x++ {
						i := y*stride + x
						if c1[i] != c2[i] {
							t.Fatalf("mmsb=%d %dx%d seed=%d: coeff (%d,%d): %08x != %08x",
								mmsb, sz.w, sz.h, seed, x, y, c2[i], c1[i])
						}
					}
				}
			}
		}
	}
}
