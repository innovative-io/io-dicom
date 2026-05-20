package media

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	implementation "github.com/innovative-io/io-dicom/internal/implclass"
)

// DICOMBuffer is a DICOM-aware byte buffer that combines raw stream operations
// (sequential byte/integer reads and writes) with DICOM-specific operations
// (reading and writing DICOM tags, meta-information, and objects).
//
// Stream-level methods (GetByte, Write, ReadSlice, etc.) are in
// dicom_buffer_stream.go; DICOM-level methods are in this file.
type DICOMBuffer struct {
	bigEndian bool
	data      []byte
	position  int
	size      int
}

// NewDICOMBuffer creates an empty DICOMBuffer with a 4096-byte pre-allocated buffer.
func NewDICOMBuffer() *DICOMBuffer {
	return &DICOMBuffer{data: make([]byte, 0, 4096)}
}

// NewDICOMBufferWithCapacity creates an empty DICOMBuffer with the given pre-allocated
// capacity, avoiding early growth reallocations when the expected output size
// is known in advance.
func NewDICOMBufferWithCapacity(capacity int) *DICOMBuffer {
	if capacity < 4096 {
		capacity = 4096
	}
	return &DICOMBuffer{data: make([]byte, 0, capacity)}
}

// NewDICOMBufferFromBytes wraps existing byte data for reading.
func NewDICOMBufferFromBytes(data []byte) *DICOMBuffer {
	return &DICOMBuffer{data: data, size: len(data)}
}

// NewDICOMBufferFromFile reads an entire file into a DICOMBuffer for reading.
func NewDICOMBufferFromFile(fileName string) (*DICOMBuffer, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return &DICOMBuffer{data: data, size: len(data)}, nil
}

func (buf *DICOMBuffer) IsBigEndian() bool {
	return buf.bigEndian
}

func (buf *DICOMBuffer) SetBigEndian(isBigEndian bool) {
	buf.bigEndian = isBigEndian
}

func (buf *DICOMBuffer) WriteAETitle(aeTitle string) {
	// DICOM PS3.8 §9.3.2: AE titles are fixed 16-byte fields, space-padded on
	// the right. Truncate silently to prevent writing malformed PDUs.
	const aeLen = 16
	if len(aeTitle) > aeLen {
		aeTitle = aeTitle[:aeLen]
	}
	endPos := buf.GetPosition() + aeLen
	buf.WriteString(aeTitle)
	pad := [1]byte{0x20}
	for buf.GetPosition() < endPos {
		buf.Write(pad[:], 1)
	}
}

// WriteByte writes a byte
func (buf *DICOMBuffer) WriteByte(value byte) error {
	b := [1]byte{value}
	_, err := buf.Write(b[:], 1)
	return err
}

// WriteUint16 writes an unsigned int
func (buf *DICOMBuffer) WriteUint16(value uint16) {
	var c [2]byte
	if buf.bigEndian {
		binary.BigEndian.PutUint16(c[:], value)
	} else {
		binary.LittleEndian.PutUint16(c[:], value)
	}
	buf.Write(c[:], 2)
}

// WriteUint32 writes an unsigned int
func (buf *DICOMBuffer) WriteUint32(value uint32) {
	var c [4]byte
	if buf.bigEndian {
		binary.BigEndian.PutUint32(c[:], value)
	} else {
		binary.LittleEndian.PutUint32(c[:], value)
	}
	buf.Write(c[:], 4)
}

func (buf *DICOMBuffer) WriteString(value string) {
	buf.Write([]byte(value), len(value))
}

func normalizeExplicitVR(tag *DICOMTag, ts *transfersyntax.TransferSyntax) string {
	vr := tag.VR
	if vr == "OB or OW" {
		if ts != nil && ts.UID != transfersyntax.ImplicitVRLittleEndian.UID && ts.UID != transfersyntax.ExplicitVRLittleEndian.UID && ts.UID != transfersyntax.DeflatedExplicitVRLittleEndian.UID {
			return "OB"
		}
		return "OW"
	}
	return vr
}

func isLongExplicitVR(vr string) bool {
	// Per DICOM PS3.5 Table 7.1-2, these VRs use 2 reserved bytes + 32-bit length.
	// OD/OF/OL/OV were added in DICOM 2011-2019; SV/UC/UR/UV added 2014-2019.
	switch vr {
	case "OB", "OD", "OF", "OL", "OV", "OW", "SQ", "SV", "UC", "UN", "UR", "UT", "UV":
		return true
	default:
		return false
	}
}

// ReadTag - read a single tag from the Stream
func (buf *DICOMBuffer) ReadTag(explicitVR bool) (*DICOMTag, error) {
	group, err := buf.ReadUint16(buf.bigEndian)
	if err != nil {
		return nil, err
	}
	element, err := buf.ReadUint16(buf.bigEndian)
	if err != nil {
		return nil, err
	}
	tag := newTag()
	tag.Group = group
	tag.Element = element

	internalVR := explicitVR

	if tag.Group == 0x0002 {
		internalVR = true
	}

	if (tag.Group != 0x0000) && (tag.Group != 0xfffe) && (internalVR) {
		tag.VR = buf.readVR()
		if isLongExplicitVR(tag.VR) {
			_, err := buf.ReadUint16(buf.bigEndian)
			if err != nil {
				releaseTag(tag)
				return nil, err
			}

			length, err := buf.ReadUint32(buf.bigEndian)
			if err != nil {
				releaseTag(tag)
				return nil, err
			}

			tag.Length = length
		} else {
			length, err := buf.ReadUint16(buf.bigEndian)
			if err != nil {
				releaseTag(tag)
				return nil, err
			}
			tag.Length = uint32(length)
		}
	} else {
		if !internalVR {
			tag.VR = GetDictionaryVR(tag.Group, tag.Element)
		}
		length, err := buf.ReadUint32(buf.bigEndian)
		if err != nil {
			releaseTag(tag)
			return nil, err
		}
		tag.Length = length
	}

	if (tag.Length != 0) && (tag.Length != 0xFFFFFFFF) {
		// ReadSlice returns a zero-copy view into the stream's backing array.
		// This is safe as long as the backing array is not reused while
		// tag.Data is live.  In the network path, NextPDU() calls
		// Reset() (not Clear) before each new message, ensuring a
		// fresh backing array is allocated rather than the old one being reused.
		data, err := buf.ReadSlice(int(tag.Length))
		if err != nil {
			releaseTag(tag)
			return nil, err
		}
		tag.Data = data
	}
	FillTag(tag)
	return tag, nil
}

// WriteTag - Write a single tag to stream
func (buf *DICOMBuffer) WriteTag(tag *DICOMTag, explicitVR bool) {
	buf.writeTag(tag, explicitVR, nil)
}

func (buf *DICOMBuffer) writeTag(tag *DICOMTag, explicitVR bool, ts *transfersyntax.TransferSyntax) {
	writeVR := normalizeExplicitVR(tag, ts)

	// Derive the length to serialize from the actual data length to guarantee the
	// header length field always matches the payload bytes written.  Undefined-length
	// (0xFFFFFFFF) tags are preserved as-is because their content is written as
	// separate child items rather than inline data.
	writeLen := tag.Length
	if writeLen != 0xFFFFFFFF {
		dataLen := uint32(len(tag.Data))
		if dataLen < writeLen {
			slog.Warn("media: writeTag data shorter than declared length; truncating",
				"group", tag.Group, "element", tag.Element,
				"declared", writeLen, "actual", dataLen)
			writeLen = dataLen
		}
	}

	buf.WriteUint16(tag.Group)
	buf.WriteUint16(tag.Element)
	if (tag.Group != 0x0000) && (tag.Group != 0xfffe) && (explicitVR) {
		var vrBuf [2]byte
		switch len(writeVR) {
		case 0:
			vrBuf[0], vrBuf[1] = ' ', ' '
		case 1:
			vrBuf[0] = writeVR[0]
			vrBuf[1] = ' '
		default:
			vrBuf[0] = writeVR[0]
			vrBuf[1] = writeVR[1]
		}
		buf.Write(vrBuf[:], 2)
		if isLongExplicitVR(writeVR) {
			buf.WriteUint16(0)
			buf.WriteUint32(writeLen)
		} else {
			buf.WriteUint16(uint16(writeLen))
		}
	} else {
		buf.WriteUint32(writeLen)
	}
	if (writeLen != 0) && (writeLen != 0xFFFFFFFF) {
		buf.Write(tag.Data, int(writeLen))
	}
}

// WriteStringTag - Writes a String to a DICOM tag
func (buf *DICOMBuffer) WriteStringTag(group uint16, element uint16, vr string, content string, explicitVR bool) {
	data := []byte(content)
	length := uint32(len(data))
	if length%2 == 1 {
		length++
		if vr == "UI" {
			data = append(data, 0x00)
		} else {
			data = append(data, 0x20)
		}
	}
	tag := &DICOMTag{
		Group:     group,
		Element:   element,
		Length:    length,
		VR:        vr,
		Data:      data,
		BigEndian: false,
	}
	buf.WriteTag(tag, explicitVR)
}

// ReadMeta - Read Meta Header
func (buf *DICOMBuffer) ReadMeta() (*transfersyntax.TransferSyntax, error) {
	var TransferSyntax *transfersyntax.TransferSyntax
	pos := 0

	buf.SetPosition(128)
	bs, err := buf.Read(4)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(bs, []byte("DICM")) {
		fin := false
		for (pos < buf.GetSize()) && (!fin) {
			pos = buf.GetPosition()
			tag, err := buf.ReadTag(true)
			if err != nil || tag == nil {
				break
			}
			if (tag.Group == 0x02) && (tag.Element == 0x010) {
				uid := tag.GetString()
				TransferSyntax = transfersyntax.GetTransferSyntaxFromUID(uid)
			}
			if tag.Group > 0x02 {
				fin = true
			}
		}
	}
	buf.SetPosition(pos)
	return TransferSyntax, nil
}

// WriteMeta - Write Meta Header
func (buf *DICOMBuffer) WriteMeta(SOPClassUID string, SOPInstanceUID string, TransferSyntax string) {
	explicitVR := true
	var largo uint32
	var tag *DICOMTag
	var preamble [128]byte
	var groupLength [4]byte

	buf.Write(preamble[:], 128)
	buf.Write([]byte("DICM"), 4)
	tag = &DICOMTag{
		Group:     0x02,
		Element:   0x00,
		Length:    4,
		VR:        "UL",
		Data:      []byte{0, 0, 0, 0},
		BigEndian: false}
	buf.WriteTag(tag, explicitVR)
	tag = &DICOMTag{
		Group:     0x02,
		Element:   0x01,
		Length:    2,
		VR:        "OB",
		Data:      []byte{0x00, 0x01},
		BigEndian: false,
	}
	buf.WriteTag(tag, explicitVR)

	buf.WriteStringTag(0x02, 0x02, "UI", SOPClassUID, explicitVR)
	buf.WriteStringTag(0x02, 0x03, "UI", SOPInstanceUID, explicitVR)
	buf.WriteStringTag(0x02, 0x10, "UI", TransferSyntax, explicitVR)

	// Implementation Class UID
	buf.WriteStringTag(0x02, 0x12, "UI", implementation.GetImplementationClassUID(), explicitVR)
	// Implementation Version Name
	buf.WriteStringTag(0x02, 0x13, "SH", implementation.GetImplementationVersion(), explicitVR)

	// Calculate group length and seek back to overwrite the placeholder.
	// The File Meta Information is always encoded as explicit VR little endian
	// per DICOM PS 3.10, so we use LittleEndian explicitly regardless of
	// the dataset transfer syntax.
	ptr := buf.GetPosition()
	largo = uint32(buf.GetSize() - 12 - 128 - 4)
	binary.LittleEndian.PutUint32(groupLength[:], largo)
	buf.SetPosition(128 + 4 + 8)
	buf.Write(groupLength[:], 4)
	buf.SetPosition(ptr)
}

// ReadObj - Read a DICOM Object from a DICOMBuffer
func (buf *DICOMBuffer) ReadObj(obj DICOMObject) error {
	for buf.GetPosition() < buf.GetSize() {
		tag, err := buf.ReadTag(obj.IsExplicitVR())
		if err != nil {
			return err
		}
		// ReadTag already performs the dictionary VR lookup for implicit VR files;
		// no second lookup is needed here.
		if tag.Length%2 != 0 && tag.VR != "SQ" && tag.Length != 0xffffffff {
			slog.Warn("media: odd-length tag", "name", tag.Name, "group", tag.Group, "element", tag.Element)
		}
		obj.Add(tag)
	}
	return nil
}

// WriteObj - Write a DICOM Object to a DICOMBuffer
func (buf *DICOMBuffer) WriteObj(obj DICOMObject) {
	for i := 0; i < obj.TagCount(); i++ {
		tag := obj.GetTagAt(i)
		buf.writeTag(tag, obj.IsExplicitVR(), obj.GetTransferSyntax())
	}
}

func (buf *DICOMBuffer) Send(rw *bufio.ReadWriter) error {
	buf.SetPosition(0)
	buffer := buf.GetData()
	buf.Clear()

	_, err := rw.Write(buffer)
	if err != nil {
		return fmt.Errorf("DICOMBuffer.Send: %w", err)
	}
	rw.Flush()
	return nil
}

func (buf *DICOMBuffer) GetAllBytes() []byte {
	return buf.GetData()
}

func (buf *DICOMBuffer) readString(length int) string {
	temp, err := buf.Read(length)
	if err != nil {
		return ""
	}
	return string(temp)
}

// readVR reads the 2-byte VR field from the stream and returns an interned
// string constant, avoiding the heap allocation that string() conversion
// would otherwise cause on every explicit-VR tag.
func (buf *DICOMBuffer) readVR() string {
	if buf.position+2 > buf.size {
		return ""
	}
	b0 := buf.data[buf.position]
	b1 := buf.data[buf.position+1]
	buf.position += 2
	return internVR(b0, b1)
}

// internVR returns a pre-allocated constant string for all 34 standard DICOM VRs
// (per DICOM PS3.5 Table 7.1-1 / 7.1-2). Unknown 2-byte sequences fall back to
// a heap-allocated string.
func internVR(b0, b1 byte) string {
	switch uint16(b0)<<8 | uint16(b1) {
	case 0x4145:
		return "AE"
	case 0x4153:
		return "AS"
	case 0x4154:
		return "AT"
	case 0x4353:
		return "CS"
	case 0x4441:
		return "DA"
	case 0x4453:
		return "DS"
	case 0x4454:
		return "DT"
	case 0x4644:
		return "FD"
	case 0x464C:
		return "FL"
	case 0x4953:
		return "IS"
	case 0x4C4F:
		return "LO"
	case 0x4C54:
		return "LT"
	case 0x4F42:
		return "OB"
	case 0x4F44:
		return "OD"
	case 0x4F46:
		return "OF"
	case 0x4F4C:
		return "OL"
	case 0x4F56:
		return "OV"
	case 0x4F57:
		return "OW"
	case 0x504E:
		return "PN"
	case 0x5348:
		return "SH"
	case 0x534C:
		return "SL"
	case 0x5351:
		return "SQ"
	case 0x5353:
		return "SS"
	case 0x5354:
		return "ST"
	case 0x5356:
		return "SV"
	case 0x544D:
		return "TM"
	case 0x5543:
		return "UC"
	case 0x5549:
		return "UI"
	case 0x554C:
		return "UL"
	case 0x554E:
		return "UN"
	case 0x5552:
		return "UR"
	case 0x5553:
		return "US"
	case 0x5554:
		return "UT"
	case 0x5556:
		return "UV"
	default:
		return string([]byte{b0, b1})
	}
}
