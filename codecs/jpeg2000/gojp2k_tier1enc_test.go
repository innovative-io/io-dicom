package jpeg2000

import (
	"math/rand"
	"testing"
)

// TestTier1RoundTrip encodes random code-block coefficients and verifies
// decodeCodeBlock recovers them exactly (lossless EBCOT round-trip).
func TestTier1RoundTrip(t *testing.T) {
	sizes := [][2]int{{4, 4}, {8, 8}, {15, 9}, {32, 32}, {64, 64}, {3, 7}, {1, 5}}
	orients := []int{bandLL, bandHL, bandLH, bandHH}
	for seed := int64(0); seed < 60; seed++ {
		r := rand.New(rand.NewSource(seed))
		sz := sizes[r.Intn(len(sizes))]
		w, h := sz[0], sz[1]
		orient := orients[r.Intn(len(orients))]
		mb := 8 + r.Intn(8) // 8..15 magnitude bit-planes
		maxMag := int32(1) << uint(mb-1)
		coeffs := make([]int32, w*h)
		// sparsity varies so we exercise run-length and dense cases
		density := r.Float64()
		for i := range coeffs {
			if r.Float64() < density {
				v := int32(r.Intn(int(maxMag)))
				if r.Intn(2) == 0 {
					v = -v
				}
				coeffs[i] = v
			}
		}
		data, npasses, nz := encodeCodeBlock(coeffs, w, h, orient, mb)
		cb := &codeBlock{x1: w, y1: h, npasses: npasses, nzeroBP: nz}
		if npasses > 0 {
			cb.segs = [][]byte{data}
		}
		got, _ := decodeCodeBlock(cb, orient, mb)
		for i := range coeffs {
			if got[i] != coeffs[i] {
				t.Fatalf("seed=%d %dx%d orient=%d mb=%d: coeff[%d]=%d, decoded=%d (npasses=%d nz=%d bytes=%d)",
					seed, w, h, orient, mb, i, coeffs[i], got[i], npasses, nz, len(data))
			}
		}
	}
}
