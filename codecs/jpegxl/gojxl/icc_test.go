package gojxl

import (
	"os"
	"testing"
)

// TestICCProfileDecode verifies that an image carrying an embedded ICC profile
// (want_icc) decodes: the ICC stream is consumed correctly so the decoder stays
// byte-aligned with the frame data, for both lossless Modular and lossy VarDCT.
// The source is an R=x, G=y, B=128 sRGB gradient (validated byte-exact vs djxl
// out of band). Fixtures are gitignored; skipped when absent.
func TestICCProfileDecode(t *testing.T) {
	for _, f := range []string{"icc_ll.jxl", "icc_lossy.jxl"} {
		data, err := os.ReadFile("testdata/" + f)
		if err != nil {
			t.Skipf("fixture %s unavailable: %v", f, err)
		}
		img, err := Decode(data)
		if err != nil {
			t.Fatalf("%s: Decode failed (ICC consume): %v", f, err)
		}
		if img.W != 80 || img.H != 60 || img.Channels != 3 {
			t.Fatalf("%s: got %dx%d ch=%d, want 80x60x3", f, img.W, img.H, img.Channels)
		}
		get := func(x, y, c int) int { return int(img.Pixels[(y*img.W+x)*3+c]) }
		if get(75, 5, 0)-get(2, 5, 0) < 180 {
			t.Errorf("%s: red gradient not recovered", f)
		}
	}
}
