package media

import (
	"fmt"
	"io"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

func benchObj(pixelBytes int) DICOMObject {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.8")
	obj.Write(tags.PatientName, "STREAM^BENCH")
	pix := make([]byte, pixelBytes)
	obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, VR: "OW", Length: uint32(len(pix)), Data: pix})
	return obj
}

// BenchmarkSendSerialize_Buffered is the old send path's serialization: WriteObj
// grows a buffer to the whole message size (a second full copy of the pixels).
func BenchmarkSendSerialize_Buffered(b *testing.B) {
	for _, sz := range []int{64 << 10, 1 << 20, 4 << 20} {
		obj := benchObj(sz)
		b.Run(fmt.Sprintf("%dKiB", sz>>10), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				buf := NewDICOMBuffer() // fresh, as peak memory would be per send
				buf.WriteObj(obj)
			}
		})
	}
}

// BenchmarkSendSerialize_Streamed is the new path: WriteObjTo streams to the
// wire, allocating only a small header scratch regardless of message size.
// Writing to io.Discard isolates the serialization memory; a real send writes
// the same payload bytes to the socket either way, so the meaningful figure
// here is B/op, not ns/op.
func BenchmarkSendSerialize_Streamed(b *testing.B) {
	for _, sz := range []int{64 << 10, 1 << 20, 4 << 20} {
		obj := benchObj(sz)
		b.Run(fmt.Sprintf("%dKiB", sz>>10), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = WriteObjTo(io.Discard, obj)
			}
		})
	}
}
