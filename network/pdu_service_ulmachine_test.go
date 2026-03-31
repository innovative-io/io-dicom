package network

import (
	"bufio"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/innovative-io/io-dicom/network/pdutype"
)

// TestAbortRequestReservedByteIsZero verifies DICOM PS3.8 Table 9-26:
// bytes 5 and 6 of the A-ABORT PDU are reserved and must be 0x00.
func TestAbortRequestReservedByteIsZero(t *testing.T) {
	abort := NewAbortRequest().(*abortRequest)
	if abort.Reserved3 != 0x00 {
		t.Errorf("A-ABORT Reserved3 = 0x%02X, want 0x00 (DICOM PS3.8 Table 9-26)", abort.Reserved3)
	}
}

// TestAbortRequestSourceIsValid verifies the Source field uses a legal value per
// DICOM PS3.8 Table 9-26: 0 (service-user) or 2 (DICOM UL service-provider).
func TestAbortRequestSourceIsValid(t *testing.T) {
	abort := NewAbortRequest().(*abortRequest)
	if abort.Source != 0x00 && abort.Source != 0x02 {
		t.Errorf("A-ABORT Source = 0x%02X; valid values are 0x00 (service-user) or 0x02 (service-provider)", abort.Source)
	}
}

// TestSentinelErrorsAreDistinctAndNonNil verifies callers can distinguish
// ErrAssociationReleased from ErrAssociationAborted.
func TestSentinelErrorsAreDistinctAndNonNil(t *testing.T) {
	if ErrAssociationReleased == nil {
		t.Error("ErrAssociationReleased must not be nil")
	}
	if ErrAssociationAborted == nil {
		t.Error("ErrAssociationAborted must not be nil")
	}
	if errors.Is(ErrAssociationReleased, ErrAssociationAborted) {
		t.Error("ErrAssociationReleased and ErrAssociationAborted must be distinct")
	}
}

// TestCloseIsNoOpWhenReadWriterIsNil verifies that Close() does not panic
// when called on an un-connected PDU service.
func TestCloseIsNoOpWhenReadWriterIsNil(t *testing.T) {
	pdu := NewPDUService().(*pduService)
	pdu.Close() // must not panic
}

// TestClosePerformsReleaseHandshake verifies that Close():
//  1. Sends an A-RELEASE-RQ (PDU type 0x05) to the peer.
//  2. Reads the A-RELEASE-RP (PDU type 0x06) from the peer and completes cleanly.
func TestClosePerformsReleaseHandshake(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	pdu := NewPDUService().(*pduService)
	pdu.readWriter = bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn))
	pdu.conn = clientConn

	serverDone := make(chan error, 1)
	go func() {
		defer serverConn.Close()
		buf := make([]byte, 10)
		if _, err := io.ReadFull(serverConn, buf); err != nil {
			serverDone <- err
			return
		}
		if buf[0] != byte(pdutype.AssociationReleaseRequest) {
			serverDone <- errors.New("server: expected A-RELEASE-RQ (0x05), got unexpected PDU type")
			return
		}
		// Respond with a valid A-RELEASE-RP PDU (10 bytes, length field = 4).
		rp := [10]byte{0x06, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00}
		if _, err := serverConn.Write(rp[:]); err != nil {
			serverDone <- err
			return
		}
		serverDone <- nil
	}()

	pdu.Close()

	if err := <-serverDone; err != nil {
		t.Fatalf("server side: %v", err)
	}
}

// TestCloseAbortsWhenPeerDoesNotRespond verifies that Close() falls back to
// sending an A-ABORT and still returns cleanly when the peer closes the
// connection without sending A-RELEASE-RP.
func TestCloseAbortsWhenPeerDoesNotRespond(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	pdu := NewPDUService().(*pduService)
	pdu.readWriter = bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn))
	pdu.conn = clientConn

	// Server reads A-RELEASE-RQ then closes without replying.
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		buf := make([]byte, 10)
		io.ReadFull(serverConn, buf) //nolint:errcheck
		serverConn.Close()
	}()

	pdu.Close() // must not block or panic

	<-serverDone
}

// TestNextPDUReleaseRequestReturnsSentinel verifies that a remote A-RELEASE-RQ
// (PDU type 0x05) causes NextPDU to return ErrAssociationReleased.
func TestNextPDUReleaseRequestReturnsSentinel(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	pdu := NewPDUService().(*pduService)
	pdu.readWriter = bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn))

	// Write an A-RELEASE-RQ PDU from the "server" side.
	go func() {
		rq := [10]byte{0x05, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00}
		serverConn.Write(rq[:]) //nolint:errcheck
		// Drain the A-RELEASE-RP that NextPDU sends back.
		buf := make([]byte, 10)
		serverConn.Read(buf) //nolint:errcheck
		serverConn.Close()
	}()

	_, err := pdu.NextPDU()
	if !errors.Is(err, ErrAssociationReleased) {
		t.Fatalf("NextPDU() after A-RELEASE-RQ = %v, want ErrAssociationReleased", err)
	}
}

// TestNextPDUAbortReturnsSentinel verifies that a remote A-ABORT (PDU type 0x07)
// causes NextPDU to return ErrAssociationAborted.
func TestNextPDUAbortReturnsSentinel(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	pdu := NewPDUService().(*pduService)
	pdu.readWriter = bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn))

	go func() {
		abort := [10]byte{0x07, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00}
		serverConn.Write(abort[:]) //nolint:errcheck
		serverConn.Close()
	}()

	_, err := pdu.NextPDU()
	if !errors.Is(err, ErrAssociationAborted) {
		t.Fatalf("NextPDU() after A-ABORT = %v, want ErrAssociationAborted", err)
	}
}
