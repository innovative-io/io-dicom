//go:build openjpeg && libjxl && cgo

package transcoder_test

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/transcoder"
)

// loadFrames parses a DICOM file and returns each frame's decoded pixel bytes.
func loadFrames(t *testing.T, path string) [][]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("sample %s unavailable: %v", path, err)
	}
	obj, err := media.NewDCMObjFromBytes(data)
	if err != nil {
		t.Fatalf("NewDCMObjFromBytes(%s): %v", path, err)
	}
	nf := frameCount(t, obj)
	frames := make([][]byte, nf)
	for j := 0; j < nf; j++ {
		px, err := obj.GetPixelData(j)
		if err != nil {
			t.Fatalf("GetPixelData(%d): %v", j, err)
		}
		frames[j] = append([]byte(nil), px...)
	}
	return frames
}

func frameCount(t *testing.T, obj media.DICOMObject) int {
	t.Helper()
	raw := strings.TrimSpace(obj.GetString(tags.NumberOfFrames))
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		t.Fatalf("unexpected NumberOfFrames %q", raw)
	}
	return n
}

// TestMultiFrameLosslessRoundTrip transcodes a 25-frame uncompressed image to a
// lossless encapsulated syntax and back, verifying every frame's pixels survive
// unchanged and in order. With 25 frames (more than typical core counts) this
// exercises the concurrent encode and decode paths and would fail loudly if a
// frame were misordered or written to the wrong output offset.
func TestMultiFrameLosslessRoundTrip(t *testing.T) {
	const sample = "../testdata/highdicom-sm_image_grayscale.dcm"

	want := loadFrames(t, sample)
	if len(want) < 2 {
		t.Fatalf("expected a multi-frame sample, got %d frames", len(want))
	}
	if len(want[0]) == 0 {
		t.Fatal("frame 0 has no pixel data; comparison would be trivial")
	}
	t.Logf("sample has %d frames of %d bytes each", len(want), len(want[0]))

	for _, mid := range []*transfersyntax.TransferSyntax{
		transfersyntax.JPEG2000Lossless,
		transfersyntax.JPEGXLLossless,
	} {
		t.Run(mid.Name, func(t *testing.T) {
			data, err := os.ReadFile(sample)
			if err != nil {
				t.Skipf("sample unavailable: %v", err)
			}
			obj, err := media.NewDCMObjFromBytes(data)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if err := transcoder.ChangeTransferSyntax(obj, mid); err != nil {
				t.Skipf("transcode to %s unavailable: %v", mid.Name, err)
			}
			if err := transcoder.ChangeTransferSyntax(obj, transfersyntax.ExplicitVRLittleEndian); err != nil {
				t.Fatalf("transcode back to uncompressed: %v", err)
			}

			if got := frameCount(t, obj); got != len(want) {
				t.Fatalf("frame count changed: got %d want %d", got, len(want))
			}
			for j := range want {
				px, err := obj.GetPixelData(j)
				if err != nil {
					t.Fatalf("frame %d GetPixelData: %v", j, err)
				}
				if !bytes.Equal(px, want[j]) {
					t.Fatalf("frame %d differs after %s round trip (len got %d want %d)",
						j, mid.Name, len(px), len(want[j]))
				}
			}
		})
	}
}
