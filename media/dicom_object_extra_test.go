package media

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

func init() {
	InitDict()
}

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

func TestDICOMObject_GetTag_PatientID(t *testing.T) {
	obj := makeSimpleObj()
	tag := obj.GetTag(tags.PatientID)
	if tag == nil {
		t.Fatal("GetTag() returned nil")
	}
	if tag.GetString() != "12345" {
		t.Fatalf("GetTag() value = %q", tag.GetString())
	}
}

func TestDICOMObject_FindTag_Missing(t *testing.T) {
	obj := makeSimpleObj()
	if obj.GetTag(&tags.Tag{Group: 0x0020, Element: 0x0010}) != nil {
		t.Fatal("GetTag() should return nil for absent group/element")
	}
}

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

func TestDICOMObject_GetUShort(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.Write(tags.BitsAllocated, 16)
	v := obj.GetUint16(tags.BitsAllocated)
	if v != 16 {
		t.Fatalf("GetUShort() = %d, want 16", v)
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

func TestDICOMObject_GetUIntByTag(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.Write(tags.FileMetaInformationGroupLength, uint32(99))
	v := obj.GetUint32(tags.FileMetaInformationGroupLength)
	if v != 99 {
		t.Fatalf("GetUInt() = %d, want 99", v)
	}
}

func TestDICOMObject_WriteToFile(t *testing.T) {
	if _, err := os.Stat("../testdata/test.dcm"); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	obj, err := NewDCMObjFromFile("../testdata/test.dcm")
	if err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(t.TempDir(), "out.dcm")
	if err := obj.WriteToFile(tmp); err != nil {
		t.Fatalf("WriteToFile() error: %v", err)
	}
	info, err := os.Stat(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("WriteToFile() produced empty file")
	}
}

func TestDICOMObject_WriteToFile_RequiresTransferSyntax(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.101")

	err := obj.WriteToFile(filepath.Join(t.TempDir(), "invalid-no-ts.dcm"))
	if err == nil {
		t.Fatal("WriteToFile() error = nil, want missing transfer syntax error")
	}
}

func TestDICOMObject_WriteToFile_RequiresSOPClassUID(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.102")

	err := obj.WriteToFile(filepath.Join(t.TempDir(), "invalid-no-sopclass.dcm"))
	if err == nil {
		t.Fatal("WriteToFile() error = nil, want missing SOP Class UID error")
	}
}

func TestDICOMObject_WriteToFile_RequiresSOPInstanceUID(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")

	err := obj.WriteToFile(filepath.Join(t.TempDir(), "invalid-no-sopinstance.dcm"))
	if err == nil {
		t.Fatal("WriteToFile() error = nil, want missing SOP Instance UID error")
	}
}

func TestDICOMObject_WriteDate(t *testing.T) {
	obj := NewEmptyDCMObj()
	d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	obj.Write(tags.StudyDate, d)
	got := obj.GetString(tags.StudyDate)
	if got != "20240615" {
		t.Fatalf("WriteDate() got %q, want 20240615", got)
	}
}

func TestDICOMObject_WriteDateRange(t *testing.T) {
	obj := NewEmptyDCMObj()
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	obj.Write(tags.StudyDate, DateRange{start, end})
	got := obj.GetString(tags.StudyDate)
	if got != "20240101-20241231" {
		t.Fatalf("WriteDateRange() got %q", got)
	}
}

func TestDICOMObject_WriteTime(t *testing.T) {
	obj := NewEmptyDCMObj()
	d := time.Date(2024, 1, 1, 14, 30, 5, 0, time.UTC)
	obj.Write(tags.StudyTime, d)
	got := obj.GetString(tags.StudyTime)
	if got != "143005" {
		t.Fatalf("WriteTime() got %q, want 143005", got)
	}
}

func TestDICOMObject_WriteUint32(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.Write(tags.FileMetaInformationGroupLength, uint32(0xDEAD))
	v := obj.GetUint32(tags.FileMetaInformationGroupLength)
	if v != 0xDEAD {
		t.Fatalf("WriteUint32() roundtrip = 0x%X", v)
	}
}

func TestDICOMObject_DumpTags_DoesNotPanic(t *testing.T) {
	if _, err := os.Stat("../testdata/test.dcm"); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	obj, err := NewDCMObjFromFile("../testdata/test.dcm")
	if err != nil {
		t.Fatal(err)
	}
	// Just verifying it doesn't panic.
	obj.DumpTags(io.Discard)
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

// Verify the UL formatTagValue path via GetUInt then via a tag with VR=UL
func TestDICOMObject_formatTagValue_UL(t *testing.T) {
	obj := NewEmptyDCMObj()
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 1024)
	obj.Add(&DICOMTag{Group: 0x0028, Element: 0x0120, VR: "UL", Length: 4, Data: data})
	// DumpTags exercises formatTagValue for UL
	obj.DumpTags(io.Discard)
}
