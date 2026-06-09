package codecs

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

// jpegSOFDims extracts width, height and component count from a JPEG SOF marker.
func jpegSOFDims(d []byte) (w, h, nc int) {
	for i := 2; i+9 < len(d); {
		if d[i] != 0xFF {
			i++
			continue
		}
		m := d[i+1]
		if m == 0xD8 || m == 0xD9 || (m >= 0xD0 && m <= 0xD7) {
			i += 2
			continue
		}
		ln := int(d[i+2])<<8 | int(d[i+3])
		if m == 0xC0 || m == 0xC1 || m == 0xC2 {
			h = int(d[i+5])<<8 | int(d[i+6])
			w = int(d[i+7])<<8 | int(d[i+8])
			nc = int(d[i+9])
			return
		}
		i += 2 + ln
	}
	return
}

// TestJXLJPEGRecompressionDispatch checks that decoding a JPEG XL
// JPEG-recompression frame (transfer syntax .111) produces the same pixels as
// decoding the original baseline JPEG it was transcoded from — i.e. the
// reconstruct-then-JPEG-decode dispatch path is wired correctly and lossless.
func TestJXLJPEGRecompressionDispatch(t *testing.T) {
	jxl, err := hex.DecodeString(recompTestJXLHex)
	if err != nil {
		t.Fatalf("bad jxl vector: %v", err)
	}
	jpg, err := hex.DecodeString(recompTestJPGHex)
	if err != nil {
		t.Fatalf("bad jpg vector: %v", err)
	}
	w, h, nc := jpegSOFDims(jpg)
	if w == 0 || h == 0 || nc == 0 {
		t.Fatalf("could not read JPEG dimensions")
	}

	ref := make([]byte, w*h*nc)
	if err := DecompressFrame(context.Background(), transfersyntax.JPEGBaseline8Bit.UID, jpg, 8, "YBR_FULL_422", ref); err != nil {
		t.Fatalf("baseline JPEG decode: %v", err)
	}
	got := make([]byte, w*h*nc)
	if err := DecompressFrame(context.Background(), transfersyntax.JPEGXLJPEGRecompression.UID, jxl, 8, "YBR_FULL_422", got); err != nil {
		t.Fatalf(".111 decode: %v", err)
	}
	if !bytes.Equal(got, ref) {
		t.Fatalf(".111 pixels differ from the baseline-JPEG decode")
	}
}
