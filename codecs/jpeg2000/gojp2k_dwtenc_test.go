package jpeg2000

import (
	"math/rand"
	"testing"
)

// TestFDWT53RoundTrip checks fdwt53 is the exact inverse of idwt53 across various
// sizes and decomposition levels (5/3 is integer-reversible).
func TestFDWT53RoundTrip(t *testing.T) {
	sizes := [][2]int{{2, 3}, {17, 37}, {64, 64}, {100, 7}, {7, 100}, {256, 256}, {130, 77}, {33, 33}}
	// maxNL caps the decomposition so no dimension is decomposed below 1 sample
	// (degenerate; a real encoder caps NL to the image size the same way).
	maxNL := func(w, h int) int {
		m := w
		if h < m {
			m = h
		}
		nl := 0
		for m > 1 && nl < 6 {
			m = (m + 1) / 2
			nl++
		}
		return nl
	}
	for _, sz := range sizes {
		for _, NL := range []int{1, 3, 5} {
			if NL > maxNL(sz[0], sz[1]) {
				continue
			}
			w, h := sz[0], sz[1]
			cs := &j2kCodestream{
				xsiz: w, ysiz: h, xtsiz: w, ytsiz: h,
				comps: []componentInfo{{precision: 8, dx: 1, dy: 1}},
				cod:   codingStyle{decompLevels: NL, cbW: 64, cbH: 64, transform: 1},
			}
			cs.cod.precinctW = make([]int, NL+1)
			cs.cod.precinctH = make([]int, NL+1)
			for i := range cs.cod.precinctW {
				cs.cod.precinctW[i] = 1 << 15
				cs.cod.precinctH[i] = 1 << 15
			}
			tc, err := cs.buildTileComponent(0, 0)
			if err != nil {
				t.Fatalf("NL=%d %dx%d build: %v", NL, w, h, err)
			}
			r := rand.New(rand.NewSource(int64(w*1000 + h + NL)))
			orig := make([]int32, w*h)
			for i := range orig {
				orig[i] = int32(r.Intn(512) - 256)
			}
			band := fdwt53(tc, orig)
			back := idwt53(tc, band)
			for i := range orig {
				if back[i] != orig[i] {
					t.Fatalf("NL=%d %dx%d: mismatch at %d: got %d want %d", NL, w, h, i, back[i], orig[i])
				}
			}
		}
	}
}
