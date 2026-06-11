package transcoder_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"

	"github.com/innovative-io/io-dicom/codecs/jpegxl"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/transcoder"
)

// makeBaselineJPEG returns a baseline 4:2:0 JPEG of the given size with a
// deterministic pattern (so each frame differs).
func makeBaselineJPEG(t *testing.T, w, h, seed int) []byte {
	t.Helper()
	img := image.NewYCbCr(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Y[img.YOffset(x, y)] = uint8((x*5 + y*3 + seed*37) & 0xff)
			ci := img.COffset(x, y)
			img.Cb[ci] = uint8((x + seed) & 0xff)
			img.Cr[ci] = uint8((y + seed) & 0xff)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}

// TestTranscodeJPEGToJXLRecompression builds a multi-frame baseline-JPEG (.50)
// object, transcodes it to JPEG XL JPEG-Recompression (.111), and verifies the
// transfer syntax changed and every reconstructed frame is byte-identical to the
// original JPEG — i.e. the transcoder wires the lossless recompression encoder
// in correctly and operates on the original JPEG bytes rather than decoded
// pixels.
func TestTranscodeJPEGToJXLRecompression(t *testing.T) {
	const w, h = 96, 64
	srcFrames := [][]byte{
		makeBaselineJPEG(t, w, h, 1),
		makeBaselineJPEG(t, w, h, 2),
	}

	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.JPEGBaseline8Bit)
	obj.WriteUint16(tags.SamplesPerPixel, 3)
	obj.WriteString(tags.PhotometricInterpretation, "YBR_FULL_422")
	obj.WriteUint16(tags.PlanarConfiguration, 0)
	obj.WriteString(tags.NumberOfFrames, "2")
	obj.WriteUint16(tags.Rows, h)
	obj.WriteUint16(tags.Columns, w)
	obj.WriteUint16(tags.BitsAllocated, 8)
	obj.WriteUint16(tags.BitsStored, 8)
	obj.WriteUint16(tags.HighBit, 7)
	obj.WriteUint16(tags.PixelRepresentation, 0)

	// Encapsulated pixel data: tag with undefined length, empty offset table, one
	// fragment per frame, sequence delimiter.
	obj.Add(&media.DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB"})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL"})
	for _, f := range srcFrames {
		obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: uint32(len(f)), VR: "DL", Data: f})
	}
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL"})

	if err := transcoder.ChangeTransferSyntax(obj, transfersyntax.JPEGXLJPEGRecompression); err != nil {
		t.Fatalf("transcode .50 -> .111: %v", err)
	}
	if got := obj.GetTransferSyntax().UID; got != transfersyntax.JPEGXLJPEGRecompression.UID {
		t.Fatalf("transfer syntax not updated: got %s", got)
	}

	// Pull the new .111 frames back out and confirm each reconstructs its source
	// JPEG byte-for-byte.
	out := encapsulatedFrames(t, obj, len(srcFrames))
	for j, frame := range out {
		if !jpegxl.IsJPEGRecompression(frame) {
			t.Fatalf("frame %d is not a JPEG XL recompression", j)
		}
		rec, err := jpegxl.ReconstructJPEG(frame)
		if err != nil {
			t.Fatalf("frame %d ReconstructJPEG: %v", j, err)
		}
		if !bytes.Equal(rec, srcFrames[j]) {
			t.Fatalf("frame %d reconstruction not byte-exact (got %d, want %d)", j, len(rec), len(srcFrames[j]))
		}
	}
}

// encapsulatedFrames returns the `n` fragment payloads following the encapsulated
// pixel-data tag (skipping the offset table).
func encapsulatedFrames(t *testing.T, obj media.DICOMObject, n int) [][]byte {
	t.Helper()
	for i := 0; i < obj.TagCount(); i++ {
		tag := obj.GetTagAt(i)
		if tag.Group == 0x7FE0 && tag.Element == 0x0010 {
			frames := make([][]byte, n)
			for j := 0; j < n; j++ {
				ft := obj.GetTagAt(i + 2 + j) // +1 offset table, +2.. frames
				if ft == nil || ft.Group != 0xFFFE || ft.Element != 0xE000 {
					t.Fatalf("missing encapsulated frame %d", j)
				}
				frames[j] = ft.Data
			}
			return frames
		}
	}
	t.Fatal("no pixel data tag found after transcode")
	return nil
}
