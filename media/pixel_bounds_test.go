package media

import (
	"context"
	"strings"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

// buildPixelObject assembles an object whose declared geometry is decoupled from
// the pixel bytes actually present — the shape a hostile instance takes.
func buildPixelObject(rows, cols, bitsAlloc, samples, frames uint16, planar uint16, photo string, pixelBytes int) DICOMObject {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.WriteUint16(tags.Rows, rows)
	obj.WriteUint16(tags.Columns, cols)
	obj.WriteUint16(tags.BitsAllocated, bitsAlloc)
	obj.WriteUint16(tags.SamplesPerPixel, samples)
	obj.WriteUint16(tags.PlanarConfiguration, planar)
	obj.WriteString(tags.PhotometricInterpretation, photo)
	obj.WriteString(tags.NumberOfFrames, itoa(int(frames)))
	obj.Add(&DICOMTag{
		Group:   0x7FE0,
		Element: 0x0010,
		VR:      "OW",
		Length:  uint32(pixelBytes),
		Data:    make([]byte, pixelBytes),
	})
	return obj
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestPlanarRGBTruncatedPixelDataDoesNotPanic covers a remotely reachable crash:
// the PlanarConfiguration=1 de-interleave indexed tag.Data using bounds derived
// from Rows/Columns/BitsAllocated with no check against the bytes actually
// present, so an instance declaring 1024x1024 RGB while carrying 4 bytes of
// pixel data panicked the process. Both accessors must now return an error.
func TestPlanarRGBTruncatedPixelDataDoesNotPanic(t *testing.T) {
	obj := buildPixelObject(1024, 1024, 8, 3, 1, 1, "RGB", 4)

	if _, err := obj.GetPixelData(0); err == nil {
		t.Fatal("GetPixelData: expected an error for truncated planar pixel data")
	} else if !strings.Contains(err.Error(), "truncated") {
		t.Logf("GetPixelData returned %v", err) // any error beats a panic
	}

	if _, err := obj.GetDecompressedFrame(context.Background(), 0); err == nil {
		t.Fatal("GetDecompressedFrame: expected an error for truncated planar pixel data")
	}
}

// TestPlanarRGBValidFirstFrameStillBoundsChecked pins the variant where the
// first frame is well formed but NumberOfFrames overstates how many follow — a
// valid leading frame must not be mistaken for proof that the rest are present.
func TestPlanarRGBValidFirstFrameStillBoundsChecked(t *testing.T) {
	// One complete 16x16 RGB frame is present; the object claims 100 frames.
	obj := buildPixelObject(16, 16, 8, 3, 100, 1, "RGB", 16*16*3)
	if _, err := obj.GetDecompressedFrame(context.Background(), 1); err == nil {
		t.Fatal("expected an error requesting a frame beyond the pixel data present")
	}
}

// TestFrameOffsetOverflowDoesNotPanic covers the second remotely reachable
// crash: the frame-range guard computed offset and offset+frameSize in uint32,
// so a large-but-legal frame number wrapped the arithmetic, passed the check,
// and sliced with high < low. The frame numbers here are positive and in the
// range callers forward unmodified from a URL or query parameter.
func TestFrameOffsetOverflowDoesNotPanic(t *testing.T) {
	// frameSize = 64*64*1 = 4096; frame 1048575 makes offset wrap uint32.
	obj := buildPixelObject(64, 64, 8, 1, 0, 0, "MONOCHROME2", 4096)
	// NumberOfFrames is written separately so it can exceed uint16.
	obj.WriteString(tags.NumberOfFrames, "1048576")

	for _, frame := range []int{1048575, 1048570, 65536} {
		if _, err := obj.GetPixelData(frame); err == nil {
			t.Fatalf("GetPixelData(%d): expected an out-of-range error", frame)
		}
		if _, err := obj.GetDecompressedFrame(context.Background(), frame); err == nil {
			t.Fatalf("GetDecompressedFrame(%d): expected an out-of-range error", frame)
		}
	}
}

// TestNegativeFrameIndexRejected closes a library-contract hole: callers today
// happen to reject frame < 1, but the accessors must not panic on their own.
func TestNegativeFrameIndexRejected(t *testing.T) {
	obj := buildPixelObject(16, 16, 8, 1, 1, 0, "MONOCHROME2", 16*16)
	if _, err := obj.GetPixelData(-1); err == nil {
		t.Fatal("GetPixelData(-1): expected an error")
	}
	if _, err := obj.GetDecompressedFrame(context.Background(), -1); err == nil {
		t.Fatal("GetDecompressedFrame(-1): expected an error")
	}
}
