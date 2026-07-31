package media_test

import (
	"fmt"
	"testing"

	"github.com/innovative-io/io-dicom/codecs"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/transcoder"
)

// useGoBackends selects the pure-Go codec backends for the duration of a test.
func useGoBackends(t *testing.T) {
	t.Helper()
	if err := codecs.UseBackends(codecs.BackendConfig{
		JPEG:     "gojpeg",
		JPEGLS:   "gojpegls",
		JPEG2000: "gojpeg2000",
		JPIP:     "gojpip",
		JPEGXL:   "gojpegxl",
	}); err != nil {
		t.Skipf("pure-Go codec backends unavailable: %v", err)
	}
}

// newMultiFrameObject builds an uncompressed grayscale object with the given
// number of frames.
//
// The representative round-trip matrix only ever exercises single-frame objects
// (newMonoRoundTripObject hard-codes NumberOfFrames "1"), which is why a
// transfer syntax that failed on every multi-frame input went unnoticed.
func newMultiFrameObject(frames int, cols, rows int) media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)

	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.5")
	obj.Write(tags.PatientName, "ROUNDTRIP^MULTIFRAME")
	obj.Write(tags.PatientID, "MF-01")
	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.PlanarConfiguration, 0)
	obj.Write(tags.NumberOfFrames, fmt.Sprintf("%d", frames))
	obj.Write(tags.Rows, uint16(rows))
	obj.Write(tags.Columns, uint16(cols))
	obj.Write(tags.BitsAllocated, 8)
	obj.Write(tags.BitsStored, 8)
	obj.Write(tags.PixelRepresentation, 0)

	n := frames * cols * rows
	pixels := make([]byte, n)
	for i := range pixels {
		pixels[i] = byte(i*7 + 3)
	}
	obj.Add(&media.DICOMTag{
		Group: 0x7FE0, Element: 0x0010,
		Length: uint32(n), VR: "OB", Data: pixels,
	})
	return obj
}

// TestMultiFrameTranscodeAcrossSyntaxes covers the encoders with more than one
// frame. JPIP passed frame 0's offset as img[offset:] — a slice running to the
// end of the whole multi-frame buffer — where every other codec uses
// frameBounds' [start,end). The encoder's exact-size check then rejected it, so
// JPIP encoding failed for every object with more than one frame.
func TestMultiFrameTranscodeAcrossSyntaxes(t *testing.T) {
	// Establish the pure-Go backends explicitly rather than relying on ambient
	// state. forceCodecBackends' cleanup in dicom_object_test.go restores every
	// codec to "passthrough" rather than to its default, so any test running
	// after it sees "no encode backend available" — this test passes in
	// isolation and fails in a full package run without this.
	useGoBackends(t)

	syntaxes := []*transfersyntax.TransferSyntax{
		transfersyntax.JPIPHTJ2KReferenced,
		transfersyntax.HTJ2KLossless,
		transfersyntax.JPEG2000Lossless,
		transfersyntax.RLELossless,
		transfersyntax.JPEGLSLossless,
		transfersyntax.DeflatedImageFrameCompression,
		transfersyntax.JPEGBaseline8Bit,
		transfersyntax.JPEGLosslessSV1,
		transfersyntax.JPEG2000,
		transfersyntax.HTJ2K,
		transfersyntax.JPEGXLLossless,
	}

	for _, ts := range syntaxes {
		for _, frames := range []int{1, 2, 5} {
			t.Run(fmt.Sprintf("%s/frames=%d", ts.Name, frames), func(t *testing.T) {
				obj := newMultiFrameObject(frames, 16, 16)
				if err := transcoder.ChangeTransferSyntax(obj, ts); err != nil {
					t.Fatalf("ChangeTransferSyntax(%s) with %d frames: %v", ts.Name, frames, err)
				}
				if got := obj.GetTransferSyntax().UID; got != ts.UID {
					t.Fatalf("transfer syntax not updated: got %s want %s", got, ts.UID)
				}
			})
		}
	}
}

// TestFailedTranscodeLeavesObjectIntact pins that a failing encode does not
// leave the object half-converted. The JPIP branch called
// beginEncapsulatedPixelData before its encode loop, so an error partway
// through left the pixel-data tag rewritten to encapsulated form with no
// sequence delimiter — neither the original object nor a valid result.
func TestFailedTranscodeLeavesObjectIntact(t *testing.T) {
	useGoBackends(t)
	obj := newMultiFrameObject(3, 16, 16)
	before := obj.GetTransferSyntax().UID
	pixelBefore := obj.GetTagAt(obj.TagCount() - 1)
	lengthBefore := pixelBefore.Length

	// An unsupported target must fail without mutating the object.
	err := transcoder.ChangeTransferSyntax(obj, transfersyntax.JPEGXLJPEGRecompression)
	if err == nil {
		t.Skip("target syntax unexpectedly supported for this input")
	}
	if got := obj.GetTransferSyntax().UID; got != before {
		t.Fatalf("transfer syntax changed despite failure: %s -> %s", before, got)
	}
	pixelAfter := obj.GetTagAt(obj.TagCount() - 1)
	if pixelAfter.Length == 0xFFFFFFFF && lengthBefore != 0xFFFFFFFF {
		t.Fatal("pixel data was converted to encapsulated form despite the failure")
	}
}
