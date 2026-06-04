package jpeg

import (
	"os"
	"testing"
)

// TestGoJPEGLosslessDecodesCTFixtures decodes the lossless SOF3 CT fixtures with
// the pure-Go decoder (no cgo) and sanity-checks the geometry. The byte-exact
// correctness check against libjpeg lives in the cgo-tagged golden test.
func TestGoJPEGLosslessDecodesCTFixtures(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"process14", "../../testdata/cornerstone-CTImage-jpeg-process14.dcm"},
		{"process14sv1", "../../testdata/cornerstone-CTImage-jpeg-process14sv1.dcm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dcm := loadBytesFromFile(tc.path, t)
			frame := extractFirstDICOMEncapsulatedFrame(t, dcm)

			f, samples, err := decodeLossless(frame)
			if err != nil {
				t.Fatalf("decodeLossless: %v", err)
			}
			if f.width == 0 || f.height == 0 {
				t.Fatalf("bad geometry %dx%d", f.width, f.height)
			}
			if len(f.comps) != 1 {
				t.Fatalf("expected 1 component (CT grayscale), got %d", len(f.comps))
			}
			if got := len(samples); got != f.width*f.height*len(f.comps) {
				t.Fatalf("sample count %d != %d", got, f.width*f.height*len(f.comps))
			}
			t.Logf("%s: %dx%d, precision %d, predictor %d", tc.name, f.width, f.height, f.precision, f.predictor)

			// Pack through the backend entry point and confirm the size matches.
			bps := 1
			if f.precision > 8 {
				bps = 2
			}
			out := make([]byte, f.width*f.height*len(f.comps)*bps)
			if err := decodeLosslessInto(frame, out); err != nil {
				t.Fatalf("decodeLosslessInto: %v", err)
			}
		})
	}
}

func TestGoJPEGRejectsNonLossless(t *testing.T) {
	// A baseline (SOF0) JPEG must be rejected as unsupported so callers fall
	// back to libjpeg rather than mis-decoding.
	jpegData := loadBytesFromFile("../../testdata/test8.jpg", t)
	if _, _, err := decodeLossless(jpegData); err == nil {
		t.Fatal("expected non-lossless JPEG to be rejected by the lossless decoder")
	}
}

// FuzzGoJPEGLossless ensures the pure-Go lossless decoder never panics on
// arbitrary input — it parses untrusted DICOM pixel data.
func FuzzGoJPEGLossless(f *testing.F) {
	if dcm, err := os.ReadFile("../../testdata/cornerstone-CTImage-jpeg-process14.dcm"); err == nil {
		f.Add(dcm) // whole file; the decoder will reject non-JPEG prefixes
	}
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xD9})                   // SOI + EOI
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xC3, 0x00, 0x02})       // truncated SOF3
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xDA, 0x00, 0x02, 0x01}) // SOS with no SOF
	f.Fuzz(func(t *testing.T, data []byte) {
		frame, samples, err := decodeLossless(data)
		if err == nil && len(samples) != frame.width*frame.height*len(frame.comps) {
			t.Fatalf("inconsistent sample count %d for %dx%dx%d", len(samples), frame.width, frame.height, len(frame.comps))
		}
	})
}
