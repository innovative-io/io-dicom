package gojxl

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// TestJPEGReconstruction decodes a JPEG XL JPEG-transcode back to the original
// JPEG bytes and checks the reconstruction is byte-exact — the end-to-end test
// of the container/jbrd/brotli/non-XYB-VarDCT/RAW-quant/coefficient/bitstream
// pipeline.
func TestJPEGReconstruction(t *testing.T) {
	jxl, err := hex.DecodeString(jpegRoundtripJXLHex)
	if err != nil {
		t.Fatalf("bad jxl vector: %v", err)
	}
	want, err := hex.DecodeString(jpegRoundtripJPGHex)
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
}
