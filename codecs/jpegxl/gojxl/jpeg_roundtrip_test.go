package gojxl

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// TestJPEGParseRoundTrip parses a baseline JPEG into the jpegData structure and
// re-encodes it, checking the result is byte-identical — the parser is the exact
// inverse of the writer.
func TestJPEGParseRoundTrip(t *testing.T) {
	for _, name := range []struct{ label, hexStr string }{
		{"444", jpegRoundtripJPGHex},
		{"420", jpegRoundtrip420JPGHex},
	} {
		t.Run(name.label, func(t *testing.T) {
			src, err := hex.DecodeString(name.hexStr)
			if err != nil {
				t.Fatalf("bad jpg vector: %v", err)
			}
			jd, err := ParseJPEG(src)
			if err != nil {
				t.Fatalf("ParseJPEG: %v", err)
			}
			got, err := EncodeJPEG(jd)
			if err != nil {
				t.Fatalf("EncodeJPEG: %v", err)
			}
			if !bytes.Equal(got, src) {
				t.Fatalf("parse/encode round-trip not byte-exact: got %d, want %d bytes", len(got), len(src))
			}
		})
	}
}

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
