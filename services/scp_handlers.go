package services

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/network/priority"
	"github.com/innovative-io/io-dicom/transcoder"
)

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

func isValidQueryRetrieveLevel(level string) bool {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "PATIENT", "STUDY", "SERIES", "IMAGE", "FRAME":
		return true
	default:
		return false
	}
}

func isValidCFindQueryRetrieveLevel(level string) bool {
	if strings.TrimSpace(level) == "" {
		return true
	}
	return isValidQueryRetrieveLevel(level)
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
// runSubopLoop drives the SCP-side event loop shared by C-GET and C-MOVE: it
// streams Pending progress responses, enforces cancellation, forwards
// interleaved commands, and writes the final response.
//
// storeReqCh is the C-GET-only channel for C-STORE sub-operations that must run
// synchronously on this event loop (C-GET ships matched instances back over the
// same association). C-MOVE has no inline store and passes a nil channel — a
// receive on a nil channel blocks forever, so that select case is simply never
// selected.
func (s *scp) runSubopLoop(
	ctx context.Context,
	rw *bufio.ReadWriter,
	conn net.Conn,
	pdu network.PDUService,
	queue *[]media.DICOMObject,
	requestCommandObj media.DICOMObject,
	assocRQ network.AssociationRequest,
	storeReqCh <-chan cgetStoreRequest,
	progressCh <-chan subopProgress,
	resultCh <-chan subopResult,
	cancel context.CancelFunc,
	writeRSP writeSubopRSP,
) error {
	messageID := requestCommandObj.GetUint16(tags.MessageID)
	state := operationCounterState{}
	canceled := false
	var cancelDeadline time.Time

	for {
		if canceled && !cancelDeadline.IsZero() && time.Now().After(cancelDeadline) {
			abortAssociation(rw, conn)
			return errors.New("scp: sub-op handler did not exit within cancel grace window")
		}

		select {
		case req := <-storeReqCh:
			// Perform the C-STORE sub-operation synchronously on the PDU event
			// loop (C-GET only; nil for C-MOVE so this case never fires).
			req.reply <- s.cgetStoreSubop(ctx, pdu, req.path)

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

			if dco.GetUint16(tags.CommandField) == dicomcommand.CCancelRequest {
				cancelMessageID := s.handleCancelCommand(pdu.Logger(), assocRQ, dco)
				if cancelMessageID == messageID {
					canceled = true
					if cancelDeadline.IsZero() {
						cancelDeadline = time.Now().Add(s.cancelGrace)
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

	messageID := requestCommandObj.GetUint16(tags.MessageID)
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

			if dco.GetUint16(tags.CommandField) == dicomcommand.CCancelRequest {
				cancelMessageID := s.handleCancelCommand(pdu.Logger(), assocRQ, dco)
				if cancelMessageID == messageID {
					canceled = true
					if cancelDeadline.IsZero() {
						cancelDeadline = time.Now().Add(s.cancelGrace)
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

// cgetStoreSubop loads a DICOM file, transcodes it to the negotiated transfer
// syntax for its SOP class, sends it to the SCU as a C-STORE-RQ, and reads
// back the C-STORE-RSP. Must be called from the PDU event loop goroutine.
func (s *scp) cgetStoreSubop(ctx context.Context, pdu network.PDUService, path string) error {
	dco, err := media.NewDCMObjFromFile(path)
	if err != nil {
		return fmt.Errorf("scp: C-GET: load %q: %w", path, err)
	}

	// Find the transfer syntax negotiated for this SOP class and transcode if
	// it differs from the file's native encoding. ChangeTransferSyntaxContext is
	// a no-op when both UIDs are the same, so this is always safe to call.
	sopUID := dco.GetString(tags.SOPClassUID)
	var negotiatedTS *transfersyntax.TransferSyntax
	for _, pc := range pdu.GetAcceptedPresentationContexts() {
		if pc.GetAbstractSyntax().GetUID() == sopUID {
			negotiatedTS = transfersyntax.GetTransferSyntaxFromUID(pc.GetTrnSyntax().GetUID())
			break
		}
	}
	if negotiatedTS != nil {
		if err := transcoder.ChangeTransferSyntaxContext(ctx, dco, negotiatedTS); err != nil {
			return fmt.Errorf("scp: C-GET: transcode to %s: %w", negotiatedTS.Name, err)
		}
	}

	if err := dimse.CStoreWriteRQ(pdu, dco, priority.Medium); err != nil {
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
				case progressCh <- subopProgress(p):
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

	return s.runSubopLoop(ctx, rw, conn, pdu, queue, requestCommandObj, assocRQ, storeReqCh, progressCh, resultCh, cancel, dimse.CGetWriteRSP)
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
			case progressCh <- subopProgress(p):
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

	// C-MOVE has no inline C-STORE sub-operation (instances go to a separate
	// destination AE), so it passes a nil store channel.
	return s.runSubopLoop(ctx, rw, conn, pdu, queue, requestCommandObj, assocRQ, nil, progressCh, resultCh, cancel, dimse.CMoveWriteRSP)
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

func (s *scp) OnCStoreRequest(f func(ctx context.Context, request network.AssociationRequest, data media.DICOMObject) uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCStoreRequest = f
}

func (s *scp) OnNEventReportRequest(f NServiceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNEventReportRequest = f
}

func (s *scp) OnNGetRequest(f NServiceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNGetRequest = f
}

func (s *scp) OnNSetRequest(f NServiceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNSetRequest = f
}

func (s *scp) OnNActionRequest(f NServiceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNActionRequest = f
}

func (s *scp) OnNCreateRequest(f NServiceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNCreateRequest = f
}

func (s *scp) OnNDeleteRequest(f NServiceHandler) {
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

func (s *scp) OnRawPDU(f func(event network.RawPDUEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onRawPDU = f
}

func (s *scp) SetTimeout(seconds int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if seconds >= 0 {
		s.timeout = seconds
	}
}
