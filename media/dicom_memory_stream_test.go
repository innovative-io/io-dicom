package media

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"os"
	"strings"
	"testing"
)

func TestMemoryStream_GetByte(t *testing.T) {
	ms := NewMemoryStreamFromBytes([]byte{0xAB, 0xCD})
	b, err := ms.GetByte()
	if err != nil || b != 0xAB {
		t.Fatalf("GetByte() = %v, %v; want 0xAB, nil", b, err)
	}
	b, err = ms.GetByte()
	if err != nil || b != 0xCD {
		t.Fatalf("GetByte() = %v, %v; want 0xCD, nil", b, err)
	}
	_, err = ms.GetByte()
	if err == nil {
		t.Fatal("GetByte() should return error at end of stream")
	}
}

func TestMemoryStream_GetUint16(t *testing.T) {
	ms := NewMemoryStreamFromBytes([]byte{0x00, 0x01, 0xFF, 0xFE})
	v, err := ms.GetUint16()
	if err != nil || v != 0x0001 {
		t.Fatalf("GetUint16() = %v, %v", v, err)
	}
	v, err = ms.GetUint16()
	if err != nil || v != 0xFFFE {
		t.Fatalf("GetUint16() = %v, %v", v, err)
	}
	// only 0 bytes left — should error
	_, err = ms.GetUint16()
	if err == nil {
		t.Fatal("GetUint16() should error past end")
	}
}

func TestMemoryStream_GetUint32(t *testing.T) {
	ms := NewMemoryStreamFromBytes([]byte{0x00, 0x00, 0x01, 0x02})
	v, err := ms.GetUint32()
	if err != nil || v != 0x00000102 {
		t.Fatalf("GetUint32() = %v, %v", v, err)
	}
	_, err = ms.GetUint32()
	if err == nil {
		t.Fatal("GetUint32() should error past end")
	}
}

func TestMemoryStream_Get(t *testing.T) {
	ms := NewMemoryStreamFromBytes([]byte{42})
	v, err := ms.Get()
	if err != nil || v != 42 {
		t.Fatalf("Get() = %v, %v", v, err)
	}
	_, err = ms.Get()
	if err == nil {
		t.Fatal("Get() should error past end")
	}
}

func TestMemoryStream_ReadData(t *testing.T) {
	ms := NewMemoryStreamFromBytes([]byte{1, 2, 3, 4, 5})
	dst := make([]byte, 3)
	if err := ms.ReadData(dst); err != nil {
		t.Fatal(err)
	}
	if dst[0] != 1 || dst[1] != 2 || dst[2] != 3 {
		t.Fatalf("ReadData() got %v", dst)
	}
	oversized := make([]byte, 10)
	if err := ms.ReadData(oversized); err == nil {
		t.Fatal("ReadData() should error when not enough data")
	}
}

func TestMemoryStream_ReadSlice(t *testing.T) {
	raw := []byte{0x01, 0x02, 0x03, 0x04}
	ms := NewMemoryStreamFromBytes(raw)
	sl, err := ms.ReadSlice(2)
	if err != nil || string(sl) != "\x01\x02" {
		t.Fatalf("ReadSlice(2) = %v, %v", sl, err)
	}
	if &sl[0] != &raw[0] {
		t.Fatal("ReadSlice should alias backing buffer")
	}
	sl2, err := ms.ReadSlice(2)
	if err != nil || string(sl2) != "\x03\x04" {
		t.Fatalf("ReadSlice second = %v", sl2)
	}
	_, err = ms.ReadSlice(1)
	if err == nil {
		t.Fatal("ReadSlice past end should error")
	}
}

func TestMemoryStream_ReadUint16Endian(t *testing.T) {
	ms := NewMemoryStreamFromBytes([]byte{0x01, 0x02, 0x03, 0x04})
	v, err := ms.ReadUint16Endian(false)
	if err != nil || v != binary.LittleEndian.Uint16([]byte{0x01, 0x02}) {
		t.Fatalf("LE ReadUint16 = %v, %v", v, err)
	}
	ms.SetPosition(0)
	v, err = ms.ReadUint16Endian(true)
	if err != nil || v != binary.BigEndian.Uint16([]byte{0x01, 0x02}) {
		t.Fatalf("BE ReadUint16 = %v, %v", v, err)
	}
}

func TestMemoryStream_ReadFully(t *testing.T) {
	data := []byte("hello world")
	buf := bytes.NewBuffer(data)
	rw := bufio.NewReadWriter(bufio.NewReader(buf), bufio.NewWriter(&bytes.Buffer{}))
	ms := NewEmptyMemoryStream()
	if err := ms.ReadFully(rw, len(data)); err != nil {
		t.Fatal(err)
	}
	if ms.GetSize() != len(data) {
		t.Fatalf("ReadFully() size = %d, want %d", ms.GetSize(), len(data))
	}
	if string(ms.GetData()) != "hello world" {
		t.Fatalf("ReadFully() data = %q", ms.GetData())
	}
}

func TestMemoryStream_Append(t *testing.T) {
	ms := NewEmptyMemoryStream()
	n, err := ms.Append([]byte{1, 2, 3})
	if err != nil || n != 3 {
		t.Fatalf("Append() = %v, %v", n, err)
	}
	if ms.GetSize() != 3 {
		t.Fatalf("Append() size = %d", ms.GetSize())
	}
	// empty append is a no-op
	n, err = ms.Append([]byte{})
	if err != nil || n != 0 {
		t.Fatalf("Append(empty) = %v, %v", n, err)
	}
}

func TestMemoryStream_Clear(t *testing.T) {
	ms := NewMemoryStreamFromBytes([]byte{1, 2, 3})
	ms.SetPosition(2)
	ms.Clear()
	if ms.GetSize() != 0 || ms.GetPosition() != 0 {
		t.Fatalf("Clear() size=%d pos=%d", ms.GetSize(), ms.GetPosition())
	}
}

func TestMemoryStream_NewFromFile_Error(t *testing.T) {
	_, err := NewMemoryStreamFromFile("/nonexistent/path/does_not_exist.dcm")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMemoryStream_NewFromFile_Success(t *testing.T) {
	if _, err := os.Stat("../samples/test.dcm"); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	ms, err := NewMemoryStreamFromFile("../samples/test.dcm")
	if err != nil {
		t.Fatal(err)
	}
	if ms.GetSize() == 0 {
		t.Fatal("expected non-empty MemoryStream from file")
	}
}

func TestMemoryStream_Write_Errors(t *testing.T) {
	ms := NewEmptyMemoryStream()
	// zero count is ok
	n, err := ms.Write([]byte{1}, 0)
	if err != nil || n != 0 {
		t.Fatalf("Write(0) = %v, %v", n, err)
	}
	// negative count
	_, err = ms.Write([]byte{1, 2}, -1)
	if err == nil {
		t.Fatal("Write(-1) should return error")
	}
	// count > len(buffer)
	_, err = ms.Write([]byte{1}, 5)
	if err == nil {
		t.Fatal("Write(count>len) should return error")
	}
	// empty buffer
	_, err = ms.Write([]byte{}, 1)
	if err == nil {
		t.Fatal("Write(empty buf) should return error")
	}
}

// Compile-time check: _ = suppresses "declared but not used" from the import.
var _ = strings.NewReader
