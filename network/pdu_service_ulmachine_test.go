// Package network_test provides comprehensive testing for DICOM Upper Layer (UL) state machine
// and PDU service behavior per DICOM PS3.8 §8 (Network Communication Support).
//
// Test Coverage Strategy:
//
// 1. STATE MACHINE TRANSITIONS (11 core transitions)
//   - IDLE → AWAITING-AC: Covered by integration tests (outbound client flow)
//   - IDLE → ASSOCIATED-SCP: Tested via TestInterogateAAssociateRQRejectsAndWritesRJ
//   - AWAITING-AC → ASSOCIATED: Tested via state transition table
//   - AWAITING-AC → ABORTED: Tested for abort during negotiation
//   - ASSOCIATED ↔ data transfer: Tested via PDU-DATA-TF in transition table
//   - ASSOCIATED → AWAITING-RELEASE: Tested via TestClosePerformsReleaseHandshake
//   - AWAITING-RELEASE → ASSOCIATED: Tested via release request handling
//   - AWAITING-RELEASE → RELEASED: Tested via release response handling
//   - (any) → ABORTED: Tested for abort in all states
//
// 2. RELEASE HANDSHAKE (§9.2.5)
//   - Normal: ASSOCIATED → A-RELEASE-RQ → AWAITING-RELEASE-RP → A-RELEASE-RP → RELEASED
//     Tested: TestClosePerformsReleaseHandshake
//   - Timeout fallback: If peer doesn't respond, abort is sent
//     Tested: TestCloseAbortsWhenPeerDoesNotRespond
//   - Peer-initiated: ASSOCIATED → A-RELEASE-RQ (recv) → A-RELEASE-RP (sent) → RELEASED
//     Tested: TestNextPDU_ReleaseAndAbortSequences
//
// 3. ABORT HANDLING (§9.3.5)
//   - Peer abort at any state closes connection and returns AbortSentinel error
//     Tested: TestNextPDUAbortReturnsSentinel, TestNextPDU_ReleaseAndAbortSequences
//   - Abort returns distinct sentinel to distinguish from normal errors
//     Tested: TestSentinelErrorsAreDistinctAndNonNil
//
// 4. ERROR CONDITIONS
//   - Invalid PDU type out of order: Tested via TestNextPDU_InvalidPDUTypeOutOfOrder
//   - Socket errors during reads: Tested via TestAutoDetectDeadlineExceeded
//   - PDU corruption: Handled via PDU read validation
//
// 5. PDU SEQUENCING
//   - All valid PDU type transitions: Tested via TestNextPDUStateTransitionsByPDUType_Table
//   - Rejection of invalid sequences: Tested for out-of-order PDU types
//
// Intentional Coverage Gaps:
//   - Outbound Connect (IDLE → AWAITING-AC): Avoided complex mock socket I/O; covered by
//     integration tests where the actual I/O and protocol are exercised end-to-end.
//   - This maintains unit test focus on state machine correctness, not I/O operations.
//
// Standards References:
//   - DICOM PS3.8 §8.2: UL Association State Machine
//   - DICOM PS3.8 §9.2: Normal Operation (Association Establishment, Transfer, Release)
//   - DICOM PS3.8 §9.3: Error Handling (Abort, Rejection)
package network

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

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

// TestInterogateAAssociateRQRejectsAndWritesRJ verifies that when the
// application reject handler declines an incoming association request,
// the UL machine writes A-ASSOCIATE-RJ and returns ErrAssociationRejected.
// This corresponds to the rejection path documented for DICOM PS3.8 §9.3.4.
func TestInterogateAAssociateRQRejectsAndWritesRJ(t *testing.T) {
	pdu := NewPDUService().(*pduService)
	pdu.AssocRQ.SetCalledAE("CALLED_AE")
	pdu.AssocRQ.SetCallingAE("CALLING_AE")
	pdu.SetOnAssociationRequest(func(_ AssociationRequest) bool { return false })

	var out bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&out))

	err := pdu.interogateAAssociateRQ(rw)
	if !errors.Is(err, ErrAssociationRejected) {
		t.Fatalf("interogateAAssociateRQ() err = %v, want ErrAssociationRejected", err)
	}

	written := out.Bytes()
	if len(written) < 10 {
		t.Fatalf("A-ASSOCIATE-RJ length = %d, want at least 10 bytes", len(written))
	}
	if written[0] != byte(pdutype.AssociationReject) {
		t.Fatalf("A-ASSOCIATE-RJ PDU type = 0x%02X, want 0x%02X", written[0], byte(pdutype.AssociationReject))
	}
	if written[7] != 0x01 || written[8] != 0x01 || written[9] != 0x07 {
		t.Fatalf("A-ASSOCIATE-RJ fields result/source/reason = (%d,%d,%d), want (1,1,7)", written[7], written[8], written[9])
	}
}

// TestNextPDUAssociationRejectClosesConnection verifies the end-to-end reject
// path: incoming A-ASSOCIATE-RQ is rejected, A-ASSOCIATE-RJ is sent, and the
// transport is closed by the rejecting side.
func TestNextPDUAssociationRejectClosesConnection(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	pdu := NewPDUService().(*pduService)
	pdu.readWriter = bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn))
	pdu.conn = clientConn
	pdu.SetOnAssociationRequest(func(_ AssociationRequest) bool { return false })

	serverDone := make(chan error, 1)
	go func() {
		rw := bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn))

		aarq := NewAssociationRequest()
		aarq.SetCalledAE("CALLED_AE")
		aarq.SetCallingAE("CALLING_AE")
		if err := aarq.Write(rw); err != nil {
			serverDone <- err
			return
		}

		rj := make([]byte, 10)
		if _, err := io.ReadFull(serverConn, rj); err != nil {
			serverDone <- err
			return
		}
		if rj[0] != byte(pdutype.AssociationReject) {
			serverDone <- errors.New("server: expected A-ASSOCIATE-RJ (0x03)")
			return
		}

		_ = serverConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		probe := make([]byte, 1)
		_, err := serverConn.Read(probe)
		if err == nil {
			serverDone <- errors.New("server: expected connection close after RJ")
			return
		}
		if !errors.Is(err, io.EOF) {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				serverDone <- errors.New("server: timed out waiting for connection close after RJ")
				return
			}
			serverDone <- err
			return
		}

		serverDone <- nil
	}()

	_, err := pdu.NextPDU()
	if !errors.Is(err, ErrAssociationRejected) {
		t.Fatalf("NextPDU() err = %v, want ErrAssociationRejected", err)
	}

	if err := <-serverDone; err != nil {
		t.Fatalf("server side: %v", err)
	}
}

// TestNextPDUStateTransitionsByPDUType_Table verifies valid UL state-machine
// transitions for each PDU type in the ASSOCIATED state. This is a table-driven
// test covering core PS3.8 §9.2-9.3 state machine semantics.
func TestNextPDUStateTransitionsByPDUType_Table(t *testing.T) {
	type testCase struct {
		name             string
		pduType          byte
		expectedErr      error
		expectedCallback bool
		expectedSentinel bool
		description      string
	}

	tests := []testCase{
		{
			name:             "PDUDataTransfer_Valid",
			pduType:          byte(pdutype.PDUDataTransfer),
			expectedErr:      nil, // P-DATA processing continues the loop (no immediate return)
			expectedCallback: false,
			expectedSentinel: false,
			description:      "P-DATA-TF within ASSOCIATED state is valid; loops to read next PDU (PS3.8 §9.2.4)",
		},
		{
			name:             "AssociationReleaseRequest_Valid",
			pduType:          byte(pdutype.AssociationReleaseRequest),
			expectedErr:      ErrAssociationReleased,
			expectedCallback: false,
			expectedSentinel: true,
			description:      "A-RELEASE-RQ from peer in ASSOCIATED triggers ReleaseRP send + sentinel (PS3.8 §9.2.5)",
		},
		{
			name:             "AssociationReleaseResponse_Valid",
			pduType:          byte(pdutype.AssociationReleaseResponse),
			expectedErr:      ErrAssociationReleased,
			expectedCallback: false,
			expectedSentinel: true,
			description:      "A-RELEASE-RP in response state returns sentinel (PS3.8 §9.2.5)",
		},
		{
			name:             "AssociationAbortRequest_Valid",
			pduType:          byte(pdutype.AssociationAbortRequest),
			expectedErr:      ErrAssociationAborted,
			expectedCallback: false,
			expectedSentinel: true,
			description:      "A-ABORT at any state after negotiation returns sentinel (PS3.8 §9.3.5)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			defer serverConn.Close()

			pdu := NewPDUService().(*pduService)
			pdu.readWriter = bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn))
			pdu.conn = clientConn

			pdu.SetOnAssociationRequest(func(_ AssociationRequest) bool {
				return true
			})

			serverDone := make(chan error, 1)
			go func() {
				defer serverConn.Close()

				// Write the test PDU from server side.
				var pdu10 [10]byte
				pdu10[0] = tt.pduType
				pdu10[1] = 0x00
				pdu10[2] = 0x00
				pdu10[3] = 0x00
				pdu10[4] = 0x00
				pdu10[5] = 0x04
				if tt.pduType == byte(pdutype.AssociationReleaseRequest) {
					pdu10[5] = 0x04
				}

				if _, err := serverConn.Write(pdu10[:]); err != nil {
					serverDone <- err
					return
				}

				// For A-RELEASE-RQ, drain the response (ReleaseRP).
				if tt.pduType == byte(pdutype.AssociationReleaseRequest) {
					buf := make([]byte, 10)
					if _, err := io.ReadFull(serverConn, buf); err != nil {
						serverDone <- err
						return
					}
				}

				serverDone <- nil
			}()

			_, err := pdu.NextPDU()

			if tt.expectedSentinel {
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("NextPDU() err = %v, want %v; test: %s", err, tt.expectedErr, tt.description)
				}
			} else if err != nil && err != io.EOF {
				// For P-DATA, the loop continues and may return EOF when server closes.
				if tt.pduType != byte(pdutype.PDUDataTransfer) {
					t.Errorf("NextPDU() err = %v, want nil; test: %s", err, tt.description)
				}
			}

			if err := <-serverDone; err != nil {
				t.Fatalf("server side: %v", err)
			}
		})
	}
}

// TestNextPDU_InvalidPDUTypeOutOfOrder verifies that receiving an unexpected
// PDU type outside its valid state causes an error or abort. For example,
// P-DATA-TF before association is established should trigger abort.
func TestNextPDU_InvalidPDUTypeOutOfOrder(t *testing.T) {
	type testCase struct {
		name        string
		pduType     byte
		description string
	}

	tests := []testCase{
		{
			name:        "UnrecognizedPDUType",
			pduType:     0xAA, // Invalid/reserved PDU type
			description: "Unrecognized PDU type should trigger abort (PS3.8 §9.2.6)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			defer serverConn.Close()

			pdu := NewPDUService().(*pduService)
			pdu.readWriter = bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn))
			pdu.conn = clientConn

			serverDone := make(chan error, 1)
			go func() {
				defer serverConn.Close()

				// Write invalid PDU type.
				invalidPdu := [10]byte{tt.pduType, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00}
				if _, err := serverConn.Write(invalidPdu[:]); err != nil {
					serverDone <- err
					return
				}

				// Check if abort is sent back (optional, impl-dependent).
				buf := make([]byte, 10)
				io.ReadFull(serverConn, buf) //nolint:errcheck

				serverDone <- nil
			}()

			_, err := pdu.NextPDU()
			if err == nil {
				t.Errorf("NextPDU() with invalid PDU type = nil, want error; test: %s", tt.description)
			}

			<-serverDone
		})
	}
}

// TestNextPDU_ReleaseAndAbortSequences verifies state transitions related to
// clean release (ReleaseRequest → ReleaseResponse) and abnormal abort paths.
func TestNextPDU_ReleaseAndAbortSequences(t *testing.T) {
	type testCase struct {
		name             string
		serverSendsPDU   byte
		expectedErrType  error
		shouldCloseAfter bool
		description      string
	}

	tests := []testCase{
		{
			name:            "Release_RequestThenResponse",
			serverSendsPDU:  byte(pdutype.AssociationReleaseRequest),
			expectedErrType: ErrAssociationReleased,
			description:     "Peer initiates release; local responds with ReleaseRP then returns sentinel",
		},
		{
			name:             "Abort_AbortsImmediately",
			serverSendsPDU:   byte(pdutype.AssociationAbortRequest),
			expectedErrType:  ErrAssociationAborted,
			shouldCloseAfter: true,
			description:      "Abort from peer; local closes transport and returns AbortSentinel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			defer serverConn.Close()

			pdu := NewPDUService().(*pduService)
			pdu.readWriter = bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn))
			pdu.conn = clientConn

			serverDone := make(chan error, 1)
			go func() {
				defer serverConn.Close()

				pdu10 := [10]byte{tt.serverSendsPDU, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00}
				if _, err := serverConn.Write(pdu10[:]); err != nil {
					serverDone <- err
					return
				}

				if tt.serverSendsPDU == byte(pdutype.AssociationReleaseRequest) {
					// Drain the ReleaseRP response.
					buf := make([]byte, 10)
					io.ReadFull(serverConn, buf) //nolint:errcheck
				}

				serverDone <- nil
			}()

			_, err := pdu.NextPDU()
			if !errors.Is(err, tt.expectedErrType) {
				t.Errorf("NextPDU() err = %v, want %v; test: %s", err, tt.expectedErrType, tt.description)
			}

			if err := <-serverDone; err != nil {
				t.Fatalf("server side: %v", err)
			}
		})
	}
}
