package services

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/innovative-io/io-dicom/network"
)

// TestSCP_WithMaxAssociations verifies that the accept loop never runs more
// association handlers concurrently than the configured limit. Each handler
// holds its slot for a short, fixed window while a peak-concurrency counter is
// observed; with a limit of 1 the peak must never exceed 1 even though several
// SCUs connect at once.
func TestSCP_WithMaxAssociations(t *testing.T) {
	const port = 1101
	_, testSCP := StartSCP(t, port, WithMaxAssociations(1))

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})

	var current atomic.Int32
	var peak atomic.Int32
	testSCP.OnCEchoRequest(func(request network.AssociationRequest) bool {
		n := current.Add(1)
		// Record the high-water mark of concurrent handlers.
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		// Hold the association slot long enough that any unbounded concurrency
		// would overlap here.
		time.Sleep(80 * time.Millisecond)
		current.Add(-1)
		return true
	})

	const clients = 4
	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dest := &network.Destination{
				Name:      "MaxAssocTest",
				CalledAE:  "SCP",
				CallingAE: "SCU",
				HostName:  "localhost",
				Port:      port,
			}
			scu := NewSCU(dest)
			if err := scu.EchoSCU(context.Background()); err != nil {
				t.Errorf("EchoSCU: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := peak.Load(); got > 1 {
		t.Fatalf("peak concurrent associations = %d, want <= 1", got)
	}
}
