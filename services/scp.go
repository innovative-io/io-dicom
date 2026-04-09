package services

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

// scpBufSize is the read/write buffer size per connection. A larger value
// reduces syscall overhead when transferring large pixel-data payloads.
const scpBufSize = 256 * 1024

// cancelPollWindow bounds how long SCP waits while polling for interleaved
// DIMSE commands, including in-flight C-CANCEL, during active operations.
const cancelPollWindow = 5 * time.Millisecond

// cancelGraceWindow bounds how long SCP waits for a handler to exit after a
// matching C-CANCEL is received before aborting the association.
const cancelGraceWindow = 250 * time.Millisecond

type operationCounterState struct {
	seen      bool
	remaining uint16
	completed uint16
	failed    uint16
	warnings  uint16
}

func (s *operationCounterState) applyProgress(remaining, completed, failed, warnings uint16) error {
	if !s.seen {
		s.seen = true
		s.remaining = remaining
		s.completed = completed
		s.failed = failed
		s.warnings = warnings
		return nil
	}

	if remaining > s.remaining {
		return fmt.Errorf("remaining sub-operations must be non-increasing: prev=%d curr=%d", s.remaining, remaining)
	}
	if completed < s.completed {
		return fmt.Errorf("completed sub-operations must be non-decreasing: prev=%d curr=%d", s.completed, completed)
	}
	if failed < s.failed {
		return fmt.Errorf("failed sub-operations must be non-decreasing: prev=%d curr=%d", s.failed, failed)
	}
	if warnings < s.warnings {
		return fmt.Errorf("warning sub-operations must be non-decreasing: prev=%d curr=%d", s.warnings, warnings)
	}

	s.remaining = remaining
	s.completed = completed
	s.failed = failed
	s.warnings = warnings
	return nil
}

func (s *operationCounterState) finalize(status uint16, remaining, completed, failed, warnings uint16, canceled bool) (uint16, uint16, uint16, uint16, uint16, error) {
	if canceled {
		status = dicomstatus.Cancel
		remaining = 0
	}

	if s.seen {
		if completed < s.completed || failed < s.failed || warnings < s.warnings {
			return 0, 0, 0, 0, 0, errors.New("final counters regress from last pending counters")
		}
	}

	if status != dicomstatus.Pending && status != dicomstatus.PendingWithWarnings && remaining != 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("final response requires remaining=0, got %d", remaining)
	}

	return status, remaining, completed, failed, warnings, nil
}

func abortAssociation(rw *bufio.ReadWriter, conn net.Conn) {
	if rw != nil {
		_ = network.NewAbortRequest().Write(rw)
		_ = rw.Flush()
	}
	if conn != nil {
		_ = conn.Close()
	}
}

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
	OnCStoreRequest(f func(request network.AssociationRequest, data media.DICOMObject) uint16)
	OnNEventReportRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	OnNGetRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	OnNSetRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	OnNActionRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	OnNCreateRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	OnNDeleteRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject))
	// OnCCancelRequest registers an optional callback invoked for incoming
	// C-CANCEL-RQ message IDs.
	OnCCancelRequest(f func(request network.AssociationRequest, messageID uint16))
	// OnCEchoRequest registers an optional handler invoked for every C-ECHO
	// request. Return false to reject the echo (no response is sent).
	OnCEchoRequest(f func(request network.AssociationRequest) bool)
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
	onNEventReportRequest func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)
	onNGetRequest         func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)
	onNSetRequest         func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)
	onNActionRequest      func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)
	onNCreateRequest      func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)
	onNDeleteRequest      func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)

	onAssociationRequest func(request network.AssociationRequest) bool
	onCFindRequest       CFindHandler
	onCGetRequest        CGetHandler
	onCMoveRequest       CMoveHandler
	onCStoreRequest      func(request network.AssociationRequest, data media.DICOMObject) uint16
	onCCancelRequest     func(request network.AssociationRequest, messageID uint16)
	onCEchoRequest       func(request network.AssociationRequest) bool
	canceledMessageIDs   map[uint16]struct{}
}

// NewSCP creates a plain-TCP DICOM SCP listening on port.
func NewSCP(port int) SCP {
	media.InitDict()

	return &scp{
		Port:               port,
		canceledMessageIDs: make(map[uint16]struct{}),
	}
}

// NewSCPWithTLS creates a TLS-enabled DICOM SCP listening on port.
// cfg must contain at least one certificate (e.g. loaded with tls.LoadX509KeyPair).
func NewSCPWithTLS(port int, cfg *tls.Config) SCP {
	media.InitDict()

	return &scp{
		Port:               port,
		tlsConfig:          normalizeServerTLSConfig(cfg),
		canceledMessageIDs: make(map[uint16]struct{}),
	}
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

func isValidQueryRetrieveLevel(level string) bool {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "PATIENT", "STUDY", "SERIES", "IMAGE", "FRAME":
		return true
	default:
		return false
	}
}

func (s *scp) markCanceled(messageID uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canceledMessageIDs[messageID] = struct{}{}
}

func (s *scp) consumeCanceled(messageID uint16) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.canceledMessageIDs[messageID]
	if ok {
		delete(s.canceledMessageIDs, messageID)
	}
	return ok
}

func isReadTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func (s *scp) handleCancelCommand(assocRQ network.AssociationRequest, dco media.DICOMObject) uint16 {
	messageID := dco.GetUShort(tags.MessageIDBeingRespondedTo)
	s.markCanceled(messageID)

	s.mu.RLock()
	cancelHandler := s.onCCancelRequest
	s.mu.RUnlock()
	if cancelHandler != nil {
		cancelHandler(assocRQ, messageID)
	}

	slog.Info("scp: C-CANCEL received", "message_id_being_responded_to", messageID)
	return messageID
}

func (s *scp) pollCommand(conn net.Conn, pdu network.PDUService) (media.DICOMObject, error) {
	if conn == nil {
		return nil, nil
	}

	if err := conn.SetReadDeadline(time.Now().Add(cancelPollWindow)); err != nil {
		return nil, err
	}
	dco, err := pdu.NextPDU()
	_ = conn.SetReadDeadline(time.Time{})

	if err != nil {
		if isReadTimeout(err) {
			return nil, nil
		}
		return nil, err
	}

	return dco, nil
}

// subopProgress is the internal representation of a single sub-operation
// progress event, shared across C-GET and C-MOVE.
type subopProgress struct {
	Remaining uint16
	Completed uint16
	Failed    uint16
	Warnings  uint16
}

// subopResult is the internal final outcome of a multi-sub-operation handler.
type subopResult struct {
	Status    uint16
	Remaining uint16
	Completed uint16
	Failed    uint16
	Warnings  uint16
	Err       error
}

// writeSubopRSP is the function signature shared by CGetWriteRSP and CMoveWriteRSP.
type writeSubopRSP func(pdu network.PDUService, req media.DICOMObject, status uint16, remaining, completed, failed, warnings uint16) error

// runSubopLoop is the shared C-GET / C-MOVE event loop. It drives the
// progress and result channels, handles C-CANCEL, polls for interleaved
// commands, and writes DIMSE responses via writeRSP.
func (s *scp) runSubopLoop(
	rw *bufio.ReadWriter,
	conn net.Conn,
	pdu network.PDUService,
	queue *[]media.DICOMObject,
	requestCommandObj media.DICOMObject,
	assocRQ network.AssociationRequest,
	progressCh <-chan subopProgress,
	resultCh <-chan subopResult,
	cancel context.CancelFunc,
	writeRSP writeSubopRSP,
) error {
	messageID := requestCommandObj.GetUShort(tags.MessageID)
	state := operationCounterState{}
	canceled := false
	var cancelDeadline time.Time

	for {
		if canceled && !cancelDeadline.IsZero() && time.Now().After(cancelDeadline) {
			abortAssociation(rw, conn)
			return errors.New("scp: sub-op handler did not exit within cancel grace window")
		}

		select {
		case progress := <-progressCh:
			if err := state.applyProgress(progress.Remaining, progress.Completed, progress.Failed, progress.Warnings); err != nil {
				cancel()
				return writeRSP(pdu, requestCommandObj, dicomstatus.FailureProcessingFailure, 0, state.completed, state.failed+1, state.warnings)
			}
			if err := writeRSP(pdu, requestCommandObj, dicomstatus.Pending, progress.Remaining, progress.Completed, progress.Failed, progress.Warnings); err != nil {
				return err
			}
		case resp := <-resultCh:
			drain := true
			for drain {
				select {
				case progress := <-progressCh:
					if err := state.applyProgress(progress.Remaining, progress.Completed, progress.Failed, progress.Warnings); err != nil {
						return writeRSP(pdu, requestCommandObj, dicomstatus.FailureProcessingFailure, 0, state.completed, state.failed+1, state.warnings)
					}
					if err := writeRSP(pdu, requestCommandObj, dicomstatus.Pending, progress.Remaining, progress.Completed, progress.Failed, progress.Warnings); err != nil {
						return err
					}
				default:
					drain = false
				}
			}

			if resp.Err != nil {
				if canceled {
					return writeRSP(pdu, requestCommandObj, dicomstatus.Cancel, 0, state.completed, state.failed, state.warnings)
				}
				return writeRSP(pdu, requestCommandObj, dicomstatus.FailureProcessingFailure, 0, state.completed, state.failed+1, state.warnings)
			}

			final := resp
			if final.Completed == 0 && final.Failed == 0 && final.Warnings == 0 {
				final.Completed = state.completed
				final.Failed = state.failed
				final.Warnings = state.warnings
			}

			status, remaining, completed, failed, warnings, err := state.finalize(final.Status, final.Remaining, final.Completed, final.Failed, final.Warnings, canceled)
			if err != nil {
				return writeRSP(pdu, requestCommandObj, dicomstatus.FailureProcessingFailure, 0, state.completed, state.failed+1, state.warnings)
			}

			return writeRSP(pdu, requestCommandObj, status, remaining, completed, failed, warnings)
		default:
			dco, err := s.pollCommand(conn, pdu)
			if err != nil {
				return err
			}
			if dco == nil {
				continue
			}

			if dco.GetUShort(tags.CommandField) == dicomcommand.CCancelRequest {
				cancelMessageID := s.handleCancelCommand(assocRQ, dco)
				if cancelMessageID == messageID {
					canceled = true
					if cancelDeadline.IsZero() {
						cancelDeadline = time.Now().Add(cancelGraceWindow)
					}
					cancel()
				}
				continue
			}

			*queue = append(*queue, dco)
		}
	}
}

func (s *scp) runCFindOperation(rw *bufio.ReadWriter, conn net.Conn, pdu network.PDUService, queue *[]media.DICOMObject, requestCommandObj media.DICOMObject, assocRQ network.AssociationRequest, findLevel string, query media.DICOMObject, handler CFindHandler) error {
	type findResponse struct {
		result CFindResult
		err    error
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	matchCh := make(chan media.DICOMObject, 16)
	resultCh := make(chan findResponse, 1)

	go func() {
		result, err := handler(ctx, assocRQ, findLevel, query, func(match media.DICOMObject) {
			select {
			case <-ctx.Done():
				return
			case matchCh <- match:
			}
		})
		resultCh <- findResponse{result: result, err: err}
	}()

	messageID := requestCommandObj.GetUShort(tags.MessageID)
	canceled := false
	var cancelDeadline time.Time

	for {
		if canceled && !cancelDeadline.IsZero() && time.Now().After(cancelDeadline) {
			abortAssociation(rw, conn)
			return errors.New("scp: C-Find handler did not exit within cancel grace window")
		}

		select {
		case match := <-matchCh:
			if match == nil {
				cancel()
				return dimse.CFindWriteRSP(pdu, requestCommandObj, media.NewEmptyDCMObj(), dicomstatus.FailureProcessingFailure)
			}
			if err := dimse.CFindWriteRSP(pdu, requestCommandObj, match, dicomstatus.Pending); err != nil {
				return err
			}
		case resp := <-resultCh:
			drain := true
			for drain {
				select {
				case match := <-matchCh:
					if match == nil {
						return dimse.CFindWriteRSP(pdu, requestCommandObj, media.NewEmptyDCMObj(), dicomstatus.FailureProcessingFailure)
					}
					if err := dimse.CFindWriteRSP(pdu, requestCommandObj, match, dicomstatus.Pending); err != nil {
						return err
					}
				default:
					drain = false
				}
			}

			if resp.err != nil {
				if canceled {
					return dimse.CFindWriteRSP(pdu, requestCommandObj, media.NewEmptyDCMObj(), dicomstatus.Cancel)
				}
				return dimse.CFindWriteRSP(pdu, requestCommandObj, media.NewEmptyDCMObj(), dicomstatus.FailureProcessingFailure)
			}

			finalStatus := resp.result.Status
			if canceled {
				finalStatus = dicomstatus.Cancel
			}
			if finalStatus == dicomstatus.Pending || finalStatus == dicomstatus.PendingWithWarnings {
				finalStatus = dicomstatus.FailureProcessingFailure
			}

			return dimse.CFindWriteRSP(pdu, requestCommandObj, media.NewEmptyDCMObj(), finalStatus)
		default:
			dco, err := s.pollCommand(conn, pdu)
			if err != nil {
				return err
			}
			if dco == nil {
				continue
			}

			if dco.GetUShort(tags.CommandField) == dicomcommand.CCancelRequest {
				cancelMessageID := s.handleCancelCommand(assocRQ, dco)
				if cancelMessageID == messageID {
					canceled = true
					if cancelDeadline.IsZero() {
						cancelDeadline = time.Now().Add(cancelGraceWindow)
					}
					cancel()
				}
				continue
			}

			*queue = append(*queue, dco)
		}
	}
}

// cgetStoreRequest is a request from the C-GET handler goroutine to send a
// C-STORE sub-operation back to the SCU over the same association.
type cgetStoreRequest struct {
	path  string
	reply chan error
}

// cgetStoreSubop loads a DICOM file, sends it to the SCU as a C-STORE-RQ, and
// reads back the C-STORE-RSP. Must be called from the PDU event loop goroutine.
func (s *scp) cgetStoreSubop(pdu network.PDUService, path string) error {
	dco, err := media.NewDCMObjFromFile(path)
	if err != nil {
		return fmt.Errorf("scp: C-GET: load %q: %w", path, err)
	}
	if err := dimse.CStoreWriteRQ(pdu, dco); err != nil {
		return fmt.Errorf("scp: C-GET: C-STORE-RQ: %w", err)
	}
	status, err := dimse.CStoreReadRSP(pdu)
	if err != nil {
		return fmt.Errorf("scp: C-GET: C-STORE-RSP: %w", err)
	}
	if status != dicomstatus.Success {
		return fmt.Errorf("scp: C-GET: C-STORE rejected with status 0x%04X", status)
	}
	return nil
}

func (s *scp) runCGetOperation(rw *bufio.ReadWriter, conn net.Conn, pdu network.PDUService, queue *[]media.DICOMObject, requestCommandObj media.DICOMObject, assocRQ network.AssociationRequest, getLevel string, query media.DICOMObject, handler CGetHandler) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	storeReqCh := make(chan cgetStoreRequest, 1)
	progressCh := make(chan subopProgress, 16)
	resultCh := make(chan subopResult, 1)

	go func() {
		result, err := handler(ctx, assocRQ, getLevel, query,
			func(path string) error {
				reply := make(chan error, 1)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case storeReqCh <- cgetStoreRequest{path: path, reply: reply}:
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case err := <-reply:
					return err
				}
			},
			func(p CGetProgress) {
				select {
				case <-ctx.Done():
					return
				case progressCh <- subopProgress{p.Remaining, p.Completed, p.Failed, p.Warnings}:
				}
			},
		)
		resultCh <- subopResult{
			Status:    result.Status,
			Remaining: result.Remaining,
			Completed: result.Completed,
			Failed:    result.Failed,
			Warnings:  result.Warnings,
			Err:       err,
		}
	}()

	messageID := requestCommandObj.GetUShort(tags.MessageID)
	state := operationCounterState{}
	canceled := false
	var cancelDeadline time.Time

	for {
		if canceled && !cancelDeadline.IsZero() && time.Now().After(cancelDeadline) {
			abortAssociation(rw, conn)
			return errors.New("scp: C-GET sub-op handler did not exit within cancel grace window")
		}

		select {
		case req := <-storeReqCh:
			// Perform C-STORE sub-operation synchronously on the PDU event loop.
			req.reply <- s.cgetStoreSubop(pdu, req.path)

		case progress := <-progressCh:
			if err := state.applyProgress(progress.Remaining, progress.Completed, progress.Failed, progress.Warnings); err != nil {
				cancel()
				return dimse.CGetWriteRSP(pdu, requestCommandObj, dicomstatus.FailureProcessingFailure, 0, state.completed, state.failed+1, state.warnings)
			}
			if err := dimse.CGetWriteRSP(pdu, requestCommandObj, dicomstatus.Pending, progress.Remaining, progress.Completed, progress.Failed, progress.Warnings); err != nil {
				return err
			}

		case resp := <-resultCh:
			// Drain any remaining pending progress messages.
			drain := true
			for drain {
				select {
				case progress := <-progressCh:
					if err := state.applyProgress(progress.Remaining, progress.Completed, progress.Failed, progress.Warnings); err != nil {
						return dimse.CGetWriteRSP(pdu, requestCommandObj, dicomstatus.FailureProcessingFailure, 0, state.completed, state.failed+1, state.warnings)
					}
					if err := dimse.CGetWriteRSP(pdu, requestCommandObj, dicomstatus.Pending, progress.Remaining, progress.Completed, progress.Failed, progress.Warnings); err != nil {
						return err
					}
				default:
					drain = false
				}
			}

			if resp.Err != nil {
				if canceled {
					return dimse.CGetWriteRSP(pdu, requestCommandObj, dicomstatus.Cancel, 0, state.completed, state.failed, state.warnings)
				}
				return dimse.CGetWriteRSP(pdu, requestCommandObj, dicomstatus.FailureProcessingFailure, 0, state.completed, state.failed+1, state.warnings)
			}

			final := resp
			if final.Completed == 0 && final.Failed == 0 && final.Warnings == 0 {
				final.Completed = state.completed
				final.Failed = state.failed
				final.Warnings = state.warnings
			}

			status, remaining, completed, failed, warnings, err := state.finalize(final.Status, final.Remaining, final.Completed, final.Failed, final.Warnings, canceled)
			if err != nil {
				return dimse.CGetWriteRSP(pdu, requestCommandObj, dicomstatus.FailureProcessingFailure, 0, state.completed, state.failed+1, state.warnings)
			}
			return dimse.CGetWriteRSP(pdu, requestCommandObj, status, remaining, completed, failed, warnings)

		default:
			dco, err := s.pollCommand(conn, pdu)
			if err != nil {
				return err
			}
			if dco == nil {
				continue
			}
			if dco.GetUShort(tags.CommandField) == dicomcommand.CCancelRequest {
				cancelMessageID := s.handleCancelCommand(assocRQ, dco)
				if cancelMessageID == messageID {
					canceled = true
					if cancelDeadline.IsZero() {
						cancelDeadline = time.Now().Add(cancelGraceWindow)
					}
					cancel()
				}
				continue
			}
			*queue = append(*queue, dco)
		}
	}
}

func (s *scp) runCMoveOperation(rw *bufio.ReadWriter, conn net.Conn, pdu network.PDUService, queue *[]media.DICOMObject, requestCommandObj media.DICOMObject, assocRQ network.AssociationRequest, moveDestAE string, moveLevel string, query media.DICOMObject, handler CMoveHandler) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	progressCh := make(chan subopProgress, 16)
	resultCh := make(chan subopResult, 1)

	go func() {
		result, err := handler(ctx, assocRQ, moveDestAE, moveLevel, query, func(p CMoveProgress) {
			select {
			case <-ctx.Done():
				return
			case progressCh <- subopProgress{p.Remaining, p.Completed, p.Failed, p.Warnings}:
			}
		})
		resultCh <- subopResult{
			Status:    result.Status,
			Remaining: result.Remaining,
			Completed: result.Completed,
			Failed:    result.Failed,
			Warnings:  result.Warnings,
			Err:       err,
		}
	}()

	return s.runSubopLoop(rw, conn, pdu, queue, requestCommandObj, assocRQ, progressCh, resultCh, cancel, dimse.CMoveWriteRSP)
}

func (s *scp) Start(ctx context.Context) error {
	var err error
	if s.tlsConfig != nil {
		s.listener, err = tls.Listen("tcp", fmt.Sprintf(":%d", s.Port), s.tlsConfig)
	} else {
		s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.Port))
	}
	if err != nil {
		return err
	}

	// Close the listener when ctx is cancelled so Accept() unblocks.
	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			slog.Error("scp: accept failed", "ERROR", err)
			return err
		}
		slog.Info("scp: new connection", "ADDRESS", conn.RemoteAddr())
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			s.handleConnection(c)
		}(conn)
	}
}

func (s *scp) Stop() error {
	if s.listener == nil {
		return nil
	}
	err := s.listener.Close()
	s.wg.Wait()
	return err
}

func (s *scp) handleConnection(conn net.Conn) {
	rw := bufio.NewReadWriter(
		bufio.NewReaderSize(conn, scpBufSize),
		bufio.NewWriterSize(conn, scpBufSize),
	)

	pdu := network.NewPDUService()
	pdu.SetConn(rw)
	pdu.SetNetConn(conn)

	s.mu.RLock()
	assocHandler := s.onAssociationRequest
	s.mu.RUnlock()
	if assocHandler != nil {
		pdu.SetOnAssociationRequest(assocHandler)
	}

	defer conn.Close()
	queuedCommands := make([]media.DICOMObject, 0, 1)

	for {
		var (
			dco media.DICOMObject
			err error
		)

		if len(queuedCommands) > 0 {
			dco = queuedCommands[0]
			queuedCommands = queuedCommands[1:]
		} else {
			dco, err = pdu.NextPDU()
			if err != nil {
				if errors.Is(err, io.EOF) {
					slog.Debug("scp: connection closed (EOF)", "ADDRESS", conn.RemoteAddr())
				} else if !errors.Is(err, network.ErrAssociationReleased) &&
					!errors.Is(err, network.ErrAssociationAborted) &&
					!errors.Is(err, network.ErrAssociationRejected) {
					slog.Error("scp: network error", "ERROR", err)
				}
				return
			}
		}
		if dco == nil {
			continue
		}
		command := dco.GetUShort(tags.CommandField)
		switch command {
		case dicomcommand.CStoreRequest:
			ddo, err := pdu.NextPDU()
			if err != nil {
				slog.Error("scp: C-Store failed to read request", "ERROR", err.Error())
				return
			}

			s.mu.RLock()
			storeHandler := s.onCStoreRequest
			s.mu.RUnlock()

			if storeHandler == nil {
				slog.Warn("scp: OnCStoreRequest not registered")
				if err := dimse.CStoreWriteRSP(pdu, dco, dicomstatus.FailureProcessingFailure); err != nil {
					slog.Error("scp: C-Store failed to write error response", "ERROR", err.Error())
					return
				}
				continue
			}

			status := storeHandler(pdu.GetAAssociationRQ(), ddo)
			if err := dimse.CStoreWriteRSP(pdu, dco, status); err != nil {
				slog.Error("scp: C-Store failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.CFindRequest:
			ddo, err := pdu.NextPDU()
			if err != nil {
				slog.Error("scp: C-Find failed to read request", "ERROR", err.Error())
				return
			}
			if !isValidQueryRetrieveLevel(ddo.GetString(tags.QueryRetrieveLevel)) {
				if err := dimse.CFindWriteRSP(pdu, dco, media.NewEmptyDCMObj(), dicomstatus.FailureIdentifierDoesNotMatchSOPClass); err != nil {
					slog.Error("scp: C-Find failed to write invalid-identifier response", "ERROR", err.Error())
					return
				}
				continue
			}
			if s.consumeCanceled(dco.GetUShort(tags.MessageID)) {
				if err := dimse.CFindWriteRSP(pdu, dco, media.NewEmptyDCMObj(), dicomstatus.Cancel); err != nil {
					slog.Error("scp: C-Find failed to write cancel response", "ERROR", err.Error())
					return
				}
				continue
			}

			s.mu.RLock()
			findHandler := s.onCFindRequest
			s.mu.RUnlock()

			if findHandler == nil {
				slog.Warn("scp: OnCFindRequest not registered")
				if err := dimse.CFindWriteRSP(pdu, dco, media.NewEmptyDCMObj(), dicomstatus.FailureProcessingFailure); err != nil {
					slog.Error("scp: C-Find failed to write error response", "ERROR", err.Error())
					return
				}
				continue
			}

			queryLevel := ddo.GetString(tags.QueryRetrieveLevel)
			if err := s.runCFindOperation(rw, conn, pdu, &queuedCommands, dco, pdu.GetAAssociationRQ(), queryLevel, ddo, findHandler); err != nil {
				slog.Error("scp: C-Find operation failed", "ERROR", err.Error())
				return
			}

		case dicomcommand.CGetRequest:
			ddo, err := pdu.NextPDU()
			if err != nil {
				slog.Error("scp: C-Get failed to read request", "ERROR", err.Error())
				return
			}
			if !isValidQueryRetrieveLevel(ddo.GetString(tags.QueryRetrieveLevel)) {
				if err := dimse.CGetWriteRSP(pdu, dco, dicomstatus.FailureIdentifierDoesNotMatchSOPClass, 0, 0, 0, 0); err != nil {
					slog.Error("scp: C-Get failed to write invalid-identifier response", "ERROR", err.Error())
					return
				}
				continue
			}
			if s.consumeCanceled(dco.GetUShort(tags.MessageID)) {
				if err := dimse.CGetWriteRSP(pdu, dco, dicomstatus.Cancel, 0, 0, 0, 0); err != nil {
					slog.Error("scp: C-Get failed to write cancel response", "ERROR", err.Error())
					return
				}
				continue
			}

			s.mu.RLock()
			getHandler := s.onCGetRequest
			s.mu.RUnlock()

			if getHandler == nil {
				slog.Warn("scp: OnCGetRequest not registered")
				if err := dimse.CGetWriteRSP(pdu, dco, dicomstatus.FailureSOPClassNotSupported, 0, 0, 0, 0); err != nil {
					slog.Error("scp: C-Get failed to write error response", "ERROR", err.Error())
					return
				}
				continue
			}

			getLevel := ddo.GetString(tags.QueryRetrieveLevel)
			if err := s.runCGetOperation(rw, conn, pdu, &queuedCommands, dco, pdu.GetAAssociationRQ(), getLevel, ddo, getHandler); err != nil {
				slog.Error("scp: C-Get operation failed", "ERROR", err.Error())
				return
			}

		case dicomcommand.CMoveRequest:
			ddo, err := pdu.NextPDU()
			if err != nil {
				slog.Error("scp: C-Move failed to read request", "ERROR", err.Error())
				return
			}
			if !isValidQueryRetrieveLevel(ddo.GetString(tags.QueryRetrieveLevel)) {
				if err := dimse.CMoveWriteRSP(pdu, dco, dicomstatus.FailureIdentifierDoesNotMatchSOPClass, 0, 0, 0, 0); err != nil {
					slog.Error("scp: C-Move failed to write invalid-identifier response", "ERROR", err.Error())
					return
				}
				continue
			}
			if s.consumeCanceled(dco.GetUShort(tags.MessageID)) {
				if err := dimse.CMoveWriteRSP(pdu, dco, dicomstatus.Cancel, 0, 0, 0, 0); err != nil {
					slog.Error("scp: C-Move failed to write cancel response", "ERROR", err.Error())
					return
				}
				continue
			}

			s.mu.RLock()
			moveHandler := s.onCMoveRequest
			s.mu.RUnlock()

			if moveHandler == nil {
				slog.Warn("scp: OnCMoveRequest not registered")
				if err := dimse.CMoveWriteRSP(pdu, dco, dicomstatus.FailureProcessingFailure, 0, 0, 0, 0); err != nil {
					slog.Error("scp: C-Move failed to write error response", "ERROR", err.Error())
					return
				}
				continue
			}

			moveLevel := ddo.GetString(tags.QueryRetrieveLevel)
			moveDestAE := dco.GetString(tags.MoveDestination)
			if err := s.runCMoveOperation(rw, conn, pdu, &queuedCommands, dco, pdu.GetAAssociationRQ(), moveDestAE, moveLevel, ddo, moveHandler); err != nil {
				slog.Error("scp: C-Move operation failed", "ERROR", err.Error())
				return
			}

		case dicomcommand.NEventReportRequest:
			var nData media.DICOMObject
			if dco.GetUShort(tags.CommandDataSetType) != dicomcommand.DataSetNone {
				nData, err = pdu.NextPDU()
				if err != nil {
					slog.Error("scp: N-Event-Report failed to read request dataset", "ERROR", err.Error())
					return
				}
			}
			s.mu.RLock()
			h := s.onNEventReportRequest
			s.mu.RUnlock()
			status := dicomstatus.FailureSOPClassNotSupported
			resp := media.NewEmptyDCMObj()
			if h != nil {
				status, resp = h(pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NEventReportResponse, status, resp); err != nil {
				slog.Error("scp: N-Event-Report failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.NGetRequest:
			var nData media.DICOMObject
			if dco.GetUShort(tags.CommandDataSetType) != dicomcommand.DataSetNone {
				nData, err = pdu.NextPDU()
				if err != nil {
					slog.Error("scp: N-Get failed to read request dataset", "ERROR", err.Error())
					return
				}
			}
			s.mu.RLock()
			h := s.onNGetRequest
			s.mu.RUnlock()
			status := dicomstatus.FailureSOPClassNotSupported
			resp := media.NewEmptyDCMObj()
			if h != nil {
				status, resp = h(pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NGetResponse, status, resp); err != nil {
				slog.Error("scp: N-Get failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.NSetRequest:
			var nData media.DICOMObject
			if dco.GetUShort(tags.CommandDataSetType) != dicomcommand.DataSetNone {
				nData, err = pdu.NextPDU()
				if err != nil {
					slog.Error("scp: N-Set failed to read request dataset", "ERROR", err.Error())
					return
				}
			}
			s.mu.RLock()
			h := s.onNSetRequest
			s.mu.RUnlock()
			status := dicomstatus.FailureSOPClassNotSupported
			resp := media.NewEmptyDCMObj()
			if h != nil {
				status, resp = h(pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NSetResponse, status, resp); err != nil {
				slog.Error("scp: N-Set failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.NActionRequest:
			var nData media.DICOMObject
			if dco.GetUShort(tags.CommandDataSetType) != dicomcommand.DataSetNone {
				nData, err = pdu.NextPDU()
				if err != nil {
					slog.Error("scp: N-Action failed to read request dataset", "ERROR", err.Error())
					return
				}
			}
			s.mu.RLock()
			h := s.onNActionRequest
			s.mu.RUnlock()
			status := dicomstatus.FailureSOPClassNotSupported
			resp := media.NewEmptyDCMObj()
			if h != nil {
				status, resp = h(pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NActionResponse, status, resp); err != nil {
				slog.Error("scp: N-Action failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.NCreateRequest:
			var nData media.DICOMObject
			if dco.GetUShort(tags.CommandDataSetType) != dicomcommand.DataSetNone {
				nData, err = pdu.NextPDU()
				if err != nil {
					slog.Error("scp: N-Create failed to read request dataset", "ERROR", err.Error())
					return
				}
			}
			s.mu.RLock()
			h := s.onNCreateRequest
			s.mu.RUnlock()
			status := dicomstatus.FailureSOPClassNotSupported
			resp := media.NewEmptyDCMObj()
			if h != nil {
				status, resp = h(pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NCreateResponse, status, resp); err != nil {
				slog.Error("scp: N-Create failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.NDeleteRequest:
			var nData media.DICOMObject
			if dco.GetUShort(tags.CommandDataSetType) != dicomcommand.DataSetNone {
				nData, err = pdu.NextPDU()
				if err != nil {
					slog.Error("scp: N-Delete failed to read request dataset", "ERROR", err.Error())
					return
				}
			}
			s.mu.RLock()
			h := s.onNDeleteRequest
			s.mu.RUnlock()
			status := dicomstatus.FailureSOPClassNotSupported
			resp := media.NewEmptyDCMObj()
			if h != nil {
				status, resp = h(pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NDeleteResponse, status, resp); err != nil {
				slog.Error("scp: N-Delete failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.CCancelRequest:
			// C-CANCEL-RQ applies to in-progress C-FIND/C-MOVE/C-GET operations.
			s.handleCancelCommand(pdu.GetAAssociationRQ(), dco)

		case dicomcommand.CEchoRequest:
			s.mu.RLock()
			echoHandler := s.onCEchoRequest
			s.mu.RUnlock()

			if echoHandler != nil && !echoHandler(pdu.GetAAssociationRQ()) {
				slog.Warn("scp: C-Echo rejected by handler")
				continue
			}
			if err := dimse.CEchoWriteRSP(pdu, dco); err != nil {
				slog.Error("scp: C-Echo failed to write response", "ERROR", err.Error())
				return
			}

		default:
			slog.Warn("scp: command not implemented, skipping", "COMMAND", command)
		}
	}
}

func (s *scp) OnAssociationRequest(f func(request network.AssociationRequest) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAssociationRequest = f
}

func (s *scp) OnCFindRequest(f CFindHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCFindRequest = f
}

func (s *scp) OnCGetRequest(f CGetHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCGetRequest = f
}

func (s *scp) OnCMoveRequest(f CMoveHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCMoveRequest = f
}

func (s *scp) OnCStoreRequest(f func(request network.AssociationRequest, data media.DICOMObject) uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCStoreRequest = f
}

func (s *scp) OnNEventReportRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNEventReportRequest = f
}

func (s *scp) OnNGetRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNGetRequest = f
}

func (s *scp) OnNSetRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNSetRequest = f
}

func (s *scp) OnNActionRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNActionRequest = f
}

func (s *scp) OnNCreateRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNCreateRequest = f
}

func (s *scp) OnNDeleteRequest(f func(request network.AssociationRequest, command media.DICOMObject, data media.DICOMObject) (status uint16, responseData media.DICOMObject)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNDeleteRequest = f
}

func (s *scp) OnCCancelRequest(f func(request network.AssociationRequest, messageID uint16)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCCancelRequest = f
}

func (s *scp) OnCEchoRequest(f func(request network.AssociationRequest) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCEchoRequest = f
}
