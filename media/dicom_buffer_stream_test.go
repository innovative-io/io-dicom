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
	buf := NewDICOMBufferFromBytes([]byte{0xAB, 0xCD})
	b, err := buf.GetByte()
	if err != nil || b != 0xAB {
		t.Fatalf("GetByte() = %v, %v; want 0xAB, nil", b, err)
	}
	b, err = buf.GetByte()
	if err != nil || b != 0xCD {
		t.Fatalf("GetByte() = %v, %v; want 0xCD, nil", b, err)
	}
	_, err = buf.GetByte()
	if err == nil {
		t.Fatal("GetByte() should return error at end of stream")
	}
}

func TestMemoryStream_ReadUint16_BigEndian(t *testing.T) {
	buf := NewDICOMBufferFromBytes([]byte{0x00, 0x01, 0xFF, 0xFE})
	v, err := buf.ReadUint16(true)
	if err != nil || v != 0x0001 {
		t.Fatalf("ReadUint16(true) = %v, %v", v, err)
	}
	v, err = buf.ReadUint16(true)
	if err != nil || v != 0xFFFE {
		t.Fatalf("ReadUint16(true) = %v, %v", v, err)
	}
	// only 0 bytes left — should error
	_, err = buf.ReadUint16(true)
	if err == nil {
		t.Fatal("ReadUint16(true) should error past end")
	}
}

func TestMemoryStream_ReadUint32_BigEndian(t *testing.T) {
	buf := NewDICOMBufferFromBytes([]byte{0x00, 0x00, 0x01, 0x02})
	v, err := buf.ReadUint32(true)
	if err != nil || v != 0x00000102 {
		t.Fatalf("ReadUint32(true) = %v, %v", v, err)
	}
	_, err = buf.ReadUint32(true)
	if err == nil {
		t.Fatal("ReadUint32(true) should error past end")
	}
}

func TestMemoryStream_GetByte_viaCast(t *testing.T) {
	buf := NewDICOMBufferFromBytes([]byte{42})
	v, err := buf.GetByte()
	if err != nil || int(v) != 42 {
		t.Fatalf("GetByte() = %v, %v", v, err)
	}
	_, err = buf.GetByte()
	if err == nil {
		t.Fatal("GetByte() should error past end")
	}
}

func TestMemoryStream_ReadData(t *testing.T) {
	buf := NewDICOMBufferFromBytes([]byte{1, 2, 3, 4, 5})
	dst := make([]byte, 3)
	if err := buf.ReadData(dst); err != nil {
		t.Fatal(err)
	}
	if dst[0] != 1 || dst[1] != 2 || dst[2] != 3 {
		t.Fatalf("ReadData() got %v", dst)
	}
	oversized := make([]byte, 10)
	if err := buf.ReadData(oversized); err == nil {
		t.Fatal("ReadData() should error when not enough data")
	}
}

func TestMemoryStream_ReadSlice(t *testing.T) {
	raw := []byte{0x01, 0x02, 0x03, 0x04}
	buf := NewDICOMBufferFromBytes(raw)
	sl, err := buf.ReadSlice(2)
	if err != nil || string(sl) != "\x01\x02" {
		t.Fatalf("ReadSlice(2) = %v, %v", sl, err)
	}
	if &sl[0] != &raw[0] {
		t.Fatal("ReadSlice should alias backing buffer")
	}
	sl2, err := buf.ReadSlice(2)
	if err != nil || string(sl2) != "\x03\x04" {
		t.Fatalf("ReadSlice second = %v", sl2)
	}
	_, err = buf.ReadSlice(1)
	if err == nil {
		t.Fatal("ReadSlice past end should error")
	}
}

func TestMemoryStream_ReadUint16(t *testing.T) {
	buf := NewDICOMBufferFromBytes([]byte{0x01, 0x02, 0x03, 0x04})
	v, err := buf.ReadUint16(false)
	if err != nil || v != binary.LittleEndian.Uint16([]byte{0x01, 0x02}) {
		t.Fatalf("LE ReadUint16 = %v, %v", v, err)
	}
	buf.SetPosition(0)
	v, err = buf.ReadUint16(true)
	if err != nil || v != binary.BigEndian.Uint16([]byte{0x01, 0x02}) {
		t.Fatalf("BE ReadUint16 = %v, %v", v, err)
	}
}

func TestMemoryStream_ReadFully(t *testing.T) {
	data := []byte("hello world")
	src := bytes.NewBuffer(data)
	rw := bufio.NewReadWriter(bufio.NewReader(src), bufio.NewWriter(&bytes.Buffer{}))
	buf := NewDICOMBuffer()
	if err := buf.ReadFully(rw, len(data)); err != nil {
		t.Fatal(err)
	}
	if buf.GetSize() != len(data) {
		t.Fatalf("ReadFully() size = %d, want %d", buf.GetSize(), len(data))
	}
	if string(buf.GetData()) != "hello world" {
		t.Fatalf("ReadFully() data = %q", buf.GetData())
	}
}

func TestMemoryStream_Append(t *testing.T) {
	buf := NewDICOMBuffer()
	n, err := buf.Append([]byte{1, 2, 3})
	if err != nil || n != 3 {
		t.Fatalf("Append() = %v, %v", n, err)
	}
	if buf.GetSize() != 3 {
		t.Fatalf("Append() size = %d", buf.GetSize())
	}
	// empty append is a no-op
	n, err = buf.Append([]byte{})
	if err != nil || n != 0 {
		t.Fatalf("Append(empty) = %v, %v", n, err)
	}
}

func TestMemoryStream_Clear(t *testing.T) {
	buf := NewDICOMBufferFromBytes([]byte{1, 2, 3})
	buf.SetPosition(2)
	buf.Clear()
	if buf.GetSize() != 0 || buf.GetPosition() != 0 {
		t.Fatalf("Clear() size=%d pos=%d", buf.GetSize(), buf.GetPosition())
	}
}

func TestMemoryStream_NewFromFile_Error(t *testing.T) {
	_, err := NewDICOMBufferFromFile("/nonexistent/path/does_not_exist.dcm")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMemoryStream_NewFromFile_Success(t *testing.T) {
	if _, err := os.Stat("../testdata/test.dcm"); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	buf, err := NewDICOMBufferFromFile("../testdata/test.dcm")
	if err != nil {
		t.Fatal(err)
	}
	if buf.GetSize() == 0 {
		t.Fatal("expected non-empty MemoryStream from file")
	}
}

func TestMemoryStream_Write_Errors(t *testing.T) {
	buf := NewDICOMBuffer()
	// zero count is ok
	n, err := buf.Write([]byte{1}, 0)
	if err != nil || n != 0 {
		t.Fatalf("Write(0) = %v, %v", n, err)
	}
	// negative count
	_, err = buf.Write([]byte{1, 2}, -1)
	if err == nil {
		t.Fatal("Write(-1) should return error")
	}
	// count > len(buffer)
	_, err = buf.Write([]byte{1}, 5)
	if err == nil {
		t.Fatal("Write(count>len) should return error")
	}
	// empty buffer
	_, err = buf.Write([]byte{}, 1)
	if err == nil {
		t.Fatal("Write(empty buf) should return error")
	}
}

// Compile-time check: _ = suppresses "declared but not used" from the import.
var _ = strings.NewReader
