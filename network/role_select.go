package network

import (
	"bufio"

	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

type roleSelect struct {
	ItemType  byte //0x54
	Reserved1 byte
	Length    uint16
	SCURole   byte
	SCPRole   byte
	uid       string
}

func newRoleSelect() *roleSelect {
	return &roleSelect{
		ItemType: pdutype.SCPSCURoleSelectionItem,
	}
}

func (scpscu *roleSelect) Size() uint16 {
	return scpscu.Length + 4
}

func (scpscu *roleSelect) Write(rw *bufio.ReadWriter) bool {
	bd := media.NewDICOMBuffer()

	bd.SetBigEndian(true)
	bd.WriteByte(scpscu.ItemType)
	bd.WriteByte(scpscu.Reserved1)
	bd.WriteUint16(scpscu.Length)
	bd.WriteUint16(uint16(len(scpscu.uid)))
	bd.Write([]byte(scpscu.uid), len(scpscu.uid))
	bd.WriteByte(scpscu.SCURole)
	bd.WriteByte(scpscu.SCPRole)

	if err := bd.Send(rw); err != nil {
		return false
	}
	return true
}

func (scpscu *roleSelect) Read(buf *media.DICOMBuffer) (err error) {
	if scpscu.ItemType, err = buf.GetByte(); err != nil {
		return err
	}
	return scpscu.ReadDynamic(buf)
}

func (scpscu *roleSelect) ReadDynamic(buf *media.DICOMBuffer) (err error) {
	if scpscu.Reserved1, err = buf.GetByte(); err != nil {
		return err
	}
	if scpscu.Length, err = buf.ReadUint16(true); err != nil {
		return err
	}
	tl, err := buf.ReadUint16(true)
	if err != nil {
		return err
	}

	// Bounds-checked, zero-copy read; see uid_item.go for why the previous
	// allocate-then-ignore-the-error form was exploitable.
	tuid, err := buf.ReadSlice(int(tl))
	if err != nil {
		return err
	}
	scpscu.uid = string(tuid)
	if scpscu.SCURole, err = buf.GetByte(); err != nil {
		return err
	}
	if scpscu.SCPRole, err = buf.GetByte(); err != nil {
		return err
	}
	return
}
