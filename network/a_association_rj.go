package network

import (
	"bufio"
	"log/slog"

	"github.com/innovative-io/io-dicom/media"
)

// PermanentRejectReasons - Permanent association reject reasons
var PermanentRejectReasons map[byte]string = map[byte]string{
	0: "No reason given",
	1: "No reason given",
	2: "Application context not supported",
	3: "Calling AE not recognized",
	7: "Called AE not recognized",
}

// TransientRejectReasons - Transient association reject reasons
var TransientRejectReasons map[byte]string = map[byte]string{
	0: "No reason given",
	1: "Temporary congestion",
	2: "Local limit exceeded",
}

// AssociationReject association reject struct
type AssociationReject interface {
	GetReason() string
	Set(result byte, reason byte)
	Size() uint32
	Write(rw *bufio.ReadWriter) error
	Read(ms media.MemoryStream) (err error)
	ReadDynamic(ms media.MemoryStream) (err error)
}

type associationReject struct {
	ItemType  byte // 0x03
	Reserved1 byte
	Length    uint32
	Reserved2 byte
	Result    byte
	Source    byte
	Reason    byte
}

// NewAssociationReject creates an association reject
func NewAssociationReject() AssociationReject {
	return &associationReject{
		ItemType:  0x03,
		Reserved1: 0x00,
		Reserved2: 0x00,
		Result:    0x01,
		Source:    0x03,
		Reason:    1,
	}
}

func (aarj *associationReject) GetReason() string {
	reason := "No reason given"
	if aarj.Result == 0x01 {
		reason = PermanentRejectReasons[aarj.Reason]
	}
	if aarj.Result == 0x02 {
		reason = TransientRejectReasons[aarj.Reason]
	}
	return reason
}

func (aarj *associationReject) Size() uint32 {
	aarj.Length = 4
	return aarj.Length + 6
}

func (aarj *associationReject) Write(rw *bufio.ReadWriter) error {
	bd := media.NewEmptyBufData()

	slog.Info("ASSOC-RJ:", "Reason", aarj.GetReason())

	bd.SetBigEndian(true)
	aarj.Size()
	bd.WriteByte(aarj.ItemType)
	bd.WriteByte(aarj.Reserved1)
	bd.WriteUint32(aarj.Length)
	bd.WriteByte(aarj.Reserved2)
	bd.WriteByte(aarj.Result)
	bd.WriteByte(aarj.Source)
	bd.WriteByte(aarj.Reason)

	return bd.Send(rw)
}

func (aarj *associationReject) Set(result byte, reason byte) {
	aarj.Result = result
	aarj.Reason = reason
}

func (aarj *associationReject) Read(ms media.MemoryStream) (err error) {
	if aarj.ItemType, err = ms.GetByte(); err != nil {
		return err
	}
	return aarj.ReadDynamic(ms)
}

func (aarj *associationReject) ReadDynamic(ms media.MemoryStream) (err error) {
	if aarj.Reserved1, err = ms.GetByte(); err != nil {
		return err
	}
	if aarj.Length, err = ms.GetUint32(); err != nil {
		return err
	}
	if aarj.Reserved2, err = ms.GetByte(); err != nil {
		return err
	}
	if aarj.Result, err = ms.GetByte(); err != nil {
		return err
	}
	if aarj.Source, err = ms.GetByte(); err != nil {
		return err
	}
	if aarj.Reason, err = ms.GetByte(); err != nil {
		return err
	}
	return
}
