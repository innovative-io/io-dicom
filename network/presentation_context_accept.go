package network

import (
	"bufio"
	"log/slog"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
)

// PresentationContextAccept accepted presentation context
type PresentationContextAccept interface {
	GetPresentationContextID() byte
	SetPresentationContextID(id byte)
	GetResult() byte
	SetResult(result byte)
	GetTrnSyntax() UIDItem
	Size() uint16
	GetAbstractSyntax() UIDItem
	SetAbstractSyntax(abstractSyntaxUID string)
	SetTransferSyntax(transferSyntaxUID string)
	Write(rw *bufio.ReadWriter) (err error)
	Read(ms media.MemoryStream) (err error)
	ReadDynamic(ms media.MemoryStream) (err error)
}

type presentationContextAccept struct {
	ItemType              byte //0x21
	Reserved1             byte
	Length                uint16
	PresentationContextID byte
	Reserved2             byte
	Result                byte
	Reserved4             byte
	AbsSyntax             uidItem
	TrnSyntax             uidItem
}

// NewPresentationContextAccept creates a PresentationContextAccept
func NewPresentationContextAccept() PresentationContextAccept {
	return &presentationContextAccept{
		ItemType:              0x21,
		PresentationContextID: Uniq8(),
		Result:                2,
	}
}

func (pc *presentationContextAccept) GetPresentationContextID() byte {
	return pc.PresentationContextID
}

func (pc *presentationContextAccept) SetPresentationContextID(id byte) {
	pc.PresentationContextID = id
}

func (pc *presentationContextAccept) GetResult() byte {
	return pc.Result
}

func (pc *presentationContextAccept) SetResult(result byte) {
	pc.Result = result
}

func (pc *presentationContextAccept) GetTrnSyntax() UIDItem {
	return &pc.TrnSyntax
}

// Size gets the size of presentation
func (pc *presentationContextAccept) Size() uint16 {
	pc.Length = 4
	pc.Length += pc.TrnSyntax.GetSize()
	return pc.Length + 4
}

func (pc *presentationContextAccept) GetAbstractSyntax() UIDItem {
	return &pc.AbsSyntax
}

func (pc *presentationContextAccept) SetAbstractSyntax(abstractSyntaxUID string) {
	pc.AbsSyntax.SetType(0x30)
	pc.AbsSyntax.SetReserved(0x00)
	pc.AbsSyntax.SetUID(abstractSyntaxUID)
	pc.AbsSyntax.SetLength(uint16(len(abstractSyntaxUID)))
}

func (pc *presentationContextAccept) SetTransferSyntax(transferSyntaxUID string) {
	pc.TrnSyntax.SetType(0x40)
	pc.TrnSyntax.SetReserved(0)
	pc.TrnSyntax.SetUID(transferSyntaxUID)
	pc.TrnSyntax.SetLength(uint16(len(transferSyntaxUID)))
}

func (pc *presentationContextAccept) Write(rw *bufio.ReadWriter) (err error) {
	bd := media.NewEmptyBufData()

	bd.SetBigEndian(true)
	pc.Size()
	bd.WriteByte(pc.ItemType)
	bd.WriteByte(pc.Reserved1)
	bd.WriteUint16(pc.Length)
	bd.WriteByte(pc.PresentationContextID)
	bd.WriteByte(pc.Reserved2)
	bd.WriteByte(pc.Result)
	bd.WriteByte(pc.Reserved4)

	sopName := ""
	tsName := ""
	if sopClass := sopclass.GetSOPClassFromUID(pc.GetAbstractSyntax().GetUID()); sopClass != nil {
		sopName = sopClass.Description
	}
	if transferSyntax := transfersyntax.GetTransferSyntaxFromUID(pc.GetTrnSyntax().GetUID()); transferSyntax != nil {
		tsName = transferSyntax.Description
	}

	slog.Info("ASSOC-AC: \tAccepted AbstractContext:", "UID", pc.GetAbstractSyntax().GetUID(), "Description", sopName)
	slog.Info("ASSOC-AC: \tAccepted TransferSyntax:", "UID", pc.GetTrnSyntax().GetUID(), "Description", tsName)

	if err = bd.Send(rw); err == nil {
		return pc.TrnSyntax.Write(rw)
	}
	return
}

func (pc *presentationContextAccept) Read(ms media.MemoryStream) (err error) {
	if pc.ItemType, err = ms.GetByte(); err != nil {
		return err
	}
	return pc.ReadDynamic(ms)
}

func (pc *presentationContextAccept) ReadDynamic(ms media.MemoryStream) (err error) {
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
	if pc.Result, err = ms.GetByte(); err != nil {
		return err
	}
	if pc.Reserved4, err = ms.GetByte(); err != nil {
		return err
	}
	return pc.TrnSyntax.Read(ms)
}
