package media_test

import (
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/transcoder"
)

// encapsulatedFragmentSizes returns the byte length of every pixel-data
// fragment (0xFFFE,0xE000 items) after the Basic Offset Table, in order.
func encapsulatedFragmentSizes(obj media.DICOMObject) []int {
	var sizes []int
	seenBOT := false
	for i := 0; i < obj.TagCount(); i++ {
		t := obj.GetTagAt(i)
		if t.Group != 0xFFFE || t.Element != 0xE000 {
			continue
		}
		if !seenBOT {
			seenBOT = true // first item is the (empty) Basic Offset Table
			continue
		}
		sizes = append(sizes, len(t.Data))
	}
	return sizes
}

// TestMultiFrameMPEGFragmentSizes pins per-frame framing for the MPEG/HEVC and
// SMPTE 2110 transcoder branches.
//
// Those branches sliced img[offset:] open-ended to the end of the whole
// multi-frame buffer, so every frame but the last was handed too many bytes.
// With the real ffmpeg/st2110 backend that trips an exact-size check and fails
// the encode outright (as the analogous JPEG bug did); with the pure-Go
// passthrough backend, which this test uses, it silently copies the surplus
// frames into the earlier fragments.
//
// The passthrough backend copies its input verbatim, so a correctly framed
// encode must produce one fragment of exactly frameSize bytes per frame. The
// bug instead yields a shrinking staircase: frame 0 gets the whole buffer,
// frame 1 all but the first frame, and so on.
func TestMultiFrameMPEGFragmentSizes(t *testing.T) {
	const cols, rows, frames = 8, 8, 4
	frameSize := cols * rows // 8-bit MONOCHROME2

	for _, ts := range []*transfersyntax.TransferSyntax{
		transfersyntax.MPEG2MPML,
		transfersyntax.MPEG4HP41,
		transfersyntax.HEVCMP51,
		transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo,
	} {
		t.Run(ts.Name, func(t *testing.T) {
			obj := newMultiFrameObject(frames, cols, rows)
			if err := transcoder.ChangeTransferSyntax(obj, ts); err != nil {
				t.Fatalf("ChangeTransferSyntax(%s) with %d frames: %v", ts.Name, frames, err)
			}

			sizes := encapsulatedFragmentSizes(obj)
			if len(sizes) != frames {
				t.Fatalf("got %d fragments, want %d (one per frame)", len(sizes), frames)
			}
			for j, got := range sizes {
				if got != frameSize {
					t.Fatalf("fragment %d is %d bytes, want %d — the frame was sliced "+
						"open-ended into the rest of the multi-frame buffer instead of "+
						"bounded to its own [start,end)", j, got, frameSize)
				}
			}
		})
	}
}
