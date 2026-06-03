package media

import (
	"bytes"
	"strings"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
)

func makeSimpleObj() DICOMObject {
	obj := NewEmptyDCMObj()
	obj.Write(tags.PatientName, "DOE^JOHN")
	obj.Write(tags.PatientID, "12345")
	return obj
}

func TestDICOMObject_GetTag(t *testing.T) {
	obj := makeSimpleObj()
	tag := obj.GetTag(tags.PatientName)
	if tag == nil {
		t.Fatal("GetTag() returned nil for existing tag")
	}
	if tag.GetString() != "DOE^JOHN" {
		t.Fatalf("GetTag() value = %q", tag.GetString())
	}
}

func TestDICOMObject_GetTag_Missing(t *testing.T) {
	obj := makeSimpleObj()
	if obj.GetTag(tags.StudyDate) != nil {
		t.Fatal("GetTag() should return nil for absent tag")
	}
}

// TestDICOMObject_GetDate exercises the deprecated package-level GetDate free
// function, which wraps obj.GetDate. Kept as a regression guard for the shim.
func TestDICOMObject_GetDate(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.Write(tags.StudyDate, "20230115")
	d := GetDate(obj, tags.StudyDate)
	if d.Year() != 2023 || d.Month() != 1 || d.Day() != 15 {
		t.Fatalf("GetDate() = %v", d)
	}
}

func TestDICOMObject_GetDate_Invalid(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.Write(tags.StudyDate, "notadate")
	d := GetDate(obj, tags.StudyDate)
	if !d.IsZero() {
		t.Fatalf("GetDate() invalid should return zero time, got %v", d)
	}
}

func TestDICOMObject_GetUInt(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.Write(tags.FileMetaInformationGroupLength, uint32(0xCAFEBABE))
	v := obj.GetUint32(tags.FileMetaInformationGroupLength)
	if v != 0xCAFEBABE {
		t.Fatalf("GetUInt() = 0x%X, want 0xCAFEBABE", v)
	}
}

func TestDICOMObject_DumpTags_WritesToProvidedWriter(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.Write(tags.PatientName, "DOE^JANE")

	var buffer bytes.Buffer
	obj.DumpTags(&buffer)

	got := buffer.String()
	if !strings.Contains(got, "Patient's Name : DOE^JANE") {
		t.Fatalf("DumpTags() output = %q, want Patient's Name line", got)
	}
}
