package wado

import (
	"net/http"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
)

// nopRW is a discarding http.ResponseWriter so the benchmark measures JSON
// construction and encoding, not socket writes.
type nopRW struct{ h http.Header }

func (n *nopRW) Header() http.Header {
	if n.h == nil {
		n.h = http.Header{}
	}
	return n.h
}
func (n *nopRW) Write(p []byte) (int, error) { return len(p), nil }
func (n *nopRW) WriteHeader(int)             {}

// benchQIDOObjects builds a realistic QIDO/metadata result set: instances with a
// typical spread of string, US, and UL attributes across patient/study/series/
// instance levels.
func benchQIDOObjects(n int) []media.DICOMObject {
	out := make([]media.DICOMObject, n)
	for i := 0; i < n; i++ {
		obj := media.NewEmptyDCMObj()
		obj.SetExplicitVR(true)
		obj.Write(tags.SpecificCharacterSet, "ISO_IR 100")
		obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.2")
		obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.1.2.3.4")
		obj.Write(tags.StudyInstanceUID, "1.2.826.0.1.3680043.10.90.1.2.3")
		obj.Write(tags.SeriesInstanceUID, "1.2.826.0.1.3680043.10.90.1.2.3.9")
		obj.Write(tags.PatientName, "DOE^JANE^Q")
		obj.Write(tags.PatientID, "MRN-0012345")
		obj.Write(tags.PatientBirthDate, "19700101")
		obj.Write(tags.PatientSex, "F")
		obj.Write(tags.StudyDate, "20260115")
		obj.Write(tags.StudyTime, "142530")
		obj.Write(tags.AccessionNumber, "ACC-998877")
		obj.Write(tags.Modality, "CT")
		obj.Write(tags.StudyDescription, "CT CHEST W CONTRAST")
		obj.Write(tags.SeriesDescription, "AXIAL 1.25MM")
		obj.Write(tags.SeriesNumber, "3")
		obj.Write(tags.InstanceNumber, "42")
		obj.Write(tags.Rows, uint16(512))
		obj.Write(tags.Columns, uint16(512))
		obj.Write(tags.BitsAllocated, uint16(16))
		obj.Write(tags.BitsStored, uint16(12))
		obj.Write(tags.NumberOfFrames, "1")
		out[i] = obj
	}
	return out
}

func BenchmarkWriteJSONMetadata(b *testing.B) {
	for _, n := range []int{1, 50, 500} {
		objs := benchQIDOObjects(n)
		b.Run(itoaB(n), func(b *testing.B) {
			b.ReportAllocs()
			rw := &nopRW{}
			for i := 0; i < b.N; i++ {
				writeJSONMetadata(rw, objs)
			}
		})
	}
}

func itoaB(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// BenchmarkWriteJSONMetadataReflect measures the previous reflection-based
// encoder for direct comparison.
func BenchmarkWriteJSONMetadataReflect(b *testing.B) {
	for _, n := range []int{1, 50, 500} {
		objs := benchQIDOObjects(n)
		b.Run(itoaB(n), func(b *testing.B) {
			b.ReportAllocs()
			rw := &nopRW{}
			for i := 0; i < b.N; i++ {
				referenceJSONMetadata(rw, objs)
			}
		})
	}
}
