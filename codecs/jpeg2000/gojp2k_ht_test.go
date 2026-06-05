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

// TestGoHTLossyDeferred confirms that lossy (irreversible 9/7) HT code-blocks are
// not yet handled by the pure-Go path and are reported as unsupported so the
// caller can fall back to the native backend, rather than mis-decoding.
func TestGoHTLossyDeferred(t *testing.T) {
	frame, err := os.ReadFile("../../testdata/htj2k-lossy-gray.j2c")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	out := make([]byte, 256*256)
	if err := goJ2Kdecode(frame, out); err == nil {
		t.Fatal("expected unsupported error for lossy HT in pure-Go path")
	}
}
