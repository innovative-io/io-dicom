package media

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Stream-level methods on DICOMBuffer.
// See dicom_buffer.go for the DICOMBuffer struct definition, constructors, and
// DICOM-specific methods (ReadTag, WriteTag, ReadMeta, WriteMeta, etc.).

func (buf *DICOMBuffer) GetByte() (byte, error) {
	if buf.position >= buf.size {
		return 0, fmt.Errorf("stream: read 1 byte at position %d, only %d byte(s) remain", buf.position, buf.size-buf.position)
	}
	b := buf.data[buf.position]
	buf.position++
	return b, nil
}

func (buf *DICOMBuffer) ReadData(dst []byte) error {
	if buf.position+len(dst) > buf.size {
		return fmt.Errorf("stream: read %d bytes at position %d, only %d byte(s) remain", len(dst), buf.position, buf.size-buf.position)
	}
	copy(dst, buf.data[buf.position:buf.position+len(dst)])
	buf.position += len(dst)
	return nil
}

// ReadFully reads exactly length bytes from rw into this buffer, growing the
// backing array in-place to avoid an intermediate allocation.
func (buf *DICOMBuffer) ReadFully(rw *bufio.ReadWriter, length int) error {
	start := len(buf.data)
	end := start + length
	if end > cap(buf.data) {
		grown := make([]byte, end)
		copy(grown, buf.data)
		buf.data = grown
	} else {
		buf.data = buf.data[:end]
	}
	if _, err := io.ReadFull(rw, buf.data[start:end]); err != nil {
		buf.data = buf.data[:start] // roll back on error
		return err
	}
	buf.size += length
	return nil
}

func (buf *DICOMBuffer) GetData() []byte {
	return buf.data[:buf.size]
}

func (buf *DICOMBuffer) GetPosition() int {
	return buf.position
}

func (buf *DICOMBuffer) SetPosition(position int) {
	buf.position = position
}

func (buf *DICOMBuffer) GetSize() int {
	return buf.size
}

// Read copies count bytes from the current position into a new slice.
func (buf *DICOMBuffer) Read(count int) ([]byte, error) {
	if count+buf.position > buf.size {
		return nil, fmt.Errorf("stream: read %d bytes at position %d, only %d byte(s) remain", count, buf.position, buf.size-buf.position)
	}
	buffer := make([]byte, count)
	copy(buffer, buf.data[buf.position:buf.position+count])
	buf.position += count
	return buffer, nil
}

// ReadSlice returns the next n bytes as a zero-copy view into the backing array
// and advances the position. The slice is valid for the lifetime of the backing
// array; callers must not hold it across a Reset or Clear call.
func (buf *DICOMBuffer) ReadSlice(n int) ([]byte, error) {
	if n+buf.position > buf.size {
		return nil, fmt.Errorf("stream: read %d bytes at position %d, only %d byte(s) remain", n, buf.position, buf.size-buf.position)
	}
	start := buf.position
	buf.position += n
	return buf.data[start : start+n], nil
}

// ReadUint16 reads a uint16 from the stream using the specified byte order.
func (buf *DICOMBuffer) ReadUint16(bigEndian bool) (uint16, error) {
	if buf.position+2 > buf.size {
		return 0, fmt.Errorf("stream: read 2 bytes at position %d, only %d byte(s) remain", buf.position, buf.size-buf.position)
	}
	b := buf.data[buf.position : buf.position+2]
	buf.position += 2
	if bigEndian {
		return binary.BigEndian.Uint16(b), nil
	}
	return binary.LittleEndian.Uint16(b), nil
}

// ReadUint32 reads a uint32 from the stream using the specified byte order.
func (buf *DICOMBuffer) ReadUint32(bigEndian bool) (uint32, error) {
	if buf.position+4 > buf.size {
		return 0, fmt.Errorf("stream: read 4 bytes at position %d, only %d byte(s) remain", buf.position, buf.size-buf.position)
	}
	b := buf.data[buf.position : buf.position+4]
	buf.position += 4
	if bigEndian {
		return binary.BigEndian.Uint32(b), nil
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (buf *DICOMBuffer) Append(data []byte) (int, error) {
	count := len(data)
	if count == 0 {
		return 0, nil
	}

	if buf.data == nil {
		return -1, errors.New("DICOMBuffer::Append, no data to append to")
	}

	buf.data = append(buf.data, data...)
	buf.size += count

	return count, nil
}

// Write copies count bytes from buffer into this DICOMBuffer at the current position.
func (buf *DICOMBuffer) Write(buffer []byte, count int) (int, error) {
	if count == 0 {
		return 0, nil
	}

	if len(buffer) == 0 {
		return -1, errors.New("DICOMBuffer::Write, nothing to write")
	}

	if count < 0 || count > len(buffer) {
		return -1, errors.New("DICOMBuffer::Write, invalid count")
	}

	if buf.data == nil {
		return -1, errors.New("DICOMBuffer::Write, no data to append to")
	}

	endPos := buf.position + count

	if endPos > len(buf.data) {
		if endPos <= cap(buf.data) {
			// Extend within pre-allocated capacity — zero allocation.
			buf.data = buf.data[:endPos]
		} else {
			// Must grow beyond current capacity; fall back to append.
			grow := endPos - len(buf.data)
			buf.data = append(buf.data, make([]byte, grow)...)
		}
	}

	copy(buf.data[buf.position:endPos], buffer[:count])

	if endPos > buf.size {
		buf.size = endPos
	}
	buf.position = endPos
	return count, nil
}

// Clear resets position and size to zero, preserving the backing array capacity.
func (buf *DICOMBuffer) Clear() {
	buf.data = buf.data[:0]
	buf.position = 0
	buf.size = 0
}

// Reset discards the backing array so the next write allocates fresh memory.
// Use this instead of Clear when callers may hold ReadSlice sub-slices that
// alias the current backing array (e.g. tag.Data from a received PDU).
func (buf *DICOMBuffer) Reset() {
	buf.data = buf.data[:0:0] // zero-length, zero-capacity: forces fresh allocation on next grow
	buf.position = 0
	buf.size = 0
}
