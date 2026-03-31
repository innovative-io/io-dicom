package media

import (
	"encoding/binary"
	"testing"
)

func TestDICOMTag_GetUInt_LittleEndian(t *testing.T) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 0x12345678)
	tag := &DICOMTag{Length: 4, Data: data, BigEndian: false}
	if got := tag.GetUInt(); got != 0x12345678 {
		t.Fatalf("GetUInt() = 0x%X, want 0x12345678", got)
	}
}

func TestDICOMTag_GetUInt_BigEndian(t *testing.T) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, 0xDEADBEEF)
	tag := &DICOMTag{Length: 4, Data: data, BigEndian: true}
	if got := tag.GetUInt(); got != 0xDEADBEEF {
		t.Fatalf("GetUInt() BE = 0x%X", got)
	}
}

func TestDICOMTag_GetUInt_WrongLength(t *testing.T) {
	tag := &DICOMTag{Length: 2, Data: []byte{0x01, 0x02}}
	if got := tag.GetUInt(); got != 0 {
		t.Fatalf("GetUInt() with 2-byte data should return 0, got %d", got)
	}
}

func TestDICOMTag_GetFloat_Valid(t *testing.T) {
	tag := &DICOMTag{Length: 4, Data: []byte("3.14"), VR: "FL"}
	v := tag.GetFloat()
	if v < 3.13 || v > 3.15 {
		t.Fatalf("GetFloat() = %v", v)
	}
}

func TestDICOMTag_GetFloat_Invalid(t *testing.T) {
	tag := &DICOMTag{Length: 4, Data: []byte("nope"), VR: "FL"}
	v := tag.GetFloat()
	if v != 0 {
		t.Fatalf("GetFloat() invalid = %v, want 0", v)
	}
}

func TestDICOMTag_GetUShort_BigEndian(t *testing.T) {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 0xABCD)
	tag := &DICOMTag{Length: 2, Data: data, BigEndian: true}
	if got := tag.GetUShort(); got != 0xABCD {
		t.Fatalf("GetUShort() BE = 0x%X", got)
	}
}

func TestDICOMTag_GetUShort_WrongLength(t *testing.T) {
	tag := &DICOMTag{Length: 0, Data: []byte{}}
	if got := tag.GetUShort(); got != 0 {
		t.Fatalf("GetUShort() with length 0 should return 0, got %d", got)
	}
}
