package services

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

// TestStoreSession_NegotiatedContextIDsAreUnique is the regression test for the
// presentation context ID exhaustion bug. BeginStoreSession proposes a full
// association's worth of storage classes; every accepted context must carry a
// distinct odd ID. Previously the IDs came from a package-global counter that
// wrapped past 255, so context #129 onward collided with #1 onward and a
// C-STORE could be resolved against the wrong abstract syntax.
func TestStoreSession_NegotiatedContextIDsAreUnique(t *testing.T) {
	const port = 11991
	_, scp := StartSCP(t, port)
	scp.OnAssociationRequest(func(network.AssociationRequest) bool { return true })
	scp.OnCStoreRequest(func(context.Context, network.AssociationRequest, media.DICOMObject) uint16 {
		return dicomstatus.Success
	})

	scu := newProbeSCU(port)
	sess, err := scu.BeginStoreSession(context.Background())
	if err != nil {
		t.Fatalf("BeginStoreSession() error = %v", err)
	}
	defer sess.Close() //nolint:errcheck

	accepted := scu.GetNegotiatedContexts()
	if len(accepted) == 0 {
		t.Fatal("no negotiated presentation contexts")
	}
	if len(accepted) > network.MaxPresentationContexts {
		t.Errorf("negotiated %d contexts, exceeds the limit of %d", len(accepted), network.MaxPresentationContexts)
	}
	seen := make(map[byte]bool, len(accepted))
	for _, pc := range accepted {
		id := pc.GetPresentationContextID()
		if id%2 == 0 {
			t.Errorf("presentation context ID %d is even, PS3.8 §9.3.2.2 requires odd", id)
		}
		if seen[id] {
			t.Errorf("duplicate presentation context ID %d in a single association", id)
		}
		seen[id] = true
	}
}

// TestBeginStoreSessionFor_ProposesOnlyRequested verifies the explicit-selection
// path negotiates just the named classes, and that instances still transfer.
func TestBeginStoreSessionFor_ProposesOnlyRequested(t *testing.T) {
	const port = 11992
	_, scp := StartSCP(t, port)
	scp.OnAssociationRequest(func(network.AssociationRequest) bool { return true })
	received := 0
	scp.OnCStoreRequest(func(context.Context, network.AssociationRequest, media.DICOMObject) uint16 {
		received++
		return dicomstatus.Success
	})

	const samplePath = "../testdata/test2.dcm"
	if _, err := os.Stat(samplePath); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}
	obj, err := media.NewDCMObjFromFile(samplePath)
	if err != nil {
		t.Fatalf("load sample: %v", err)
	}

	scu := newProbeSCU(port)
	// Duplicates and empties are filtered, so this should negotiate 2 contexts.
	sess, err := scu.BeginStoreSessionFor(context.Background(),
		obj.GetString(tagSOPClassUID()),
		sopclass.MRImageStorage.UID,
		obj.GetString(tagSOPClassUID()),
		"",
	)
	if err != nil {
		t.Fatalf("BeginStoreSessionFor() error = %v", err)
	}
	defer sess.Close() //nolint:errcheck

	if got := len(scu.GetNegotiatedContexts()); got != 2 {
		t.Errorf("negotiated %d contexts, want 2 (duplicate and empty UIDs should be filtered)", got)
	}

	for i := 0; i < 3; i++ {
		if err := sess.Store(context.Background(), obj); err != nil {
			t.Fatalf("Store #%d error = %v", i+1, err)
		}
	}
	if received != 3 {
		t.Errorf("SCP received %d instances over the session, want 3", received)
	}
}

func TestBeginStoreSessionFor_NoUIDs(t *testing.T) {
	scu := newProbeSCU(11993)
	if _, err := scu.BeginStoreSessionFor(context.Background()); err == nil {
		t.Fatal("BeginStoreSessionFor() with no UIDs should error")
	}
}

// TestOpenAssociation_RejectsTooManyContexts verifies an over-limit proposal is
// refused outright instead of emitting an A-ASSOCIATE-RQ with duplicate IDs.
func TestOpenAssociation_RejectsTooManyContexts(t *testing.T) {
	scu := newProbeSCU(11994)
	uids := make([]string, 0, network.MaxPresentationContexts+1)
	for _, c := range sopclass.GetStorageSOPClasses() {
		uids = append(uids, c.UID)
		if len(uids) > network.MaxPresentationContexts {
			break
		}
	}
	_, err := scu.BeginStoreSessionFor(context.Background(), uids...)
	if err == nil {
		t.Fatal("expected an error when proposing more than the context limit")
	}
	if !strings.Contains(err.Error(), "presentation contexts") {
		t.Errorf("error = %v, want it to explain the presentation context limit", err)
	}
}

func newProbeSCU(port int) SCU {
	return NewSCU(&network.Destination{
		Name: "probe", HostName: "127.0.0.1", Port: port,
		CalledAE: "DEST", CallingAE: "PROBE",
	})
}

// TestConcurrentSessions_ContextIDsDoNotInterleave covers the second half of the
// ID bug: IDs used to come from a package-global counter that each association
// reset before use, so two associations opened concurrently interleaved their
// allocations and could land duplicate IDs inside a single A-ASSOCIATE-RQ.
// Index-derived IDs are per-association by construction.
func TestConcurrentSessions_ContextIDsDoNotInterleave(t *testing.T) {
	const port = 11995
	_, scp := StartSCP(t, port)
	scp.OnAssociationRequest(func(network.AssociationRequest) bool { return true })
	scp.OnCStoreRequest(func(context.Context, network.AssociationRequest, media.DICOMObject) uint16 {
		return dicomstatus.Success
	})

	classes := sopclass.GetStorageSOPClasses()
	uids := make([]string, 0, 64)
	for i := 0; i < 64 && i < len(classes); i++ {
		uids = append(uids, classes[i].UID)
	}

	const workers = 6
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scu := newProbeSCU(port)
			sess, err := scu.BeginStoreSessionFor(context.Background(), uids...)
			if err != nil {
				errs <- fmt.Errorf("BeginStoreSessionFor: %w", err)
				return
			}
			defer sess.Close() //nolint:errcheck

			seen := map[byte]bool{}
			for _, pc := range scu.GetNegotiatedContexts() {
				id := pc.GetPresentationContextID()
				if seen[id] {
					errs <- fmt.Errorf("duplicate presentation context ID %d within one association", id)
					return
				}
				seen[id] = true
			}
			errs <- nil
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

// TestGetSCU_StaysWithinContextLimit covers the other caller that proposed more
// contexts than an association can carry: GetSCU offered the C-GET information
// model plus every storage SOP class (209 in total), so the sub-operation
// contexts it negotiated carried colliding IDs.
func TestGetSCU_StaysWithinContextLimit(t *testing.T) {
	scu := NewSCU(&network.Destination{
		Name: "probe", HostName: "127.0.0.1", Port: 11996,
		CalledAE: "DEST", CallingAE: "PROBE",
	})
	// No SCP is listening, so this fails at connect — but it must get that far.
	// Before the fix it failed earlier, at the context limit check.
	_, err := scu.GetSCU(context.Background(), media.NewEmptyDCMObj())
	if err != nil && strings.Contains(err.Error(), "presentation contexts") {
		t.Fatalf("GetSCU proposed more contexts than an association allows: %v", err)
	}
}
