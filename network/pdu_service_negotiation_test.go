package network

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
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

func TestSelectPreferredTransferSyntaxRejectsKnownButUnsupported(t *testing.T) {
	offered := []UIDItem{NewUIDItem(transfersyntax.ExplicitVRBigEndian.UID, 0x40)}

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

func TestInterogateAAssociateRQ_PresentationContextRejectReason_AbstractSyntaxNotSupported(t *testing.T) {
	pdu := NewPDUService().(*pduService)
	pdu.SetOnAssociationRequest(func(_ AssociationRequest) bool { return true })

	pc := NewPresentationContext()
	pc.SetPresentationContextID(1)
	pc.SetAbstractSyntax("1.2.3.4.5.6.7.8.9")
	pc.AddTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)
	pdu.AssocRQ.AddPresContexts(pc)

	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&bytes.Buffer{}))
	if err := pdu.interogateAAssociateRQ(rw); err != nil {
		t.Fatalf("interogateAAssociateRQ() err = %v, want nil", err)
	}

	pcs := pdu.AssocAC.GetPresContextAccepts()
	if len(pcs) != 1 {
		t.Fatalf("presentation contexts in AC = %d, want 1", len(pcs))
	}
	if got := pcs[0].GetResult(); got != 3 {
		t.Fatalf("presentation context result = %d, want 3 (abstract syntax not supported)", got)
	}
}

func TestInterogateAAssociateRQ_PresentationContextRejectReason_TransferSyntaxNotSupported(t *testing.T) {
	pdu := NewPDUService().(*pduService)
	pdu.SetOnAssociationRequest(func(_ AssociationRequest) bool { return true })

	pc := NewPresentationContext()
	pc.SetPresentationContextID(1)
	pc.SetAbstractSyntax(sopclass.CTImageStorage.UID)
	pc.AddTransferSyntax(transfersyntax.ExplicitVRBigEndian.UID)
	pdu.AssocRQ.AddPresContexts(pc)

	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&bytes.Buffer{}))
	if err := pdu.interogateAAssociateRQ(rw); err != nil {
		t.Fatalf("interogateAAssociateRQ() err = %v, want nil", err)
	}

	pcs := pdu.AssocAC.GetPresContextAccepts()
	if len(pcs) != 1 {
		t.Fatalf("presentation contexts in AC = %d, want 1", len(pcs))
	}
	if got := pcs[0].GetResult(); got != 4 {
		t.Fatalf("presentation context result = %d, want 4 (transfer syntaxes not supported)", got)
	}
}

func TestNormalizeClientTLSConfig_EnforcesTLS12(t *testing.T) {
	if got := normalizeClientTLSConfig(nil).MinVersion; got != tls.VersionTLS12 {
		t.Fatalf("normalizeClientTLSConfig(nil).MinVersion = %d, want %d", got, tls.VersionTLS12)
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS10}
	if got := normalizeClientTLSConfig(cfg).MinVersion; got != tls.VersionTLS12 {
		t.Fatalf("normalizeClientTLSConfig(v1.0).MinVersion = %d, want %d", got, tls.VersionTLS12)
	}
}

func TestInterogateAAssociateRQ_MixedPresentationContextAcceptance(t *testing.T) {
	pdu := NewPDUService().(*pduService)
	pdu.SetOnAssociationRequest(func(_ AssociationRequest) bool { return true })

	acceptedPC := NewPresentationContext()
	acceptedPC.SetPresentationContextID(1)
	acceptedPC.SetAbstractSyntax(sopclass.CTImageStorage.UID)
	acceptedPC.AddTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)

	rejectedPC := NewPresentationContext()
	rejectedPC.SetPresentationContextID(3)
	rejectedPC.SetAbstractSyntax("1.2.3.4.5.6.7.8.9")
	rejectedPC.AddTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)

	pdu.AssocRQ.AddPresContexts(acceptedPC)
	pdu.AssocRQ.AddPresContexts(rejectedPC)

	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&bytes.Buffer{}))
	if err := pdu.interogateAAssociateRQ(rw); err != nil {
		t.Fatalf("interogateAAssociateRQ() err = %v, want nil", err)
	}

	pcs := pdu.AssocAC.GetPresContextAccepts()
	if len(pcs) != 2 {
		t.Fatalf("presentation contexts in AC = %d, want 2", len(pcs))
	}

	results := map[byte]byte{}
	for _, pc := range pcs {
		results[pc.GetPresentationContextID()] = pc.GetResult()
	}

	if got := results[1]; got != 0 {
		t.Fatalf("accepted PC result = %d, want 0", got)
	}
	if got := results[3]; got != 3 {
		t.Fatalf("rejected PC result = %d, want 3", got)
	}

	if len(pdu.AcceptedPresentationContexts) != 1 {
		t.Fatalf("AcceptedPresentationContexts = %d, want 1", len(pdu.AcceptedPresentationContexts))
	}
}
