package services

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

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

func (s *scp) handleCancelCommand(assocRQ network.AssociationRequest, dco media.DICOMObject) uint16 {
	messageID := dco.GetUint16(tags.MessageIDBeingRespondedTo)
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

func (s *scp) handleConnection(ctx context.Context, conn net.Conn) {
	rw := bufio.NewReadWriter(
		bufio.NewReaderSize(conn, s.bufSize),
		bufio.NewWriterSize(conn, s.bufSize),
	)

	pdu := s.newPDUService()
	pdu.SetConn(rw)
	pdu.SetNetConn(conn)

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
		command := dco.GetUint16(tags.CommandField)
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

			status := storeHandler(ctx, pdu.GetAAssociationRQ(), ddo)
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
			if !isValidCFindQueryRetrieveLevel(ddo.GetString(tags.QueryRetrieveLevel)) {
				if err := dimse.CFindWriteRSP(pdu, dco, media.NewEmptyDCMObj(), dicomstatus.FailureIdentifierDoesNotMatchSOPClass); err != nil {
					slog.Error("scp: C-Find failed to write invalid-identifier response", "ERROR", err.Error())
					return
				}
				continue
			}
			if s.consumeCanceled(dco.GetUint16(tags.MessageID)) {
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
			if s.consumeCanceled(dco.GetUint16(tags.MessageID)) {
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
			if s.consumeCanceled(dco.GetUint16(tags.MessageID)) {
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
			if dco.GetUint16(tags.CommandDataSetType) != dicomcommand.DataSetNone {
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
				status, resp = h(ctx, pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NEventReportResponse, status, resp); err != nil {
				slog.Error("scp: N-Event-Report failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.NGetRequest:
			var nData media.DICOMObject
			if dco.GetUint16(tags.CommandDataSetType) != dicomcommand.DataSetNone {
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
				status, resp = h(ctx, pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NGetResponse, status, resp); err != nil {
				slog.Error("scp: N-Get failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.NSetRequest:
			var nData media.DICOMObject
			if dco.GetUint16(tags.CommandDataSetType) != dicomcommand.DataSetNone {
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
				status, resp = h(ctx, pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NSetResponse, status, resp); err != nil {
				slog.Error("scp: N-Set failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.NActionRequest:
			var nData media.DICOMObject
			if dco.GetUint16(tags.CommandDataSetType) != dicomcommand.DataSetNone {
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
				status, resp = h(ctx, pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NActionResponse, status, resp); err != nil {
				slog.Error("scp: N-Action failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.NCreateRequest:
			var nData media.DICOMObject
			if dco.GetUint16(tags.CommandDataSetType) != dicomcommand.DataSetNone {
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
				status, resp = h(ctx, pdu.GetAAssociationRQ(), dco, nData)
			}
			if err := dimse.NWriteRSP(pdu, dco, dicomcommand.NCreateResponse, status, resp); err != nil {
				slog.Error("scp: N-Create failed to write response", "ERROR", err.Error())
				return
			}

		case dicomcommand.NDeleteRequest:
			var nData media.DICOMObject
			if dco.GetUint16(tags.CommandDataSetType) != dicomcommand.DataSetNone {
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
				status, resp = h(ctx, pdu.GetAAssociationRQ(), dco, nData)
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
				return
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
