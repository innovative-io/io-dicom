package jpeg

import (
	"os"
	"testing"
)

// TestGoJPEGDCTDecodes12BitFixtures decodes the Extended (SOF1) 12-bit fixtures
// with the pure-Go DCT decoder (no cgo) and checks geometry/precision. The
// closeness check against libjpeg lives in the cgo-tagged golden test.
func TestGoJPEGDCTDecodes12BitFixtures(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"process2-4", "../../testdata/cornerstone-CTImage-jpeg-process2-4.dcm"},
		{"JPGExtended", "../../testdata/pydicom-JPGExtended.dcm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dcm := loadBytesFromFile(tc.path, t)
			frame := extractFirstDICOMEncapsulatedFrame(t, dcm)
			f, err := decodeDCT(frame)
			if err != nil {
				t.Fatalf("decodeDCT: %v", err)
			}
			if f.precision != 12 || f.width == 0 || f.height == 0 {
				t.Fatalf("unexpected geometry %dx%d P=%d", f.width, f.height, f.precision)
			}
			out := make([]byte, f.width*f.height*len(f.comps)*2)
			if err := decodeDCTInto(frame, out); err != nil {
				t.Fatalf("decodeDCTInto: %v", err)
			}
			t.Logf("%s: %dx%d P=%d nc=%d", tc.name, f.width, f.height, f.precision, len(f.comps))
		})
	}
}

// FuzzGoJPEGDCT ensures the pure-Go DCT decoder never panics on arbitrary input.
func FuzzGoJPEGDCT(f *testing.F) {
	if dcm, err := os.ReadFile("../../testdata/cornerstone-CTImage-jpeg-process2-4.dcm"); err == nil {
		f.Add(dcm)
	}
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xC1, 0x00, 0x02})             // truncated SOF1
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x03, 0x00})       // short DQT
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xC1, 0x00, 0x0B, 0x0C, 0xFF}) // huge dims header
	f.Fuzz(func(t *testing.T, data []byte) {
		out := make([]byte, 1<<16)
		_ = gojpegDecodeInto(data, out) // must not panic
	})
}
