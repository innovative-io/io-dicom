package media

import (
	"fmt"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
)

// benchLookupObj builds an object with n filler tags at depth 0, plus one
// present non-empty tag and one present zero-length tag. Absent and zero-length
// tags are common in real DICOM, and both miss findTagGE's primary index — so
// these benchmarks guard the depth-0 index that keeps those lookups O(1).
// Before that index existed, a miss walked the whole tag list.
func benchLookupObj(n int) DICOMObject {
	obj := NewEmptyDCMObj()
	obj.WriteString(tags.PatientName, "DOE^JOHN") // present, non-empty
	obj.WriteString(tags.StudyDescription, "")    // present, zero length
	for i := 0; i < n; i++ {
		obj.WriteString(&tags.Tag{
			Group:   0x0011,
			Element: uint16(0x1000 + i),
			VR:      "LO",
			VM:      "1",
			Name:    fmt.Sprintf("Filler%d", i),
		}, "filler-value")
	}
	return obj
}

// benchAbsentTag is never written into the benchmark objects.
var benchAbsentTag = &tags.Tag{Group: 0x7777, Element: 0x0001, VR: "LO", VM: "1", Name: "Absent"}

// BenchmarkTagLookup measures the three findTagGE outcomes against growing tag
// counts. All three should be flat in the tag count; any regression to a linear
// scan shows up as the absent/zero-length cases scaling with tags=N.
func BenchmarkTagLookup(b *testing.B) {
	for _, n := range []int{116, 1000, 3000} {
		obj := benchLookupObj(n)
		b.Run(fmt.Sprintf("tags=%d", obj.TagCount()), func(b *testing.B) {
			b.Run("present_nonempty", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = obj.GetString(tags.PatientName)
				}
			})
			b.Run("absent", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = obj.GetString(benchAbsentTag)
				}
			})
			b.Run("present_zerolen", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = obj.GetString(tags.StudyDescription)
				}
			})
		})
	}
}

// BenchmarkMetadataHeaderScan mimics a viewer/WADO metadata read: a batch of
// commonly-absent tags fetched per request.
func BenchmarkMetadataHeaderScan(b *testing.B) {
	obj := benchLookupObj(1000)
	absent := []*tags.Tag{
		tags.ImagerPixelSpacing, tags.SliceThickness, tags.InstitutionName,
		tags.ImageOrientationPatient, tags.ImagePositionPatient,
		tags.FrameOfReferenceUID, tags.WindowCenter, tags.WindowWidth,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range absent {
			_ = obj.GetString(t)
		}
	}
}
