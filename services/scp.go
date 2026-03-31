package services

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

// SCP - Interface to scp
type SCP interface {
	Start() error
	Stop() error
	OnAssociationRequest(f func(request network.AssociationRequest) bool)
	OnCFindRequest(f func(request network.AssociationRequest, findLevel string, data media.DICOMObject) ([]media.DICOMObject, uint16))
	OnCMoveRequest(f func(request network.AssociationRequest, moveLevel string, data media.DICOMObject) uint16)
	OnCStoreRequest(f func(request network.AssociationRequest, data media.DICOMObject) uint16)
	handleConnection(conn net.Conn)
}

type scp struct {
	Port                 int
	listener             net.Listener
	onAssociationRequest func(request network.AssociationRequest) bool
	onCFindRequest       func(request network.AssociationRequest, findLevel string, data media.DICOMObject) ([]media.DICOMObject, uint16)
	onCMoveRequest       func(request network.AssociationRequest, moveLevel string, data media.DICOMObject) uint16
	onCStoreRequest      func(request network.AssociationRequest, data media.DICOMObject) uint16
}

// NewSCP - Creates an interface to scu
func NewSCP(port int) SCP {
	media.InitDict()

	return &scp{
		Port: port,
	}
}

func (s *scp) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.Port))
	if err != nil {
		return err
	}

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Listener was closed by Stop() — exit cleanly.
			return nil
		}
		slog.Info("handleConnection, new connection", "ADDRESS", conn.RemoteAddr())
		go s.handleConnection(conn)
	}
}

func (s *scp) Stop() error {
	return s.listener.Close()
}

func (s *scp) handleConnection(conn net.Conn) {
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	pdu := network.NewPDUService()
	pdu.SetConn(rw)

	if s.onAssociationRequest != nil {
		pdu.SetOnAssociationRequest(s.onAssociationRequest)
	}

	defer conn.Close()

	var err error
	var dco media.DICOMObject
	for err == nil {
		dco, err = pdu.NextPDU()
		if dco == nil {
			continue
		}
		command := dco.GetUShort(tags.CommandField)
		switch command {
		case dicomcommand.CStoreRequest:
			ddo, err := dimse.CStoreReadRQ(pdu, dco)
			if err != nil {
				slog.Error("handleConnection, C-Store failed to read request", "ERROR", err.Error())
				return
			}

			if s.onCStoreRequest == nil {
				slog.Error("handleConnection, OnCStoreRequest not implemented")
				if err := dimse.CStoreWriteRSP(pdu, dco, dicomstatus.FailureUnableToProcess); err != nil {
					slog.Error("handleConnection, C-Store failed to write error response", "ERROR", err.Error())
				}
				return
			}

			status := s.onCStoreRequest(pdu.GetAAssociationRQ(), ddo)
			if err := dimse.CStoreWriteRSP(pdu, dco, status); err != nil {
				slog.Error("handleConnection, C-Store failed to write response", "ERROR", err.Error())
				return
			}
		case dicomcommand.CFindRequest:
			ddo, err := dimse.CFindReadRQ(pdu)
			if err != nil {
				slog.Error("handleConnection, C-Find failed to read request!")
				return
			}
			queryLevel := ddo.GetString(tags.QueryRetrieveLevel)

			status := dicomstatus.Success

			if s.onCFindRequest == nil {
				slog.Error("handleConnection, OnCFindRequest not implemented")
				if err := dimse.CFindWriteRSP(pdu, dco, dco, dicomstatus.FailureUnableToProcess); err != nil {
					slog.Error("handleConnection, C-Find failed to write error response", "ERROR", err.Error())
				}
				return
			}

			results, status := s.onCFindRequest(pdu.GetAAssociationRQ(), queryLevel, ddo)
			if len(results) > 0 {
				for index, result := range results {
					if index == len(results)-1 {
						break
					}
					if err := dimse.CFindWriteRSP(pdu, dco, result, dicomstatus.Pending); err != nil {
						slog.Error("handleConnection, C-Find failed to write response", "ERROR", err.Error())
						return
					}
				}

				if err := dimse.CFindWriteRSP(pdu, dco, results[len(results)-1], status); err != nil {
					slog.Error("handleConnection, C-Find failed to write response", "ERROR", err.Error())
					return
				}
			} else {
				if err := dimse.CFindWriteRSP(pdu, dco, dco, status); err != nil {
					slog.Error("handleConnection, C-Find failed to write response", "ERROR", err.Error())
					return
				}
			}
		case dicomcommand.CMoveRequest:
			ddo, err := dimse.CMoveReadRQ(pdu)
			if err != nil {
				slog.Error("handleConnection, C-Move failed to read request!")
				return
			}
			moveLevel := ddo.GetString(tags.QueryRetrieveLevel)

			if s.onCMoveRequest == nil {
				slog.Error("handleConnection, OnCMoveRequest not implemented")
				if err := dimse.CMoveWriteRSP(pdu, dco, dicomstatus.FailureUnableToProcess, 0x00); err != nil {
					slog.Error("handleConnection, C-Move failed to write error response", "ERROR", err.Error())
				}
				return
			}

			status := s.onCMoveRequest(pdu.GetAAssociationRQ(), moveLevel, ddo)

			if err := dimse.CMoveWriteRSP(pdu, dco, status, 0x00); err != nil {
				slog.Error("handleConnection, C-Move failed to write response", "ERROR", err.Error())
				return
			}
		case dicomcommand.CEchoRequest:
			if dimse.CEchoReadRQ(dco) {
				if err := dimse.CEchoWriteRSP(pdu, dco); err != nil {
					slog.Error("handleConnection, C-Echo failed to write response!")
					return
				}
			}
		default:
			slog.Error("handleConnection, service not implemented", "COMMAND", command)
			return
		}
	}
}

func (s *scp) OnAssociationRequest(f func(request network.AssociationRequest) bool) {
	s.onAssociationRequest = f
}

func (s *scp) OnCFindRequest(f func(request network.AssociationRequest, findLevel string, data media.DICOMObject) ([]media.DICOMObject, uint16)) {
	s.onCFindRequest = f
}

func (s *scp) OnCMoveRequest(f func(request network.AssociationRequest, moveLevel string, data media.DICOMObject) uint16) {
	s.onCMoveRequest = f
}

func (s *scp) OnCStoreRequest(f func(request network.AssociationRequest, data media.DICOMObject) uint16) {
	s.onCStoreRequest = f
}
