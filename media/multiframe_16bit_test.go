package media_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/transcoder"
)

// newMultiFrame16Object builds an uncompressed 16-bit grayscale object whose
// frames are individually distinguishable, so a frame decoded from the wrong
// byte offset cannot accidentally match.
func newMultiFrame16Object(frames, cols, rows int) (media.DICOMObject, []byte) {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)

	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.6")
	obj.Write(tags.PatientName, "ROUNDTRIP^MF16")
	obj.Write(tags.PatientID, "MF16-01")
	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.PlanarConfiguration, 0)
	obj.Write(tags.NumberOfFrames, fmt.Sprintf("%d", frames))
	obj.Write(tags.Rows, uint16(rows))
	obj.Write(tags.Columns, uint16(cols))
	obj.Write(tags.BitsAllocated, 16)
	obj.Write(tags.BitsStored, 16)
	obj.Write(tags.HighBit, 15)
	obj.Write(tags.PixelRepresentation, 0)

	perFrame := cols * rows
	pixels := make([]byte, frames*perFrame*2)
	for f := 0; f < frames; f++ {
		for i := 0; i < perFrame; i++ {
			// A per-frame base keeps every frame's samples in a disjoint range.
			v := uint16((f+1)*0x1000 + i)
			binary.LittleEndian.PutUint16(pixels[(f*perFrame+i)*2:], v)
		}
	}
	obj.Add(&media.DICOMTag{
		Group: 0x7FE0, Element: 0x0010,
		Length: uint32(len(pixels)), VR: "OW", Data: pixels,
	})
	return obj, pixels
}

// TestMultiFrame16BitLosslessRoundTrip pins per-frame pixel fidelity for the
// 16-bit encoders, which the existing multi-frame coverage never checked: it
// asserts only that ChangeTransferSyntax returns nil, and builds 8-bit objects.
//
// The transcoder computes offset as a BYTE offset
// (j * cols * rows * bitsAllocated / 8) but handed the 16-bit JPEG encoders
// img[offset/2:] — a sample index. For frame 0 that is 0 either way, which is
// why single-frame images, the common case, hid it. From frame 1 on, the
// encoder reads from half the intended byte position and encodes the wrong
// pixels.
//
// These syntaxes are mathematically lossless, so a correct round-trip must
// reproduce every input byte.
func TestMultiFrame16BitLosslessRoundTrip(t *testing.T) {
	useGoBackends(t)

	const cols, rows, frames = 8, 8, 3
	frameBytes := cols * rows * 2

	for _, ts := range []*transfersyntax.TransferSyntax{
		transfersyntax.JPEGLosslessSV1,
		transfersyntax.JPEGLossless,
	} {
		t.Run(ts.Name, func(t *testing.T) {
			obj, want := newMultiFrame16Object(frames, cols, rows)
			if err := transcoder.ChangeTransferSyntax(obj, ts); err != nil {
				// Not a skip: the encoders are present. An over-long slice
				// fails encodeLosslessJPEG's exact-size check
				// (width*height*samples*bps != len(raw)).
				t.Fatalf("%s encode of a %d-frame object failed: %v", ts.Name, frames, err)
			}

			ctx := context.Background()
			for f := 0; f < frames; f++ {
				got, err := obj.GetDecompressedFrame(ctx, f)
				if err != nil {
					t.Fatalf("frame %d: GetDecompressedFrame: %v", f, err)
				}
				exp := want[f*frameBytes : (f+1)*frameBytes]
				if len(got) < frameBytes {
					t.Fatalf("frame %d: got %d bytes, want %d", f, len(got), frameBytes)
				}
				got = got[:frameBytes]
				for i := range exp {
					if got[i] != exp[i] {
						gotV := binary.LittleEndian.Uint16(got[i&^1:])
						wantV := binary.LittleEndian.Uint16(exp[i&^1:])
						t.Fatalf("frame %d differs at byte %d: sample %#04x, want %#04x "+
							"— the frame was encoded from the wrong offset in the "+
							"multi-frame pixel buffer", f, i, gotV, wantV)
					}
				}
			}
		})
	}
}

// TestMultiFrame16BitDCTEncodes covers the lossy 16-bit branches, which shared
// the same offset arithmetic. These are DCT syntaxes so the pixels do not
// round-trip exactly; what matters is that every frame encodes at all and the
// object ends up with one fragment per frame.
func TestMultiFrame16BitDCTEncodes(t *testing.T) {
	useGoBackends(t)

	const cols, rows, frames = 8, 8, 3
	for _, ts := range []*transfersyntax.TransferSyntax{
		transfersyntax.JPEGExtended12Bit,
		transfersyntax.JPEGBaseline8Bit,
	} {
		t.Run(ts.Name, func(t *testing.T) {
			obj, _ := newMultiFrame16Object(frames, cols, rows)
			if err := transcoder.ChangeTransferSyntax(obj, ts); err != nil {
				t.Fatalf("%s encode of a %d-frame object failed: %v", ts.Name, frames, err)
			}
			if got := obj.GetTransferSyntax().UID; got != ts.UID {
				t.Fatalf("transfer syntax not updated: got %s want %s", got, ts.UID)
			}
			ctx := context.Background()
			for f := 0; f < frames; f++ {
				got, err := obj.GetDecompressedFrame(ctx, f)
				if err != nil {
					t.Fatalf("frame %d: GetDecompressedFrame: %v", f, err)
				}
				if len(got) < cols*rows {
					t.Fatalf("frame %d decoded to %d bytes, too short for %dx%d",
						f, len(got), cols, rows)
				}
			}
		})
	}
}
