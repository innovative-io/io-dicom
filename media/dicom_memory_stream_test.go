package media

import (
	"bufio"
	"bytes"
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

func TestMemoryStream_GetInt(t *testing.T) {
	ms := NewMemoryStreamFromBytes([]byte{0x00, 0x00, 0x00, 0x07})
	v, err := ms.GetInt()
	if err != nil || v != 7 {
		t.Fatalf("GetInt() = %v, %v", v, err)
	}
	_, err = ms.GetInt()
	if err == nil {
		t.Fatal("GetInt() should error past end")
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

func TestMemoryStream_SetSize(t *testing.T) {
	ms := NewMemoryStreamFromBytes([]byte{1, 2, 3, 4})
	if ms.GetSize() != 4 {
		t.Fatalf("initial size = %d", ms.GetSize())
	}
	ms.SetSize(2)
	if ms.GetSize() != 2 {
		t.Fatalf("after SetSize(2) size = %d", ms.GetSize())
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
