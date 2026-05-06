package media

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDICOMTag_GetUInt_LittleEndian(t *testing.T) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 0x12345678)
	tag := &DICOMTag{Length: 4, Data: data, BigEndian: false}
	if got := tag.GetUint32(); got != 0x12345678 {
		t.Fatalf("GetUInt() = 0x%X, want 0x12345678", got)
	}
}

func TestDICOMTag_GetUInt_BigEndian(t *testing.T) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, 0xDEADBEEF)
	tag := &DICOMTag{Length: 4, Data: data, BigEndian: true}
	if got := tag.GetUint32(); got != 0xDEADBEEF {
		t.Fatalf("GetUInt() BE = 0x%X", got)
	}
}

func TestDICOMTag_GetUInt_WrongLength(t *testing.T) {
	tag := &DICOMTag{Length: 2, Data: []byte{0x01, 0x02}}
	if got := tag.GetUint32(); got != 0 {
		t.Fatalf("GetUInt() with 2-byte data should return 0, got %d", got)
	}
}

func TestDICOMTag_GetUInt_TruncatedData(t *testing.T) {
	tag := &DICOMTag{Length: 4, Data: []byte{0x01, 0x02, 0x03}}
	if got := tag.GetUint32(); got != 0 {
		t.Fatalf("GetUInt() with truncated data should return 0, got %d", got)
	}
}

func TestDICOMTag_GetFloat_Valid(t *testing.T) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, math.Float32bits(3.14))
	tag := &DICOMTag{Length: 4, Data: data, VR: "FL", BigEndian: false}
	v := tag.GetFloat()
	if v < 3.13 || v > 3.15 {
		t.Fatalf("GetFloat() = %v", v)
	}
}

func TestDICOMTag_GetFloat_TextFallback(t *testing.T) {
	tag := &DICOMTag{Length: 4, Data: []byte("3.14"), VR: "DS"}
	v := tag.GetFloat()
	if v < 3.13 || v > 3.15 {
		t.Fatalf("GetFloat() DS fallback = %v", v)
	}
}

func TestDICOMTag_GetFloat_Invalid(t *testing.T) {
	tag := &DICOMTag{Length: 4, Data: []byte{0x01}, VR: "FL"}
	v := tag.GetFloat()
	if v != 0 {
		t.Fatalf("GetFloat() invalid = %v, want 0", v)
	}
}

func TestDICOMTag_GetFloat64_Valid(t *testing.T) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, math.Float64bits(2.718281828))
	tag := &DICOMTag{Length: 8, Data: data, VR: "FD", BigEndian: false}
	if got := tag.GetFloat64(); got < 2.718 || got > 2.719 {
		t.Fatalf("GetFloat64() = %v", got)
	}
}

func TestDICOMTag_GetUShort_BigEndian(t *testing.T) {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 0xABCD)
	tag := &DICOMTag{Length: 2, Data: data, BigEndian: true}
	if got := tag.GetUint16(); got != 0xABCD {
		t.Fatalf("GetUShort() BE = 0x%X", got)
	}
}

func TestDICOMTag_GetUShort_WrongLength(t *testing.T) {
	tag := &DICOMTag{Length: 0, Data: []byte{}}
	if got := tag.GetUint16(); got != 0 {
		t.Fatalf("GetUShort() with length 0 should return 0, got %d", got)
	}
}

func TestDICOMTag_GetUShort_TruncatedData(t *testing.T) {
	tag := &DICOMTag{Length: 2, Data: []byte{0x01}}
	if got := tag.GetUint16(); got != 0 {
		t.Fatalf("GetUShort() with truncated data should return 0, got %d", got)
	}
}

func TestDICOMTag_GetString_TruncatedData(t *testing.T) {
	tag := &DICOMTag{Length: 8, Data: []byte("ABC")}
	if got := tag.GetString(); got != "ABC" {
		t.Fatalf("GetString() with truncated data = %q, want %q", got, "ABC")
	}
}

func TestDICOMTag_GetString_RespectsTagLength(t *testing.T) {
	tag := &DICOMTag{Length: 3, Data: []byte("ABCDE")}
	if got := tag.GetString(); got != "ABC" {
		t.Fatalf("GetString() should clamp to tag length, got %q", got)
	}
}

// TestReadSeq_TruncatedData verifies that ReadSeq does not loop infinitely when
// the embedded sequence buffer is truncated mid-tag.  Prior to the fix, ReadTag
// returning an error left the stream position unchanged, causing an infinite loop.
func TestReadSeq_TruncatedData(t *testing.T) {
	// Write one complete implicit-VR tag (group 0008, element 0060, length 2, data "CT")
	// followed by a deliberately truncated tag (header only, no data).
	buf := NewDICOMBuffer()
	// complete tag: (0008,0060) US length=2 data="CT"
	buf.WriteUint16(0x0008) // group
	buf.WriteUint16(0x0060) // element
	buf.WriteUint32(2)      // length
	buf.Write([]byte("CT"), 2)
	// truncated tag: group+element only, no length bytes
	buf.WriteUint16(0x0010)
	buf.WriteUint16(0x0010)

	tag := &DICOMTag{
		Group:   0x4000,
		Element: 0x0000,
		VR:      "SQ",
		Length:  uint32(buf.GetSize()),
		Data:    buf.GetAllBytes(),
	}

	// Must return without hanging. The truncated trailing tag is silently dropped.
	seq := tag.ReadSeq(false /* implicit VR */)
	if seq == nil {
		t.Fatal("ReadSeq returned nil")
	}
	// Only the complete first tag should be present.
	if seq.TagCount() != 1 {
		t.Fatalf("ReadSeq returned %d tags, want 1", seq.TagCount())
	}
}
