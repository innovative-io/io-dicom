package network

import (
	"bufio"

	"github.com/innovative-io/io-dicom/media"
)

// AbortRequest - AbortRequest
type AbortRequest interface {
	GetReason() string
	Size() uint32
	Write(rw *bufio.ReadWriter) error
	Read(ms media.MemoryStream) (err error)
	ReadDynamic(ms media.MemoryStream) (err error)
}

type abortRequest struct {
	ItemType  byte // 0x07
	Reserved1 byte
	Length    uint32
	Reserved2 byte
	Reserved3 byte
	Source    byte
	Reason    byte
}

// NewAbortRequest - NewAbortRequest
func NewAbortRequest() AbortRequest {
	return &abortRequest{
		ItemType:  0x07,
		Reserved1: 0x00,
		Reserved2: 0x00,
		Reserved3: 0x01,
		Source:    0x03,
		Reason:    0x01,
	}
}

func (aarq *abortRequest) GetReason() string {
	return PermanentRejectReasons[aarq.Reason]
}

func (aarq *abortRequest) Size() uint32 {
	aarq.Length = 4
	return aarq.Length + 6
}

func (aarq *abortRequest) Write(rw *bufio.ReadWriter) error {
	bd := media.NewEmptyBufData()

	bd.SetBigEndian(true)
	aarq.Size()
	bd.WriteByte(aarq.ItemType)
	bd.WriteByte(aarq.Reserved1)
	bd.WriteUint32(aarq.Length)
	bd.WriteByte(aarq.Reserved2)
	bd.WriteByte(aarq.Reserved3)
	bd.WriteByte(aarq.Source)
	bd.WriteByte(aarq.Reason)

	return bd.Send(rw)
}

func (aarq *abortRequest) Read(ms media.MemoryStream) (err error) {
	if aarq.ItemType, err = ms.GetByte(); err != nil {
		return err
	}
	return aarq.ReadDynamic(ms)
}

func (aarq *abortRequest) ReadDynamic(ms media.MemoryStream) (err error) {
	if aarq.Reserved1, err = ms.GetByte(); err != nil {
		return err
	}
	if aarq.Length, err = ms.GetUint32(); err != nil {
		return err
	}
	if aarq.Reserved2, err = ms.GetByte(); err != nil {
		return err
	}
	if aarq.Reserved3, err = ms.GetByte(); err != nil {
		return err
	}
	if aarq.Source, err = ms.GetByte(); err != nil {
		return err
	}
	if aarq.Reason, err = ms.GetByte(); err != nil {
		return err
	}
	return
}
