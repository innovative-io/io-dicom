package services

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
)

// SCP - Interface to scp
type SCP interface {
	// Start begins accepting connections and blocks until the listener is
	// closed — either by Stop() or by ctx being cancelled.
	Start(ctx context.Context) error
	// Stop closes the listener and waits for all in-flight connections to finish.
	Stop() error
	OnAssociationRequest(f func(request network.AssociationRequest) bool)
	OnCFindRequest(f CFindHandler)
	OnCGetRequest(f CGetHandler)
	OnCMoveRequest(f CMoveHandler)
	OnCStoreRequest(f func(ctx context.Context, request network.AssociationRequest, data media.DICOMObject) uint16)
	OnNEventReportRequest(f func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	OnNGetRequest(f func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	OnNSetRequest(f func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	OnNActionRequest(f func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	OnNCreateRequest(f func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	OnNDeleteRequest(f func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	// OnCCancelRequest registers an optional callback invoked for incoming
	// C-CANCEL-RQ message IDs.
	OnCCancelRequest(f func(request network.AssociationRequest, messageID uint16))
	// OnCEchoRequest registers an optional handler invoked for every C-ECHO
	// request. Return false to reject the echo (no response is sent).
	OnCEchoRequest(f func(request network.AssociationRequest) bool)
	OnRawPDU(f func(event network.RawPDUEvent))
}

// CGetProgress reports incremental sub-operation counts sent back to a C-GET SCU during retrieval.
type CGetProgress struct {
	Remaining uint16
	Completed uint16
	Failed    uint16
	Warnings  uint16
}

// CFindResult carries the final status code returned to the C-FIND SCU.
type CFindResult struct {
	Status uint16
}

// CGetResult carries the final status and sub-operation counts returned to the C-GET SCU.
type CGetResult struct {
	Status    uint16
	Remaining uint16
	Completed uint16
	Failed    uint16
	Warnings  uint16
}

// CMoveProgress reports incremental sub-operation counts sent back to a C-MOVE SCU during retrieval.
type CMoveProgress struct {
	Remaining uint16
	Completed uint16
	Failed    uint16
	Warnings  uint16
}

// CMoveResult carries the final status and sub-operation counts returned to the C-MOVE SCU.
type CMoveResult struct {
	Status    uint16
	Remaining uint16
	Completed uint16
	Failed    uint16
	Warnings  uint16
}

// CFindHandler is the callback signature for C-FIND requests. Call emit for each matching result object.
type CFindHandler func(ctx context.Context, request network.AssociationRequest, findLevel string, data media.DICOMObject, emit func(media.DICOMObject)) (CFindResult, error)

// CGetHandler is the callback signature for C-GET requests.
// Call storeFile for each matching DICOM file to send it back to the SCU as a
// C-STORE sub-operation over the same association; it returns nil on success or
// a non-nil error if the transfer fails or the SCU rejected the store.
// Call emit to report intermediate sub-operation progress counts.
// Return CGetResult with the final Status and completed/failed/warning counts.
type CGetHandler func(ctx context.Context, request network.AssociationRequest, getLevel string, data media.DICOMObject, storeFile func(path string) error, emit func(CGetProgress)) (CGetResult, error)

// CMoveHandler is the callback signature for C-MOVE requests. Call emit to report sub-operation progress.
type CMoveHandler func(ctx context.Context, request network.AssociationRequest, moveDestAE string, moveLevel string, data media.DICOMObject, emit func(CMoveProgress)) (CMoveResult, error)

type scp struct {
	Port                  int
	tlsConfig             *tls.Config
	listener              net.Listener
	wg                    sync.WaitGroup
	mu                    sync.RWMutex
	onNEventReportRequest func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)
	onNGetRequest         func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)
	onNSetRequest         func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)
	onNActionRequest      func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)
	onNCreateRequest      func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)
	onNDeleteRequest      func(ctx context.Context, request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)

	onAssociationRequest func(request network.AssociationRequest) bool
	onCFindRequest       CFindHandler
	onCGetRequest        CGetHandler
	onCMoveRequest       CMoveHandler
	onCStoreRequest      func(ctx context.Context, request network.AssociationRequest, data media.DICOMObject) uint16
	onCCancelRequest     func(request network.AssociationRequest, messageID uint16)
	onCEchoRequest       func(request network.AssociationRequest) bool
	onRawPDU             func(event network.RawPDUEvent)
	canceledMessageIDs   map[uint16]struct{}

	bufSize      int
	cancelPoll   time.Duration
	cancelGrace  time.Duration
	implClassUID string
	implVersion  string
}

// SCPOption configures an SCP at construction time.
type SCPOption func(*scp)

// WithReadBufferSize sets the per-connection read/write buffer size in bytes.
// Larger values reduce syscall overhead for high-throughput connections.
// The default is 256 KiB.
func WithReadBufferSize(n int) SCPOption {
	return func(s *scp) {
		if n > 0 {
			s.bufSize = n
		}
	}
}

// WithCancelPollWindow sets how long the SCP waits when polling for
// interleaved commands (e.g. C-CANCEL) during an active operation.
// The default is 5 ms.
func WithCancelPollWindow(d time.Duration) SCPOption {
	return func(s *scp) {
		if d > 0 {
			s.cancelPoll = d
		}
	}
}

// WithCancelGraceWindow sets how long the SCP waits for a handler to exit
// after receiving a matching C-CANCEL before aborting the association.
// The default is 250 ms.
func WithCancelGraceWindow(d time.Duration) SCPOption {
	return func(s *scp) {
		if d > 0 {
			s.cancelGrace = d
		}
	}
}

// WithImplementationClass overrides the implementation class UID and version
// name sent in A-ASSOCIATE-AC PDUs for connections accepted by this SCP. By
// default the library global values from internal/implclass are used.
func WithImplementationClass(uid, version string) SCPOption {
	return func(s *scp) {
		s.implClassUID = uid
		s.implVersion = version
	}
}

// NewSCP creates a plain-TCP DICOM SCP listening on port.
func NewSCP(port int, opts ...SCPOption) SCP {
	s := &scp{
		Port:               port,
		canceledMessageIDs: make(map[uint16]struct{}),
		bufSize:            scpBufSize,
		cancelPoll:         cancelPollWindow,
		cancelGrace:        cancelGraceWindow,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// NewSCPWithTLS creates a TLS-enabled DICOM SCP listening on port.
// cfg must contain at least one certificate (e.g. loaded with tls.LoadX509KeyPair).
func NewSCPWithTLS(port int, cfg *tls.Config, opts ...SCPOption) SCP {
	s := &scp{
		Port:               port,
		tlsConfig:          normalizeServerTLSConfig(cfg),
		canceledMessageIDs: make(map[uint16]struct{}),
		bufSize:            scpBufSize,
		cancelPoll:         cancelPollWindow,
		cancelGrace:        cancelGraceWindow,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func normalizeServerTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
	clone := cfg.Clone()
	if clone.MinVersion == 0 || clone.MinVersion < tls.VersionTLS12 {
		clone.MinVersion = tls.VersionTLS12
	}
	return clone
}

// newPDUService creates a PDUService, applying a per-SCP implementation class
// override if one has been configured.
func (s *scp) newPDUService() network.PDUService {
	if s.implClassUID != "" || s.implVersion != "" {
		return network.NewPDUService(network.WithImplementationClass(s.implClassUID, s.implVersion))
	}
	return network.NewPDUService()
}
