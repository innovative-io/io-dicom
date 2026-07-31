package network

import (
	"bufio"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// TestStalledMidPDUReadIsBounded is the mid-transfer dribble regression.
//
// The services-layer idle timeout bounds a peer that goes quiet *between*
// commands, but once a command was accepted the dataset reads that followed had
// no deadline at all. A peer could announce a PDU and then simply stop feeding
// bytes, held only by the total-message ceiling — which bounds bytes, not time.
//
// The deadline is per read, so this must fire while the payload is incomplete.
func TestStalledMidPDUReadIsBounded(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	pdu := NewPDUService(WithReadProgressTimeout(150 * time.Millisecond)).(*pduService)
	pdu.SetNetConn(server)
	pdu.SetConn(bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)))
	pdu.negotiated = true

	// Announce a 4 KiB P-DATA PDU, send only a sliver of it, then stall.
	go func() {
		var hdr [10]byte
		hdr[0] = 0x04
		binary.BigEndian.PutUint32(hdr[2:6], 4096)
		_, _ = client.Write(hdr[:])
		_, _ = client.Write(make([]byte, 16))
		select {} // never send the rest
	}()

	done := make(chan error, 1)
	go func() {
		_, _, err := pdu.readIncomingPDU()
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a peer that stopped mid-PDU was allowed to complete a read")
		}
		t.Logf("stalled mid-PDU read released with: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("a peer that announced a PDU and stopped sending held the read " +
			"open indefinitely — the per-read progress deadline did not fire")
	}
}

// TestProgressDeadlineDoesNotOverrideShorterPoll pins the interaction that made
// this awkward to bound in the first place: the SCP polls for an interleaved
// C-CANCEL with a very short deadline, and a per-read progress timeout applied
// naively would stretch that poll to its own much longer value.
func TestProgressDeadlineDoesNotOverrideShorterPoll(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	pdu := NewPDUService(WithReadProgressTimeout(30 * time.Second)).(*pduService)
	pdu.SetNetConn(server)
	pdu.SetConn(bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)))
	pdu.negotiated = true

	// The caller's deadline is far sooner than the progress timeout.
	if err := pdu.SetReadDeadline(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	start := time.Now()
	_, _, err := pdu.readIncomingPDU() // peer sends nothing at all
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected the short poll deadline to expire")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("the caller's 20ms deadline was overridden by the 30s progress "+
			"timeout: the read blocked for %v — C-CANCEL polling would stall", elapsed)
	}
}

// TestProgressTimeoutCanBeDisabled keeps the bound opt-out for deployments that
// manage their own deadlines.
func TestProgressTimeoutCanBeDisabled(t *testing.T) {
	if got := NewPDUService().(*pduService).progressTimeout; got != defaultReadProgressTimeout {
		t.Fatalf("default progress timeout is %v, want %v", got, defaultReadProgressTimeout)
	}
	if got := NewPDUService(WithReadProgressTimeout(0)).(*pduService).progressTimeout; got != 0 {
		t.Fatalf("WithReadProgressTimeout(0) left %v, want the bound disabled", got)
	}
}
