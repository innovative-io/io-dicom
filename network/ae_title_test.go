package network

import "testing"

func TestAssociationRequestAETitlePreservesInternalSpaces(t *testing.T) {
	aarq := NewAssociationRequest().(*associationRequest)

	aarq.SetCallingAE("NODE A")
	aarq.SetCalledAE("PACS B")

	if got := aarq.GetCallingAE(); got != "NODE A" {
		t.Fatalf("GetCallingAE() = %q, want %q", got, "NODE A")
	}
	if got := aarq.GetCalledAE(); got != "PACS B" {
		t.Fatalf("GetCalledAE() = %q, want %q", got, "PACS B")
	}
}

func TestAssociationAcceptAETitlePreservesInternalSpaces(t *testing.T) {
	aaac := newAssociationAccept()

	aaac.SetCallingAE("NODE A")
	aaac.SetCalledAE("PACS B")

	if got := aaac.GetCallingAE(); got != "NODE A" {
		t.Fatalf("GetCallingAE() = %q, want %q", got, "NODE A")
	}
	if got := aaac.GetCalledAE(); got != "PACS B" {
		t.Fatalf("GetCalledAE() = %q, want %q", got, "PACS B")
	}
}

func TestFormatAETitleTruncatesToSixteenBytes(t *testing.T) {
	aarq := NewAssociationRequest().(*associationRequest)

	aarq.SetCallingAE("ABCDEFGHIJKLMNOPQ")

	if got := aarq.GetCallingAE(); got != "ABCDEFGHIJKLMNOP" {
		t.Fatalf("GetCallingAE() = %q, want %q", got, "ABCDEFGHIJKLMNOP")
	}
}
