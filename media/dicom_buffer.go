package media

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"log"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
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
	MS        MemoryStream
}

// NewEmptyBufData -
func NewEmptyBufData() DICOMBuffer {
	return &dicomBuffer{
		BigEndian: false,
		MS:        NewEmptyMemoryStream(),
	}
}

// NewBufDataFromBytes -
func NewBufDataFromBytes(data []byte) DICOMBuffer {
	return &dicomBuffer{
		BigEndian: false,
		MS:        NewMemoryStreamFromBytes(data),
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
		MS:        ms,
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
	c, err := bd.MS.Read(1)
	if err != nil {
		return 0, err
	}
	return c[0], nil
}

func (bd *dicomBuffer) ReadUint16() (uint16, error) {
	c, err := bd.MS.Read(2)
	if err != nil {
		return 0, err
	}
	if bd.BigEndian {
		return binary.BigEndian.Uint16(c), nil
	}
	return binary.LittleEndian.Uint16(c), nil
}

func (bd *dicomBuffer) ReadUint32() (uint32, error) {
	c, err := bd.MS.Read(4)
	if err != nil {
		return 0, err
	}
	if bd.BigEndian {
		return binary.BigEndian.Uint32(c), nil
	}
	return binary.LittleEndian.Uint32(c), nil
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
		tag.VR = bd.readString(2)
		if (tag.VR == "OB") || (tag.VR == "OW") || (tag.VR == "SQ") || (tag.VR == "UN") || (tag.VR == "UT") {
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
		if data, err := bd.MS.Read(int(tag.Length)); err == nil {
			tag.Data = data
		} else {
			return nil, err
		}
	}
	FillTag(tag)
	return tag, nil
}

// WriteTag - Write a single tag to stream
func (bd *dicomBuffer) WriteTag(tag *DICOMTag, explicitVR bool) {
	bd.WriteUint16(tag.Group)
	bd.WriteUint16(tag.Element)
	if (tag.Group != 0x0000) && (tag.Group != 0xfffe) && (explicitVR) {
		bd.MS.Write([]byte(tag.VR), 2)
		if (tag.VR == "OB") || (tag.VR == "OW") || (tag.VR == "SQ") || (tag.VR == "UN") || (tag.VR == "UT") {
			bd.WriteUint16(0)
			bd.WriteUint32(tag.Length)
		} else {
			bd.WriteUint16(uint16(tag.Length))
		}
	} else {
		bd.WriteUint32(tag.Length)
	}
	if (tag.Length != 0) && (tag.Length != 0xFFFFFFFF) {
		bd.MS.Write(tag.Data, int(tag.Length))
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
	bd.WriteStringTag(0x02, 0x12, "UI", "123456", explicitVR)
	// Implementation Version Name
	bd.WriteStringTag(0x02, 0x13, "SH", "odb", explicitVR)

	// calculate group length and go Back to group size tag
	ptr := bd.GetPosition()
	largo = uint32(bd.GetSize() - 12 - 128 - 4)
	binary.BigEndian.PutUint32(groupLength[:], largo)
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
		if !obj.IsExplicitVR() {
			tag.VR = GetDictionaryVR(tag.Group, tag.Element)
		}
		if tag.Length%2 != 0 && tag.VR != "SQ" && tag.Length != 0xffffffff {
			log.Printf("%s is odd", tag.Name)
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
		bd.WriteTag(tag, obj.IsExplicitVR())
	}
}

func (bd *dicomBuffer) Send(rw *bufio.ReadWriter) error {
	bd.SetPosition(0)
	buffer, _ := bd.MS.Read(bd.GetSize())
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
	temp, _ := bd.MS.Read(length)
	return string(temp)
}
