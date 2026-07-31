package network

import (
	"strings"
	"testing"

	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

// TestNextPDUNeverReturnsNilNil is the core contract behind a remote crash.
//
// NextPDU returns (nil, nil) exactly once, to signal "association established,
// no data object". Every other path must return a non-nil error, because the
// callers that read a command's dataset dereference the returned object
// immediately (services/scp_association.go does ddo.GetString(...) with no nil
// check, and passes ddo to user-supplied store handlers). A nil dereference in
// the per-connection goroutine has no recover() above it, so it terminates the
// whole process — every in-flight association and the listener.
//
// The attack was: negotiate, send a command flagged dataset-present, then send
// an A-ASSOCIATE-RQ instead of the dataset. Without a state guard that RQ was
// renegotiated and NextPDU returned (nil, nil) straight into the dataset read.
func TestNextPDUNeverReturnsNilNil(t *testing.T) {
	cases := []struct {
		name     string
		itemType byte
	}{
		{"second A-ASSOCIATE-RQ on an established association", pdutype.AssociationRequest},
		{"A-ASSOCIATE-AC out of sequence", pdutype.AssociationAccept},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A 10-byte PDU carrying no body: enough for the dispatch to run.
			pdu := pduWithReader(pduHeader(tc.itemType, 4))
			// Simulate an association that has already been negotiated, which is
			// the state the attack relies on.
			pdu.negotiated = true

			obj, err := pdu.NextPDU()
			if err == nil {
				t.Fatalf("expected an error, got obj=%v err=nil — a caller reading a "+
					"dataset would dereference this and crash the process", obj)
			}
			if obj != nil {
				t.Fatalf("expected a nil object alongside the error, got %v", obj)
			}
			if !strings.Contains(err.Error(), "unexpected A-ASSOCIATE") {
				t.Fatalf("expected a protocol-violation error, got %v", err)
			}
		})
	}
}

// TestFirstAssociationRequestStillNegotiates guards the fix from over-reaching:
// the first A-ASSOCIATE-RQ must still be processed normally. A truncated RQ is
// used here, so negotiation fails with a parse error rather than (nil, nil) —
// the point is that it is NOT rejected by the state guard.
func TestFirstAssociationRequestStillNegotiates(t *testing.T) {
	pdu := pduWithReader(pduHeader(pdutype.AssociationRequest, 4))
	if pdu.negotiated {
		t.Fatal("a fresh pduService must not start out negotiated")
	}

	_, err := pdu.NextPDU()
	if err != nil && strings.Contains(err.Error(), "unexpected A-ASSOCIATE-RQ") {
		t.Fatalf("the first A-ASSOCIATE-RQ must not be rejected by the state guard: %v", err)
	}
}
