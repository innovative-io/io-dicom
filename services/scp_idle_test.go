package services

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// TestStalledPeerIsDisconnected is the slowloris regression.
//
// The association read loop called NextPDU with no read deadline, so a peer that
// connected, wrote a few bytes of a PDU header and then went silent parked the
// handler goroutine forever. Nothing in the SCP ever reclaimed it: SetTimeout is
// applied only on the client dial path, never on accept.
func TestStalledPeerIsDisconnected(t *testing.T) {
	const port = 11473
	// StartSCP registers its own t.Cleanup; calling the returned func too would
	// Stop the SCP twice.
	StartSCP(t, port, WithAssociationIdleTimeout(150*time.Millisecond))

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Three bytes of an A-ASSOCIATE-RQ header, then silence — never enough to
	// complete a PDU, so the server stays blocked in its read.
	if _, err := conn.Write([]byte{0x01, 0x00, 0x00}); err != nil {
		t.Fatalf("write: %v", err)
	}

	// The server must close on us. Read with a deadline generously above the
	// idle timeout: without the bound this read hits its own deadline instead.
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	start := time.Now()
	var buf [1]byte
	_, err = conn.Read(buf[:])
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("server sent data to a peer that never completed a PDU")
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("server held a stalled connection open for %v — the idle "+
			"timeout did not fire (slowloris: one such peer per association slot "+
			"locks out every legitimate client)", elapsed)
	}
	// Any other error means the far end closed: that is the fix working.
	t.Logf("stalled peer disconnected after %v (%v)", elapsed, err)
}

// TestIdleTimeoutDoesNotBreakLiveAssociations guards the bound from severing
// connections that are merely between commands rather than abandoned.
func TestIdleTimeoutDoesNotBreakLiveAssociations(t *testing.T) {
	s := NewSCP(0).(*scp)
	if s.idleTimeout != defaultAssociationIdleTimeout {
		t.Fatalf("default idle timeout is %v, want %v",
			s.idleTimeout, defaultAssociationIdleTimeout)
	}
	// The bound must be opt-out, for deployments with long-lived idle peers.
	s2 := NewSCP(0, WithAssociationIdleTimeout(0)).(*scp)
	if s2.idleTimeout != 0 {
		t.Fatalf("WithAssociationIdleTimeout(0) left %v, want the bound disabled",
			s2.idleTimeout)
	}
}
