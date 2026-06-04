package network

import (
	"bufio"
	"crypto/x509"
	"errors"
	"log/slog"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

// AssociationRequest - AssociationRequest
type AssociationRequest interface {
	GetAppContext() UIDItem
	SetAppContext(context UIDItem)
	GetCallingAE() string
	SetCallingAE(aeTitle string)
	GetCalledAE() string
	SetCalledAE(aeTitle string)
	GetRemoteAddress() string
	SetRemoteAddress(address string)
	GetPresContexts() []PresentationContext
	GetMaxSubLength() uint32
	SetMaxSubLength(length uint32)
	GetImplementationClass() UIDItem
	SetImplementationClassUID(uid string)
	// GetImplementationVersionName returns the implementation version name string
	// sent in the A-ASSOCIATE-RQ user information item, or empty string if absent.
	GetImplementationVersionName() string
	SetImplementationVersionName(name string)
	GetPeerCertificates() []*x509.Certificate
	SetPeerCertificates(certs []*x509.Certificate)
	GetCorrelationID() string
	SetCorrelationID(id string)
	Size() uint32
	Write(rw *bufio.ReadWriter) error
	Read(buf *media.DICOMBuffer) error
	AddPresContexts(presentationContext PresentationContext)
}

type associationRequest struct {
	ItemType         byte // 0x01
	Reserved1        byte
	Length           uint32
	ProtocolVersion  uint16 // 0x01
	Reserved2        uint16
	CallingAE        [16]byte // 16 bytes transfered
	CalledAE         [16]byte // 16 bytes transfered
	Reserved3        [32]byte
	AppContext       UIDItem
	PresContexts     []PresentationContext
	UserInfo         *userInformation
	PeerCertificates []*x509.Certificate
	RemoteAddress    string
	CorrelationID    string
	logger           *slog.Logger
}

// NewAssociationRequest returns a new AssociationRequest with default settings.
func NewAssociationRequest() AssociationRequest {
	return newAssociationRequest()
}

func newAssociationRequest() *associationRequest {
	return &associationRequest{
		ItemType:        pdutype.AssociationRequest,
		Reserved1:       0x00,
		ProtocolVersion: ProtocolVersionCurrent,
		Reserved2:       0x00,
		AppContext: &uidItem{
			itemType:  pdutype.ApplicationContextItem,
			reserved1: 0x00,
			uid:       sopclass.DICOMApplicationContext.UID,
			length:    uint16(len(sopclass.DICOMApplicationContext.UID)),
		},
		PresContexts: make([]PresentationContext, 0),
		UserInfo:     newUserInformation(),
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

func (aarq *associationRequest) GetRemoteAddress() string {
	return aarq.RemoteAddress
}

func (aarq *associationRequest) SetRemoteAddress(address string) {
	aarq.RemoteAddress = address
}

func (aarq *associationRequest) GetPresContexts() []PresentationContext {
	return aarq.PresContexts
}

func (aarq *associationRequest) GetUserInformation() *userInformation {
	return aarq.UserInfo
}

func (aarq *associationRequest) SetUserInformation(userInfo *userInformation) {
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

func (aarq *associationRequest) GetImplementationVersionName() string {
	return aarq.UserInfo.GetImplementationVersion().GetUID()
}

func (aarq *associationRequest) GetPeerCertificates() []*x509.Certificate {
	return aarq.PeerCertificates
}

func (aarq *associationRequest) SetPeerCertificates(certs []*x509.Certificate) {
	aarq.PeerCertificates = certs
}

func (aarq *associationRequest) GetCorrelationID() string {
	return aarq.CorrelationID
}

func (aarq *associationRequest) SetCorrelationID(id string) {
	aarq.CorrelationID = id
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
	bd := media.NewDICOMBuffer()

	aaLog := loggerOrDefault(aarq.logger)
	aaLog.Debug("encoding A-ASSOCIATE-RQ",
		"calling_ae", aarq.GetCallingAE(), "called_ae", aarq.GetCalledAE(),
		"impl_class", aarq.GetUserInformation().GetImplementationClass().GetUID(),
		"impl_version", aarq.GetUserInformation().GetImplementationVersion().GetUID(),
		"max_pdu_length", aarq.GetUserInformation().GetMaxSubLength().GetMaximumLength())

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

	aaLog.Debug("application context", "uid", aarq.AppContext.GetUID(), "description", sopClassDescription(aarq.AppContext.GetUID()))
	if err := aarq.AppContext.Write(rw); err != nil {
		return err
	}
	for presIndex, presContext := range aarq.PresContexts {
		aaLog.Debug("proposed presentation context",
			"index", presIndex+1, "pcid", presContext.GetPresentationContextID(),
			"abstract_syntax", presContext.GetAbstractSyntax().GetUID(),
			"abstract_syntax_description", sopClassDescription(presContext.GetAbstractSyntax().GetUID()))
		for _, transSyntax := range presContext.GetTransferSyntaxes() {
			aaLog.Debug("proposed transfer syntax",
				"uid", transSyntax.GetUID(), "description", transferSyntaxDescription(transSyntax.GetUID()))
		}
		if err := presContext.Write(rw); err != nil {
			return err
		}
	}
	return aarq.UserInfo.Write(rw)
}

func (aarq *associationRequest) Read(buf *media.DICOMBuffer) (err error) {
	if aarq.ProtocolVersion, err = buf.ReadUint16(true); err != nil {
		return err
	}
	if aarq.Reserved2, err = buf.ReadUint16(true); err != nil {
		return err
	}

	buf.ReadData(aarq.CalledAE[:])
	buf.ReadData(aarq.CallingAE[:])
	buf.ReadData(aarq.Reserved3[:])

	Count := int(buf.GetSize() - 4 - 16 - 16 - 32)
	for Count > 0 {
		startPos := buf.GetPosition()
		TempByte, err := buf.GetByte()
		if err != nil {
			return err
		}

		switch TempByte {
		case pdutype.ApplicationContextItem:
			aarq.AppContext.SetType(TempByte)
			if err := aarq.AppContext.ReadDynamic(buf); err != nil {
				return err
			}
		case pdutype.PresentationContextItem:
			PresContext := NewPresentationContext()
			if err := PresContext.ReadDynamic(buf); err != nil {
				return err
			}
			aarq.PresContexts = append(aarq.PresContexts, PresContext)
		case pdutype.UserInformationItem: // User Information
			return aarq.UserInfo.ReadDynamic(buf)
		default:
			loggerOrDefault(aarq.logger).Error("unknown A-ASSOCIATE-RQ item", "item_type", TempByte)
			return errors.New("aarq::ReadDynamic, unknown item type")
		}
		// Decrement by bytes actually consumed; the per-item uint16 Size()/GetSize()
		// could wrap on a malformed large length and corrupt the accounting.
		Count -= buf.GetPosition() - startPos
	}

	if Count == 0 {
		return nil
	}

	return errors.New("aarq::ReadDynamic, Count is not zero")
}

func (aarq *associationRequest) AddPresContexts(presentationContext PresentationContext) {
	aarq.PresContexts = append(aarq.PresContexts, presentationContext)
}
