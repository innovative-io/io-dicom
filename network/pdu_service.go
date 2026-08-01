package network

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	implementation "github.com/innovative-io/io-dicom/internal/implclass"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

// ErrAssociationReleased is returned by NextPDU when the peer performs an A-RELEASE handshake.
var ErrAssociationReleased = errors.New("DICOM association released")

// ErrAssociationAborted is returned by NextPDU when the peer sends an A-ABORT PDU.
var ErrAssociationAborted = errors.New("DICOM association aborted")

// ErrAssociationRejected is returned by NextPDU when the local handler rejects the
// incoming A-ASSOCIATE-RQ. The SCP must close the transport connection after rejection
// (DICOM PS 3.8 §9.3.4).
var ErrAssociationRejected = errors.New("DICOM association rejected")

type RawPDUDirection string

const (
	RawPDUDirectionInbound  RawPDUDirection = "inbound"
	RawPDUDirectionOutbound RawPDUDirection = "outbound"
)

type RawPDUEvent struct {
	Direction     RawPDUDirection
	PDUType       byte
	Data          []byte
	CallingAE     string
	CalledAE      string
	RemoteAddress string
}

// PDUService - struct for PDUService
type PDUService interface {
	GetTransferSyntax(pcid byte) *transfersyntax.TransferSyntax
	SetTimeout(timeout int)
	// Connect dials IP:Port over plain TCP and negotiates an A-ASSOCIATE.
	// The context controls cancellation of the dial; use context.WithTimeout to
	// apply an overall connection deadline via the caller rather than SetTimeout.
	Connect(ctx context.Context, IP string, Port string) error
	// ConnectTLS dials the remote AE over TLS and negotiates an A-ASSOCIATE.
	// Pass nil for cfg to use the system certificate pool without a client certificate.
	ConnectTLS(ctx context.Context, IP string, Port string, cfg *tls.Config) error
	Close() error
	GetAAssociationRQ() AssociationRequest
	GetCalledAE() string
	GetCallingAE() string
	GetRemoteAddress() string
	SetCalledAE(calledAE string)
	SetCallingAE(callingAE string)
	SetConn(rw *bufio.ReadWriter)
	SetNetConn(conn net.Conn)
	// SetReadDeadline bounds the next read with an absolute deadline. Pass the
	// zero Time to clear it. Callers must use this rather than setting the
	// deadline on the connection directly, so the per-read progress timeout can
	// take it into account instead of overwriting it.
	SetReadDeadline(t time.Time) error
	NextPDU() (media.DICOMObject, error)
	AddPresContexts(presentationContext PresentationContext)
	GetPresentationContextID() byte
	// GetAcceptedPresentationContexts returns the presentation contexts accepted
	// by the remote SCP after a successful association negotiation (Connect/ConnectTLS).
	GetAcceptedPresentationContexts() []PresentationContextAccept
	SetOnAssociationRequest(f func(request AssociationRequest) bool)
	SetOnRawPDU(f func(event RawPDUEvent))
	Write(DCO media.DICOMObject, ItemType byte) error
	// SetLogger sets the structured logger used for this connection's PDU-level
	// events. Passing a logger derived with per-association attributes (e.g.
	// slog.With("assoc_id", ...)) tags every line so logs from concurrent
	// associations can be correlated. A nil logger restores slog.Default().
	SetLogger(logger *slog.Logger)
	// Logger returns the logger configured for this connection, never nil.
	Logger() *slog.Logger
}

// PDUServiceOption configures a PDUService at construction time.
type PDUServiceOption func(*pduService)

// WithImplementationClass overrides the implementation class UID and version
// name sent in A-ASSOCIATE-RQ and A-ASSOCIATE-AC PDUs. By default the library
// global values from internal/implclass are used.
func WithImplementationClass(uid, version string) PDUServiceOption {
	return func(p *pduService) {
		p.implClassUID = uid
		p.implVersion = version
	}
}

// WithMaxMessageSize overrides the ceiling on one DIMSE message accumulated
// across P-DATA fragments. A non-positive value restores the default
// (defaultMaxMessageBytes).
func WithMaxMessageSize(n int) PDUServiceOption {
	return func(p *pduService) {
		if n <= 0 {
			n = defaultMaxMessageBytes
		}
		p.maxMessageBytes = n
	}
}

// WithReadProgressTimeout bounds how long a single read may take without
// completing. A non-positive value disables the bound; omitted, the default is
// defaultReadProgressTimeout.
//
// This is a progress bound, not a transfer bound: it is re-armed before every
// read, so an arbitrarily large transfer may take arbitrarily long as long as
// it keeps moving. Only a peer that stops feeding bytes mid-PDU trips it.
func WithReadProgressTimeout(d time.Duration) PDUServiceOption {
	return func(p *pduService) {
		p.progressTimeout = d
	}
}

// WithLogger sets the structured logger used for PDU-level events on this
// connection. By default the library logs through slog.Default(). Pass a logger
// derived with per-association attributes to correlate concurrent associations.
func WithLogger(logger *slog.Logger) PDUServiceOption {
	return func(p *pduService) {
		p.SetLogger(logger)
	}
}

type pduService struct {
	AcceptedPresentationContexts []PresentationContextAccept
	conn                         net.Conn
	readWriter                   *bufio.ReadWriter
	progressTimeout              time.Duration
	externalDeadline             time.Time
	buf                          *media.DICOMBuffer
	pdutype                      int
	pdulength                    uint32
	AssocRQ                      *associationRequest
	AssocAC                      *associationAccept
	AssocRJ                      AssociationReject
	ReleaseRQ                    ReleaseRequest
	ReleaseRP                    ReleaseResponse
	AbortRQ                      AbortRequest
	Pdata                        PresentationDataTransfer
	Timeout                      int
	OnAssociationRequest         func(request AssociationRequest) bool
	onRawPDU                     func(event RawPDUEvent)
	implClassUID                 string
	implVersion                  string
	logger                       *slog.Logger
	// maxMessageBytes bounds the total size of one DIMSE message accumulated
	// across P-DATA fragments. See defaultMaxMessageBytes.
	maxMessageBytes int
	// negotiated records that an association has been established on this
	// connection. PS3.8 §9.3.1 permits exactly one A-ASSOCIATE-RQ per
	// association; without this guard the same connection can renegotiate
	// repeatedly, which both accumulates presentation-context state without
	// bound and makes NextPDU return (nil, nil) at a point where the caller is
	// reading a command's dataset and will dereference the result.
	negotiated bool
	// hdrScratch backs the 10-byte PDU header read. It cannot be a local in
	// readIncomingPDU: passing a local's slice to io.ReadFull's io.Reader
	// interface makes escape analysis heap-allocate it, which cost one
	// allocation per inbound PDU. One association is read by a single
	// goroutine, so sharing it across calls is safe.
	hdrScratch [10]byte
}

// NewPDUService creates a PDUService. Pass PDUServiceOption values to
// configure non-default behaviour (e.g. WithImplementationClass).
func NewPDUService(opts ...PDUServiceOption) PDUService {
	p := &pduService{
		maxMessageBytes: defaultMaxMessageBytes,
		progressTimeout: defaultReadProgressTimeout,
		buf:             media.NewDICOMBuffer(),
		AssocRQ:         newAssociationRequest(),
		AssocAC:         newAssociationAccept(),
		AssocRJ:         NewAssociationReject(),
		ReleaseRQ:       NewReleaseRequest(),
		ReleaseRP:       NewReleaseResponse(),
		AbortRQ:         NewAbortRequest(),
	}
	p.SetLogger(nil)
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// SetLogger sets the connection logger and propagates it to the association
// sub-structures so their PDU-encoding traces share the same correlation
// attributes. A nil logger resets to slog.Default().
func (pdu *pduService) SetLogger(logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	pdu.logger = logger
	pdu.AssocRQ.logger = logger
	pdu.AssocAC.logger = logger
	if aarj, ok := pdu.AssocRJ.(*associationReject); ok {
		aarj.logger = logger
	}
}

// Logger returns the connection logger, never nil.
func (pdu *pduService) Logger() *slog.Logger {
	if pdu.logger == nil {
		return slog.Default()
	}
	return pdu.logger
}

// loggerOrDefault returns l, or slog.Default() when l is nil. Used by the
// association sub-structures, which may be exercised standalone (e.g. in tests)
// before a pduService propagates its connection logger to them.
func loggerOrDefault(l *slog.Logger) *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l
}

const maxPduLength uint32 = 16384

// maxIncomingPDULength is an absolute ceiling on the byte length of any PDU we
// will read off the wire, enforced before allocating the receive buffer. It is
// deliberately far larger than the negotiated P-DATA-TF maximum (maxPduLength)
// so legitimate association PDUs carrying many presentation contexts still fit,
// while a hostile or buggy peer advertising a near-4 GiB PDU length cannot force
// a multi-gigabyte allocation (DoS). 16 MiB is orders of magnitude above any
// conformant PDU yet trivially bounded.
const maxIncomingPDULength uint32 = 16 << 20

// defaultMaxMessageBytes bounds one DIMSE message accumulated across P-DATA
// fragments.
//
// maxIncomingPDULength caps an INDIVIDUAL PDU at 16 MiB, but nothing bounded the
// sum: NextPDU loops while the last-fragment bit is clear, appending every PDV
// into Pdata.Buffer, so a peer that simply never sets that bit grows the buffer
// without limit. 1 GiB is far above any real DIMSE message (a large multi-frame
// instance is tens of MB) while keeping a hostile peer bounded.
//
// Override with WithMaxMessageSize when an archive genuinely handles larger
// objects.
const defaultMaxMessageBytes = 1 << 30

// defaultReadProgressTimeout bounds a single read on an established
// association.
//
// The idle timeout in the services layer covers a peer that goes quiet between
// commands, but once a command had been accepted the dataset reads that
// followed were unbounded: a peer could send a valid C-STORE header and then
// dribble its dataset forever, held only by the total-message ceiling, which
// bounds bytes rather than time.
//
// One PDU is capped at maxIncomingPDULength (16 MiB), so this only requires a
// peer to sustain progress within a single read — it never limits how long a
// whole study takes to arrive.
const defaultReadProgressTimeout = 60 * time.Second

const releaseHandshakeTimeout = 5 * time.Second

// resolveImplClass returns the implementation class UID and version to use
// for this connection. Per-instance values (set via WithImplementationClass)
// take precedence over the library-wide global defaults.
func (pdu *pduService) resolveImplClass() (uid, version string) {
	if pdu.implClassUID != "" || pdu.implVersion != "" {
		return pdu.implClassUID, pdu.implVersion
	}
	return implementation.GetImplementationClassUID(), implementation.GetImplementationVersion()
}

func (pdu *pduService) SetConn(rw *bufio.ReadWriter) {
	pdu.readWriter = rw
}

// SetReadDeadline records an absolute deadline for the next read and applies it
// immediately. applyReadDeadline keeps it in force when it is sooner than the
// per-read progress timeout, so a short poll (e.g. the SCP's C-CANCEL window)
// is never lengthened by the progress bound.
func (pdu *pduService) SetReadDeadline(t time.Time) error {
	pdu.externalDeadline = t
	if pdu.conn == nil {
		return nil
	}
	return pdu.conn.SetReadDeadline(t)
}

// applyReadDeadline arms the connection for one read, using whichever of the
// caller's deadline and the progress timeout expires first.
func (pdu *pduService) applyReadDeadline() error {
	if pdu.conn == nil {
		return nil
	}
	var eff time.Time
	if pdu.progressTimeout > 0 {
		eff = time.Now().Add(pdu.progressTimeout)
	}
	if !pdu.externalDeadline.IsZero() && (eff.IsZero() || pdu.externalDeadline.Before(eff)) {
		eff = pdu.externalDeadline
	}
	if eff.IsZero() {
		return nil
	}
	return pdu.conn.SetReadDeadline(eff)
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

// selectPreferredTransferSyntax accepts the first transfer syntax offered by
// the SCU that is supported by this implementation. Accepting the SCU's first
// (preferred) offer avoids requiring the SCU to transcode data it already has
// in a specific encoding — critical for storage SCPs that must preserve the
// original transfer syntax.
func selectPreferredTransferSyntax(offered []UIDItem) (string, bool) {
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

func selectPresentationContextIDForAbstractSyntax(accepted []PresentationContextAccept, abstractSyntaxUID string) (byte, bool) {
	if abstractSyntaxUID == "" {
		return 0, false
	}

	for _, context := range accepted {
		if context.GetResult() == 0 && context.GetAbstractSyntax().GetUID() == abstractSyntaxUID {
			return context.GetPresentationContextID(), true
		}
	}

	return 0, false
}

func negotiatedAbstractSyntaxForObject(dco media.DICOMObject) string {
	if dco == nil {
		return ""
	}

	if uid := dco.GetString(tags.AffectedSOPClassUID); uid != "" {
		return uid
	}
	if uid := dco.GetString(tags.RequestedSOPClassUID); uid != "" {
		return uid
	}
	if uid := dco.GetString(tags.SOPClassUID); uid != "" {
		return uid
	}

	return ""
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

func (pdu *pduService) Connect(ctx context.Context, IP string, Port string) error {
	pdu.AcceptedPresentationContexts = nil
	pdu.Pdata.PresentationContextID = 0

	if pdu.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(pdu.Timeout)*time.Second)
		defer cancel()
	}

	pdu.logger.Debug("dialing", "host", IP, "port", Port)
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", IP+":"+Port)
	if err != nil {
		return fmt.Errorf("pduservice::Connect: %w", err)
	}
	pdu.logger.Debug("tcp connected", "host", IP, "port", Port)
	return pdu.finishConnect(conn)
}

// ConnectTLS dials the remote AE with TLS and negotiates an A-ASSOCIATE.
// Pass nil for cfg to use the system certificate pool with no client certificate.
func (pdu *pduService) ConnectTLS(ctx context.Context, IP string, Port string, cfg *tls.Config) error {
	pdu.AcceptedPresentationContexts = nil
	pdu.Pdata.PresentationContextID = 0

	if pdu.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(pdu.Timeout)*time.Second)
		defer cancel()
	}

	pdu.logger.Debug("dialing (tls)", "host", IP, "port", Port)
	conn, err := (&tls.Dialer{Config: normalizeClientTLSConfig(cfg)}).DialContext(ctx, "tcp", IP+":"+Port)
	if err != nil {
		return fmt.Errorf("pduservice::ConnectTLS - %w", err)
	}
	pdu.logger.Debug("tcp connected (tls)", "host", IP, "port", Port)
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
	scuUID, scuVer := pdu.resolveImplClass()
	pdu.AssocRQ.SetImplementationClassUID(scuUID)
	pdu.AssocRQ.SetImplementationVersionName(scuVer)

	pdu.logger.Debug("sending A-ASSOCIATE-RQ", "calling_ae", pdu.AssocRQ.GetCallingAE(), "called_ae", pdu.AssocRQ.GetCalledAE())
	if err := pdu.writeEncodedPDU(byte(pdutype.AssociationRequest), func(rw *bufio.ReadWriter) error {
		return pdu.AssocRQ.Write(rw)
	}); err != nil {
		return err
	}

	pdu.logger.Debug("waiting for A-ASSOCIATE response")
	itemType, rawData, err := pdu.readIncomingPDU()
	if err != nil {
		return err
	}

	switch itemType {
	case pdutype.AssociationAccept:
		pdu.buf.SetPosition(1)
		if err := pdu.AssocAC.ReadDynamic(pdu.buf); err != nil {
			return err
		}
		pdu.emitRawPDU(RawPDUDirectionInbound, itemType, rawData)
		if !pdu.interogateAAssociateAC() {
			return errors.New("pduservice::Connect - No accepted presentation contexts found")
		}
		// The SCU side is now established; any later A-ASSOCIATE PDU on this
		// connection is out of sequence (see NextPDU).
		pdu.negotiated = true
		maxSendPDV := pdu.AssocAC.GetMaxSubLength()
		if maxSendPDV == 0 || maxSendPDV > maxPduLength {
			maxSendPDV = maxPduLength
		}
		theirMaxPDU := pdu.AssocAC.GetMaxSubLength()
		theirMaxStr := fmt.Sprintf("%d", theirMaxPDU)
		if theirMaxPDU == 0 {
			theirMaxStr = "unlimited"
		}
		pdu.logger.Info("association accepted", "max_send_pdv", maxSendPDV-6, "their_max_pdu", theirMaxStr)
		for _, pc := range pdu.AcceptedPresentationContexts {
			pcID := pc.GetPresentationContextID()
			sopDesc := ""
			for _, rqPC := range pdu.AssocRQ.GetPresContexts() {
				if rqPC.GetPresentationContextID() == pcID {
					if sop := sopclass.GetSOPClassFromUID(rqPC.GetAbstractSyntax().GetUID()); sop != nil {
						sopDesc = sop.Description
					} else {
						sopDesc = rqPC.GetAbstractSyntax().GetUID()
					}
					break
				}
			}
			tsDesc := ""
			if ts := transfersyntax.GetTransferSyntaxFromUID(pc.GetTrnSyntax().GetUID()); ts != nil {
				tsDesc = ts.Description
			} else {
				tsDesc = pc.GetTrnSyntax().GetUID()
			}
			pdu.logger.Debug("presentation context accepted", "pcid", pcID, "sop_class", sopDesc, "transfer_syntax", tsDesc)
		}
		return nil
	case pdutype.AssociationReject:
		pdu.buf.SetPosition(1)
		if err := pdu.AssocRJ.ReadDynamic(pdu.buf); err != nil {
			return err
		}
		pdu.emitRawPDU(RawPDUDirectionInbound, itemType, rawData)
		return fmt.Errorf("pduservice::Connect - Association rejected - %s", pdu.AssocRJ.GetReason())
	case pdutype.AssociationAbortRequest:
		pdu.buf.SetPosition(1)
		if err := pdu.AbortRQ.ReadDynamic(pdu.buf); err != nil {
			return err
		}
		pdu.emitRawPDU(RawPDUDirectionInbound, itemType, rawData)
		return fmt.Errorf("pduservice::Connect - Association aborted - %s", pdu.AbortRQ.GetReason())
	default:
		return fmt.Errorf("pduservice::Connect - Corrupt transmision - %b", itemType)
	}
}

func (pdu *pduService) Close() error {
	if pdu.readWriter == nil {
		return nil
	}

	pdu.logger.Debug("releasing association")
	if err := pdu.writeEncodedPDU(byte(pdutype.AssociationReleaseRequest), func(rw *bufio.ReadWriter) error {
		return pdu.ReleaseRQ.Write(rw)
	}); err != nil {
		pdu.logger.Warn("failed to send A-RELEASE-RQ; sending A-ABORT", "error", err)
		_ = pdu.writeEncodedPDU(byte(pdutype.AssociationAbortRequest), func(rw *bufio.ReadWriter) error {
			return pdu.AbortRQ.Write(rw)
		})
		pdu.closeConn()
		return err
	}

	if pdu.conn != nil {
		_ = pdu.conn.SetDeadline(time.Now().Add(releaseHandshakeTimeout))
	}

	itemType, rawData, err := pdu.readIncomingPDU()
	if err != nil {
		pdu.logger.Warn("timeout waiting for A-RELEASE-RP; sending A-ABORT", "error", err)
		_ = pdu.writeEncodedPDU(byte(pdutype.AssociationAbortRequest), func(rw *bufio.ReadWriter) error {
			return pdu.AbortRQ.Write(rw)
		})
		pdu.closeConn()
		return err
	}

	pdu.emitRawPDU(RawPDUDirectionInbound, itemType, rawData)
	if int(itemType) != pdutype.AssociationReleaseResponse {
		pdu.logger.Warn("unexpected PDU waiting for A-RELEASE-RP", "pdu_type", itemType)
	}

	pdu.closeConn()
	return nil
}

func (pdu *pduService) NextPDU() (command media.DICOMObject, err error) {
	if pdu.Pdata.Buffer != nil {
		// Reset (not Clear) so the old backing array is released rather than
		// reused.  Callers may hold zero-copy tag.Data slices that alias the
		// previous buffer; reusing the same backing array would corrupt them.
		pdu.Pdata.Buffer.Reset()
	} else {
		pdu.Pdata.Buffer = media.NewDICOMBuffer()
	}

	pdu.Pdata.MsgStatus = 0
	if pdu.Pdata.Length != 0 {
		DCO := media.NewEmptyDCMObj()
		if err := pdu.Pdata.ReadDynamic(pdu.buf); err != nil {
			return nil, err
		}
		if pdu.Pdata.MsgStatus > 0 {
			if !pdu.parseRawVRIntoDCM(DCO) {
				pdu.AbortRQ.Write(pdu.readWriter)
				return nil, errors.New("pduservice::Read - ParseRawVRIntoDCM failed")
			}
			return DCO, nil
		}
	}

	for {
		itemType, rawData, err := pdu.readIncomingPDU()
		if err != nil {
			return nil, err
		}

		switch int(itemType) {
		case pdutype.AssociationRequest:
			if pdu.negotiated {
				// PS3.8 §9.3.1 allows one A-ASSOCIATE-RQ per association; state
				// AA-8 requires A-ABORT for a PDU received out of sequence.
				// Returning an error here (rather than falling through to the
				// (nil, nil) below) is what stops a caller reading a command's
				// dataset from dereferencing a nil object.
				return nil, pdu.abortWithError("unexpected A-ASSOCIATE-RQ on an established association")
			}
			pdu.buf.SetPosition(6)
			if err := pdu.AssocRQ.Read(pdu.buf); err != nil {
				return nil, err
			}
			pdu.emitRawPDU(RawPDUDirectionInbound, itemType, rawData)
			if err := pdu.interogateAAssociateRQ(pdu.readWriter); err != nil {
				if errors.Is(err, ErrAssociationRejected) {
					// After sending A-ASSOCIATE-RJ, rejecting AE must close transport.
					pdu.closeConn()
				}
				return nil, err
			}
			pdu.negotiated = true
			// The one legitimate (nil, nil): the association is now established
			// and this PDU carried no data object. Callers reading a dataset must
			// never reach this point — the guard above ensures they do not.
			return nil, nil
		case pdutype.AssociationAccept:
			// An A-ASSOCIATE-AC is only valid while Connect is negotiating, and
			// finishConnect consumes it there. Reaching NextPDU means the peer
			// sent it out of sequence.
			pdu.emitRawPDU(RawPDUDirectionInbound, itemType, rawData)
			return nil, pdu.abortWithError("unexpected A-ASSOCIATE-AC")
		case pdutype.PDUDataTransfer:
			pdu.emitRawPDU(RawPDUDirectionInbound, itemType, rawData)
			pdu.buf.SetPosition(1)
			if err := pdu.Pdata.ReadDynamic(pdu.buf); err != nil {
				return nil, err
			}
			// Bound the accumulated message, not just each PDU. Without this a
			// peer that never sets the last-fragment bit grows Pdata.Buffer
			// without limit.
			if limit := pdu.maxMessageBytes; limit > 0 && pdu.Pdata.Buffer.GetSize() > limit {
				return nil, pdu.abortWithError(fmt.Sprintf(
					"DIMSE message exceeds %d bytes", limit))
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
			pdu.logger.Debug("A-RELEASE-RQ received")
			pdu.emitRawPDU(RawPDUDirectionInbound, itemType, rawData)
			pdu.buf.SetPosition(1)
			if err := pdu.ReleaseRQ.ReadDynamic(pdu.buf); err != nil {
				return nil, err
			}
			if err := pdu.writeEncodedPDU(byte(pdutype.AssociationReleaseResponse), func(rw *bufio.ReadWriter) error {
				return pdu.ReleaseRP.Write(rw)
			}); err != nil {
				return nil, err
			}
			return nil, ErrAssociationReleased
		case pdutype.AssociationReleaseResponse:
			pdu.logger.Debug("A-RELEASE-RP received")
			pdu.emitRawPDU(RawPDUDirectionInbound, itemType, rawData)
			return nil, ErrAssociationReleased
		case pdutype.AssociationAbortRequest:
			pdu.logger.Info("A-ABORT received")
			pdu.emitRawPDU(RawPDUDirectionInbound, itemType, rawData)
			return nil, ErrAssociationAborted
		default:
			_ = pdu.writeEncodedPDU(byte(pdutype.AssociationAbortRequest), func(rw *bufio.ReadWriter) error {
				return pdu.AbortRQ.Write(rw)
			})
			return nil, errors.New("pduservice::Read - unknown ItemType")
		}
	}
}

// abortWithError sends an A-ABORT for a PDU received out of sequence and
// returns the error to report. Used for protocol violations that PS3.8 §9.3.1
// (state AA-8) requires be aborted rather than processed.
//
// NextPDU must never return (nil, nil) to a caller that is reading a dataset:
// every such caller dereferences the returned object, and a nil dereference in
// the per-connection goroutine terminates the process.
func (pdu *pduService) abortWithError(reason string) error {
	pdu.logger.Warn("aborting association: protocol violation", "reason", reason)
	_ = pdu.writeEncodedPDU(byte(pdutype.AssociationAbortRequest), func(rw *bufio.ReadWriter) error {
		return pdu.AbortRQ.Write(rw)
	})
	return fmt.Errorf("pduservice: %s", reason)
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

func (pdu *pduService) GetRemoteAddress() string {
	if pdu.conn == nil || pdu.conn.RemoteAddr() == nil {
		return ""
	}
	return pdu.conn.RemoteAddr().String()
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

func (pdu *pduService) GetAcceptedPresentationContexts() []PresentationContextAccept {
	result := make([]PresentationContextAccept, len(pdu.AcceptedPresentationContexts))
	copy(result, pdu.AcceptedPresentationContexts)
	return result
}

func (pdu *pduService) SetOnAssociationRequest(f func(request AssociationRequest) bool) {
	pdu.OnAssociationRequest = f
}

func (pdu *pduService) SetOnRawPDU(f func(event RawPDUEvent)) {
	pdu.onRawPDU = f
}

func (pdu *pduService) emitRawPDU(direction RawPDUDirection, pduType byte, data []byte) {
	if pdu.onRawPDU == nil || len(data) == 0 {
		return
	}
	pdu.onRawPDU(RawPDUEvent{
		Direction:     direction,
		PDUType:       pduType,
		Data:          append([]byte(nil), data...),
		CallingAE:     pdu.AssocRQ.GetCallingAE(),
		CalledAE:      pdu.AssocRQ.GetCalledAE(),
		RemoteAddress: pdu.GetRemoteAddress(),
	})
}

func (pdu *pduService) readIncomingPDU() (byte, []byte, error) {
	header := &pdu.hdrScratch
	if err := pdu.applyReadDeadline(); err != nil {
		return 0, nil, err
	}
	if _, err := io.ReadFull(pdu.readWriter, header[:]); err != nil {
		return 0, nil, err
	}

	// Decode the fixed header directly. Wrapping it in a DICOMBuffer just to
	// read three fields cost three bounds-checked method calls per PDU for no
	// benefit — the layout here is fixed by PS3.8 §9.3: item type, one reserved
	// byte, then a big-endian uint32 length.
	itemType := header[0]
	pduLength := binary.BigEndian.Uint32(header[2:6])
	if pduLength < 4 {
		return 0, nil, fmt.Errorf("pdu: malformed PDU length %d (minimum is 4)", pduLength)
	}
	if pduLength > maxIncomingPDULength {
		return 0, nil, fmt.Errorf("pdu: PDU length %d exceeds maximum %d", pduLength, maxIncomingPDULength)
	}

	remaining := int(pduLength) - 4
	data := make([]byte, 10+remaining)
	copy(data, header[:])
	if remaining > 0 {
		// Re-arm: the body is a separate read, and a peer that delivered a
		// header must still make progress on the payload.
		if err := pdu.applyReadDeadline(); err != nil {
			return 0, nil, err
		}
		if _, err := io.ReadFull(pdu.readWriter, data[10:]); err != nil {
			return 0, nil, err
		}
	}

	pdu.buf = media.NewDICOMBufferFromBytes(data)
	pdu.pdutype = int(itemType)
	pdu.pdulength = pduLength
	return itemType, data, nil
}

func (pdu *pduService) writeEncodedPDU(pduType byte, writer func(rw *bufio.ReadWriter) error) error {
	if pdu.readWriter == nil {
		return errors.New("pduservice::writeEncodedPDU - nil readWriter")
	}

	// Fast path: no raw PDU listener — encode directly into the connection writer,
	// avoiding the intermediate bytes.Buffer and fake read-side bufio.Reader entirely.
	if pdu.onRawPDU == nil {
		if err := writer(pdu.readWriter); err != nil {
			return err
		}
		return pdu.readWriter.Flush()
	}

	// Slow path: buffer the encoded PDU so we can hand a copy to the listener.
	var buffer bytes.Buffer
	tempRW := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&buffer))
	if err := writer(tempRW); err != nil {
		return err
	}
	if err := tempRW.Flush(); err != nil {
		return err
	}
	if _, err := pdu.readWriter.Write(buffer.Bytes()); err != nil {
		return err
	}
	if err := pdu.readWriter.Flush(); err != nil {
		return err
	}
	// emitRawPDU copies the slice internally, so buffer.Bytes() is safe to pass directly.
	pdu.emitRawPDU(RawPDUDirectionOutbound, pduType, buffer.Bytes())
	return nil
}

func (pdu *pduService) Write(DCO media.DICOMObject, ItemType byte) error {
	// The send path no longer touches Pdata.Buffer: WriteObjTo streams the
	// dataset directly into PDVs below. Pdata.Buffer is now owned solely by the
	// receive path (NextPDU), which allocates it on demand.

	if pcid, ok := selectPresentationContextIDForAbstractSyntax(pdu.AcceptedPresentationContexts, negotiatedAbstractSyntaxForObject(DCO)); ok {
		pdu.Pdata.PresentationContextID = pcid
	} else if pdu.Pdata.PresentationContextID == 0 {
		if defaultPCID, ok := selectDefaultPresentationContextID(pdu.AcceptedPresentationContexts); ok {
			pdu.Pdata.PresentationContextID = defaultPCID
		}
	}

	if ts := pdu.GetTransferSyntax(pdu.Pdata.PresentationContextID); ts != nil {
		DCO.SetTransferSyntax(ts)
		DCO.SetBigEndian(ts.UID == transfersyntax.ExplicitVRBigEndian.UID)
		DCO.SetExplicitVR(ts.UID != transfersyntax.ImplicitVRLittleEndian.UID)
	}

	if pdu.Pdata.PresentationContextID == 0 {
		return errors.New("pduservice::Write - PresentationContextID==0")
	}

	pdu.Pdata.MsgHeader = ItemType
	if pdu.AssocAC.GetMaxSubLength() > maxPduLength {
		pdu.AssocAC.SetMaxSubLength(maxPduLength)
	}

	// Block size = negotiated max PDU length minus the 6-byte P-DATA-TF + PDV
	// framing overhead. A peer that negotiated 0 ("unlimited") or an implausibly
	// large value is treated as maxPduLength, matching Connect()'s handling and
	// avoiding an underflow on the subtraction below.
	maxSub := pdu.AssocAC.GetMaxSubLength()
	if maxSub == 0 || maxSub > maxPduLength {
		maxSub = maxPduLength
	}
	blockSize := int(maxSub) - 6
	pdu.Pdata.BlockSize = uint32(blockSize) // kept for readers of Pdata state

	if ItemType > 0x00 {
		sopClassUID := DCO.GetString(tags.AffectedSOPClassUID)
		sopClass := sopclass.GetSOPClassFromUID(sopClassUID)
		if sopClass != nil {
			pdu.logger.Debug("negotiating SOP class", "uid", sopClass.UID, "description", sopClass.Description)
		} else {
			pdu.logger.Debug("negotiating SOP class", "uid", sopClassUID, "description", "Unknown SOP Class")
		}
	}

	// Stream the dataset straight into P-DATA PDVs. The old path serialised the
	// entire object into Pdata.Buffer first (a full second copy of the pixel
	// data) and then chunked that buffer; WriteObjTo emits the identical byte
	// stream through pdvChunkWriter, so peak send memory is one block, not the
	// whole message.
	return pdu.writeEncodedPDU(byte(pdutype.PDUDataTransfer), func(rw *bufio.ReadWriter) error {
		cw := newPDVChunkWriter(rw, blockSize, pdu.Pdata.PresentationContextID, ItemType)
		if _, err := media.WriteObjTo(cw, DCO); err != nil {
			return err
		}
		return cw.Close()
	})
}

func (pdu *pduService) interogateAAssociateAC() bool {
	pdu.AcceptedPresentationContexts = nil

	// Build a PCID → abstract-syntax map from the outgoing RQ so we can
	// annotate accepted contexts with their SOP class UID. The A-ASSOCIATE-AC
	// wire format omits abstract syntax per DICOM PS3.8 §9.3.3.2. Without this
	// backfill, selectPresentationContextIDForAbstractSyntax always fails on the
	// SCU side and pdu.Write() falls back to the default PCID for every message,
	// making PCID selection fragile when more than one context is negotiated.
	rqAbsSyntax := make(map[byte]string, len(pdu.AssocRQ.GetPresContexts()))
	for _, rqPC := range pdu.AssocRQ.GetPresContexts() {
		rqAbsSyntax[rqPC.GetPresentationContextID()] = rqPC.GetAbstractSyntax().GetUID()
	}

	for _, presContextAccept := range pdu.AssocAC.GetPresContextAccepts() {
		if presContextAccept.GetResult() == 0 {
			if absSyn, ok := rqAbsSyntax[presContextAccept.GetPresentationContextID()]; ok {
				presContextAccept.SetAbstractSyntax(absSyn)
			}
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
	previousReadWriter := pdu.readWriter
	if pdu.readWriter == nil {
		pdu.readWriter = rw
		defer func() {
			pdu.readWriter = previousReadWriter
		}()
	}

	if pdu.conn != nil && pdu.conn.RemoteAddr() != nil {
		pdu.AssocRQ.SetRemoteAddress(pdu.conn.RemoteAddr().String())
	} else {
		pdu.AssocRQ.SetRemoteAddress("")
	}

	if tlsConn, ok := pdu.conn.(*tls.Conn); ok {
		state := tlsConn.ConnectionState()
		pdu.AssocRQ.SetPeerCertificates(state.PeerCertificates)
	} else {
		pdu.AssocRQ.SetPeerCertificates(nil)
	}

	if pdu.OnAssociationRequest == nil || !pdu.OnAssociationRequest(pdu.AssocRQ) {
		pdu.logger.Warn("rejecting association: rejected by application handler")
		// Result=1 (permanent), Source=1 (UL-service-user), Reason=7 (called AE not recognised)
		// per DICOM PS3.8 Table 9-21.
		pdu.AssocRJ.Set(1, 1, 7)
		if err := pdu.writeEncodedPDU(byte(pdutype.AssociationReject), func(rw *bufio.ReadWriter) error {
			return pdu.AssocRJ.Write(rw)
		}); err != nil {
			return err
		}
		// Per DICOM PS 3.8 §9.3.4 the rejecting AE must close the transport connection.
		return ErrAssociationRejected
	}

	pdu.AcceptedPresentationContexts = nil
	pdu.AssocAC = newAssociationAccept()
	pdu.AssocAC.logger = pdu.logger
	pdu.AssocAC.SetCalledAE(pdu.AssocRQ.GetCalledAE())
	pdu.AssocAC.SetCallingAE(pdu.AssocRQ.GetCallingAE())
	pdu.AssocAC.SetAppContext(pdu.AssocRQ.GetAppContext())

	pdu.logger.Debug("association request received", "calling_ae", pdu.AssocRQ.GetCallingAE(), "called_ae", pdu.AssocRQ.GetCalledAE())
	pdu.logger.Debug("association request impl class", "impl_class", pdu.AssocRQ.GetImplementationClass().GetUID())
	pdu.logger.Debug("association request max PDU length", "max_pdu_length", pdu.AssocRQ.GetMaxSubLength())

	for presIndex, PresContext := range pdu.AssocRQ.GetPresContexts() {
		pdu.logger.Debug("proposed presentation context", "index", presIndex)

		sopClass := sopclass.GetSOPClassFromUID(PresContext.GetAbstractSyntax().GetUID())
		sopUID := PresContext.GetAbstractSyntax().GetUID()
		sopDescription := ""
		if sopClass != nil {
			sopUID = sopClass.UID
			sopDescription = sopClass.Description
		}
		pdu.logger.Debug("proposed abstract syntax", "uid", sopUID, "description", sopDescription)
		for _, TransferSyn := range PresContext.GetTransferSyntaxes() {
			tsName := ""
			transferSyntax := transfersyntax.GetTransferSyntaxFromUID(TransferSyn.GetUID())
			if transferSyntax != nil {
				tsName = transferSyntax.Description
			}
			pdu.logger.Debug("proposed transfer syntax", "uid", TransferSyn.GetUID(), "description", tsName)
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
		userInfo := newUserInformation()
		userInfo.MaxSubLength.SetMaximumLength(maxPduLength)
		scpUID, scpVer := pdu.resolveImplClass()
		userInfo.SetImplementationClassUID(scpUID)
		userInfo.SetImplementationVersionName(scpVer)
		pdu.AssocAC.setUserInfo(userInfo)
		return pdu.writeEncodedPDU(byte(pdutype.AssociationAccept), func(rw *bufio.ReadWriter) error {
			return pdu.AssocAC.Write(rw)
		})
	}

	pdu.logger.Warn("rejecting association: no presentation contexts negotiated")
	return pdu.writeEncodedPDU(byte(pdutype.AssociationReject), func(rw *bufio.ReadWriter) error {
		return pdu.AssocRJ.Write(rw)
	})
}

func (pdu *pduService) parseRawVRIntoDCM(DCO media.DICOMObject) bool {
	TrnSyntax := pdu.GetTransferSyntax(pdu.Pdata.PresentationContextID)
	if TrnSyntax == nil {
		pdu.logger.Error("no transfer syntax for presentation context", "pcid", pdu.Pdata.PresentationContextID)
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
