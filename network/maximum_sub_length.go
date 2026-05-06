package network

import (
	"bufio"

	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

type maximumPDULength struct {
	ItemType      byte //0x51
	Reserved1     byte
	Length        uint16
	MaximumLength uint32
}

func newMaximumPDULength() *maximumPDULength {
	return &maximumPDULength{
		ItemType: pdutype.MaximumSubLengthItem,
		Length:   4,
	}
}

func (maxim *maximumPDULength) GetMaximumLength() uint32 {
	return maxim.MaximumLength
}

func (maxim *maximumPDULength) SetMaximumLength(length uint32) {
	maxim.MaximumLength = length
}

func (maxim *maximumPDULength) Size() uint16 {
	return maxim.Length + 4
}

func (maxim *maximumPDULength) Write(rw *bufio.ReadWriter) bool {
	bd := media.NewDICOMBuffer()

	bd.SetBigEndian(true)
	bd.WriteByte(maxim.ItemType)
	bd.WriteByte(maxim.Reserved1)
	bd.WriteUint16(maxim.Length)
	bd.WriteUint32(maxim.MaximumLength)

	if err := bd.Send(rw); err != nil {
		return false
	}
	return true
}

func (maxim *maximumPDULength) Read(buf *media.DICOMBuffer) (err error) {
	if maxim.ItemType, err = buf.GetByte(); err != nil {
		return err
	}
	return maxim.ReadDynamic(buf)
}

func (maxim *maximumPDULength) ReadDynamic(buf *media.DICOMBuffer) (err error) {
	if maxim.Reserved1, err = buf.GetByte(); err != nil {
		return err
	}
	if maxim.Length, err = buf.ReadUint16(true); err != nil {
		return err
	}
	if maxim.MaximumLength, err = buf.ReadUint32(true); err != nil {
		return err
	}
	return
}
