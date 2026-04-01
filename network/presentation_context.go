package network

import (
	"bufio"
	"errors"

	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/pdutype"
)

// PresentationContext - PresentationContext
type PresentationContext interface {
	GetPresentationContextID() byte
	SetPresentationContextID(id byte)
	GetAbstractSyntax() UIDItem
	SetAbstractSyntax(abstractSyntaxUID string)
	AddTransferSyntax(transferSyntaxUID string)
	GetTransferSyntaxes() []UIDItem
	Size() uint16
	Write(rw *bufio.ReadWriter) error
	Read(ms media.MemoryStream) error
	ReadDynamic(ms media.MemoryStream) error
}

type presentationContext struct {
	ItemType              byte //0x20
	Reserved1             byte
	Length                uint16
	PresentationContextID byte
	Reserved2             byte
	Reserved3             byte
	Reserved4             byte
	AbsSyntax             uidItem
	TrnSyntaxs            []UIDItem
}

// NewPresentationContext - NewPresentationContext
func NewPresentationContext() PresentationContext {
	return &presentationContext{
		ItemType:              pdutype.PresentationContextItem,
		PresentationContextID: Uniq8odd(),
	}
}

func (pc *presentationContext) GetPresentationContextID() byte {
	return pc.PresentationContextID
}

func (pc *presentationContext) SetPresentationContextID(id byte) {
	pc.PresentationContextID = id
}

func (pc *presentationContext) GetAbstractSyntax() UIDItem {
	return &pc.AbsSyntax
}

func (pc *presentationContext) SetAbstractSyntax(abstractSyntaxUID string) {
	pc.AbsSyntax.SetType(pdutype.AbstractSyntaxItem)
	pc.AbsSyntax.SetReserved(0x00)
	pc.AbsSyntax.SetUID(abstractSyntaxUID)
	pc.AbsSyntax.SetLength(uint16(len(abstractSyntaxUID)))
}

func (pc *presentationContext) AddTransferSyntax(transferSyntaxUID string) {
	transferSyntax := NewUIDItem(transferSyntaxUID, pdutype.TransferSyntaxItem)
	pc.TrnSyntaxs = append(pc.TrnSyntaxs, transferSyntax)
}

func (pc *presentationContext) GetTransferSyntaxes() []UIDItem {
	return pc.TrnSyntaxs
}

func (pc *presentationContext) Size() uint16 {
	pc.Length = 4 + pc.AbsSyntax.GetSize()
	for _, transferSyntax := range pc.TrnSyntaxs {
		pc.Length += transferSyntax.GetSize()
	}
	return pc.Length + 4
}

func (pc *presentationContext) Write(rw *bufio.ReadWriter) error {
	bd := media.NewEmptyBufData()

	bd.SetBigEndian(true)
	pc.Size()
	bd.WriteByte(pc.ItemType)
	bd.WriteByte(pc.Reserved1)
	bd.WriteUint16(pc.Length)
	bd.WriteByte(pc.PresentationContextID)
	bd.WriteByte(pc.Reserved2)
	bd.WriteByte(pc.Reserved3)
	bd.WriteByte(pc.Reserved4)
	if err := bd.Send(rw); err != nil {
		return err
	}
	if err := pc.AbsSyntax.Write(rw); err != nil {
		return err
	}
	for _, transferSyntax := range pc.TrnSyntaxs {
		if err := transferSyntax.Write(rw); err != nil {
			return err
		}
	}
	return nil
}

func (pc *presentationContext) Read(ms media.MemoryStream) (err error) {
	if pc.ItemType, err = ms.GetByte(); err != nil {
		return err
	}
	return pc.ReadDynamic(ms)
}

func (pc *presentationContext) ReadDynamic(ms media.MemoryStream) (err error) {
	if pc.Reserved1, err = ms.GetByte(); err != nil {
		return err
	}
	if pc.Length, err = ms.GetUint16(); err != nil {
		return err
	}
	if pc.PresentationContextID, err = ms.GetByte(); err != nil {
		return err
	}
	if pc.Reserved2, err = ms.GetByte(); err != nil {
		return err
	}
	if pc.Reserved3, err = ms.GetByte(); err != nil {
		return err
	}
	if pc.Reserved4, err = ms.GetByte(); err != nil {
		return err
	}
	if err := pc.AbsSyntax.Read(ms); err != nil {
		return err
	}

	remainingBytes := pc.Length - 4 - pc.AbsSyntax.GetSize()
	for remainingBytes > 0 {
		var transferSyntax uidItem
		transferSyntax.Read(ms)
		remainingBytes = remainingBytes - transferSyntax.GetSize()
		if transferSyntax.GetSize() > 0 {
			pc.TrnSyntaxs = append(pc.TrnSyntaxs, &transferSyntax)
		}
	}

	if remainingBytes == 0 {
		return nil
	}

	return errors.New("pc::ReadDynamic, remainingBytes is not zero")
}
