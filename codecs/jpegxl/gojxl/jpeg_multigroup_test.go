package gojxl

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

// TestJPEGToJXLMultiDCGroup exercises the JPEG->.111 encoder on an image larger
// than a single DC group (each DC group spans 2048 px), which requires emitting
// one LfGroup section per DC group plus the matching TOC entries. A
// self-contained synthetic baseline JPEG is produced with the standard library
// (no gitignored testdata), then transcoded and reconstructed; the result must
// be byte-identical to the parsed source. Covers both 4:4:4 and 4:2:0.
func TestJPEGToJXLMultiDCGroup(t *testing.T) {
	const dim = 2100 // > 2048 → at least 2 DC groups per axis

	cases := []struct {
		name  string
		ratio image.YCbCrSubsampleRatio
	}{
		{"444", image.YCbCrSubsampleRatio444},
		{"420", image.YCbCrSubsampleRatio420},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img := image.NewYCbCr(image.Rect(0, 0, dim, dim), tc.ratio)
			for y := 0; y < dim; y++ {
				for x := 0; x < dim; x++ {
					img.Y[img.YOffset(x, y)] = uint8((x*7 + y*3) & 0xff)
					ci := img.COffset(x, y)
					img.Cb[ci] = uint8((x * 2) & 0xff)
					img.Cr[ci] = uint8((y * 2) & 0xff)
				}
			}
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
				t.Fatalf("jpeg.Encode: %v", err)
			}
			src := buf.Bytes()

			jd, err := ParseJPEG(src)
			if err != nil {
				t.Fatalf("ParseJPEG: %v", err)
			}
			if fd := computeFrameDimensions(uint32(jd.width), uint32(jd.height), 1, 1); fd.numDCGroups < 2 {
				t.Fatalf("test image is not multi-DC-group (numDCGroups=%d)", fd.numDCGroups)
			}

			jxl, err := EncodeJXLFromJPEGData(jd)
			if err != nil {
				t.Fatalf("EncodeJXLFromJPEGData: %v", err)
			}
			if !IsJPEGRecompression(jxl) {
				t.Fatal("encoded file not recognized as a JPEG recompression")
			}
			rec, err := ReconstructJPEG(jxl)
			if err != nil {
				t.Fatalf("ReconstructJPEG: %v", err)
			}
			want, err := EncodeJPEG(jd)
			if err != nil {
				t.Fatalf("EncodeJPEG: %v", err)
			}
			if !bytes.Equal(rec, want) {
				t.Fatalf("multi-DC-group transcode not byte-exact (got %d, want %d)", len(rec), len(want))
			}
		})
	}
}
