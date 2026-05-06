package network

import (
	"bufio"

	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

// Abort Source values per DICOM PS3.8 Table 9-26.
const (
	AbortSourceServiceUser         byte = 0x00
	AbortSourceServiceProviderACSE byte = 0x02
	AbortSourceServiceProviderPres byte = 0x03
)

// Abort Reason values per DICOM PS3.8 Table 9-26.
const (
	AbortReasonNotSpecified             byte = 0x00
	AbortReasonUnrecognizedPDU          byte = 0x01
	AbortReasonUnexpectedPDU            byte = 0x02
	AbortReasonUnrecognizedPDUParameter byte = 0x04
	AbortReasonUnexpectedPDUParameter   byte = 0x05
	AbortReasonInvalidPDUParameterValue byte = 0x06
)

var abortReasonsBySource = map[byte]map[byte]string{
	AbortSourceServiceUser: {
		AbortReasonNotSpecified: "No reason given",
	},
	AbortSourceServiceProviderACSE: {
		AbortReasonNotSpecified:             "No reason given",
		AbortReasonUnrecognizedPDU:          "Unrecognized PDU",
		AbortReasonUnexpectedPDU:            "Unexpected PDU",
		AbortReasonUnrecognizedPDUParameter: "Unrecognized PDU parameter",
		AbortReasonUnexpectedPDUParameter:   "Unexpected PDU parameter",
		AbortReasonInvalidPDUParameterValue: "Invalid PDU parameter value",
	},
	AbortSourceServiceProviderPres: {
		AbortReasonNotSpecified:             "No reason given",
		AbortReasonUnrecognizedPDU:          "Unrecognized PDU",
		AbortReasonUnexpectedPDU:            "Unexpected PDU",
		AbortReasonUnrecognizedPDUParameter: "Unrecognized PDU parameter",
		AbortReasonUnexpectedPDUParameter:   "Unexpected PDU parameter",
		AbortReasonInvalidPDUParameterValue: "Invalid PDU parameter value",
	},
}

// AbortRequest - AbortRequest
type AbortRequest interface {
	GetReason() string
	Size() uint32
	Write(rw *bufio.ReadWriter) error
	Read(buf *media.DICOMBuffer) (err error)
	ReadDynamic(buf *media.DICOMBuffer) (err error)
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
		ItemType:  pdutype.AssociationAbortRequest,
		Reserved1: 0x00,
		Reserved2: 0x00,
		Reserved3: 0x00, // must be 0x00 per DICOM PS3.8 Table 9-26
		Source:    AbortSourceServiceUser,
		Reason:    AbortReasonNotSpecified,
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
	bd := media.NewDICOMBuffer()

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

func (aarq *abortRequest) Read(buf *media.DICOMBuffer) (err error) {
	if aarq.ItemType, err = buf.GetByte(); err != nil {
		return err
	}
	return aarq.ReadDynamic(buf)
}

func (aarq *abortRequest) ReadDynamic(buf *media.DICOMBuffer) (err error) {
	if aarq.Reserved1, err = buf.GetByte(); err != nil {
		return err
	}
	if aarq.Length, err = buf.ReadUint32(true); err != nil {
		return err
	}
	if aarq.Reserved2, err = buf.GetByte(); err != nil {
		return err
	}
	if aarq.Reserved3, err = buf.GetByte(); err != nil {
		return err
	}
	if aarq.Source, err = buf.GetByte(); err != nil {
		return err
	}
	if aarq.Reason, err = buf.GetByte(); err != nil {
		return err
	}
	return
}
