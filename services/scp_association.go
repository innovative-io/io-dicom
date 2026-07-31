package services

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

// assocCounter assigns each accepted association a process-unique, monotonically
// increasing id used to correlate its log lines.
var assocCounter atomic.Uint64

func nextAssocID() uint64 {
	return assocCounter.Add(1)
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

func (s *scp) handleCancelCommand(log *slog.Logger, assocRQ network.AssociationRequest, dco media.DICOMObject) uint16 {
	messageID := dco.GetUint16(tags.MessageIDBeingRespondedTo)
	s.markCanceled(messageID)

	s.mu.RLock()
	cancelHandler := s.onCCancelRequest
	s.mu.RUnlock()
	if cancelHandler != nil {
		cancelHandler(assocRQ, messageID)
	}

	log.Info("C-CANCEL received", "message_id_being_responded_to", messageID)
	return messageID
}

func (s *scp) pollCommand(conn net.Conn, pdu network.PDUService) (media.DICOMObject, error) {
	if conn == nil {
		return nil, nil
	}

	if err := conn.SetReadDeadline(time.Now().Add(s.cancelPoll)); err != nil {
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

// handleNRequest services one DICOM normalized (N-) request: it reads the
// optional request dataset, dispatches to handler (or replies
// FailureSOPClassNotSupported when none is registered), and writes the response
// identified by responseCmd. label is the human-readable service name used in
// log messages (e.g. "N-Get"). It returns true when the caller must terminate
// the association because an unrecoverable read/write error was logged.
func (s *scp) handleNRequest(ctx context.Context, pdu network.PDUService, dco media.DICOMObject, label string, responseCmd uint16, handler NServiceHandler) (stop bool) {
	var nData media.DICOMObject
	if dco.GetUint16(tags.CommandDataSetType) != dicomcommand.DataSetNone {
		d, err := pdu.NextPDU()
		if err != nil {
			pdu.Logger().Error(label+" failed to read request dataset", "error", err)
			return true
		}
		nData = d
	}

	status := dicomstatus.FailureSOPClassNotSupported
	resp := media.NewEmptyDCMObj()
	if handler != nil {
		status, resp = handler(ctx, pdu.GetAAssociationRQ(), dco, nData)
	}
	if err := dimse.NWriteRSP(pdu, dco, responseCmd, status, resp); err != nil {
		pdu.Logger().Error(label+" failed to write response", "error", err)
		return true
	}
	return false
}

func (s *scp) handleConnection(ctx context.Context, conn net.Conn) {
	rw := bufio.NewReadWriter(
		bufio.NewReaderSize(conn, s.bufSize),
		bufio.NewWriterSize(conn, s.bufSize),
	)

	pdu := s.newPDUService()
	pdu.SetConn(rw)
	pdu.SetNetConn(conn)

	// Per-association logger: tag every line with a process-unique id and the
	// peer address from the outset, then enrich with the negotiated AE titles
	// once the association is established.
	connLog := s.logger.With(
		"assoc_id", nextAssocID(),
		"remote_addr", conn.RemoteAddr().String(),
	)
	pdu.SetLogger(connLog)

	s.mu.RLock()
	assocHandler := s.onAssociationRequest
	onRawPDU := s.onRawPDU
	s.mu.RUnlock()
	pdu.SetOnRawPDU(onRawPDU)
	if assocHandler != nil {
		pdu.SetOnAssociationRequest(assocHandler)
	}

	defer conn.Close()
	queuedCommands := make([]media.DICOMObject, 0, 1)
	negotiated := false

	for {
		var (
			dco media.DICOMObject
			err error
		)

		if len(queuedCommands) > 0 {
			dco = queuedCommands[0]
			queuedCommands = queuedCommands[1:]
		} else {
			// Bound the wait for the next command. This read is where a stalled
			// peer parks: it previously had no deadline, so one connection that
			// sent a few bytes and stopped held its goroutine — and a slot in the
			// association semaphore — indefinitely.
			//
			// The deadline is cleared immediately after, so the dataset reads that
			// follow a command are not bounded by it and a slow but active
			// transfer is never interrupted. pollCommand manages its own, much
			// shorter, cancel-poll deadline on a separate path and is unaffected.
			if s.idleTimeout > 0 {
				if dlErr := conn.SetReadDeadline(time.Now().Add(s.idleTimeout)); dlErr != nil {
					pdu.Logger().Error("failed to set idle deadline", "error", dlErr)
					return
				}
			}
			dco, err = pdu.NextPDU()
			if s.idleTimeout > 0 {
				_ = conn.SetReadDeadline(time.Time{})
			}
			if err != nil {
				if isReadTimeout(err) {
					pdu.Logger().Warn("closing idle association",
						"idle_timeout", s.idleTimeout)
					return
				}
				if errors.Is(err, io.EOF) {
					pdu.Logger().Debug("connection closed (EOF)")
				} else if !errors.Is(err, network.ErrAssociationReleased) &&
					!errors.Is(err, network.ErrAssociationAborted) &&
					!errors.Is(err, network.ErrAssociationRejected) {
					pdu.Logger().Error("network error", "error", err)
				}
				return
			}
		}
		if !negotiated {
			negotiated = true
			connLog = connLog.With("calling_ae", pdu.GetCallingAE(), "called_ae", pdu.GetCalledAE())
			pdu.SetLogger(connLog)
		}
		if dco == nil {
			continue
		}
		command := dco.GetUint16(tags.CommandField)
		switch command {
		case dicomcommand.CStoreRequest:
			ddo, err := pdu.NextPDU()
			if err != nil {
				pdu.Logger().Error("C-Store failed to read request", "error", err)
				return
			}

			s.mu.RLock()
			storeHandler := s.onCStoreRequest
			s.mu.RUnlock()

			if storeHandler == nil {
				pdu.Logger().Warn("OnCStoreRequest not registered")
				if err := dimse.CStoreWriteRSP(pdu, dco, dicomstatus.FailureProcessingFailure); err != nil {
					pdu.Logger().Error("C-Store failed to write error response", "error", err)
					return
				}
				continue
			}

			status := storeHandler(ctx, pdu.GetAAssociationRQ(), ddo)
			if err := dimse.CStoreWriteRSP(pdu, dco, status); err != nil {
				pdu.Logger().Error("C-Store failed to write response", "error", err)
				return
			}

		case dicomcommand.CFindRequest:
			ddo, err := pdu.NextPDU()
			if err != nil {
				pdu.Logger().Error("C-Find failed to read request", "error", err)
				return
			}
			if !isValidCFindQueryRetrieveLevel(ddo.GetString(tags.QueryRetrieveLevel)) {
				if err := dimse.CFindWriteRSP(pdu, dco, media.NewEmptyDCMObj(), dicomstatus.FailureIdentifierDoesNotMatchSOPClass); err != nil {
					pdu.Logger().Error("C-Find failed to write invalid-identifier response", "error", err)
					return
				}
				continue
			}
			if s.consumeCanceled(dco.GetUint16(tags.MessageID)) {
				if err := dimse.CFindWriteRSP(pdu, dco, media.NewEmptyDCMObj(), dicomstatus.Cancel); err != nil {
					pdu.Logger().Error("C-Find failed to write cancel response", "error", err)
					return
				}
				continue
			}

			s.mu.RLock()
			findHandler := s.onCFindRequest
			s.mu.RUnlock()

			if findHandler == nil {
				pdu.Logger().Warn("OnCFindRequest not registered")
				if err := dimse.CFindWriteRSP(pdu, dco, media.NewEmptyDCMObj(), dicomstatus.FailureProcessingFailure); err != nil {
					pdu.Logger().Error("C-Find failed to write error response", "error", err)
					return
				}
				continue
			}

			queryLevel := ddo.GetString(tags.QueryRetrieveLevel)
			if err := s.runCFindOperation(rw, conn, pdu, &queuedCommands, dco, pdu.GetAAssociationRQ(), queryLevel, ddo, findHandler); err != nil {
				pdu.Logger().Error("C-Find operation failed", "error", err)
				return
			}

		case dicomcommand.CGetRequest:
			ddo, err := pdu.NextPDU()
			if err != nil {
				pdu.Logger().Error("C-Get failed to read request", "error", err)
				return
			}
			if !isValidQueryRetrieveLevel(ddo.GetString(tags.QueryRetrieveLevel)) {
				if err := dimse.CGetWriteRSP(pdu, dco, dicomstatus.FailureIdentifierDoesNotMatchSOPClass, 0, 0, 0, 0); err != nil {
					pdu.Logger().Error("C-Get failed to write invalid-identifier response", "error", err)
					return
				}
				continue
			}
			if s.consumeCanceled(dco.GetUint16(tags.MessageID)) {
				if err := dimse.CGetWriteRSP(pdu, dco, dicomstatus.Cancel, 0, 0, 0, 0); err != nil {
					pdu.Logger().Error("C-Get failed to write cancel response", "error", err)
					return
				}
				continue
			}

			s.mu.RLock()
			getHandler := s.onCGetRequest
			s.mu.RUnlock()

			if getHandler == nil {
				pdu.Logger().Warn("OnCGetRequest not registered")
				if err := dimse.CGetWriteRSP(pdu, dco, dicomstatus.FailureSOPClassNotSupported, 0, 0, 0, 0); err != nil {
					pdu.Logger().Error("C-Get failed to write error response", "error", err)
					return
				}
				continue
			}

			getLevel := ddo.GetString(tags.QueryRetrieveLevel)
			if err := s.runCGetOperation(rw, conn, pdu, &queuedCommands, dco, pdu.GetAAssociationRQ(), getLevel, ddo, getHandler); err != nil {
				pdu.Logger().Error("C-Get operation failed", "error", err)
				return
			}

		case dicomcommand.CMoveRequest:
			ddo, err := pdu.NextPDU()
			if err != nil {
				pdu.Logger().Error("C-Move failed to read request", "error", err)
				return
			}
			if !isValidQueryRetrieveLevel(ddo.GetString(tags.QueryRetrieveLevel)) {
				if err := dimse.CMoveWriteRSP(pdu, dco, dicomstatus.FailureIdentifierDoesNotMatchSOPClass, 0, 0, 0, 0); err != nil {
					pdu.Logger().Error("C-Move failed to write invalid-identifier response", "error", err)
					return
				}
				continue
			}
			if s.consumeCanceled(dco.GetUint16(tags.MessageID)) {
				if err := dimse.CMoveWriteRSP(pdu, dco, dicomstatus.Cancel, 0, 0, 0, 0); err != nil {
					pdu.Logger().Error("C-Move failed to write cancel response", "error", err)
					return
				}
				continue
			}

			s.mu.RLock()
			moveHandler := s.onCMoveRequest
			s.mu.RUnlock()

			if moveHandler == nil {
				pdu.Logger().Warn("OnCMoveRequest not registered")
				if err := dimse.CMoveWriteRSP(pdu, dco, dicomstatus.FailureProcessingFailure, 0, 0, 0, 0); err != nil {
					pdu.Logger().Error("C-Move failed to write error response", "error", err)
					return
				}
				continue
			}

			moveLevel := ddo.GetString(tags.QueryRetrieveLevel)
			moveDestAE := dco.GetString(tags.MoveDestination)
			if err := s.runCMoveOperation(rw, conn, pdu, &queuedCommands, dco, pdu.GetAAssociationRQ(), moveDestAE, moveLevel, ddo, moveHandler); err != nil {
				pdu.Logger().Error("C-Move operation failed", "error", err)
				return
			}

		case dicomcommand.NEventReportRequest:
			s.mu.RLock()
			h := s.onNEventReportRequest
			s.mu.RUnlock()
			if s.handleNRequest(ctx, pdu, dco, "N-Event-Report", dicomcommand.NEventReportResponse, h) {
				return
			}

		case dicomcommand.NGetRequest:
			s.mu.RLock()
			h := s.onNGetRequest
			s.mu.RUnlock()
			if s.handleNRequest(ctx, pdu, dco, "N-Get", dicomcommand.NGetResponse, h) {
				return
			}

		case dicomcommand.NSetRequest:
			s.mu.RLock()
			h := s.onNSetRequest
			s.mu.RUnlock()
			if s.handleNRequest(ctx, pdu, dco, "N-Set", dicomcommand.NSetResponse, h) {
				return
			}

		case dicomcommand.NActionRequest:
			s.mu.RLock()
			h := s.onNActionRequest
			s.mu.RUnlock()
			if s.handleNRequest(ctx, pdu, dco, "N-Action", dicomcommand.NActionResponse, h) {
				return
			}

		case dicomcommand.NCreateRequest:
			s.mu.RLock()
			h := s.onNCreateRequest
			s.mu.RUnlock()
			if s.handleNRequest(ctx, pdu, dco, "N-Create", dicomcommand.NCreateResponse, h) {
				return
			}

		case dicomcommand.NDeleteRequest:
			s.mu.RLock()
			h := s.onNDeleteRequest
			s.mu.RUnlock()
			if s.handleNRequest(ctx, pdu, dco, "N-Delete", dicomcommand.NDeleteResponse, h) {
				return
			}

		case dicomcommand.CCancelRequest:
			// C-CANCEL-RQ applies to in-progress C-FIND/C-MOVE/C-GET operations.
			s.handleCancelCommand(pdu.Logger(), pdu.GetAAssociationRQ(), dco)

		case dicomcommand.CEchoRequest:
			s.mu.RLock()
			echoHandler := s.onCEchoRequest
			s.mu.RUnlock()

			if echoHandler != nil && !echoHandler(pdu.GetAAssociationRQ()) {
				pdu.Logger().Warn("C-Echo rejected by handler")
				return
			}
			if err := dimse.CEchoWriteRSP(pdu, dco); err != nil {
				pdu.Logger().Error("C-Echo failed to write response", "error", err)
				return
			}

		default:
			pdu.Logger().Warn("command not implemented, skipping", "command", command)
		}
	}
}
