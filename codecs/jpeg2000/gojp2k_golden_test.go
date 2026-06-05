package jpeg2000

import (
	"os"
	"testing"
)

// Frozen-golden decode tests. The reference rasters (golden-*.raw) were produced
// by the pure-Go decoder after it was validated byte-exact against openjpeg (see
// git history prior to the openjpeg backend removal); they now serve as
// regression references that need no native dependency. Both the codestream
// fixtures and the golden rasters live under testdata/ (gitignored), so these
// tests skip when the fixtures are absent (e.g. in CI), exactly like the other
// fixture-backed tests.
func TestGoJ2KGoldenDecode(t *testing.T) {
	cases := []struct {
		name          string
		dcm           string // DICOM-encapsulated fixture (J2K frame extracted)
		j2k           string // raw codestream fixture (used when dcm is empty)
		golden        string
		w, h, nc, bps int
	}{
		{"ct-5x3-lossless", "cornerstone-CTImage-jpeg2000-lossless.dcm", "", "golden-ct-5x3-lossless.raw", 512, 512, 1, 2},
		{"pydicom-9x7", "pydicom-JPEG2000.dcm", "", "golden-pydicom-9x7.raw", 256, 1024, 1, 2},
		{"rgb-5x3-lossless", "", "test.j2k", "golden-rgb-5x3-lossless.raw", 1576, 1134, 3, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want, err := os.ReadFile("../../testdata/" + c.golden)
			if err != nil {
				t.Skipf("golden unavailable: %v", err)
			}
			// Guard against a mis-sized golden: goJ2Kdecode only checks the output
			// buffer is large enough, so an oversized golden could mask a regression.
			if exp := c.w * c.h * c.nc * c.bps; len(want) != exp {
				t.Fatalf("golden %s size %d != expected %d (%dx%d nc=%d bps=%d)",
					c.golden, len(want), exp, c.w, c.h, c.nc, c.bps)
			}
			var frame []byte
			if c.j2k != "" {
				frame, err = os.ReadFile("../../testdata/" + c.j2k)
				if err != nil {
					t.Skipf("fixture unavailable: %v", err)
				}
			} else {
				dcm, err := os.ReadFile("../../testdata/" + c.dcm)
				if err != nil {
					t.Skipf("fixture unavailable: %v", err)
				}
				frame = extractFirstJ2KFrame(t, dcm)
			}
			got := make([]byte, len(want))
			if err := goJ2Kdecode(frame, got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			nd, first := 0, -1
			for i := range want {
				if want[i] != got[i] {
					if first < 0 {
						first = i
					}
					nd++
				}
			}
			if nd != 0 {
				t.Fatalf("%s: %d/%d bytes differ (first @%d: got %d want %d)",
					c.name, nd, len(want), first, got[first], want[first])
			}
		})
	}
}
