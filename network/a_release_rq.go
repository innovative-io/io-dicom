package network

import (
	"bufio"

	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/pdutype"
)

// ReleaseRequest ReleaseRequest
type ReleaseRequest interface {
	Size() uint32
	Write(rw *bufio.ReadWriter) error
	Read(ms media.MemoryStream) (err error)
	ReadDynamic(ms media.MemoryStream) (err error)
}

type releaseRequest struct {
	ItemType  byte // 0x05
	Reserved1 byte
	Length    uint32
	Reserved2 uint32
}

// NewReleaseRequest NewReleaseRequest
func NewReleaseRequest() ReleaseRequest {
	return &releaseRequest{
		ItemType:  pdutype.AssociationReleaseRequest,
		Reserved1: 0x00,
		Reserved2: 0x00,
	}
}

func (arrq *releaseRequest) Size() uint32 {
	arrq.Length = 4
	return arrq.Length + 6
}

func (arrq *releaseRequest) Write(rw *bufio.ReadWriter) error {
	bd := media.NewEmptyBufData()

	bd.SetBigEndian(true)
	arrq.Size()
	bd.WriteByte(arrq.ItemType)
	bd.WriteByte(arrq.Reserved1)
	bd.WriteUint32(arrq.Length)
	bd.WriteUint32(arrq.Reserved2)

	return bd.Send(rw)
}

func (arrq *releaseRequest) Read(ms media.MemoryStream) (err error) {
	if arrq.ItemType, err = ms.GetByte(); err != nil {
		return err
	}
	return arrq.ReadDynamic(ms)
}

func (arrq *releaseRequest) ReadDynamic(ms media.MemoryStream) (err error) {
	if arrq.Reserved1, err = ms.GetByte(); err != nil {
		return err
	}
	if arrq.Length, err = ms.GetUint32(); err != nil {
		return err
	}
	if arrq.Reserved2, err = ms.GetUint32(); err != nil {
		return err
	}
	return
}
