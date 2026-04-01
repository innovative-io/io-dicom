package network

import (
	"bufio"

	"github.com/innovative-io/io-dicom/media"
)

var abortReasonsBySource = map[byte]map[byte]string{
	0: {
		0: "No reason given",
	},
	2: {
		0: "No reason given",
		1: "Unrecognized PDU",
		2: "Unexpected PDU",
		4: "Unrecognized PDU parameter",
		5: "Unexpected PDU parameter",
		6: "Invalid PDU parameter value",
	},
	3: {
		0: "No reason given",
		1: "Unrecognized PDU",
		2: "Unexpected PDU",
		4: "Unrecognized PDU parameter",
		5: "Unexpected PDU parameter",
		6: "Invalid PDU parameter value",
	},
}

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
		Reserved3: 0x00, // must be 0x00 per DICOM PS3.8 Table 9-26
		Source:    0x00, // 0x00 = service-user per DICOM PS3.8 Table 9-26
		Reason:    0x00, // 0x00 = no reason given
	}
}

func (aarq *abortRequest) GetReason() string {
	if reasons, ok := abortReasonsBySource[aarq.Source]; ok {
		if reason, ok := reasons[aarq.Reason]; ok {
			return reason
		}
	}
	return "No reason given"
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
