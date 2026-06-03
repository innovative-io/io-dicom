package network

import (
	"bufio"
	"errors"
	"log/slog"
	"strconv"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	implementation "github.com/innovative-io/io-dicom/internal/implclass"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

// AssociationAccept AssociationAccept
type AssociationAccept interface {
	GetAppContext() UIDItem
	SetAppContext(context UIDItem)
	GetCallingAE() string
	SetCallingAE(aeTitle string)
	GetCalledAE() string
	SetCalledAE(aeTitle string)
	AddPresContextAccept(context PresentationContextAccept)
	GetPresContextAccepts() []PresentationContextAccept
	GetMaxSubLength() uint32
	SetMaxSubLength(length uint32)
	Size() uint32
	Write(rw *bufio.ReadWriter) error
	Read(buf *media.DICOMBuffer) (err error)
	ReadDynamic(buf *media.DICOMBuffer) (err error)
}

type associationAccept struct {
	ItemType           byte
	Reserved1          byte
	Length             uint32
	ProtocolVersion    uint16
	Reserved2          uint16
	CallingAE          [16]byte
	CalledAE           [16]byte
	Reserved3          [32]byte
	AppContext         UIDItem
	PresContextAccepts []PresentationContextAccept
	UserInfo           *userInformation
	logger             *slog.Logger
}

func newAssociationAccept() *associationAccept {
	return &associationAccept{
		ItemType:        pdutype.AssociationAccept,
		Reserved1:       0x00,
		ProtocolVersion: ProtocolVersionCurrent,
		Reserved2:       0x00,
		AppContext: &uidItem{
			itemType:  pdutype.ApplicationContextItem,
			reserved1: 0x00,
			uid:       sopclass.DICOMApplicationContext.UID,
			length:    uint16(len(sopclass.DICOMApplicationContext.UID)),
		},
		PresContextAccepts: make([]PresentationContextAccept, 0),
		UserInfo:           newUserInformation(),
	}
}

func (aaac *associationAccept) GetAppContext() UIDItem {
	return aaac.AppContext
}

func (aaac *associationAccept) SetAppContext(context UIDItem) {
	aaac.AppContext = context
}

func (aaac *associationAccept) GetCallingAE() string {
	return parseAETitle(aaac.CallingAE)
}

func (aaac *associationAccept) SetCallingAE(aeTitle string) {
	formatAETitle(&aaac.CallingAE, aeTitle)
}

func (aaac *associationAccept) GetCalledAE() string {
	return parseAETitle(aaac.CalledAE)
}

func (aaac *associationAccept) SetCalledAE(aeTitle string) {
	formatAETitle(&aaac.CalledAE, aeTitle)
}

func (aaac *associationAccept) AddPresContextAccept(context PresentationContextAccept) {
	aaac.PresContextAccepts = append(aaac.PresContextAccepts, context)
}

func (aaac *associationAccept) GetPresContextAccepts() []PresentationContextAccept {
	return aaac.PresContextAccepts
}

func (aaac *associationAccept) getUserInfo() *userInformation {
	return aaac.UserInfo
}

func (aaac *associationAccept) setUserInfo(userInfo *userInformation) {
	aaac.UserInfo = userInfo
}

func (aaac *associationAccept) GetMaxSubLength() uint32 {
	return aaac.UserInfo.MaxSubLength.GetMaximumLength()
}

func (aaac *associationAccept) SetMaxSubLength(length uint32) {
	aaac.UserInfo.MaxSubLength.SetMaximumLength(length)
}

func (aaac *associationAccept) Size() uint32 {
	aaac.Length = 4 + 16 + 16 + 32
	aaac.Length += uint32(aaac.AppContext.GetSize())

	for _, PresContextAccept := range aaac.PresContextAccepts {
		aaac.Length += uint32(PresContextAccept.Size())
	}

	aaac.Length += uint32(aaac.UserInfo.Size())
	return aaac.Length + 6
}

func (aaac *associationAccept) Write(rw *bufio.ReadWriter) error {
	bd := media.NewDICOMBuffer()

	loggerOrDefault(aaac.logger).Debug("encoding A-ASSOCIATE-AC",
		"calling_ae", aaac.GetCallingAE(), "called_ae", aaac.GetCalledAE(),
		"impl_class", aaac.UserInfo.GetImplementationClass().GetUID(),
		"impl_version", aaac.UserInfo.GetImplementationVersion().GetUID(),
		"max_pdu_length", aaac.GetMaxSubLength(),
		"max_ops_invoked", aaac.UserInfo.AsyncOpWindow.GetMaxNumberOperationsInvoked(),
		"max_ops_performed", aaac.UserInfo.AsyncOpWindow.GetMaxNumberOperationsPerformed())

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

	if err := bd.Send(rw); err != nil {
		return err
	}

	if err := aaac.AppContext.Write(rw); err != nil {
		return err
	}
	for _, presContextAccept := range aaac.PresContextAccepts {
		// Propagate the connection logger so the context's encode traces carry
		// the same per-association correlation attributes rather than falling
		// back to slog.Default().
		if pc, ok := presContextAccept.(*presentationContextAccept); ok {
			pc.logger = aaac.logger
		}
		if err := presContextAccept.Write(rw); err != nil {
			return err
		}
	}
	return aaac.UserInfo.Write(rw)
}

func (aaac *associationAccept) Read(buf *media.DICOMBuffer) (err error) {
	if aaac.ItemType, err = buf.GetByte(); err != nil {
		return err
	}
	return aaac.ReadDynamic(buf)
}

func (aaac *associationAccept) ReadDynamic(buf *media.DICOMBuffer) (err error) {
	if aaac.Reserved1, err = buf.GetByte(); err != nil {
		return err
	}
	if aaac.Length, err = buf.ReadUint32(true); err != nil {
		return err
	}
	if aaac.ProtocolVersion, err = buf.ReadUint16(true); err != nil {
		return err
	}
	if aaac.Reserved2, err = buf.ReadUint16(true); err != nil {
		return err
	}

	buf.ReadData(aaac.CalledAE[:])
	buf.ReadData(aaac.CallingAE[:])
	buf.ReadData(aaac.Reserved3[:])

	Count := int(aaac.Length - 4 - 16 - 16 - 32)

	for Count > 0 {
		TempByte, err := buf.GetByte()
		if err != nil {
			return err
		}

		switch TempByte {
		case pdutype.ApplicationContextItem:
			aaac.AppContext.ReadDynamic(buf)
			Count = Count - int(aaac.AppContext.GetSize())
		case pdutype.PresentationContextAcceptItem:
			PresContextAccept := NewPresentationContextAccept()
			PresContextAccept.ReadDynamic(buf)
			Count = Count - int(PresContextAccept.Size())
			aaac.PresContextAccepts = append(aaac.PresContextAccepts, PresContextAccept)
		case pdutype.UserInformationItem: // User Information
			aaac.UserInfo.ReadDynamic(buf)
			Count = Count - int(aaac.UserInfo.Size())
		default:
			Count = -1
			return errors.New("aaac::ReadDynamic, unknown Item " + strconv.Itoa(int(TempByte)))
		}
	}

	aaLog := loggerOrDefault(aaac.logger)
	aaLog.Debug("decoded A-ASSOCIATE-AC",
		"calling_ae", aaac.GetCallingAE(), "called_ae", aaac.GetCalledAE(),
		"our_impl_class", implementation.GetImplementationClassUID(),
		"our_impl_version", implementation.GetImplementationVersion(),
		"their_impl_class", aaac.UserInfo.GetImplementationClass().GetUID(),
		"their_impl_version", aaac.UserInfo.GetImplementationVersion().GetUID(),
		"app_context", aaac.AppContext.GetUID(),
		"app_context_description", sopclass.GetSOPClassFromUID(aaac.AppContext.GetUID()).Description,
		"our_max_pdu_length", maxPduLength, "their_max_pdu_length", aaac.GetMaxSubLength())
	for presIndex, presContextAccept := range aaac.PresContextAccepts {
		aaLog.Debug("accepted presentation context",
			"index", presIndex+1, "pcid", presContextAccept.GetPresentationContextID(),
			"transfer_syntax", presContextAccept.GetTrnSyntax().GetUID(),
			"transfer_syntax_description", transfersyntax.GetTransferSyntaxFromUID(presContextAccept.GetTrnSyntax().GetUID()).Description)
	}
	if Count == 0 {
		return nil
	}

	return errors.New("aarq::ReadDynamic, Count is not zero")
}
