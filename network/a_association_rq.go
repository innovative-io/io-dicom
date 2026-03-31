package network

import (
	"bufio"
	"errors"
	"log/slog"
	"strconv"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
)

// AssociationRequest - AssociationRequest
type AssociationRequest interface {
	GetAppContext() UIDItem
	SetAppContext(context UIDItem)
	GetCallingAE() string
	SetCallingAE(aeTitle string)
	GetCalledAE() string
	SetCalledAE(aeTitle string)
	GetPresContexts() []PresentationContext
	GetUserInformation() UserInformation
	SetUserInformation(userInfo UserInformation)
	GetMaxSubLength() uint32
	SetMaxSubLength(length uint32)
	GetImplementationClass() UIDItem
	SetImplementationClassUID(uid string)
	SetImplementationVersionName(name string)
	Size() uint32
	Write(rw *bufio.ReadWriter) error
	Read(ms media.MemoryStream) error
	AddPresContexts(presentationContext PresentationContext)
}

type associationRequest struct {
	ItemType        byte // 0x01
	Reserved1       byte
	Length          uint32
	ProtocolVersion uint16 // 0x01
	Reserved2       uint16
	CallingAE       [16]byte // 16 bytes transfered
	CalledAE        [16]byte // 16 bytes transfered
	Reserved3       [32]byte
	AppContext      UIDItem
	PresContexts    []PresentationContext
	UserInfo        UserInformation
}

// NewAssociationRequest - NewAssociationRequest
func NewAssociationRequest() AssociationRequest {
	return &associationRequest{
		ItemType:        0x01,
		Reserved1:       0x00,
		ProtocolVersion: 0x01,
		Reserved2:       0x00,
		AppContext: &uidItem{
			itemType:  0x10,
			reserved1: 0x00,
			uid:       sopclass.DICOMApplicationContext.UID,
			length:    uint16(len(sopclass.DICOMApplicationContext.UID)),
		},
		PresContexts: make([]PresentationContext, 0),
		UserInfo:     NewUserInformation(),
	}
}

func (aarq *associationRequest) GetAppContext() UIDItem {
	return aarq.AppContext
}

func (aarq *associationRequest) SetAppContext(context UIDItem) {
	aarq.AppContext = context
}

func (aarq *associationRequest) GetCallingAE() string {
	return parseAETitle(aarq.CallingAE)
}

func (aarq *associationRequest) SetCallingAE(aeTitle string) {
	formatAETitle(&aarq.CallingAE, aeTitle)
}

func (aarq *associationRequest) GetCalledAE() string {
	return parseAETitle(aarq.CalledAE)
}

func (aarq *associationRequest) SetCalledAE(aeTitle string) {
	formatAETitle(&aarq.CalledAE, aeTitle)
}

func (aarq *associationRequest) GetPresContexts() []PresentationContext {
	return aarq.PresContexts
}

func (aarq *associationRequest) GetUserInformation() UserInformation {
	return aarq.UserInfo
}

func (aarq *associationRequest) SetUserInformation(userInfo UserInformation) {
	aarq.UserInfo = userInfo
}

func (aarq *associationRequest) GetMaxSubLength() uint32 {
	return aarq.UserInfo.GetMaxSubLength().GetMaximumLength()
}

func (aarq *associationRequest) SetMaxSubLength(length uint32) {
	aarq.UserInfo.GetMaxSubLength().SetMaximumLength(length)
}

func (aarq *associationRequest) GetImplementationClass() UIDItem {
	return aarq.UserInfo.GetImplementationClass()
}

func (aarq *associationRequest) SetImplementationClassUID(uid string) {
	aarq.UserInfo.SetImplementationClassUID(uid)
}

func (aarq *associationRequest) SetImplementationVersionName(name string) {
	aarq.UserInfo.SetImplementationVersionName(name)
}

func (aarq *associationRequest) Size() uint32 {
	aarq.Length = 4 + 16 + 16 + 32
	aarq.Length += uint32(aarq.AppContext.GetSize())

	for _, PresContext := range aarq.PresContexts {
		aarq.Length += uint32(PresContext.Size())
	}

	aarq.Length += uint32(aarq.UserInfo.Size())
	return aarq.Length + 6
}

func (aarq *associationRequest) Write(rw *bufio.ReadWriter) error {
	bd := media.NewEmptyBufData()

	slog.Info("ASSOC-RQ:", "CallingAE", aarq.GetCallingAE(), "CalledAE", aarq.GetCalledAE())
	slog.Info("ASSOC-RQ:", "ImpClass", aarq.GetUserInformation().GetImplementationClass().GetUID())
	slog.Info("ASSOC-RQ:", "ImpVersion", aarq.GetUserInformation().GetImplementationVersion().GetUID())
	slog.Info("ASSOC-RQ:", "MaxPDULength", aarq.GetUserInformation().GetMaxSubLength().GetMaximumLength())
	slog.Info("ASSOC-RQ:", "MaxOpsInvoked", aarq.GetUserInformation().GetAsyncOperationWindow().GetMaxNumberOperationsInvoked(), "MaxOpsPerformed", aarq.GetUserInformation().GetAsyncOperationWindow().GetMaxNumberOperationsPerformed())

	bd.SetBigEndian(true)
	aarq.Size()
	bd.WriteByte(aarq.ItemType)
	bd.WriteByte(aarq.Reserved1)
	bd.WriteUint32(aarq.Length)
	bd.WriteUint16(aarq.ProtocolVersion)
	bd.WriteUint16(aarq.Reserved2)
	bd.Write(aarq.CalledAE[:], 16)
	bd.Write(aarq.CallingAE[:], 16)
	bd.Write(aarq.Reserved3[:], 32)

	if err := bd.Send(rw); err != nil {
		return err
	}

	slog.Info("ASSOC-RQ: ApplicationContext", "UID", aarq.AppContext.GetUID(), "Description", sopclass.GetSOPClassFromUID(aarq.AppContext.GetUID()).Description)
	if err := aarq.AppContext.Write(rw); err != nil {
		return err
	}
	for presIndex, presContext := range aarq.PresContexts {
		slog.Info("ASSOC-RQ: PresentationContext", "Index", presIndex+1)
		slog.Info("ASSOC-RQ: \tAbstractSyntax:", "UID", presContext.GetAbstractSyntax().GetUID(), "Description", sopclass.GetSOPClassFromUID(presContext.GetAbstractSyntax().GetUID()).Description)
		for _, transSyntax := range presContext.GetTransferSyntaxes() {
			slog.Info("ASSOC-RQ: \tTransferSyntax:", "UID", transSyntax.GetUID(), "Description", transfersyntax.GetTransferSyntaxFromUID(transSyntax.GetUID()).Description)
		}
		if err := presContext.Write(rw); err != nil {
			return err
		}
	}
	return aarq.UserInfo.Write(rw)
}

func (aarq *associationRequest) Read(ms media.MemoryStream) (err error) {
	if aarq.ProtocolVersion, err = ms.GetUint16(); err != nil {
		return err
	}
	if aarq.Reserved2, err = ms.GetUint16(); err != nil {
		return err
	}

	ms.ReadData(aarq.CalledAE[:])
	ms.ReadData(aarq.CallingAE[:])
	ms.ReadData(aarq.Reserved3[:])

	Count := int(ms.GetSize() - 4 - 16 - 16 - 32)
	for Count > 0 {
		TempByte, err := ms.GetByte()
		if err != nil {
			return err
		}

		switch TempByte {
		case 0x10:
			aarq.AppContext.SetType(TempByte)
			aarq.AppContext.ReadDynamic(ms)
			Count = Count - int(aarq.AppContext.GetSize())
		case 0x20:
			PresContext := NewPresentationContext()
			PresContext.ReadDynamic(ms)
			Count = Count - int(PresContext.Size())
			aarq.PresContexts = append(aarq.PresContexts, PresContext)
		case 0x50: // User Information
			aarq.UserInfo.ReadDynamic(ms)
			return nil
		default:
			slog.Error("aarq::ReadDynamic, unknown Item " + strconv.Itoa(int(TempByte)))
			Count = -1
		}
	}

	if Count == 0 {
		return nil
	}

	return errors.New("aarq::ReadDynamic, Count is not zero")
}

func (aarq *associationRequest) AddPresContexts(presentationContext PresentationContext) {
	aarq.PresContexts = append(aarq.PresContexts, presentationContext)
}
