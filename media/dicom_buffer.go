package media

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"log/slog"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/implementation"
)

// DICOMBuffer - is an interface to buffer manipulation class
type DICOMBuffer interface {
	ClearMemoryStream()
	IsBigEndian() bool
	SetBigEndian(isBigEndian bool)
	GetPosition() int
	SetPosition(position int)
	GetSize() int
	Read(count int) ([]byte, error)
	ReadByte() (byte, error)
	ReadUint16() (uint16, error)
	ReadUint32() (uint32, error)
	Write(data []byte, count int) (int, error)
	WriteAETitle(aeTitle string)
	WriteByte(value byte) error
	WriteUint16(value uint16)
	WriteUint32(value uint32)
	WriteString(value string)
	ReadTag(explicitVR bool) (*DICOMTag, error)
	WriteTag(tag *DICOMTag, explicitVR bool)
	WriteStringTag(group uint16, element uint16, vr string, content string, explicitVR bool)
	ReadMeta() (*transfersyntax.TransferSyntax, error)
	WriteMeta(SOPClassUID string, SOPInstanceUID string, TransferSyntax string)
	ReadObj(obj DICOMObject) error
	WriteObj(obj DICOMObject)
	Send(rw *bufio.ReadWriter) error
	GetAllBytes() []byte
}

type dicomBuffer struct {
	BigEndian bool
	// MS is stored as the concrete *memoryStream (not the MemoryStream interface)
	// so that calls like WriteUint16/WriteUint32 can pass stack-allocated arrays
	// without the compiler forcing them onto the heap via interface-call escape.
	MS *memoryStream
}

// NewEmptyBufData -
func NewEmptyBufData() DICOMBuffer {
	return &dicomBuffer{
		BigEndian: false,
		MS:        newEmptyMemoryStream(),
	}
}

// NewEmptyBufDataWithCapacity creates an empty DICOMBuffer with a pre-allocated
// backing buffer of the given byte capacity, avoiding early growth reallocations.
func NewEmptyBufDataWithCapacity(capacity int) DICOMBuffer {
	return &dicomBuffer{
		BigEndian: false,
		MS:        newMemoryStreamWithCapacity(capacity),
	}
}

// NewBufDataFromBytes -
func NewBufDataFromBytes(data []byte) DICOMBuffer {
	return &dicomBuffer{
		BigEndian: false,
		MS:        NewMemoryStreamFromBytes(data).(*memoryStream),
	}
}

// NewBufDataFromFile -
func NewBufDataFromFile(fileName string) (DICOMBuffer, error) {
	ms, err := NewMemoryStreamFromFile(fileName)
	if err != nil {
		return nil, err
	}
	return &dicomBuffer{
		BigEndian: false,
		MS:        ms.(*memoryStream),
	}, nil
}

func (bd *dicomBuffer) ClearMemoryStream() {
	bd.MS.Clear()
}

func (bd *dicomBuffer) IsBigEndian() bool {
	return bd.BigEndian
}

func (bd *dicomBuffer) SetBigEndian(isBigEndian bool) {
	bd.BigEndian = isBigEndian
}

func (bd *dicomBuffer) GetPosition() int {
	return bd.MS.GetPosition()
}

func (bd *dicomBuffer) SetPosition(position int) {
	bd.MS.SetPosition(position)
}

func (bd *dicomBuffer) GetSize() int {
	return bd.MS.GetSize()
}

func (bd *dicomBuffer) Read(count int) ([]byte, error) {
	return bd.MS.Read(count)
}

func (bd *dicomBuffer) ReadByte() (byte, error) {
	return bd.MS.GetByte()
}

func (bd *dicomBuffer) ReadUint16() (uint16, error) {
	return bd.MS.ReadUint16Endian(bd.BigEndian)
}

func (bd *dicomBuffer) ReadUint32() (uint32, error) {
	return bd.MS.ReadUint32Endian(bd.BigEndian)
}

func (bd *dicomBuffer) Write(data []byte, count int) (int, error) {
	return bd.MS.Write(data, count)
}

func (bd *dicomBuffer) WriteAETitle(aeTitle string) {
	endPos := bd.GetPosition() + 16
	bd.WriteString(aeTitle)
	pad := [1]byte{0x20}
	for bd.GetPosition() < endPos {
		bd.Write(pad[:], 1)
	}
}

// WriteByte writes a byte
func (bd *dicomBuffer) WriteByte(value byte) error {
	b := [1]byte{value}
	_, err := bd.MS.Write(b[:], 1)
	return err
}

// WriteUint16 writes an unsigned int
func (bd *dicomBuffer) WriteUint16(value uint16) {
	var c [2]byte
	if bd.BigEndian {
		binary.BigEndian.PutUint16(c[:], value)
	} else {
		binary.LittleEndian.PutUint16(c[:], value)
	}
	bd.MS.Write(c[:], 2)
}

// WriteUint32 writes an unsigned int
func (bd *dicomBuffer) WriteUint32(value uint32) {
	var c [4]byte
	if bd.BigEndian {
		binary.BigEndian.PutUint32(c[:], value)
	} else {
		binary.LittleEndian.PutUint32(c[:], value)
	}
	bd.MS.Write(c[:], 4)
}

func (bd *dicomBuffer) WriteString(value string) {
	bd.MS.Write([]byte(value), len(value))
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
func (bd *dicomBuffer) ReadTag(explicitVR bool) (*DICOMTag, error) {
	group, err := bd.ReadUint16()
	if err != nil {
		return nil, err
	}
	element, err := bd.ReadUint16()
	if err != nil {
		return nil, err
	}
	tag := &DICOMTag{
		Group:   group,
		Element: element,
	}

	internalVR := explicitVR

	if tag.Group == 0x0002 {
		internalVR = true
	}

	if (tag.Group != 0x0000) && (tag.Group != 0xfffe) && (internalVR) {
		tag.VR = bd.readVR()
		if isLongExplicitVR(tag.VR) {
			_, err := bd.ReadUint16()
			if err != nil {
				return nil, err
			}

			length, err := bd.ReadUint32()
			if err != nil {
				return nil, err
			}

			tag.Length = length
		} else {
			length, err := bd.ReadUint16()
			if err != nil {
				return nil, err
			}
			tag.Length = uint32(length)
		}
	} else {
		if !internalVR {
			tag.VR = GetDictionaryVR(tag.Group, tag.Element)
		}
		length, err := bd.ReadUint32()
		if err != nil {
			return nil, err
		}
		tag.Length = length
	}

	if (tag.Length != 0) && (tag.Length != 0xFFFFFFFF) {
		data, err := bd.MS.Read(int(tag.Length))
		if err != nil {
			return nil, err
		}
		tag.Data = data
	}
	FillTag(tag)
	return tag, nil
}

// WriteTag - Write a single tag to stream
func (bd *dicomBuffer) WriteTag(tag *DICOMTag, explicitVR bool) {
	bd.writeTag(tag, explicitVR, nil)
}

func (bd *dicomBuffer) writeTag(tag *DICOMTag, explicitVR bool, ts *transfersyntax.TransferSyntax) {
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

	bd.WriteUint16(tag.Group)
	bd.WriteUint16(tag.Element)
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
		bd.MS.Write(vrBuf[:], 2)
		if isLongExplicitVR(writeVR) {
			bd.WriteUint16(0)
			bd.WriteUint32(writeLen)
		} else {
			bd.WriteUint16(uint16(writeLen))
		}
	} else {
		bd.WriteUint32(writeLen)
	}
	if (writeLen != 0) && (writeLen != 0xFFFFFFFF) {
		bd.MS.Write(tag.Data, int(writeLen))
	}
}

// WriteStringTag - Writes a String to a DICOM tag
func (bd *dicomBuffer) WriteStringTag(group uint16, element uint16, vr string, content string, explicitVR bool) {
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
	bd.WriteTag(tag, explicitVR)
}

// ReadMeta - Read Meta Header
func (bd *dicomBuffer) ReadMeta() (*transfersyntax.TransferSyntax, error) {
	var TransferSyntax *transfersyntax.TransferSyntax
	pos := 0

	bd.SetPosition(128)
	bs, err := bd.MS.Read(4)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(bs, []byte("DICM")) {
		fin := false
		for (pos < bd.GetSize()) && (!fin) {
			pos = bd.GetPosition()
			tag, err := bd.ReadTag(true)
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
	bd.SetPosition(pos)
	return TransferSyntax, nil
}

// WriteMeta - Write Meta Header
func (bd *dicomBuffer) WriteMeta(SOPClassUID string, SOPInstanceUID string, TransferSyntax string) {
	explicitVR := true
	var largo uint32
	var tag *DICOMTag
	var preamble [128]byte
	var groupLength [4]byte

	bd.MS.Write(preamble[:], 128)
	bd.MS.Write([]byte("DICM"), 4)
	tag = &DICOMTag{
		Group:     0x02,
		Element:   0x00,
		Length:    4,
		VR:        "UL",
		Data:      []byte{0, 0, 0, 0},
		BigEndian: false}
	bd.WriteTag(tag, explicitVR)
	tag = &DICOMTag{
		Group:     0x02,
		Element:   0x01,
		Length:    2,
		VR:        "OB",
		Data:      []byte{0x00, 0x01},
		BigEndian: false,
	}
	bd.WriteTag(tag, explicitVR)

	bd.WriteStringTag(0x02, 0x02, "UI", SOPClassUID, explicitVR)
	bd.WriteStringTag(0x02, 0x03, "UI", SOPInstanceUID, explicitVR)
	bd.WriteStringTag(0x02, 0x10, "UI", TransferSyntax, explicitVR)

	// Implementation Class UID
	bd.WriteStringTag(0x02, 0x12, "UI", implementation.GetImplementationClassUID(), explicitVR)
	// Implementation Version Name
	bd.WriteStringTag(0x02, 0x13, "SH", implementation.GetImplementationVersion(), explicitVR)

	// Calculate group length and seek back to overwrite the placeholder.
	// The File Meta Information is always encoded as explicit VR little endian
	// per DICOM PS 3.10, so we use LittleEndian explicitly regardless of
	// the dataset transfer syntax.
	ptr := bd.GetPosition()
	largo = uint32(bd.GetSize() - 12 - 128 - 4)
	binary.LittleEndian.PutUint32(groupLength[:], largo)
	bd.SetPosition(128 + 4 + 8)
	bd.MS.Write(groupLength[:], 4)
	bd.SetPosition(ptr)
}

// ReadObj - Read a DICOM Object from a DICOMBuffer
func (bd *dicomBuffer) ReadObj(obj DICOMObject) error {
	for bd.GetPosition() < bd.GetSize() {
		tag, err := bd.ReadTag(obj.IsExplicitVR())
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
func (bd *dicomBuffer) WriteObj(obj DICOMObject) {
	//	bd.BigEndian = BigEndian
	// Si lo limpio elimino el meta!!
	//	bd.MS.Clear()
	for i := 0; i < obj.TagCount(); i++ {
		tag := obj.GetTagAt(i)
		bd.writeTag(tag, obj.IsExplicitVR(), obj.GetTransferSyntax())
	}
}

func (bd *dicomBuffer) Send(rw *bufio.ReadWriter) error {
	bd.SetPosition(0)
	buffer := bd.MS.GetData()
	bd.MS.Clear()

	_, err := rw.Write(buffer)
	if err != nil {
		return errors.New("ERROR, bufdata::Send, " + err.Error())
	}
	rw.Flush()
	return nil
}

func (bd *dicomBuffer) GetAllBytes() []byte {
	return bd.MS.GetData()
}

func (bd *dicomBuffer) readString(length int) string {
	temp, err := bd.MS.Read(length)
	if err != nil {
		return ""
	}
	return string(temp)
}

// readVR reads the 2-byte VR field from the stream and returns an interned
// string constant, avoiding the heap allocation that string() conversion
// would otherwise cause on every explicit-VR tag.
func (bd *dicomBuffer) readVR() string {
	if bd.MS.Position+2 > bd.MS.Size {
		return ""
	}
	b0 := bd.MS.Data[bd.MS.Position]
	b1 := bd.MS.Data[bd.MS.Position+1]
	bd.MS.Position += 2
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
