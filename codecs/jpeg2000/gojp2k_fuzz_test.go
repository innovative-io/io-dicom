package jpeg2000

import (
	"os"
	"testing"
)

// FuzzGoJ2K ensures the pure-Go JPEG 2000 backend never panics on arbitrary
// input (the backend converts panics to errors, so a panic escaping is a bug).
func FuzzGoJ2K(f *testing.F) {
	if dcm, err := os.ReadFile("../../testdata/cornerstone-CTImage-jpeg2000-lossless.dcm"); err == nil {
		if i := indexSOC(dcm); i >= 0 {
			f.Add(dcm[i:])
		}
	}
	// Seed HTJ2K (ISO 15444-15) codestreams so the fuzzer exercises the HT
	// Cleanup decoder (MEL/VLC/MagSgn) on mutated inputs.
	for _, p := range []string{"htj2k-reversible-gray", "htj2k-lossy-gray"} {
		if j2c, err := os.ReadFile("../../testdata/" + p + ".j2c"); err == nil {
			f.Add(j2c)
		}
	}
	f.Add([]byte{0xFF, 0x4F, 0xFF, 0x51})
	f.Add([]byte{0xFF, 0x4F})
	f.Fuzz(func(t *testing.T, data []byte) {
		out := make([]byte, 1<<16)
		_ = gojpeg2000Backend{}.Decode(data, out) // must not panic
	})
}
