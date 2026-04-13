package media

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// MemoryStream - is an interface to a memory stream
type MemoryStream interface {
	GetData() []byte
	Get() (int, error)
	GetByte() (byte, error)
	GetUint16() (uint16, error)
	GetUint32() (uint32, error)
	GetPosition() int
	SetPosition(position int)
	GetSize() int
	Append(data []byte) (int, error)
	ReadData(input []byte) error
	Read(count int) ([]byte, error)
	// ReadSlice returns the next n bytes as a view into the stream buffer and advances the position.
	// The slice aliases stream memory; it remains valid for the lifetime of the underlying backing array.
	ReadSlice(n int) ([]byte, error)
	ReadUint16Endian(bigEndian bool) (uint16, error)
	ReadUint32Endian(bigEndian bool) (uint32, error)
	ReadFully(rw *bufio.ReadWriter, length int) error
	Write(buffer []byte, count int) (int, error)
	Clear()
}

type memoryStream struct {
	Data     []byte
	Position int
	Size     int
}

// NewEmptyMemoryStream - Creates an interface to a new empty memoryStream
func NewEmptyMemoryStream() MemoryStream {
	return newEmptyMemoryStream()
}

// newEmptyMemoryStream returns the concrete *memoryStream for intra-package use
// where the concrete type avoids interface-call escape of stack variables.
func newEmptyMemoryStream() *memoryStream {
	return &memoryStream{
		Data:     make([]byte, 0, 4096),
		Position: 0,
		Size:     0,
	}
}

// newMemoryStreamWithCapacity returns the concrete *memoryStream for intra-package use.
func newMemoryStreamWithCapacity(capacity int) *memoryStream {
	if capacity < 4096 {
		capacity = 4096
	}
	return &memoryStream{
		Data:     make([]byte, 0, capacity),
		Position: 0,
		Size:     0,
	}
}

// NewMemoryStreamFromBytes - Creates an interface to a new memoryStream from bytes
func NewMemoryStreamFromBytes(data []byte) MemoryStream {
	return &memoryStream{
		Data:     data,
		Position: 0,
		Size:     len(data),
	}
}

// NewMemoryStreamFromFile - Creates an interface to a new memoryStream from file
func NewMemoryStreamFromFile(fileName string) (MemoryStream, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	return &memoryStream{
		Data:     data,
		Position: 0,
		Size:     len(data),
	}, nil
}

func (ms *memoryStream) GetByte() (byte, error) {
	if ms.Position >= ms.Size {
		return 0, errors.New("no more data to read")
	}
	b := ms.Data[ms.Position]
	ms.Position++
	return b, nil
}

func (ms *memoryStream) GetUint16() (uint16, error) {
	if ms.Position+1 >= ms.Size {
		return 0, errors.New("no more data to read")
	}
	b := ms.Data[ms.Position : ms.Position+2]
	ms.Position += 2
	return binary.BigEndian.Uint16(b), nil
}

func (ms *memoryStream) GetUint32() (uint32, error) {
	if ms.Position+3 >= ms.Size {
		return 0, errors.New("no more data to read")
	}
	b := ms.Data[ms.Position : ms.Position+4]
	ms.Position += 4
	return binary.BigEndian.Uint32(b), nil
}

func (ms *memoryStream) Get() (int, error) {
	if ms.Position >= ms.Size {
		return 0, errors.New("no more data to read")
	}
	b := ms.Data[ms.Position]
	ms.Position++
	return int(b), nil
}

func (ms *memoryStream) ReadData(dst []byte) error {
	if ms.Position+len(dst) > ms.Size {
		return errors.New("no more data to read")
	}
	copy(dst, ms.Data[ms.Position:ms.Position+len(dst)])
	ms.Position += len(dst)
	return nil
}

func (ms *memoryStream) ReadFully(rw *bufio.ReadWriter, length int) error {
	data := make([]byte, length)
	if _, err := io.ReadFull(rw, data); err != nil {
		return err
	}
	ms.Data = append(ms.Data, data...)
	ms.Size += length
	return nil
}

func (ms *memoryStream) GetData() []byte {
	return ms.Data[:ms.Size]
}

func (ms *memoryStream) GetPosition() int {
	return ms.Position
}

func (ms *memoryStream) SetPosition(position int) {
	ms.Position = position
}

func (ms *memoryStream) GetSize() int {
	return ms.Size
}

// Read - Read from MemoryStream into Buffer count bytes
func (ms *memoryStream) Read(count int) ([]byte, error) {
	if count+ms.Position > ms.Size {
		return nil, errors.New("MemoryStream::Read, count+ms.Position > ms.Size")
	}
	buffer := make([]byte, count)
	copy(buffer, ms.Data[ms.Position:ms.Position+count])
	ms.Position = ms.Position + count
	return buffer, nil
}

// ReadSlice returns a subslice view without copying; see MemoryStream.ReadSlice.
func (ms *memoryStream) ReadSlice(n int) ([]byte, error) {
	if n+ms.Position > ms.Size {
		return nil, errors.New("MemoryStream::ReadSlice, n+ms.Position > ms.Size")
	}
	start := ms.Position
	ms.Position += n
	return ms.Data[start : start+n], nil
}

func (ms *memoryStream) ReadUint16Endian(bigEndian bool) (uint16, error) {
	if ms.Position+2 > ms.Size {
		return 0, errors.New("MemoryStream::ReadUint16Endian: not enough data")
	}
	b := ms.Data[ms.Position : ms.Position+2]
	ms.Position += 2
	if bigEndian {
		return binary.BigEndian.Uint16(b), nil
	}
	return binary.LittleEndian.Uint16(b), nil
}

func (ms *memoryStream) ReadUint32Endian(bigEndian bool) (uint32, error) {
	if ms.Position+4 > ms.Size {
		return 0, errors.New("MemoryStream::ReadUint32Endian: not enough data")
	}
	b := ms.Data[ms.Position : ms.Position+4]
	ms.Position += 4
	if bigEndian {
		return binary.BigEndian.Uint32(b), nil
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (ms *memoryStream) Append(data []byte) (int, error) {
	count := len(data)
	if count == 0 {
		return 0, nil
	}

	if ms.Data == nil {
		return -1, errors.New("MemoryStream::Append, no Data to append to")
	}

	ms.Data = append(ms.Data, data...)
	ms.Size += count

	return count, nil
}

// Write - Write from Buffer into MemoryStream count bytes
func (ms *memoryStream) Write(buffer []byte, count int) (int, error) {
	if count == 0 {
		return 0, nil
	}

	if len(buffer) == 0 {
		return -1, errors.New("MemoryStream::Write, nothing to write")
	}

	if count < 0 || count > len(buffer) {
		return -1, errors.New("MemoryStream::Write, invalid count")
	}

	if ms.Data == nil {
		return -1, errors.New("MemoryStream::Write, no Data to append to")
	}

	endPos := ms.Position + count

	if endPos > len(ms.Data) {
		if endPos <= cap(ms.Data) {
			// Extend within pre-allocated capacity — zero allocation.
			ms.Data = ms.Data[:endPos]
		} else {
			// Must grow beyond current capacity; fall back to append.
			grow := endPos - len(ms.Data)
			ms.Data = append(ms.Data, make([]byte, grow)...)
		}
	}

	copy(ms.Data[ms.Position:endPos], buffer[:count])

	if endPos > ms.Size {
		ms.Size = endPos
	}
	ms.Position = endPos
	return count, nil
}

// Clear - Clears the memoryStream
func (ms *memoryStream) Clear() {
	ms.Data = ms.Data[:0]
	ms.Position = 0
	ms.Size = 0
}
