package media

// TestGetDecompressedFrame_NonConformantCGETTransferSyntax tests the auto-
// detect fallback in decompressSingleFrame: a non-conformant C-GET SCP accepts
// ExplicitVRLittleEndian during association negotiation but sends the pixel
// data in its native compressed format (JPEG / JPEG 2000) without transcoding.
// The file is saved with ELE as the transfer syntax and encapsulated
// (Length=0xFFFFFFFF) pixel data.  GetDecompressedFrame must decode correctly.

import (
	"context"
	"testing"

	"github.com/innovative-io/io-dicom/codecs/jpeg"
	"github.com/innovative-io/io-dicom/codecs/jpeg2000"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

// encodeJPEG8Frame encodes a grayscale 8-bit frame using the DICOM IJG encoder.
func encodeJPEG8Frame(t *testing.T, pixels []byte, cols, rows int) []byte {
	t.Helper()
	var jpegData []byte
	var jpegBytes int
	if err := jpeg.EIJG8encode(pixels, uint16(cols), uint16(rows), 1, &jpegData, &jpegBytes, 0); err != nil {
		t.Fatalf("EIJG8encode: %v", err)
	}
	return jpegData[:jpegBytes]
}

// encodeJ2KFrame encodes a grayscale 8-bit frame as JPEG 2000 lossless.
func encodeJ2KFrame(t *testing.T, pixels []byte, cols, rows int) []byte {
	t.Helper()
	var j2kData []byte
	var j2kBytes int
	if err := jpeg2000.J2KencodeContext(context.Background(), pixels, uint16(cols), uint16(rows), 1, 8, &j2kData, &j2kBytes, 0); err != nil {
		t.Fatalf("J2KencodeContext: %v", err)
	}
	return j2kData[:j2kBytes]
}

// buildNonConformantCGETObject builds a DICOM object whose transfer syntax is
// ELE (as negotiated with the SCP) but whose pixel data is stored as an
// encapsulated compressed fragment — simulating a non-conformant C-GET SCP.
func buildNonConformantCGETObject(ts *transfersyntax.TransferSyntax, fragment []byte, cols, rows int) DICOMObject {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(ts)
	obj.SetExplicitVR(ts.UID != transfersyntax.ImplicitVRLittleEndian.UID)
	obj.SetBigEndian(false)

	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.NumberOfFrames, "1")
	obj.Write(tags.Rows, rows)
	obj.Write(tags.Columns, cols)
	obj.Write(tags.BitsAllocated, 8)
	obj.Write(tags.SamplesPerPixel, 1)

	// Encapsulated pixel data with an empty Basic Offset Table and one fragment.
	obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB"})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL"})                                     // empty BOT
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: uint32(len(fragment)), VR: "DL", Data: fragment}) // compressed frame
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL"})                                     // SQ delimiter
	return obj
}

func TestGetDecompressedFrame_NonConformantCGET_JPEG(t *testing.T) {
	const cols, rows = 8, 8
	// Create a deterministic 8×8 grayscale pixel pattern.
	pixels := make([]byte, cols*rows)
	for i := range pixels {
		pixels[i] = byte(i * 10)
	}

	fragment := encodeJPEG8Frame(t, pixels, cols, rows)

	for _, ts := range []*transfersyntax.TransferSyntax{
		transfersyntax.ExplicitVRLittleEndian,
		transfersyntax.ImplicitVRLittleEndian,
	} {
		t.Run(ts.Name, func(t *testing.T) {
			obj := buildNonConformantCGETObject(ts, fragment, cols, rows)

			out, err := obj.GetDecompressedFrame(context.Background(), 0)
			if err != nil {
				t.Fatalf("GetDecompressedFrame(%s): unexpected error: %v", ts.Name, err)
			}
			// Verify the output has the correct uncompressed size.
			// Pixel-exact comparison is not required here — the goal of this
			// test is to confirm that the auto-detect fallback dispatches to a
			// JPEG decoder rather than returning "encapsulated uncompressed
			// frame too small". Fidelity is covered by the codec tests.
			if len(out) != cols*rows {
				t.Fatalf("GetDecompressedFrame(%s): got %d bytes, want %d", ts.Name, len(out), cols*rows)
			}
		})
	}
}

func TestGetDecompressedFrame_NonConformantCGET_JPEG2000(t *testing.T) {
	if !jpeg2000.CGOEnabled {
		// Needs the openjpeg backend to both build the JPEG 2000 fragment and
		// decode it; there is no pure-Go JPEG 2000 codec.
		t.Skip("requires the openjpeg native backend")
	}
	const cols, rows = 8, 8
	pixels := make([]byte, cols*rows)
	for i := range pixels {
		pixels[i] = byte(i * 10)
	}

	fragment := encodeJ2KFrame(t, pixels, cols, rows)

	for _, ts := range []*transfersyntax.TransferSyntax{
		transfersyntax.ExplicitVRLittleEndian,
		transfersyntax.ImplicitVRLittleEndian,
	} {
		t.Run(ts.Name, func(t *testing.T) {
			obj := buildNonConformantCGETObject(ts, fragment, cols, rows)

			out, err := obj.GetDecompressedFrame(context.Background(), 0)
			if err != nil {
				t.Fatalf("GetDecompressedFrame(%s): unexpected error: %v", ts.Name, err)
			}
			if len(out) != cols*rows {
				t.Fatalf("GetDecompressedFrame(%s): got %d bytes, want %d", ts.Name, len(out), cols*rows)
			}
			for i, got := range out {
				if got != pixels[i] {
					t.Fatalf("pixel[%d]: got %d, want %d", i, got, pixels[i])
				}
			}
		})
	}
}
