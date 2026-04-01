package network

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/implementation"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/pdutype"
)

// ErrAssociationReleased is returned by NextPDU when the peer performs an A-RELEASE handshake.
var ErrAssociationReleased = errors.New("DICOM association released")

// ErrAssociationAborted is returned by NextPDU when the peer sends an A-ABORT PDU.
var ErrAssociationAborted = errors.New("DICOM association aborted")

// ErrAssociationRejected is returned by NextPDU when the local handler rejects the
// incoming A-ASSOCIATE-RQ. The SCP must close the transport connection after rejection
// (DICOM PS 3.8 §9.3.4).
var ErrAssociationRejected = errors.New("DICOM association rejected")

// PDUService - struct for PDUService
type PDUService interface {
	GetTransferSyntax(pcid byte) *transfersyntax.TransferSyntax
	SetTimeout(timeout int)
	Connect(IP string, Port string) error
	// ConnectTLS dials the remote AE over TLS and negotiates an A-ASSOCIATE.
	// Pass nil for cfg to use the system certificate pool without a client certificate.
	ConnectTLS(IP string, Port string, cfg *tls.Config) error
	Close()
	GetAAssociationRQ() AssociationRequest
	GetCalledAE() string
	GetCallingAE() string
	SetCalledAE(calledAE string)
	SetCallingAE(callingAE string)
	SetConn(rw *bufio.ReadWriter)
	SetNetConn(conn net.Conn)
	NextPDU() (media.DICOMObject, error)
	AddPresContexts(presentationContext PresentationContext)
	GetPresentationContextID() byte
	SetOnAssociationRequest(f func(request AssociationRequest) bool)
	Write(DCO media.DICOMObject, ItemType byte) error
}

type pduService struct {
	AcceptedPresentationContexts []PresentationContextAccept
	conn                         net.Conn
	readWriter                   *bufio.ReadWriter
	ms                           media.MemoryStream
	pdutype                      int
	pdulength                    uint32
	AssocRQ                      AssociationRequest
	AssocAC                      AssociationAccept
	AssocRJ                      AssociationReject
	ReleaseRQ                    ReleaseRequest
	ReleaseRP                    ReleaseResponse
	AbortRQ                      AbortRequest
	Pdata                        PresentationDataTransfer
	Timeout                      int
	OnAssociationRequest         func(request AssociationRequest) bool
}

// NewPDUService - creates a pointer to PDUService
func NewPDUService() PDUService {
	return &pduService{
		ms:        media.NewEmptyMemoryStream(),
		AssocRQ:   NewAssociationRequest(),
		AssocAC:   NewAssociationAccept(),
		AssocRJ:   NewAssociationReject(),
		ReleaseRQ: NewReleaseRequest(),
		ReleaseRP: NewReleaseResponse(),
		AbortRQ:   NewAbortRequest(),
	}
}

const maxPduLength uint32 = 16384

const releaseHandshakeTimeout = 5 * time.Second

func (pdu *pduService) SetConn(rw *bufio.ReadWriter) {
	pdu.readWriter = rw
}

func (pdu *pduService) SetNetConn(conn net.Conn) {
	pdu.conn = conn
}

func (pdu *pduService) closeConn() {
	if pdu.conn != nil {
		_ = pdu.conn.Close()
		pdu.conn = nil
	}
	pdu.readWriter = nil
}

func selectPreferredTransferSyntax(offered []UIDItem) (string, bool) {
	preferred := []string{
		transfersyntax.ExplicitVRLittleEndian.UID,
		transfersyntax.ImplicitVRLittleEndian.UID,
		transfersyntax.ExplicitVRBigEndian.UID,
	}

	for _, candidate := range preferred {
		if !transfersyntax.SupportedTransferSyntax(candidate) {
			continue
		}
		for _, item := range offered {
			if item.GetUID() == candidate {
				return candidate, true
			}
		}
	}

	for _, item := range offered {
		if transfersyntax.SupportedTransferSyntax(item.GetUID()) {
			return item.GetUID(), true
		}
	}

	return "", false
}

func selectDefaultPresentationContextID(accepted []PresentationContextAccept) (byte, bool) {
	preferred := []string{
		transfersyntax.ExplicitVRLittleEndian.UID,
		transfersyntax.ImplicitVRLittleEndian.UID,
		transfersyntax.ExplicitVRBigEndian.UID,
	}

	for _, candidate := range preferred {
		for _, context := range accepted {
			if context.GetResult() == 0 && context.GetTrnSyntax().GetUID() == candidate {
				return context.GetPresentationContextID(), true
			}
		}
	}

	for _, context := range accepted {
		if context.GetResult() == 0 {
			return context.GetPresentationContextID(), true
		}
	}

	return 0, false
}

func (pdu *pduService) GetTransferSyntax(pcid byte) *transfersyntax.TransferSyntax {
	for _, pca := range pdu.AcceptedPresentationContexts {
		if pca.GetPresentationContextID() == pcid {
			return transfersyntax.GetTransferSyntaxFromUID(pca.GetTrnSyntax().GetUID())
		}
	}
	return nil
}

func (pdu *pduService) SetTimeout(timeout int) {
	pdu.Timeout = timeout
}

func (pdu *pduService) Connect(IP string, Port string) error {
	pdu.AcceptedPresentationContexts = nil
	pdu.Pdata.PresentationContextID = 0

	conn, err := net.Dial("tcp", IP+":"+Port)
	if err != nil {
		return errors.New("pduservice::Connect - " + err.Error())
	}
	return pdu.finishConnect(conn)
}

// ConnectTLS dials the remote AE with TLS and negotiates an A-ASSOCIATE.
// Pass nil for cfg to use the system certificate pool with no client certificate.
func (pdu *pduService) ConnectTLS(IP string, Port string, cfg *tls.Config) error {
	pdu.AcceptedPresentationContexts = nil
	pdu.Pdata.PresentationContextID = 0

	conn, err := tls.Dial("tcp", IP+":"+Port, normalizeClientTLSConfig(cfg))
	if err != nil {
		return fmt.Errorf("pduservice::ConnectTLS - %w", err)
	}
	return pdu.finishConnect(conn)
}

func normalizeClientTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
	clone := cfg.Clone()
	if clone.MinVersion == 0 || clone.MinVersion < tls.VersionTLS12 {
		clone.MinVersion = tls.VersionTLS12
	}
	return clone
}

// finishConnect completes an A-ASSOCIATE handshake over an already-established
// connection (plain TCP or TLS).
func (pdu *pduService) finishConnect(conn net.Conn) error {
	pdu.conn = conn
	if pdu.Timeout > 0 {
		conn.SetDeadline(time.Now().Add(time.Duration(pdu.Timeout) * time.Second))
	}

	rw := bufio.NewReadWriter(
		bufio.NewReaderSize(conn, 256*1024),
		bufio.NewWriterSize(conn, 256*1024),
	)

	pdu.readWriter = rw
	pdu.AssocRQ.SetMaxSubLength(maxPduLength)
	pdu.AssocRQ.SetImplementationClassUID(implementation.GetImplementationClassUID())
	pdu.AssocRQ.SetImplementationVersionName(implementation.GetImplementationVersion())

	if err := pdu.AssocRQ.Write(pdu.readWriter); err != nil {
		return err
	}

	pdu.ms = media.NewEmptyMemoryStream()

	if err := pdu.ms.ReadFully(rw, 10); err != nil {
		return err
	}

	ItemType, err := pdu.ms.GetByte()
	if err != nil {
		return err
	}

	if _, err = pdu.ms.GetByte(); err != nil {
		return err
	}

	if pdu.pdulength, err = pdu.ms.GetUint32(); err != nil {
		return err
	}

	switch ItemType {
	case pdutype.AssociationAccept:
		if err := pdu.readPDU(); err != nil {
			return err
		}
		pdu.ms.SetPosition(1)
		pdu.AssocAC.ReadDynamic(pdu.ms)
		if !pdu.interogateAAssociateAC() {
			return errors.New("pduservice::Connect - No accepted presentation contexts found")
		}
		return nil
	case pdutype.AssociationReject:
		if err := pdu.readPDU(); err != nil {
			return err
		}
		pdu.ms.SetPosition(1)
		pdu.AssocRJ.ReadDynamic(pdu.ms)
		return fmt.Errorf("pduservice::Connect - Association rejected - %s", pdu.AssocRJ.GetReason())
	case pdutype.AssociationAbortRequest:
		if err := pdu.readPDU(); err != nil {
			return err
		}
		pdu.ms.SetPosition(1)
		pdu.AbortRQ.ReadDynamic(pdu.ms)
		return fmt.Errorf("pduservice::Connect - Association aborted - %s", pdu.AbortRQ.GetReason())
	default:
		return fmt.Errorf("pduservice::Connect - Corrupt transmision - %b", ItemType)
	}
}

func (pdu *pduService) Close() {
	if pdu.readWriter == nil {
		return
	}

	if err := pdu.ReleaseRQ.Write(pdu.readWriter); err != nil {
		slog.Warn("pduservice::Close - failed to send A-RELEASE-RQ, sending A-ABORT", "error", err)
		_ = pdu.AbortRQ.Write(pdu.readWriter)
		pdu.closeConn()
		return
	}

	if pdu.conn != nil {
		_ = pdu.conn.SetDeadline(time.Now().Add(releaseHandshakeTimeout))
	}

	ms := media.NewEmptyMemoryStream()
	if err := ms.ReadFully(pdu.readWriter, 10); err != nil {
		slog.Warn("pduservice::Close - timeout waiting for A-RELEASE-RP, sending A-ABORT", "error", err)
		_ = pdu.AbortRQ.Write(pdu.readWriter)
	} else {
		itemType, _ := ms.GetByte()
		if int(itemType) != pdutype.AssociationReleaseResponse {
			slog.Warn("pduservice::Close - unexpected PDU waiting for A-RELEASE-RP", "type", itemType)
		}
	}

	pdu.closeConn()
}

func (pdu *pduService) NextPDU() (command media.DICOMObject, err error) {
	if pdu.Pdata.Buffer != nil {
		pdu.Pdata.Buffer.ClearMemoryStream()
	} else {
		pdu.Pdata.Buffer = media.NewEmptyBufData()
	}

	pdu.Pdata.MsgStatus = 0
	if pdu.Pdata.Length != 0 {
		DCO := media.NewEmptyDCMObj()
		pdu.Pdata.ReadDynamic(pdu.ms)
		if pdu.Pdata.MsgStatus > 0 {
			if !pdu.parseRawVRIntoDCM(DCO) {
				pdu.AbortRQ.Write(pdu.readWriter)
				return nil, errors.New("pduservice::Read - ParseRawVRIntoDCM failed")
			}
			return DCO, nil
		}
	}

	for {
		pdu.ms = media.NewEmptyMemoryStream()

		if err := pdu.ms.ReadFully(pdu.readWriter, 10); err != nil {
			return nil, err
		}

		pdu.ms.SetPosition(0)

		if pdu.pdutype, err = pdu.ms.Get(); err != nil {
			return nil, err
		}

		if _, err = pdu.ms.Get(); err != nil {
			return nil, err
		}

		if pdu.pdulength, err = pdu.ms.GetUint32(); err != nil {
			return nil, err
		}

		switch pdu.pdutype {
		case pdutype.AssociationRequest:
			if err := pdu.readPDU(); err != nil {
				return nil, err
			}
			if err := pdu.AssocRQ.Read(pdu.ms); err != nil {
				return nil, err
			}
			if err := pdu.interogateAAssociateRQ(pdu.readWriter); err != nil {
				if errors.Is(err, ErrAssociationRejected) {
					// After sending A-ASSOCIATE-RJ, rejecting AE must close transport.
					pdu.closeConn()
				}
				return nil, err
			}
			return nil, nil
		case pdutype.AssociationAccept:
			if err := pdu.readPDU(); err != nil {
				return nil, err
			}
			return nil, nil
		case pdutype.PDUDataTransfer:
			if err := pdu.readPDU(); err != nil {
				return nil, err
			}
			pdu.ms.SetPosition(1)
			if err := pdu.Pdata.ReadDynamic(pdu.ms); err != nil {
				return nil, err
			}
			if pdu.Pdata.MsgStatus > 0 {
				DCO := media.NewEmptyDCMObj()
				if !pdu.parseRawVRIntoDCM(DCO) {
					pdu.AbortRQ.Write(pdu.readWriter)
					return nil, errors.New("pduservice::Read - ParseRawVRIntoDCM failed")
				}
				return DCO, nil
			}
		case pdutype.AssociationReleaseRequest:
			slog.Info("ASSOC-R-RQ:", "CallingAE", pdu.AssocRQ.GetCallingAE(), "CalledAE", pdu.AssocRQ.GetCalledAE())
			pdu.ReleaseRQ.ReadDynamic(pdu.ms)
			pdu.ReleaseRP.Write(pdu.readWriter)
			return nil, ErrAssociationReleased
		case pdutype.AssociationReleaseResponse:
			slog.Info("ASSOC-R-RP:", "CallingAE", pdu.AssocRQ.GetCallingAE(), "CalledAE", pdu.AssocRQ.GetCalledAE())
			return nil, ErrAssociationReleased
		case pdutype.AssociationAbortRequest:
			slog.Info("ASSOC-ABORT-RQ:", "CallingAE", pdu.AssocRQ.GetCallingAE(), "CalledAE", pdu.AssocRQ.GetCalledAE())
			return nil, ErrAssociationAborted
		default:
			pdu.AbortRQ.Write(pdu.readWriter)
			return nil, errors.New("pduservice::Read - unknown ItemType")
		}
	}
}

func (pdu *pduService) GetAAssociationRQ() AssociationRequest {
	return pdu.AssocRQ
}

func (pdu *pduService) GetCalledAE() string {
	return pdu.AssocRQ.GetCalledAE()
}

func (pdu *pduService) GetCallingAE() string {
	return pdu.AssocRQ.GetCallingAE()
}

func (pdu *pduService) SetCalledAE(calledAE string) {
	pdu.AssocRQ.SetCalledAE(calledAE)
}

func (pdu *pduService) SetCallingAE(callingAE string) {
	pdu.AssocRQ.SetCallingAE(callingAE)
}

func (pdu *pduService) AddPresContexts(presentationContext PresentationContext) {
	pdu.AssocRQ.AddPresContexts(presentationContext)
}

func (pdu *pduService) GetPresentationContextID() byte {
	return pdu.Pdata.PresentationContextID
}

func (pdu *pduService) SetOnAssociationRequest(f func(request AssociationRequest) bool) {
	pdu.OnAssociationRequest = f
}

func (pdu *pduService) Write(DCO media.DICOMObject, ItemType byte) error {
	if pdu.Pdata.Buffer != nil {
		pdu.Pdata.Buffer.ClearMemoryStream()
	} else {
		pdu.Pdata.Buffer = media.NewEmptyBufData()
	}

	if ts := pdu.GetTransferSyntax(pdu.Pdata.PresentationContextID); ts != nil {
		DCO.SetTransferSyntax(ts)
		DCO.SetBigEndian(ts.UID == transfersyntax.ExplicitVRBigEndian.UID)
		DCO.SetExplicitVR(ts.UID != transfersyntax.ImplicitVRLittleEndian.UID)
	}

	if pdu.Pdata.PresentationContextID == 0 {
		return errors.New("pduservice::Write - PresentationContextID==0")
	}

	if !pdu.parseDCMIntoRaw(DCO) {
		return errors.New("pduservice::Write - ParseDCMIntoRaw failed")
	}

	pdu.Pdata.MsgHeader = ItemType
	if pdu.AssocAC.GetUserInformation().GetMaxSubLength().GetMaximumLength() > maxPduLength {
		pdu.AssocAC.SetMaxSubLength(maxPduLength)
	}

	// Fixed MaxLength - 6 20200811
	pdu.Pdata.BlockSize = pdu.AssocAC.GetMaxSubLength() - 6

	if ItemType > 0x00 {
		sopClassUID := DCO.GetString(tags.AffectedSOPClassUID)
		sopClass := sopclass.GetSOPClassFromUID(sopClassUID)
		if sopClass != nil {
			slog.Debug("PDU-Service: SOP Class", "UID", sopClass.UID, "Description", sopClass.Description, "CalledAE", pdu.GetCalledAE())
		} else {
			slog.Debug("PDU-Service: SOP Class", "UID", sopClassUID, "Description", "Unknown SOP Class", "CalledAE", pdu.GetCalledAE())
		}
	}

	return pdu.Pdata.Write(pdu.readWriter)
}

func (pdu *pduService) interogateAAssociateAC() bool {
	pdu.AcceptedPresentationContexts = nil

	for _, presContextAccept := range pdu.AssocAC.GetPresContextAccepts() {
		if presContextAccept.GetResult() == 0 {
			pdu.AcceptedPresentationContexts = append(pdu.AcceptedPresentationContexts, presContextAccept)
		}
	}

	if presentationContextID, ok := selectDefaultPresentationContextID(pdu.AcceptedPresentationContexts); ok {
		pdu.Pdata.PresentationContextID = presentationContextID
		return true
	}

	return false
}

func (pdu *pduService) interogateAAssociateRQ(rw *bufio.ReadWriter) error {
	if pdu.OnAssociationRequest == nil || !pdu.OnAssociationRequest(pdu.AssocRQ) {
		slog.Warn("pdu: rejecting association - rejected by application handler", "CalledAE", pdu.AssocRQ.GetCalledAE(), "CallingAE", pdu.AssocRQ.GetCallingAE())
		// Result=1 (permanent), Source=1 (UL-service-user), Reason=7 (called AE not recognised)
		// per DICOM PS3.8 Table 9-21.
		pdu.AssocRJ.Set(1, 1, 7)
		if err := pdu.AssocRJ.Write(rw); err != nil {
			return err
		}
		// Per DICOM PS 3.8 §9.3.4 the rejecting AE must close the transport connection.
		return ErrAssociationRejected
	}

	pdu.AcceptedPresentationContexts = nil
	pdu.AssocAC = NewAssociationAccept()
	pdu.AssocAC.SetCalledAE(pdu.AssocRQ.GetCalledAE())
	pdu.AssocAC.SetCallingAE(pdu.AssocRQ.GetCallingAE())
	pdu.AssocAC.SetAppContext(pdu.AssocRQ.GetAppContext())
	pdu.AssocAC.SetUserInformation(pdu.AssocRQ.GetUserInformation())

	slog.Info("ASSOC-RQ:", "CallingAE", pdu.AssocRQ.GetCallingAE(), "CalledAE", pdu.AssocRQ.GetCalledAE())
	slog.Debug("ASSOC-RQ:", "ImpClass", pdu.AssocRQ.GetUserInformation().GetImplementationClass().GetUID())
	slog.Debug("ASSOC-RQ:", "ImpVersion", pdu.AssocRQ.GetUserInformation().GetImplementationVersion().GetUID())
	slog.Debug("ASSOC-RQ:", "MaxPDULength", pdu.AssocRQ.GetUserInformation().GetMaxSubLength().GetMaximumLength())
	slog.Debug("ASSOC-RQ:", "MaxOpsInvoked", pdu.AssocRQ.GetUserInformation().GetAsyncOperationWindow().GetMaxNumberOperationsInvoked(), "MaxOpsPerformed", pdu.AssocRQ.GetUserInformation().GetAsyncOperationWindow().GetMaxNumberOperationsPerformed())

	for presIndex, PresContext := range pdu.AssocRQ.GetPresContexts() {
		slog.Debug("ASSOC-RQ: PresentationContext", "Index", presIndex)

		sopClass := sopclass.GetSOPClassFromUID(PresContext.GetAbstractSyntax().GetUID())
		sopUID := PresContext.GetAbstractSyntax().GetUID()
		sopDescription := ""
		if sopClass != nil {
			sopUID = sopClass.UID
			sopDescription = sopClass.Description
		}
		slog.Debug("ASSOC-RQ: \tAbstractContext", "UID", sopUID, "Description", sopDescription)
		for _, TransferSyn := range PresContext.GetTransferSyntaxes() {
			tsName := ""
			transferSyntax := transfersyntax.GetTransferSyntaxFromUID(TransferSyn.GetUID())
			if transferSyntax != nil {
				tsName = transferSyntax.Description
			}
			slog.Debug("ASSOC-RQ: \tTransferSyntax:", "UID", TransferSyn.GetUID(), "Description", tsName)
		}

		PresContextAccept := NewPresentationContextAccept()
		PresContextAccept.SetResult(4)
		PresContextAccept.SetAbstractSyntax(PresContext.GetAbstractSyntax().GetUID())
		if sopclass.GetSOPClassFromUID(PresContext.GetAbstractSyntax().GetUID()) == nil {
			// 3: abstract syntax not supported (PS3.8 Table 9-18).
			PresContextAccept.SetResult(3)
			if len(PresContext.GetTransferSyntaxes()) > 0 {
				PresContextAccept.SetTransferSyntax(PresContext.GetTransferSyntaxes()[0].GetUID())
			} else {
				PresContextAccept.SetTransferSyntax(transfersyntax.ImplicitVRLittleEndian.UID)
			}
		} else if tsUID, ok := selectPreferredTransferSyntax(PresContext.GetTransferSyntaxes()); ok {
			PresContextAccept.SetResult(0)
			PresContextAccept.SetTransferSyntax(tsUID)
			pdu.AcceptedPresentationContexts = append(pdu.AcceptedPresentationContexts, PresContextAccept)
		} else if len(PresContext.GetTransferSyntaxes()) > 0 {
			// 4: transfer syntaxes not supported (PS3.8 Table 9-18).
			PresContextAccept.SetResult(4)
			PresContextAccept.SetTransferSyntax(PresContext.GetTransferSyntaxes()[0].GetUID())
		} else {
			PresContextAccept.SetResult(4)
			PresContextAccept.SetTransferSyntax(transfersyntax.ImplicitVRLittleEndian.UID)
		}

		PresContextAccept.SetPresentationContextID(PresContext.GetPresentationContextID())
		pdu.AssocAC.AddPresContextAccept(PresContextAccept)
	}

	if len(pdu.AcceptedPresentationContexts) > 0 {
		MaxSubLength := NewMaximumPDULength()
		UserInfo := NewUserInformation()

		MaxSubLength.SetMaximumLength(maxPduLength)
		UserInfo.SetImplementationClassUID(implementation.GetImplementationClassUID())
		UserInfo.SetImplementationVersionName(implementation.GetImplementationVersion())
		UserInfo.SetMaxSubLength(MaxSubLength)
		pdu.AssocAC.SetUserInformation(UserInfo)
		return pdu.AssocAC.Write(rw)
	}

	slog.Warn("pdu: rejecting association - no presentation contexts could be negotiated", "CalledAE", pdu.AssocRQ.GetCalledAE(), "CallingAE", pdu.AssocRQ.GetCallingAE())
	return pdu.AssocRJ.Write(rw)
}

func (pdu *pduService) parseDCMIntoRaw(DCO media.DICOMObject) bool {
	pdu.Pdata.Buffer.WriteObj(DCO)
	return true
}

func (pdu *pduService) parseRawVRIntoDCM(DCO media.DICOMObject) bool {
	TrnSyntax := pdu.GetTransferSyntax(pdu.Pdata.PresentationContextID)
	if TrnSyntax == nil {
		slog.Error("pdu: no transfer syntax for presentation context", "PCID", pdu.Pdata.PresentationContextID)
		return false
	}
	DCO.SetTransferSyntax(TrnSyntax)
	DCO.SetExplicitVR(TrnSyntax.UID != transfersyntax.ImplicitVRLittleEndian.UID)
	if TrnSyntax.UID == transfersyntax.ExplicitVRBigEndian.UID {
		DCO.SetBigEndian(true)
	}
	pdu.Pdata.Buffer.SetPosition(0)
	return pdu.Pdata.Buffer.ReadObj(DCO) == nil
}

func (pdu *pduService) readPDU() error {
	if pdu.pdulength < 4 {
		return fmt.Errorf("pdu: malformed PDU length %d (minimum is 4)", pdu.pdulength)
	}
	return pdu.ms.ReadFully(pdu.readWriter, int(pdu.pdulength)-4)
}
