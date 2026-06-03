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

func TestAssociationRequest_GetImplementationVersionName(t *testing.T) {
	aarq := NewAssociationRequest()

	// Before setting, should return empty string.
	if got := aarq.GetImplementationVersionName(); got != "" {
		t.Fatalf("GetImplementationVersionName() before set = %q, want %q", got, "")
	}

	aarq.SetImplementationVersionName("MY-IMPL-1.0")
	if got := aarq.GetImplementationVersionName(); got != "MY-IMPL-1.0" {
		t.Fatalf("GetImplementationVersionName() = %q, want %q", got, "MY-IMPL-1.0")
	}
}
