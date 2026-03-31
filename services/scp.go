package services

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

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

// SCP - Interface to scp
type SCP interface {
	// Start begins accepting connections and blocks until the listener is
	// closed — either by Stop() or by ctx being cancelled.
	Start(ctx context.Context) error
	// Stop closes the listener and waits for all in-flight connections to finish.
	Stop() error
	OnAssociationRequest(f func(request network.AssociationRequest) bool)
	OnCFindRequest(f func(request network.AssociationRequest, findLevel string, data media.DICOMObject) ([]media.DICOMObject, uint16))
	OnCMoveRequest(f func(request network.AssociationRequest, moveLevel string, data media.DICOMObject) uint16)
	OnCStoreRequest(f func(request network.AssociationRequest, data media.DICOMObject) uint16)
	// OnCEchoRequest registers an optional handler invoked for every C-ECHO
	// request. Return false to reject the echo (no response is sent).
	OnCEchoRequest(f func(request network.AssociationRequest) bool)
}

type scp struct {
	Port     int
	listener net.Listener
	wg       sync.WaitGroup
	mu       sync.RWMutex

	onAssociationRequest func(request network.AssociationRequest) bool
	onCFindRequest       func(request network.AssociationRequest, findLevel string, data media.DICOMObject) ([]media.DICOMObject, uint16)
	onCMoveRequest       func(request network.AssociationRequest, moveLevel string, data media.DICOMObject) uint16
	onCStoreRequest      func(request network.AssociationRequest, data media.DICOMObject) uint16
	onCEchoRequest       func(request network.AssociationRequest) bool
}

// NewSCP - Creates an interface to scp
func NewSCP(port int) SCP {
	media.InitDict()

	return &scp{
		Port: port,
	}
}

func (s *scp) Start(ctx context.Context) error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.Port))
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

	for {
		dco, err := pdu.NextPDU()
		if err != nil {
			if !errors.Is(err, network.ErrAssociationReleased) && !errors.Is(err, network.ErrAssociationAborted) {
				slog.Error("scp: network error", "ERROR", err)
			}
			return
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
				if err := dimse.CStoreWriteRSP(pdu, dco, dicomstatus.FailureUnableToProcess); err != nil {
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

			s.mu.RLock()
			findHandler := s.onCFindRequest
			s.mu.RUnlock()

			if findHandler == nil {
				slog.Warn("scp: OnCFindRequest not registered")
				if err := dimse.CFindWriteRSP(pdu, dco, media.NewEmptyDCMObj(), dicomstatus.FailureUnableToProcess); err != nil {
					slog.Error("scp: C-Find failed to write error response", "ERROR", err.Error())
					return
				}
				continue
			}

			queryLevel := ddo.GetString(tags.QueryRetrieveLevel)
			results, status := findHandler(pdu.GetAAssociationRQ(), queryLevel, ddo)
			// Send each match as Pending per DICOM PS3.7 C.4.1.4.
			for _, result := range results {
				if err := dimse.CFindWriteRSP(pdu, dco, result, dicomstatus.Pending); err != nil {
					slog.Error("scp: C-Find failed to write pending response", "ERROR", err.Error())
					return
				}
			}
			// Final status-only response (CommandDataSetType = 0x0101).
			if err := dimse.CFindWriteRSP(pdu, dco, media.NewEmptyDCMObj(), status); err != nil {
				slog.Error("scp: C-Find failed to write final response", "ERROR", err.Error())
				return
			}

		case dicomcommand.CMoveRequest:
			ddo, err := pdu.NextPDU()
			if err != nil {
				slog.Error("scp: C-Move failed to read request", "ERROR", err.Error())
				return
			}

			s.mu.RLock()
			moveHandler := s.onCMoveRequest
			s.mu.RUnlock()

			if moveHandler == nil {
				slog.Warn("scp: OnCMoveRequest not registered")
				if err := dimse.CMoveWriteRSP(pdu, dco, dicomstatus.FailureUnableToProcess, 0x00); err != nil {
					slog.Error("scp: C-Move failed to write error response", "ERROR", err.Error())
					return
				}
				continue
			}

			moveLevel := ddo.GetString(tags.QueryRetrieveLevel)
			status := moveHandler(pdu.GetAAssociationRQ(), moveLevel, ddo)
			if err := dimse.CMoveWriteRSP(pdu, dco, status, 0x00); err != nil {
				slog.Error("scp: C-Move failed to write response", "ERROR", err.Error())
				return
			}

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

func (s *scp) OnCFindRequest(f func(request network.AssociationRequest, findLevel string, data media.DICOMObject) ([]media.DICOMObject, uint16)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCFindRequest = f
}

func (s *scp) OnCMoveRequest(f func(request network.AssociationRequest, moveLevel string, data media.DICOMObject) uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCMoveRequest = f
}

func (s *scp) OnCStoreRequest(f func(request network.AssociationRequest, data media.DICOMObject) uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCStoreRequest = f
}

func (s *scp) OnCEchoRequest(f func(request network.AssociationRequest) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCEchoRequest = f
}
