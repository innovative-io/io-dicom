package gojxl

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// TestJPEGReconstruction decodes JPEG XL JPEG-transcodes back to the original
// JPEG bytes and checks the reconstruction is byte-exact — the end-to-end test
// of the container/jbrd/brotli/non-XYB-VarDCT/RAW-quant/coefficient/bitstream
// pipeline, for both 4:4:4 and 4:2:0 (chroma-subsampled) inputs.
func TestJPEGReconstruction(t *testing.T) {
	cases := []struct {
		name, jxlHex, jpgHex string
	}{
		{"444", jpegRoundtripJXLHex, jpegRoundtripJPGHex},
		{"420", jpegRoundtrip420JXLHex, jpegRoundtrip420JPGHex},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jxl, err := hex.DecodeString(tc.jxlHex)
			if err != nil {
				t.Fatalf("bad jxl vector: %v", err)
			}
			want, err := hex.DecodeString(tc.jpgHex)
			if err != nil {
				t.Fatalf("bad jpg vector: %v", err)
			}
			jd, err := DecodeJPEGFromJXL(jxl)
			if err != nil {
				t.Fatalf("DecodeJPEGFromJXL: %v", err)
			}
			got, err := EncodeJPEG(jd)
			if err != nil {
				t.Fatalf("EncodeJPEG: %v", err)
			}
			if !bytes.Equal(got, want) {
				di := -1
				for i := 0; i < len(got) && i < len(want); i++ {
					if got[i] != want[i] {
						di = i
						break
					}
				}
				t.Fatalf("reconstruction mismatch: got %d bytes, want %d bytes, first diff at %d",
					len(got), len(want), di)
			}
		})
	}
}
