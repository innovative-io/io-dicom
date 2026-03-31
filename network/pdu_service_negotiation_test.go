package network

import (
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

func TestSelectPreferredTransferSyntaxPrefersLittleEndian(t *testing.T) {
	offered := []UIDItem{
		NewUIDItem(transfersyntax.JPEG2000Lossless.UID, 0x40),
		NewUIDItem(transfersyntax.ExplicitVRLittleEndian.UID, 0x40),
		NewUIDItem(transfersyntax.ImplicitVRLittleEndian.UID, 0x40),
	}

	got, ok := selectPreferredTransferSyntax(offered)
	if !ok {
		t.Fatal("selectPreferredTransferSyntax() returned ok=false")
	}
	if got != transfersyntax.ExplicitVRLittleEndian.UID {
		t.Fatalf("selectPreferredTransferSyntax() = %q, want %q", got, transfersyntax.ExplicitVRLittleEndian.UID)
	}
}

func TestSelectPreferredTransferSyntaxRejectsUnknownOnly(t *testing.T) {
	offered := []UIDItem{NewUIDItem("1.2.3.4.5.6.7.8.9", 0x40)}

	got, ok := selectPreferredTransferSyntax(offered)
	if ok {
		t.Fatalf("selectPreferredTransferSyntax() ok=true with UID %q, want ok=false", got)
	}
}

func TestInterogateAAssociateACAcceptsBigEndianOnlyContext(t *testing.T) {
	pdu := NewPDUService().(*pduService)
	accepted := NewPresentationContextAccept()
	accepted.SetResult(0)
	accepted.SetPresentationContextID(7)
	accepted.SetTransferSyntax(transfersyntax.ExplicitVRBigEndian.UID)
	pdu.AssocAC.AddPresContextAccept(accepted)

	if ok := pdu.interogateAAssociateAC(); !ok {
		t.Fatal("interogateAAssociateAC() = false, want true")
	}
	if got := pdu.GetPresentationContextID(); got != 7 {
		t.Fatalf("GetPresentationContextID() = %d, want %d", got, 7)
	}
}
