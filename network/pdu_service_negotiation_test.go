package network

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
)

// TestSelectPreferredTransferSyntaxAcceptsFirstOffered verifies that the SCP
// accepts the SCU's first (preferred) transfer syntax, not the SCP's own
// preference.  This is required for storage SCPs so that the SCU can send data
// in its native encoding without transcoding.
func TestSelectPreferredTransferSyntaxAcceptsFirstOffered(t *testing.T) {
	offered := []UIDItem{
		NewUIDItem(transfersyntax.JPEG2000Lossless.UID, 0x40),
		NewUIDItem(transfersyntax.ExplicitVRLittleEndian.UID, 0x40),
		NewUIDItem(transfersyntax.ImplicitVRLittleEndian.UID, 0x40),
	}

	got, ok := selectPreferredTransferSyntax(offered)
	if !ok {
		t.Fatal("selectPreferredTransferSyntax() returned ok=false")
	}
	// Must accept the SCU's first offered transfer syntax (JPEG2000Lossless here).
	if got != transfersyntax.JPEG2000Lossless.UID {
		t.Fatalf("selectPreferredTransferSyntax() = %q, want %q", got, transfersyntax.JPEG2000Lossless.UID)
	}
}

// TestSelectPreferredTransferSyntaxExplicitVRLittleEndianFirst verifies that
// ExplicitVRLittleEndian is accepted when it is the SCU's first offered syntax.
func TestSelectPreferredTransferSyntaxExplicitVRLittleEndianFirst(t *testing.T) {
	offered := []UIDItem{
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

func TestSelectPresentationContextIDForAbstractSyntax(t *testing.T) {
	studyRoot := NewPresentationContextAccept()
	studyRoot.SetResult(0)
	studyRoot.SetPresentationContextID(1)
	studyRoot.SetAbstractSyntax(sopclass.StudyRootQueryRetrieveInformationModelFind.UID)
	studyRoot.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)

	patientRoot := NewPresentationContextAccept()
	patientRoot.SetResult(0)
	patientRoot.SetPresentationContextID(3)
	patientRoot.SetAbstractSyntax(sopclass.PatientRootQueryRetrieveInformationModelFind.UID)
	patientRoot.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)

	got, ok := selectPresentationContextIDForAbstractSyntax([]PresentationContextAccept{studyRoot, patientRoot}, sopclass.PatientRootQueryRetrieveInformationModelFind.UID)
	if !ok {
		t.Fatal("selectPresentationContextIDForAbstractSyntax() ok=false, want true")
	}
	if got != 3 {
		t.Fatalf("selectPresentationContextIDForAbstractSyntax() = %d, want 3", got)
	}
}

func TestWriteSelectsPresentationContextByAbstractSyntax(t *testing.T) {
	pdu := NewPDUService().(*pduService)
	pdu.Pdata.Buffer = nil
	pdu.AssocAC = NewAssociationAccept()
	pdu.AssocAC.GetUserInformation().GetMaxSubLength().SetMaximumLength(maxPduLength)
	pdu.readWriter = bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&bytes.Buffer{}))

	studyRoot := NewPresentationContextAccept()
	studyRoot.SetResult(0)
	studyRoot.SetPresentationContextID(1)
	studyRoot.SetAbstractSyntax(sopclass.StudyRootQueryRetrieveInformationModelFind.UID)
	studyRoot.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)

	patientRoot := NewPresentationContextAccept()
	patientRoot.SetResult(0)
	patientRoot.SetPresentationContextID(3)
	patientRoot.SetAbstractSyntax(sopclass.PatientRootQueryRetrieveInformationModelFind.UID)
	patientRoot.SetTransferSyntax(transfersyntax.ImplicitVRLittleEndian.UID)

	pdu.AcceptedPresentationContexts = []PresentationContextAccept{studyRoot, patientRoot}
	pdu.Pdata.PresentationContextID = 1

	cmd := media.NewEmptyDCMObj()
	cmd.WriteString(tags.AffectedSOPClassUID, sopclass.PatientRootQueryRetrieveInformationModelFind.UID)
	cmd.WriteUint16(tags.CommandField, 0x0020)
	cmd.WriteUint16(tags.MessageID, 1)

	if err := pdu.Write(cmd, byte(1)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if got := pdu.GetPresentationContextID(); got != 3 {
		t.Fatalf("GetPresentationContextID() = %d, want 3", got)
	}
	if cmd.GetTransferSyntax() == nil || cmd.GetTransferSyntax().UID != transfersyntax.ImplicitVRLittleEndian.UID {
		t.Fatalf("command transfer syntax = %v, want Implicit VR Little Endian", cmd.GetTransferSyntax())
	}
}
