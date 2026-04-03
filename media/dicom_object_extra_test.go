package media

import (
	"encoding/binary"
	"os"
	"path/filepath"
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
	obj.WriteString(tags.PatientName, "DOE^JOHN")
	obj.WriteString(tags.PatientID, "12345")
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

func TestDICOMObject_GetTagGE(t *testing.T) {
	obj := makeSimpleObj()
	tag := obj.GetTagGE(tags.PatientID.Group, tags.PatientID.Element)
	if tag == nil {
		t.Fatal("GetTagGE() returned nil")
	}
	if tag.GetString() != "12345" {
		t.Fatalf("GetTagGE() value = %q", tag.GetString())
	}
}

func TestDICOMObject_GetTagGE_Missing(t *testing.T) {
	obj := makeSimpleObj()
	if obj.GetTagGE(0x0020, 0x0010) != nil {
		t.Fatal("GetTagGE() should return nil for absent group/element")
	}
}

func TestDICOMObject_GetDate(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.WriteString(tags.StudyDate, "20230115")
	d := obj.GetDate(tags.StudyDate)
	if d.Year() != 2023 || d.Month() != 1 || d.Day() != 15 {
		t.Fatalf("GetDate() = %v", d)
	}
}

func TestDICOMObject_GetDate_Invalid(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.WriteString(tags.StudyDate, "notadate")
	d := obj.GetDate(tags.StudyDate)
	if !d.IsZero() {
		t.Fatalf("GetDate() invalid should return zero time, got %v", d)
	}
}

func TestDICOMObject_GetUShort(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.WriteUint16(tags.BitsAllocated, 16)
	v := obj.GetUShort(tags.BitsAllocated)
	if v != 16 {
		t.Fatalf("GetUShort() = %d, want 16", v)
	}
}

func TestDICOMObject_GetUShortGE(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.WriteUint16(tags.BitsAllocated, 8)
	v := obj.GetUShortGE(tags.BitsAllocated.Group, tags.BitsAllocated.Element)
	if v != 8 {
		t.Fatalf("GetUShortGE() = %d, want 8", v)
	}
}

func TestDICOMObject_GetUInt(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.WriteUint32(tags.PixelDataProviderURL, 0xCAFEBABE)
	v := obj.GetUInt(tags.PixelDataProviderURL)
	if v != 0xCAFEBABE {
		t.Fatalf("GetUInt() = 0x%X, want 0xCAFEBABE", v)
	}
}

func TestDICOMObject_GetUIntGE(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.WriteUint32(tags.PixelDataProviderURL, 99)
	v := obj.GetUIntGE(tags.PixelDataProviderURL.Group, tags.PixelDataProviderURL.Element)
	if v != 99 {
		t.Fatalf("GetUIntGE() = %d, want 99", v)
	}
}

func TestDICOMObject_WriteToFile(t *testing.T) {
	if _, err := os.Stat("../samples/test.dcm"); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	obj, err := NewDCMObjFromFile("../samples/test.dcm")
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
	obj.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.WriteString(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.101")

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
	obj.WriteString(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.102")

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
	obj.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")

	err := obj.WriteToFile(filepath.Join(t.TempDir(), "invalid-no-sopinstance.dcm"))
	if err == nil {
		t.Fatal("WriteToFile() error = nil, want missing SOP Instance UID error")
	}
}

func TestDICOMObject_WriteDate(t *testing.T) {
	obj := NewEmptyDCMObj()
	d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	obj.WriteDate(tags.StudyDate, d)
	got := obj.GetString(tags.StudyDate)
	if got != "20240615" {
		t.Fatalf("WriteDate() got %q, want 20240615", got)
	}
}

func TestDICOMObject_WriteDateRange(t *testing.T) {
	obj := NewEmptyDCMObj()
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	obj.WriteDateRange(tags.StudyDate, start, end)
	got := obj.GetString(tags.StudyDate)
	if got != "20240101-20241231" {
		t.Fatalf("WriteDateRange() got %q", got)
	}
}

func TestDICOMObject_WriteTime(t *testing.T) {
	obj := NewEmptyDCMObj()
	d := time.Date(2024, 1, 1, 14, 30, 5, 0, time.UTC)
	obj.WriteTime(tags.StudyTime, d)
	got := obj.GetString(tags.StudyTime)
	if got != "143005" {
		t.Fatalf("WriteTime() got %q, want 143005", got)
	}
}

func TestDICOMObject_WriteUint32(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.WriteUint32(tags.PixelDataProviderURL, 0xDEAD)
	v := obj.GetUInt(tags.PixelDataProviderURL)
	if v != 0xDEAD {
		t.Fatalf("WriteUint32() roundtrip = 0x%X", v)
	}
}

func TestDICOMObject_WriteUint32GE(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.WriteUint32GE(0x0028, 0x0010, "UL", 512)
	v := obj.GetUIntGE(0x0028, 0x0010)
	if v != 512 {
		t.Fatalf("WriteUint32GE() roundtrip = %d", v)
	}
}

func TestDICOMObject_DumpTags_DoesNotPanic(t *testing.T) {
	if _, err := os.Stat("../samples/test.dcm"); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	obj, err := NewDCMObjFromFile("../samples/test.dcm")
	if err != nil {
		t.Fatal(err)
	}
	// Just verifying it doesn't panic.
	obj.DumpTags()
}

// Verify the UL formatTagValue path via GetUIntGE then via a tag with VR=UL
func TestDICOMObject_formatTagValue_UL(t *testing.T) {
	obj := NewEmptyDCMObj()
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 1024)
	obj.Add(&DICOMTag{Group: 0x0028, Element: 0x0120, VR: "UL", Length: 4, Data: data})
	// DumpTags exercises formatTagValue for UL
	obj.DumpTags()
}
