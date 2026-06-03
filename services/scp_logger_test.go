package services

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/innovative-io/io-dicom/network"
)

// syncBuffer is a goroutine-safe io.Writer so the SCP's logging goroutine and
// the test goroutine can share one buffer without racing.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestSCP_WithLogger_RoutesAndCorrelates verifies that WithLogger directs the
// SCP's structured output to the injected logger (rather than slog.Default())
// and that each association's lines carry the assoc_id and remote_addr
// correlation attributes.
func TestSCP_WithLogger_RoutesAndCorrelates(t *testing.T) {
	const port = 1102
	var sink syncBuffer
	logger := slog.New(slog.NewJSONHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug}))

	_, testSCP := StartSCP(t, port, WithLogger(logger))
	testSCP.OnAssociationRequest(func(network.AssociationRequest) bool { return true })
	testSCP.OnCEchoRequest(func(network.AssociationRequest) bool { return true })

	dest := &network.Destination{
		Name:      "LoggerTest",
		CalledAE:  "SCP",
		CallingAE: "SCU",
		HostName:  "localhost",
		Port:      port,
	}
	if err := NewSCU(dest).EchoSCU(context.Background()); err != nil {
		t.Fatalf("EchoSCU: %v", err)
	}

	out := sink.String()
	if out == "" {
		t.Fatal("injected logger received no output; SCP is still logging to slog.Default()")
	}
	if !strings.Contains(out, "assoc_id") {
		t.Errorf("SCP log lines missing assoc_id correlation attribute:\n%s", out)
	}
	if !strings.Contains(out, "remote_addr") {
		t.Errorf("SCP log lines missing remote_addr correlation attribute:\n%s", out)
	}
	// The negotiated calling AE should appear once the association is established.
	if !strings.Contains(out, "SCU") {
		t.Errorf("SCP log lines missing negotiated AE title:\n%s", out)
	}
	// The A-ASSOCIATE-AC writer is built fresh during negotiation; its logger
	// must still be propagated so the accept line reaches the injected logger
	// rather than slog.Default().
	if !strings.Contains(out, "A-ASSOCIATE-AC") {
		t.Errorf("A-ASSOCIATE-AC lines leaked to the default logger instead of the injected one:\n%s", out)
	}
}
