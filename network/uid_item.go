package network

import (
	"bufio"

	"github.com/innovative-io/io-dicom/media"
)

// UIDitem - UIDitem
type UIDItem interface {
	GetLength() uint16
	GetReserved() byte
	GetSize() uint16
	GetType() byte
	GetUID() string
	SetReserved(reserve byte)
	SetLength(length uint16)
	SetType(itemType byte)
	SetUID(uid string)
	Write(rw *bufio.ReadWriter) error
	Read(buf *media.DICOMBuffer) (err error)
	ReadDynamic(buf *media.DICOMBuffer) (err error)
}

type uidItem struct {
	itemType  byte
	reserved1 byte
	length    uint16
	uid       string
}

func NewUIDItem(uid string, itemType byte) UIDItem {
	return &uidItem{
		itemType: itemType,
		uid:      uid,
		length:   uint16(len(uid)),
	}
}

func (u *uidItem) GetLength() uint16 {
	return u.length
}

func (u *uidItem) GetReserved() byte {
	return u.reserved1
}

func (u *uidItem) GetSize() uint16 {
	return u.length + 4
}

func (u *uidItem) GetType() byte {
	return u.itemType
}

func (u *uidItem) GetUID() string {
	return u.uid
}

func (u *uidItem) SetReserved(reserve byte) {
	u.reserved1 = reserve
}

func (u *uidItem) SetLength(length uint16) {
	u.length = length
}

func (u *uidItem) SetType(itemType byte) {
	u.itemType = itemType
}

func (u *uidItem) SetUID(uid string) {
	u.uid = uid
}

func (u *uidItem) Write(rw *bufio.ReadWriter) error {
	bd := media.NewDICOMBuffer()

	bd.SetBigEndian(true)
	bd.WriteByte(u.itemType)
	bd.WriteByte(u.reserved1)
	bd.WriteUint16(u.length)
	bd.WriteString(u.uid)

	return bd.Send(rw)
}

func (u *uidItem) Read(buf *media.DICOMBuffer) (err error) {
	if u.itemType, err = buf.GetByte(); err != nil {
		return err
	}
	return u.ReadDynamic(buf)
}

func (u *uidItem) ReadDynamic(buf *media.DICOMBuffer) (err error) {
	if u.reserved1, err = buf.GetByte(); err != nil {
		return err
	}
	if u.length, err = buf.ReadUint16(true); err != nil {
		return err
	}

	// ReadSlice bounds-checks against the bytes actually present and returns a
	// zero-copy view; string() then makes the single copy we keep.
	//
	// The previous form allocated make([]byte, u.length) BEFORE any bounds check
	// and discarded ReadData's error. u.length is an attacker-controlled uint16,
	// so a 4-byte item on the wire could declare 65535 bytes: the allocation
	// happened anyway, the error was dropped, and the 64 KiB buffer was retained
	// as the item's UID. A presentation-context item can pack thousands of such
	// sub-items, which an audit measured at a 65,622-byte A-ASSOCIATE-RQ
	// retaining 1,023 MiB.
	data, err := buf.ReadSlice(int(u.length))
	if err != nil {
		return err
	}
	u.uid = string(data)

	return
}
