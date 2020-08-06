package network

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"git.onebytedata.com/OneByteDataPlatform/go-dicom/media"
)

// ReadByte reads a byte
func ReadByte(rw *bufio.ReadWriter) (byte, error) {
	c := make([]byte, 1)
	_, err := rw.Read(c)
	if err != nil {
		return 0, err
	}
	return c[0], nil
}

// ReadUint16 read unsigned int
func ReadUint16(rw *bufio.ReadWriter) (uint16, error) {
	c := make([]byte, 2)
	_, err := rw.Read(c)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(c), nil
}

// ReadUint32 read unsigned int
func ReadUint32(rw *bufio.ReadWriter) (uint32, error) {
	c := make([]byte, 4)
	_, err := rw.Read(c)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(c), nil
}

// PresentationContextAccept accepted presentation context
type PresentationContextAccept interface {
	GetPresentationContextID() byte
	SetPresentationContextID(id byte)
	GetResult() byte
	SetResult(result byte)
	GetTrnSyntax() UIDitem
	Size() uint16
	GetAbstractSyntax() UIDitem
	SetAbstractSyntax(Abst string)
	SetTransferSyntax(Tran string)
	Write(rw *bufio.ReadWriter) (err error)
	Read(rw *bufio.ReadWriter) (err error)
	ReadDynamic(rw *bufio.ReadWriter) (err error)
}

type presentationContextAccept struct {
	ItemType              byte //0x21
	Reserved1             byte
	Length                uint16
	PresentationContextID byte
	Reserved2             byte
	Result                byte
	Reserved4             byte
	AbsSyntax             UIDitem
	TrnSyntax             UIDitem
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

func (pc *presentationContextAccept) GetTrnSyntax() UIDitem {
	return pc.TrnSyntax
}

// Size gets the size of presentation
func (pc *presentationContextAccept) Size() uint16 {
	pc.Length = 4
	pc.Length += pc.TrnSyntax.Size()
	return pc.Length + 4
}

func (pc *presentationContextAccept) GetAbstractSyntax() UIDitem {
	return pc.AbsSyntax
}

func (pc *presentationContextAccept) SetAbstractSyntax(Abst string) {
	pc.AbsSyntax.ItemType = 0x30
	pc.AbsSyntax.Reserved1 = 0x00
	pc.AbsSyntax.UIDName = Abst
	pc.AbsSyntax.Length = uint16(len(Abst))
}

func (pc *presentationContextAccept) SetTransferSyntax(Tran string) {
	pc.TrnSyntax.ItemType = 0x40
	pc.TrnSyntax.Reserved1 = 0
	pc.TrnSyntax.UIDName = Tran
	pc.TrnSyntax.Length = uint16(len(Tran))
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

	log.Printf("INFO, ASSOC-AC: \tAccepted Presentation Context %s\n", pc.GetAbstractSyntax().UIDName)
	log.Printf("INFO, ASSOC-AC: \tAccepted Transfer Synxtax %s\n", pc.GetTrnSyntax().UIDName)

	if err = bd.Send(rw); err == nil {
		return pc.TrnSyntax.Write(rw)
	}
	return
}

func (pc *presentationContextAccept) Read(rw *bufio.ReadWriter) (err error) {
	pc.ItemType, err = ReadByte(rw)
	if err != nil {
		return
	}
	return pc.ReadDynamic(rw)
}

func (pc *presentationContextAccept) ReadDynamic(rw *bufio.ReadWriter) (err error) {
	pc.Reserved1, err = ReadByte(rw)
	if err != nil {
		return
	}
	pc.Length, err = ReadUint16(rw)
	if err != nil {
		return
	}
	pc.PresentationContextID, err = ReadByte(rw)
	if err != nil {
		return
	}
	pc.Reserved2, err = ReadByte(rw)
	if err != nil {
		return
	}
	pc.Result, err = ReadByte(rw)
	if err != nil {
		return
	}
	pc.Reserved4, err = ReadByte(rw)
	if err != nil {
		return
	}

	return pc.TrnSyntax.Read(rw)
}

// AAssociationAC AAssociationAC
type AAssociationAC interface {
	GetAppContext() UIDitem
	SetAppContext(context UIDitem)
	GetCallingAE() string
	SetCallingAE(AET string)
	GetCalledAE() string
	SetCalledAE(AET string)
	AddPresContextAccept(context PresentationContextAccept)
	GetPresContextAccepts() []PresentationContextAccept
	GetUserInformation() UserInformation
	SetUserInformation(UserInfo UserInformation)
	GetMaxSubLength() uint32
	SetMaxSubLength(length uint32)
	Size() uint32
	Write(rw *bufio.ReadWriter) error
	Read(rw *bufio.ReadWriter) (err error)
	ReadDynamic(rw *bufio.ReadWriter) (err error)
}

type aassociationAC struct {
	ItemType           byte // 0x02
	Reserved1          byte
	Length             uint32
	ProtocolVersion    uint16 // 0x01
	Reserved2          uint16
	CallingAE          [16]byte // 16 bytes transfered
	CalledAE           [16]byte // 16 bytes transfered
	Reserved3          [32]byte
	AppContext         UIDitem
	PresContextAccepts []PresentationContextAccept
	UserInfo           UserInformation
}

// NewAAssociationAC NewAAssociationAC
func NewAAssociationAC() AAssociationAC {
	return &aassociationAC{
		ItemType:        0x02,
		Reserved1:       0x00,
		ProtocolVersion: 0x01,
		Reserved2:       0x00,
		AppContext: UIDitem{
			ItemType:  0x10,
			Reserved1: 0x00,
			UIDName:   "1.2.840.10008.3.1.1.1",
			Length:    uint16(len("1.2.840.10008.3.1.1.1")),
		},
		PresContextAccepts: make([]PresentationContextAccept, 0),
		UserInfo:           NewUserInformation(),
	}
}

func (aaac *aassociationAC) GetAppContext() UIDitem {
	return aaac.AppContext
}

func (aaac *aassociationAC) SetAppContext(context UIDitem) {
	aaac.AppContext = context
}

func (aaac *aassociationAC) GetCallingAE() string {
	temp := strings.ReplaceAll(fmt.Sprintf("%s", aaac.CallingAE), "\x20", "\x00")
	return strings.ReplaceAll(temp, "\x00", "")
}

func (aaac *aassociationAC) SetCallingAE(AET string) {
	copy(aaac.CallingAE[:], AET)
}

func (aaac *aassociationAC) GetCalledAE() string {
	temp := strings.ReplaceAll(fmt.Sprintf("%s", aaac.CalledAE), "\x20", "\x00")
	return strings.ReplaceAll(temp, "\x00", "")
}

func (aaac *aassociationAC) SetCalledAE(AET string) {
	copy(aaac.CalledAE[:], AET)
}

func (aaac *aassociationAC) AddPresContextAccept(context PresentationContextAccept) {
	aaac.PresContextAccepts = append(aaac.PresContextAccepts, context)
}

func (aaac *aassociationAC) GetPresContextAccepts() []PresentationContextAccept {
	return aaac.PresContextAccepts
}

func (aaac *aassociationAC) GetUserInformation() UserInformation {
	return aaac.UserInfo
}

func (aaac *aassociationAC) SetUserInformation(UserInfo UserInformation) {
	aaac.UserInfo = UserInfo
}

func (aaac *aassociationAC) GetMaxSubLength() uint32 {
	return aaac.UserInfo.GetMaxSubLength().GetMaximumLength()
}

func (aaac *aassociationAC) SetMaxSubLength(length uint32) {
	aaac.UserInfo.GetMaxSubLength().SetMaximumLength(length)
}

func (aaac *aassociationAC) Size() uint32 {
	aaac.Length = 4 + 16 + 16 + 32
	aaac.Length += uint32(aaac.AppContext.Size())

	for _, PresContextAccept := range aaac.PresContextAccepts {
		aaac.Length += uint32(PresContextAccept.Size())
	}

	aaac.Length += uint32(aaac.UserInfo.Size())
	return aaac.Length + 6
}

func (aaac *aassociationAC) Write(rw *bufio.ReadWriter) error {
	bd := media.NewEmptyBufData()

	fmt.Println()

	log.Printf("INFO, ASSOC-AC: %s <-- %s\n", aaac.CallingAE, aaac.CalledAE)
	log.Printf("INFO, ASSOC-AC: \tImpClass %s\n", aaac.UserInfo.GetImpClass().UIDName)
	log.Printf("INFO, ASSOC-AC: \tImpVersion %s\n\n", aaac.UserInfo.GetImpVersion().UIDName)

	bd.SetBigEndian(true)
	aaac.Size()
	bd.WriteByte(aaac.ItemType)
	bd.WriteByte(aaac.Reserved1)
	bd.WriteUint32(aaac.Length)
	bd.WriteUint16(aaac.ProtocolVersion)
	bd.WriteUint16(aaac.Reserved2)
	bd.Write(aaac.CalledAE[:], 16)
	bd.Write(aaac.CallingAE[:], 16)
	bd.Write(aaac.Reserved3[:], 32)

	err := bd.Send(rw)
	if err != nil {
		return err
	}
	err = aaac.AppContext.Write(rw)
	if err != nil {
		return err
	}
	for _, PresContextAccept := range aaac.PresContextAccepts {
		PresContextAccept.Write(rw)
	}
	return aaac.UserInfo.Write(rw)
}

func (aaac *aassociationAC) Read(rw *bufio.ReadWriter) (err error) {
	aaac.ItemType, err = ReadByte(rw)
	if err != nil {
		return
	}
	return aaac.ReadDynamic(rw)
}

func (aaac *aassociationAC) ReadDynamic(rw *bufio.ReadWriter) (err error) {
	aaac.Reserved1, err = ReadByte(rw)
	if err != nil {
		return
	}
	aaac.Length, err = ReadUint32(rw)
	if err != nil {
		return
	}
	aaac.ProtocolVersion, err = ReadUint16(rw)
	if err != nil {
		return
	}
	aaac.Reserved2, err = ReadUint16(rw)
	if err != nil {
		return
	}

	rw.Read(aaac.CalledAE[:])
	rw.Read(aaac.CallingAE[:])
	rw.Read(aaac.Reserved3[:])

	Count := int(aaac.Length - 4 - 16 - 16 - 32)
	for Count > 0 {
		TempByte, err := ReadByte(rw)
		if err != nil {
			return err
		}

		switch TempByte {
		case 0x10:
			aaac.AppContext.ReadDynamic(rw)
			Count = Count - int(aaac.AppContext.Size())
			break
		case 0x21:
			PresContextAccept := NewPresentationContextAccept()
			PresContextAccept.ReadDynamic(rw)
			Count = Count - int(PresContextAccept.Size())
			aaac.PresContextAccepts = append(aaac.PresContextAccepts, PresContextAccept)
			break
		case 0x50: // User Information
			aaac.UserInfo.ReadDynamic(rw)
			Count = Count - int(aaac.UserInfo.Size())
			break
		default:
			Count = -1
			return errors.New("ERROR, aaac::ReadDynamic, unknown Item, " + strconv.Itoa(int(TempByte)))
		}
	}

	log.Printf("INFO, ASSOC-AC: %s --> %s\n", aaac.GetCallingAE(), aaac.GetCalledAE())
	log.Printf("INFO, ASSOC-AC: \tImpClass %s\n", aaac.GetUserInformation().GetImpClass().UIDName)
	log.Printf("INFO, ASSOC-AC: \tImpVersion %s\n\n", aaac.GetUserInformation().GetImpVersion().UIDName)

	if Count == 0 {
		return nil
	}

	return errors.New("ERROR, aarq::ReadDynamic, Count is not zero")
}
