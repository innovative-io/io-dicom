package jpeg2000

import (
	"os"
	"testing"
)

// TestGoHTReversibleGolden decodes pure-Go HTJ2K (ISO 15444-15) reversible
// codestreams and requires a byte-exact match against the reference raster that
// openjph's ojph_expand produced. The .raw fixtures are little-endian samples
// (1 byte for 8-bit, 2 bytes for 16-bit) matching this codec's output
// convention. Fixtures were generated with:
//
//	ojph_compress -i img.pgm -o out.j2c -reversible true -prog_order LRCP
//	ojph_expand   -i out.j2c -o ref.pgm   # → ref.raw (pixels only)
func TestGoHTReversibleGolden(t *testing.T) {
	cases := []struct {
		name string
		w, h int
	}{
		{"reversible-gray", 256, 256},   // 8-bit, full 64×64 blocks
		{"reversible-gray16", 200, 140}, // 16-bit, partial blocks
		{"reversible-odd", 130, 77},     // 8-bit, narrow/short edge blocks
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			frame, err := os.ReadFile("../../testdata/htj2k-" + c.name + ".j2c")
			if err != nil {
				t.Skipf("fixture unavailable: %v", err)
			}
			want, err := os.ReadFile("../../testdata/htj2k-" + c.name + ".raw")
			if err != nil {
				t.Skipf("golden unavailable: %v", err)
			}
			out := make([]byte, len(want))
			if err := goJ2Kdecode(frame, out); err != nil {
				t.Fatalf("pure-Go HT decode: %v", err)
			}
			nd, first := 0, -1
			for i := range want {
				if want[i] != out[i] {
					if first < 0 {
						first = i
					}
					nd++
				}
			}
			if nd != 0 {
				t.Fatalf("%s: %d/%d bytes differ (first @%d: got %d want %d)",
					c.name, nd, len(want), first, out[first], want[first])
			}
			t.Logf("%s: byte-exact (%d bytes)", c.name, len(want))
		})
	}
}

// TestGoHTLossyGolden decodes pure-Go HTJ2K lossy (irreversible 9/7) codestreams
// and compares against openjph's ojph_expand output within a small tolerance
// (the 9/7 inverse transform is floating-point, so it matches to ±1 rather than
// bit-exactly).
func TestGoHTLossyGolden(t *testing.T) {
	cases := []struct {
		name           string
		bps            int
		maxAbs, maxOut int
	}{
		{"lossy-gray", 1, 1, 0},   // 8-bit
		{"lossy-gray16", 2, 1, 0}, // 16-bit
		{"lossy-odd", 1, 1, 0},    // 8-bit, odd dimensions
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			frame, err := os.ReadFile("../../testdata/htj2k-" + c.name + ".j2c")
			if err != nil {
				t.Skipf("fixture unavailable: %v", err)
			}
			want, err := os.ReadFile("../../testdata/htj2k-" + c.name + ".raw")
			if err != nil {
				t.Skipf("golden unavailable: %v", err)
			}
			out := make([]byte, len(want))
			if err := goJ2Kdecode(frame, out); err != nil {
				t.Fatalf("pure-Go HT lossy decode: %v", err)
			}
			readSample := func(b []byte, i int) int {
				if c.bps == 1 {
					return int(b[i])
				}
				return int(uint16(b[i*2]) | uint16(b[i*2+1])<<8)
			}
			n := len(want) / c.bps
			maxd, sumd, outl := 0, 0, 0
			for i := 0; i < n; i++ {
				d := readSample(want, i) - readSample(out, i)
				if d < 0 {
					d = -d
				}
				sumd += d
				if d > maxd {
					maxd = d
				}
				if d > c.maxAbs {
					outl++
				}
			}
			t.Logf("%s: maxDiff=%d meanDiff=%.4f outliers(>%d)=%d/%d",
				c.name, maxd, float64(sumd)/float64(n), c.maxAbs, outl, n)
			if outl > c.maxOut {
				t.Fatalf("%s: %d samples exceed |Δ|=%d (maxDiff=%d)", c.name, outl, c.maxAbs, maxd)
			}
		})
	}
}
