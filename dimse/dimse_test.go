package dimse_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/network/priority"
)

// ── mockPDU ───────────────────────────────────────────────────────────────────

type mockPDU struct {
	written  []media.DICOMObject
	nextPDUs []media.DICOMObject
	readIdx  int
	assocRQ  network.AssociationRequest
	pcid     byte
}

func newMockPDU(sopClassUID string) *mockPDU {
	pc := network.NewPresentationContext()
	pc.SetPresentationContextID(1)
	pc.SetAbstractSyntax(sopClassUID)
	pc.AddTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)
	rq := network.NewAssociationRequest()
	rq.AddPresContexts(pc)
	return &mockPDU{assocRQ: rq, pcid: 1}
}

func (m *mockPDU) Write(dco media.DICOMObject, _ byte) error {
	m.written = append(m.written, dco)
	return nil
}

func (m *mockPDU) NextPDU() (media.DICOMObject, error) {
	if m.readIdx >= len(m.nextPDUs) {
		return nil, errors.New("no more PDUs")
	}
	obj := m.nextPDUs[m.readIdx]
	m.readIdx++
	return obj, nil
}

func (m *mockPDU) GetAAssociationRQ() network.AssociationRequest { return m.assocRQ }
func (m *mockPDU) GetPresentationContextID() byte                { return m.pcid }
func (m *mockPDU) GetTransferSyntax(_ byte) *transfersyntax.TransferSyntax {
	return transfersyntax.ExplicitVRLittleEndian
}
func (m *mockPDU) SetTimeout(_ int)                                                {}
func (m *mockPDU) Connect(_ context.Context, _, _ string) error                    { return nil }
func (m *mockPDU) ConnectTLS(_ context.Context, _, _ string, _ *tls.Config) error  { return nil }
func (m *mockPDU) Close() error                                                    { return nil }
func (m *mockPDU) GetCalledAE() string                                             { return "CALLED" }
func (m *mockPDU) GetCallingAE() string                                            { return "CALLING" }
func (m *mockPDU) GetRemoteAddress() string                                        { return "127.0.0.1:104" }
func (m *mockPDU) SetCalledAE(_ string)                                            {}
func (m *mockPDU) SetCallingAE(_ string)                                           {}
func (m *mockPDU) SetConn(_ *bufio.ReadWriter)                                     {}
func (m *mockPDU) SetNetConn(_ net.Conn)                                           {}
func (m *mockPDU) AddPresContexts(_ network.PresentationContext)                   {}
func (m *mockPDU) SetOnAssociationRequest(_ func(network.AssociationRequest) bool) {}
func (m *mockPDU) SetOnRawPDU(_ func(network.RawPDUEvent))                         {}
func (m *mockPDU) GetAcceptedPresentationContexts() []network.PresentationContextAccept {
	return nil
}

const ctUID = "1.2.840.10008.5.1.4.1.1.2"

func echoRQObj() media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.Write(tags.CommandField, dicomcommand.CEchoRequest)
	obj.Write(tags.MessageID, 1)
	obj.Write(tags.CommandDataSetType, 0x0101)
	return obj
}

func findRQObj(queryLevel string) media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.Write(tags.CommandField, dicomcommand.CFindRequest)
	obj.Write(tags.MessageID, 1)
	obj.Write(tags.CommandDataSetType, 0x0102)
	obj.Write(tags.QueryRetrieveLevel, queryLevel)
	return obj
}

func dicomDataObj(sopClassUID, sopInstanceUID string) media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.Write(tags.SOPClassUID, sopClassUID)
	obj.Write(tags.SOPInstanceUID, sopInstanceUID)
	return obj
}

// ── C-ECHO ────────────────────────────────────────────────────────────────────

func TestCEchoReadRQ_True(t *testing.T) {
	if !dimse.CEchoReadRQ(echoRQObj()) {
		t.Error("CEchoReadRQ: want true for CEchoRequest")
	}
}

func TestCEchoReadRQ_FalseForNonEcho(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.Write(tags.CommandField, dicomcommand.CStoreRequest)
	if dimse.CEchoReadRQ(obj) {
		t.Error("CEchoReadRQ: want false for CStoreRequest")
	}
}

func TestCEchoWriteRQ_WritesCommand(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CEchoWriteRQ(m); err != nil {
		t.Fatalf("CEchoWriteRQ: %v", err)
	}
	if len(m.written) != 1 {
		t.Fatalf("CEchoWriteRQ: want 1 write, got %d", len(m.written))
	}
	w := m.written[0]
	if cf := w.GetUint16(tags.CommandField); cf != dicomcommand.CEchoRequest {
		t.Errorf("CEchoWriteRQ: CommandField %04X want %04X", cf, dicomcommand.CEchoRequest)
	}
	if cdt := w.GetUint16(tags.CommandDataSetType); cdt != 0x0101 {
		t.Errorf("CEchoWriteRQ: CommandDataSetType %04X want 0101", cdt)
	}
}

func TestCEchoWriteRQ_ErrorMissingSOPClass(t *testing.T) {
	m := newMockPDU("")
	if err := dimse.CEchoWriteRQ(m); err == nil {
		t.Error("CEchoWriteRQ: want error when AffectedSOPClassUID is missing")
	}
}

func TestCEchoReadRSP_Success(t *testing.T) {
	rsp := media.NewEmptyDCMObj()
	rsp.Write(tags.CommandField, dicomcommand.CEchoResponse)
	rsp.Write(tags.CommandDataSetType, 0x0101)
	rsp.Write(tags.MessageIDBeingRespondedTo, 1)
	rsp.Write(tags.Status, dicomstatus.Success)
	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{rsp}
	if err := dimse.CEchoReadRSP(m); err != nil {
		t.Errorf("CEchoReadRSP: %v", err)
	}
}

func TestCEchoReadRSP_ErrorInvalidCommandDataSetType(t *testing.T) {
	rsp := media.NewEmptyDCMObj()
	rsp.Write(tags.CommandField, dicomcommand.CEchoResponse)
	rsp.Write(tags.CommandDataSetType, 0x0102)
	rsp.Write(tags.MessageIDBeingRespondedTo, 1)
	rsp.Write(tags.Status, dicomstatus.Success)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{rsp}
	if err := dimse.CEchoReadRSP(m); err == nil {
		t.Error("CEchoReadRSP: want error for invalid CommandDataSetType")
	}
}

func TestCEchoReadRSP_ErrorMissingMessageIDBeingRespondedTo(t *testing.T) {
	rsp := media.NewEmptyDCMObj()
	rsp.Write(tags.CommandField, dicomcommand.CEchoResponse)
	rsp.Write(tags.CommandDataSetType, 0x0101)
	rsp.Write(tags.MessageIDBeingRespondedTo, 0)
	rsp.Write(tags.Status, dicomstatus.Success)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{rsp}
	if err := dimse.CEchoReadRSP(m); err == nil {
		t.Error("CEchoReadRSP: want error for missing MessageIDBeingRespondedTo")
	}
}

func TestCEchoReadRSP_ErrorOnNoPDU(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CEchoReadRSP(m); err == nil {
		t.Error("CEchoReadRSP: want error when NextPDU fails")
	}
}

func TestCEchoWriteRSP_WritesResponse(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := echoRQObj()
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	if err := dimse.CEchoWriteRSP(m, rq); err != nil {
		t.Fatalf("CEchoWriteRSP: %v", err)
	}
	if len(m.written) == 0 {
		t.Fatal("CEchoWriteRSP: nothing written")
	}
	rsp := m.written[0]
	if cf := rsp.GetUint16(tags.CommandField); cf != dicomcommand.CEchoResponse {
		t.Errorf("CEchoWriteRSP: CommandField %04X want CEchoResponse", cf)
	}
	if st := rsp.GetUint16(tags.Status); st != dicomstatus.Success {
		t.Errorf("CEchoWriteRSP: Status %04X want Success", st)
	}
}

func TestCEchoWriteRSP_ErrorWhenNoSOPClass(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CEchoWriteRSP(m, media.NewEmptyDCMObj()); err == nil {
		t.Error("CEchoWriteRSP: want error when AffectedSOPClassUID absent")
	}
}

func TestCEchoWriteRSP_ErrorWhenNoMessageID(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	if err := dimse.CEchoWriteRSP(m, rq); err == nil {
		t.Error("CEchoWriteRSP: want error when MessageID is missing")
	}
}

func TestCEchoWriteRSP_ErrorWhenNilCommand(t *testing.T) {
	m := newMockPDU(ctUID)
	var rq media.DICOMObject
	if err := dimse.CEchoWriteRSP(m, rq); err == nil {
		t.Error("CEchoWriteRSP: want error when commandObj is nil")
	}
}

// ── C-FIND ────────────────────────────────────────────────────────────────────

func TestCFindWriteRQ_WritesCommandAndData(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CFindWriteRQ(m, findRQObj("STUDY"), priority.Medium); err != nil {
		t.Fatalf("CFindWriteRQ: %v", err)
	}
	if len(m.written) != 2 {
		t.Fatalf("CFindWriteRQ: want 2 writes, got %d", len(m.written))
	}
	if cf := m.written[0].GetUint16(tags.CommandField); cf != dicomcommand.CFindRequest {
		t.Errorf("CFindWriteRQ: CommandField %04X want CFindRequest", cf)
	}
}

func TestCFindWriteRQ_ErrorMissingSOPClass(t *testing.T) {
	m := newMockPDU("")
	if err := dimse.CFindWriteRQ(m, findRQObj("STUDY"), priority.Medium); err == nil {
		t.Error("CFindWriteRQ: want error when AffectedSOPClassUID is missing")
	}
}

func TestCCancelWriteRQ_WritesCommand(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CCancelWriteRQ(m, 41); err != nil {
		t.Fatalf("CCancelWriteRQ: %v", err)
	}
	if len(m.written) != 1 {
		t.Fatalf("CCancelWriteRQ: want 1 write, got %d", len(m.written))
	}
	cmd := m.written[0]
	if cf := cmd.GetUint16(tags.CommandField); cf != dicomcommand.CCancelRequest {
		t.Errorf("CCancelWriteRQ: CommandField %04X want CCancelRequest", cf)
	}
	if ds := cmd.GetUint16(tags.CommandDataSetType); ds != 0x0101 {
		t.Errorf("CCancelWriteRQ: CommandDataSetType %04X want 0101", ds)
	}
	if mid := cmd.GetUint16(tags.MessageIDBeingRespondedTo); mid != 41 {
		t.Errorf("CCancelWriteRQ: MessageIDBeingRespondedTo %d want 41", mid)
	}
}

func TestCCancelWriteRQ_ErrorMissingTargetMessageID(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CCancelWriteRQ(m, 0); err == nil {
		t.Error("CCancelWriteRQ: want error when MessageIDBeingRespondedTo is missing")
	}
}

func TestCFindReadRSP_PendingThenFinal(t *testing.T) {
	pending := media.NewEmptyDCMObj()
	pending.Write(tags.CommandField, dicomcommand.CFindResponse)
	pending.Write(tags.Status, dicomstatus.Pending)
	pending.Write(tags.CommandDataSetType, 0x0102)

	dataset := media.NewEmptyDCMObj()
	dataset.Write(tags.PatientID, "P001")

	final := media.NewEmptyDCMObj()
	final.Write(tags.CommandField, dicomcommand.CFindResponse)
	final.Write(tags.Status, dicomstatus.Success)
	final.Write(tags.CommandDataSetType, 0x0101)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{pending, dataset, final}

	_, st1, err1 := dimse.CFindReadRSP(m)
	if err1 != nil {
		t.Fatalf("CFindReadRSP (pending): %v", err1)
	}
	if st1 != dicomstatus.Pending {
		t.Errorf("CFindReadRSP: status %04X want Pending", st1)
	}

	_, st2, err2 := dimse.CFindReadRSP(m)
	if err2 != nil {
		t.Fatalf("CFindReadRSP (final): %v", err2)
	}
	if st2 != dicomstatus.Success {
		t.Errorf("CFindReadRSP: final status %04X want Success", st2)
	}
}

func TestCFindReadRSP_ErrorOnNoPDU(t *testing.T) {
	m := newMockPDU(ctUID)
	if _, _, err := dimse.CFindReadRSP(m); err == nil {
		t.Error("CFindReadRSP: want error when NextPDU fails")
	}
}

func TestCFindReadRSP_ErrorPendingWithoutDataset(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CFindResponse)
	cmd.Write(tags.Status, dicomstatus.Pending)
	cmd.Write(tags.CommandDataSetType, 0x0101)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	if _, _, err := dimse.CFindReadRSP(m); err == nil {
		t.Fatal("CFindReadRSP: want error for pending response without dataset")
	}
}

func TestCFindReadRSP_ErrorFinalWithDataset(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CFindResponse)
	cmd.Write(tags.Status, dicomstatus.Success)
	cmd.Write(tags.CommandDataSetType, 0x0102)

	ds := media.NewEmptyDCMObj()
	ds.Write(tags.PatientID, "P001")

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd, ds}

	if _, _, err := dimse.CFindReadRSP(m); err == nil {
		t.Fatal("CFindReadRSP: want error for final response with dataset")
	}
}

func TestCFindReadRSP_ErrorInvalidCommandDataSetType(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CFindResponse)
	cmd.Write(tags.Status, dicomstatus.Success)
	cmd.Write(tags.CommandDataSetType, 0x9999)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	if _, _, err := dimse.CFindReadRSP(m); err == nil {
		t.Fatal("CFindReadRSP: want error for invalid CommandDataSetType")
	}
}

func TestCFindReadRSP_ErrorInvalidStatusCode(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CFindResponse)
	cmd.Write(tags.Status, 0x2222)
	cmd.Write(tags.CommandDataSetType, 0x0101)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	if _, _, err := dimse.CFindReadRSP(m); err == nil {
		t.Fatal("CFindReadRSP: want error for invalid status code")
	}
}

func TestCFindWriteRSP_Pending(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := findRQObj("STUDY")
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	result := media.NewEmptyDCMObj()
	result.Write(tags.PatientID, "P001")
	if err := dimse.CFindWriteRSP(m, rq, result, dicomstatus.Pending); err != nil {
		t.Fatalf("CFindWriteRSP (pending): %v", err)
	}
	if len(m.written) != 2 {
		t.Fatalf("CFindWriteRSP: want 2 writes (cmd+data), got %d", len(m.written))
	}
}

func TestCFindWriteRSP_Final(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := findRQObj("STUDY")
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	if err := dimse.CFindWriteRSP(m, rq, media.NewEmptyDCMObj(), dicomstatus.Success); err != nil {
		t.Fatalf("CFindWriteRSP (final): %v", err)
	}
}

func TestCFindWriteRSP_ErrorNoSOPClass(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CFindWriteRSP(m, media.NewEmptyDCMObj(), media.NewEmptyDCMObj(), dicomstatus.Success); err == nil {
		t.Error("CFindWriteRSP: want error when AffectedSOPClassUID absent")
	}
}

func TestCFindWriteRSP_ErrorNoMessageID(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	if err := dimse.CFindWriteRSP(m, rq, media.NewEmptyDCMObj(), dicomstatus.Success); err == nil {
		t.Error("CFindWriteRSP: want error when MessageID is missing")
	}
}

func TestCFindWriteRSP_ErrorNilRequest(t *testing.T) {
	m := newMockPDU(ctUID)
	var rq media.DICOMObject
	if err := dimse.CFindWriteRSP(m, rq, media.NewEmptyDCMObj(), dicomstatus.Success); err == nil {
		t.Error("CFindWriteRSP: want error when requestCommandObj is nil")
	}
}

func TestCFindWriteRSP_ErrorPendingWithoutDataset(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := findRQObj("STUDY")
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)

	if err := dimse.CFindWriteRSP(m, rq, media.NewEmptyDCMObj(), dicomstatus.Pending); err == nil {
		t.Error("CFindWriteRSP: want error for pending response without dataset")
	}
}

func TestCFindWriteRSP_ErrorFinalWithDataset(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := findRQObj("STUDY")
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)

	resp := media.NewEmptyDCMObj()
	resp.Write(tags.PatientID, "P001")

	if err := dimse.CFindWriteRSP(m, rq, resp, dicomstatus.Success); err == nil {
		t.Error("CFindWriteRSP: want error for final response with dataset")
	}
}

func TestCFindWriteRSP_ErrorInvalidStatusCode(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := findRQObj("STUDY")
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)

	if err := dimse.CFindWriteRSP(m, rq, media.NewEmptyDCMObj(), 0x2222); err == nil {
		t.Error("CFindWriteRSP: want error for invalid status code")
	}
}

// ── C-STORE ───────────────────────────────────────────────────────────────────

func TestCStoreWriteRQ_WritesCommandAndData(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CStoreWriteRQ(m, dicomDataObj(ctUID, "1.2.3.4.5.6"), priority.Medium); err != nil {
		t.Fatalf("CStoreWriteRQ: %v", err)
	}
	if len(m.written) != 2 {
		t.Fatalf("CStoreWriteRQ: want 2 writes, got %d", len(m.written))
	}
	if cf := m.written[0].GetUint16(tags.CommandField); cf != dicomcommand.CStoreRequest {
		t.Errorf("CStoreWriteRQ: CommandField %04X want CStoreRequest", cf)
	}
}

func TestCStoreWriteRQ_CommandGroupLengthNonZeroForEvenUID(t *testing.T) {
	evenUID := "1.2.840.10008.5.1.4.1"
	m := newMockPDU(ctUID)
	if err := dimse.CStoreWriteRQ(m, dicomDataObj(ctUID, evenUID), priority.Medium); err != nil {
		t.Fatalf("CStoreWriteRQ (even UID): %v", err)
	}
	if m.written[0].GetUint32(tags.CommandGroupLength) == 0 {
		t.Error("CStoreWriteRQ: CommandGroupLength must not be zero for even-length UID")
	}
}

func TestCStoreWriteRQ_AffectedSOPInstanceUIDAlwaysPresent(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CStoreWriteRQ(m, dicomDataObj(ctUID, "1.2.3"), priority.Medium); err != nil {
		t.Fatalf("CStoreWriteRQ: %v", err)
	}
	if uid := m.written[0].GetString(tags.AffectedSOPInstanceUID); uid == "" {
		t.Error("CStoreWriteRQ: AffectedSOPInstanceUID must always be present (PS3.7 §C.3.1)")
	}
}

func TestCStoreWriteRQ_ErrorMissingSOPInstanceUID(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CStoreWriteRQ(m, dicomDataObj(ctUID, ""), priority.Medium); err == nil {
		t.Error("CStoreWriteRQ: want error when SOPInstanceUID is missing")
	}
}

func TestCStoreWriteRQ_ErrorMissingSOPClassUID(t *testing.T) {
	// Neither the data object nor the PDU negotiation provides a SOP class UID.
	m := newMockPDU("")
	if err := dimse.CStoreWriteRQ(m, dicomDataObj("", "1.2.3"), priority.Medium); err == nil {
		t.Error("CStoreWriteRQ: want error when AffectedSOPClassUID is missing")
	}
}

func TestCStoreReadRSP_Success(t *testing.T) {
	rsp := media.NewEmptyDCMObj()
	rsp.Write(tags.CommandField, dicomcommand.CStoreResponse)
	rsp.Write(tags.CommandDataSetType, 0x0101)
	rsp.Write(tags.MessageIDBeingRespondedTo, 1)
	rsp.Write(tags.Status, dicomstatus.Success)
	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{rsp}
	st, err := dimse.CStoreReadRSP(m)
	if err != nil {
		t.Fatalf("CStoreReadRSP: %v", err)
	}
	if st != dicomstatus.Success {
		t.Errorf("CStoreReadRSP: status %04X want Success", st)
	}
}

func TestCStoreReadRSP_ErrorOnNoPDU(t *testing.T) {
	m := newMockPDU(ctUID)
	if _, err := dimse.CStoreReadRSP(m); err == nil {
		t.Error("CStoreReadRSP: want error when NextPDU fails")
	}
}

func TestCStoreReadRSP_ErrorInvalidStatusCode(t *testing.T) {
	rsp := media.NewEmptyDCMObj()
	rsp.Write(tags.CommandField, dicomcommand.CStoreResponse)
	rsp.Write(tags.Status, 0xFE00)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{rsp}

	if _, err := dimse.CStoreReadRSP(m); err == nil {
		t.Fatal("CStoreReadRSP: want error for invalid C-STORE status code")
	}
}

func TestCStoreReadRSP_ErrorInvalidCommandDataSetType(t *testing.T) {
	rsp := media.NewEmptyDCMObj()
	rsp.Write(tags.CommandField, dicomcommand.CStoreResponse)
	rsp.Write(tags.CommandDataSetType, 0x0102)
	rsp.Write(tags.MessageIDBeingRespondedTo, 1)
	rsp.Write(tags.Status, dicomstatus.Success)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{rsp}

	if _, err := dimse.CStoreReadRSP(m); err == nil {
		t.Fatal("CStoreReadRSP: want error for invalid CommandDataSetType")
	}
}

func TestCStoreReadRSP_ErrorMissingMessageIDBeingRespondedTo(t *testing.T) {
	rsp := media.NewEmptyDCMObj()
	rsp.Write(tags.CommandField, dicomcommand.CStoreResponse)
	rsp.Write(tags.CommandDataSetType, 0x0101)
	rsp.Write(tags.MessageIDBeingRespondedTo, 0)
	rsp.Write(tags.Status, dicomstatus.Success)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{rsp}

	if _, err := dimse.CStoreReadRSP(m); err == nil {
		t.Fatal("CStoreReadRSP: want error when MessageIDBeingRespondedTo is missing")
	}
}

func TestCStoreWriteRSP_WritesResponse(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.Write(tags.AffectedSOPInstanceUID, "1.2.3.4")
	rq.Write(tags.MessageID, 42)
	if err := dimse.CStoreWriteRSP(m, rq, dicomstatus.Success); err != nil {
		t.Fatalf("CStoreWriteRSP: %v", err)
	}
	if len(m.written) == 0 {
		t.Fatal("CStoreWriteRSP: nothing written")
	}
	rsp := m.written[0]
	if cf := rsp.GetUint16(tags.CommandField); cf != dicomcommand.CStoreResponse {
		t.Errorf("CStoreWriteRSP: CommandField %04X want CStoreResponse", cf)
	}
	if st := rsp.GetUint16(tags.Status); st != dicomstatus.Success {
		t.Errorf("CStoreWriteRSP: Status %04X want Success", st)
	}
}

func TestCStoreWriteRSP_AllowsWarningStatus(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.Write(tags.AffectedSOPInstanceUID, "1.2.3")
	rq.Write(tags.MessageID, 5)

	if err := dimse.CStoreWriteRSP(m, rq, dicomstatus.WarningElementsDiscarded); err != nil {
		t.Fatalf("CStoreWriteRSP: %v", err)
	}
	if got := m.written[0].GetUint16(tags.Status); got != dicomstatus.WarningElementsDiscarded {
		t.Fatalf("CStoreWriteRSP: status %04X want %04X", got, dicomstatus.WarningElementsDiscarded)
	}
}

func TestCStoreWriteRSP_CommandGroupLengthUsesInstanceUID(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID) // 25 chars
	rq.Write(tags.AffectedSOPInstanceUID, "1.2.3")
	rq.Write(tags.MessageID, 1)
	if err := dimse.CStoreWriteRSP(m, rq, dicomstatus.Success); err != nil {
		t.Fatalf("CStoreWriteRSP: %v", err)
	}
	if m.written[0].GetUint32(tags.CommandGroupLength) == 0 {
		t.Error("CStoreWriteRSP: CommandGroupLength must not be zero")
	}
}

func TestCStoreWriteRSP_ErrorNoSOPClass(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CStoreWriteRSP(m, media.NewEmptyDCMObj(), dicomstatus.Success); err == nil {
		t.Error("CStoreWriteRSP: want error when AffectedSOPClassUID absent")
	}
}

func TestCStoreWriteRSP_ErrorNoSOPInstanceUID(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	if err := dimse.CStoreWriteRSP(m, rq, dicomstatus.Success); err == nil {
		t.Error("CStoreWriteRSP: want error when AffectedSOPInstanceUID absent")
	}
}

func TestCStoreWriteRSP_ErrorNilRequest(t *testing.T) {
	m := newMockPDU(ctUID)
	var rq media.DICOMObject
	if err := dimse.CStoreWriteRSP(m, rq, dicomstatus.Success); err == nil {
		t.Error("CStoreWriteRSP: want error when requestCommandObj is nil")
	}
}

func TestCStoreWriteRSP_ErrorInvalidStatusCode(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.Write(tags.AffectedSOPInstanceUID, "1.2.3")
	rq.Write(tags.MessageID, 3)

	if err := dimse.CStoreWriteRSP(m, rq, 0xFE00); err == nil {
		t.Error("CStoreWriteRSP: want error for invalid C-STORE status code")
	}
}

func TestCStoreWriteRSP_ErrorMissingMessageID(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.Write(tags.AffectedSOPInstanceUID, "1.2.3")

	if err := dimse.CStoreWriteRSP(m, rq, dicomstatus.Success); err == nil {
		t.Error("CStoreWriteRSP: want error when MessageID is missing")
	}
}

// ── C-MOVE ────────────────────────────────────────────────────────────────────

func TestCMoveWriteRQ_WritesCommandAndData(t *testing.T) {
	m := newMockPDU(ctUID)
	q := media.NewEmptyDCMObj()
	q.Write(tags.QueryRetrieveLevel, "STUDY")
	if err := dimse.CMoveWriteRQ(m, q, "DEST_AE", priority.Medium); err != nil {
		t.Fatalf("CMoveWriteRQ: %v", err)
	}
	if len(m.written) != 2 {
		t.Fatalf("CMoveWriteRQ: want 2 writes, got %d", len(m.written))
	}
	if cf := m.written[0].GetUint16(tags.CommandField); cf != dicomcommand.CMoveRequest {
		t.Errorf("CMoveWriteRQ: CommandField %04X want CMoveRequest", cf)
	}
}

func TestCMoveWriteRSP_AllSubopCountsPresent(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.Write(tags.MessageID, 1)
	if err := dimse.CMoveWriteRSP(m, rq, dicomstatus.WarningSubOperationsCompleteOneOrMoreFailures, 0, 3, 1, 0); err != nil {
		t.Fatalf("CMoveWriteRSP: %v", err)
	}
	rsp := m.written[0]
	if cf := rsp.GetUint16(tags.CommandField); cf != dicomcommand.CMoveResponse {
		t.Errorf("CMoveWriteRSP: CommandField %04X want CMoveResponse", cf)
	}
	_ = rsp.GetUint16(tags.NumberOfRemainingSuboperations)
	_ = rsp.GetUint16(tags.NumberOfCompletedSuboperations)
	_ = rsp.GetUint16(tags.NumberOfFailedSuboperations)
	_ = rsp.GetUint16(tags.NumberOfWarningSuboperations)
}

func TestCMoveReadRSP_PendingThenFinal(t *testing.T) {
	pending := media.NewEmptyDCMObj()
	pending.Write(tags.CommandField, dicomcommand.CMoveResponse)
	pending.Write(tags.Status, dicomstatus.Pending)
	pending.Write(tags.CommandDataSetType, 0x0102) // Dataset follows
	pending.Write(tags.NumberOfRemainingSuboperations, 2)

	dataset := media.NewEmptyDCMObj()

	final := media.NewEmptyDCMObj()
	final.Write(tags.CommandField, dicomcommand.CMoveResponse)
	final.Write(tags.Status, dicomstatus.Success)
	final.Write(tags.CommandDataSetType, 0x0101) // No dataset

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{pending, dataset, final}

	var cnt int
	_, st1, err1 := dimse.CMoveReadRSP(m, &cnt)
	if err1 != nil {
		t.Fatalf("CMoveReadRSP (pending): %v", err1)
	}
	if st1 != dicomstatus.Pending {
		t.Errorf("CMoveReadRSP: status %04X want Pending", st1)
	}
	if cnt != 2 {
		t.Errorf("CMoveReadRSP: pending count %d want 2", cnt)
	}

	_, st2, err2 := dimse.CMoveReadRSP(m, &cnt)
	if err2 != nil {
		t.Fatalf("CMoveReadRSP (final): %v", err2)
	}
	if st2 != dicomstatus.Success {
		t.Errorf("CMoveReadRSP: final status %04X want Success", st2)
	}
	if cnt != -1 {
		t.Errorf("CMoveReadRSP: final pending count %d want -1", cnt)
	}
}

func TestCMoveReadRSP_ErrorOnNoPDU(t *testing.T) {
	m := newMockPDU(ctUID)
	var c int
	if _, _, err := dimse.CMoveReadRSP(m, &c); err == nil {
		t.Error("CMoveReadRSP: want error when NextPDU fails")
	}
}

func TestCMoveReadRSP_ErrorInvalidCommandDataSetType(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CMoveResponse)
	cmd.Write(tags.Status, dicomstatus.Success)
	cmd.Write(tags.CommandDataSetType, 0x4321)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	var c int
	if _, _, err := dimse.CMoveReadRSP(m, &c); err == nil {
		t.Fatal("CMoveReadRSP: want error for invalid CommandDataSetType")
	}
}

// ── C-MOVE-RQ Parsing (NEW) ───────────────────────────────────────────────────

func TestCMoveReadRQ_SuccessWithIdentifier(t *testing.T) {
	// Construct a C-MOVE-RQ command
	cmd := media.NewEmptyDCMObj()
	cmd.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	cmd.Write(tags.CommandField, dicomcommand.CMoveRequest)
	cmd.Write(tags.MessageID, 1)
	cmd.Write(tags.AffectedSOPClassUID, ctUID)
	cmd.Write(tags.MoveDestination, "DEST_AE")
	cmd.Write(tags.Priority, 0) // Low
	cmd.Write(tags.CommandDataSetType, 0x0102)

	// Construct identifier dataset
	identifier := media.NewEmptyDCMObj()
	identifier.Write(tags.QueryRetrieveLevel, "STUDY")
	identifier.Write(tags.PatientID, "P001")

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd, identifier}

	req, err := dimse.CMoveReadRQ(m)
	if err != nil {
		t.Fatalf("CMoveReadRQ: %v", err)
	}

	if req.AffectedSOPClassUID != ctUID {
		t.Errorf("CMoveReadRQ: AffectedSOPClassUID %s want %s", req.AffectedSOPClassUID, ctUID)
	}
	if req.MessageID != 1 {
		t.Errorf("CMoveReadRQ: MessageID %d want 1", req.MessageID)
	}
	if req.MoveDestination != "DEST_AE" {
		t.Errorf("CMoveReadRQ: MoveDestination %s want DEST_AE", req.MoveDestination)
	}
	if req.Priority != 0 {
		t.Errorf("CMoveReadRQ: Priority %d want 0", req.Priority)
	}
	if req.IdentifierDataSet == nil {
		t.Error("CMoveReadRQ: IdentifierDataSet is nil")
	}
	if level := req.GetMoveLevel(); level != "STUDY" {
		t.Errorf("CMoveReadRQ: GetMoveLevel() %s want STUDY", level)
	}
}

func TestCMoveReadRQ_SuccessWithoutIdentifier(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	cmd.Write(tags.CommandField, dicomcommand.CMoveRequest)
	cmd.Write(tags.MessageID, 1)
	cmd.Write(tags.AffectedSOPClassUID, ctUID)
	cmd.Write(tags.MoveDestination, "DEST_AE")
	cmd.Write(tags.Priority, 2) // High
	cmd.Write(tags.CommandDataSetType, 0x0101)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	req, err := dimse.CMoveReadRQ(m)
	if err != nil {
		t.Fatalf("CMoveReadRQ: %v", err)
	}

	if req.IdentifierDataSet != nil {
		t.Error("CMoveReadRQ: IdentifierDataSet should be nil when CommandDataSetType=0x0101")
	}
	if req.Priority != 2 {
		t.Errorf("CMoveReadRQ: Priority %d want 2", req.Priority)
	}
}

func TestCMoveReadRQ_ErrorMissingSOPClassUID(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	cmd.Write(tags.CommandField, dicomcommand.CMoveRequest)
	cmd.Write(tags.MessageID, 1)
	// Missing AffectedSOPClassUID
	cmd.Write(tags.MoveDestination, "DEST_AE")
	cmd.Write(tags.CommandDataSetType, 0x0101)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	_, err := dimse.CMoveReadRQ(m)
	if err == nil {
		t.Error("CMoveReadRQ: want error when AffectedSOPClassUID is missing")
	}
}

func TestCMoveReadRQ_ErrorMissingMessageID(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	cmd.Write(tags.CommandField, dicomcommand.CMoveRequest)
	// Missing MessageID
	cmd.Write(tags.AffectedSOPClassUID, ctUID)
	cmd.Write(tags.MoveDestination, "DEST_AE")
	cmd.Write(tags.CommandDataSetType, 0x0101)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	_, err := dimse.CMoveReadRQ(m)
	if err == nil {
		t.Error("CMoveReadRQ: want error when MessageID is missing")
	}
}

func TestCMoveReadRQ_ErrorMissingMoveDestination(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	cmd.Write(tags.CommandField, dicomcommand.CMoveRequest)
	cmd.Write(tags.MessageID, 1)
	cmd.Write(tags.AffectedSOPClassUID, ctUID)
	// Missing MoveDestination
	cmd.Write(tags.CommandDataSetType, 0x0101)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	_, err := dimse.CMoveReadRQ(m)
	if err == nil {
		t.Error("CMoveReadRQ: want error when MoveDestination is missing")
	}
}

func TestCMoveReadRQ_ErrorInvalidCommandDataSetType(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	cmd.Write(tags.CommandField, dicomcommand.CMoveRequest)
	cmd.Write(tags.MessageID, 1)
	cmd.Write(tags.AffectedSOPClassUID, ctUID)
	cmd.Write(tags.MoveDestination, "DEST_AE")
	cmd.Write(tags.CommandDataSetType, 0x0103) // Invalid

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	_, err := dimse.CMoveReadRQ(m)
	if err == nil {
		t.Error("CMoveReadRQ: want error for invalid CommandDataSetType")
	}
}

func TestCMoveReadRQ_ErrorWrongCommandField(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	cmd.Write(tags.CommandField, dicomcommand.CFindRequest) // Wrong command
	cmd.Write(tags.MessageID, 1)
	cmd.Write(tags.AffectedSOPClassUID, ctUID)
	cmd.Write(tags.MoveDestination, "DEST_AE")
	cmd.Write(tags.CommandDataSetType, 0x0101)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	_, err := dimse.CMoveReadRQ(m)
	if err == nil {
		t.Error("CMoveReadRQ: want error when CommandField is not C-MOVE Request")
	}
}

// ── C-MOVE-RQ Writing (Enhanced) ───────────────────────────────────────────────

func TestCMoveWriteRQ_LowPriority(t *testing.T) {
	m := newMockPDU(ctUID)
	q := media.NewEmptyDCMObj()
	q.Write(tags.QueryRetrieveLevel, "STUDY")
	if err := dimse.CMoveWriteRQ(m, q, "DEST_AE", 2); err != nil {
		t.Fatalf("CMoveWriteRQ: %v", err)
	}
	if len(m.written) != 2 {
		t.Fatalf("CMoveWriteRQ: want 2 writes, got %d", len(m.written))
	}
	if pri := m.written[0].GetUint16(tags.Priority); pri != 2 {
		t.Errorf("CMoveWriteRQ: Priority %d want 2", pri)
	}
}

func TestCMoveWriteRQ_EmptyDestination(t *testing.T) {
	m := newMockPDU(ctUID)
	q := media.NewEmptyDCMObj()
	if err := dimse.CMoveWriteRQ(m, q, "", 0); err == nil {
		t.Error("CMoveWriteRQ: want error for empty destination")
	}
}

func TestCMoveWriteRQ_EmptyDataset(t *testing.T) {
	m := newMockPDU(ctUID)
	q := media.NewEmptyDCMObj()
	if err := dimse.CMoveWriteRQ(m, q, "DEST_AE", 1); err != nil {
		t.Fatalf("CMoveWriteRQ: %v", err)
	}
	// When dataObj is empty, should only write 1 PDU (command only)
	if len(m.written) != 1 {
		t.Fatalf("CMoveWriteRQ (empty dataset): want 1 write, got %d", len(m.written))
	}
	// CommandDataSetType should be 0x0101 (no dataset)
	if cdt := m.written[0].GetUint16(tags.CommandDataSetType); cdt != 0x0101 {
		t.Errorf("CMoveWriteRQ: CommandDataSetType %04X want 0x0101", cdt)
	}
}

// ── C-MOVE Response Stats (NEW) ────────────────────────────────────────────────

func TestGetCMoveResponseStats(t *testing.T) {
	rsp := media.NewEmptyDCMObj()
	rsp.Write(tags.CommandField, dicomcommand.CMoveResponse)
	rsp.Write(tags.Status, dicomstatus.Pending)
	rsp.Write(tags.CommandDataSetType, 0x0101)
	rsp.Write(tags.NumberOfRemainingSuboperations, 5)
	rsp.Write(tags.NumberOfCompletedSuboperations, 3)
	rsp.Write(tags.NumberOfFailedSuboperations, 1)
	rsp.Write(tags.NumberOfWarningSuboperations, 0)

	stats := dimse.GetCMoveResponseStats(rsp)
	if stats.Remaining != 5 {
		t.Errorf("GetCMoveResponseStats: Remaining %d want 5", stats.Remaining)
	}
	if stats.Completed != 3 {
		t.Errorf("GetCMoveResponseStats: Completed %d want 3", stats.Completed)
	}
	if stats.Failed != 1 {
		t.Errorf("GetCMoveResponseStats: Failed %d want 1", stats.Failed)
	}
	if stats.Warnings != 0 {
		t.Errorf("GetCMoveResponseStats: Warnings %d want 0", stats.Warnings)
	}
}

// ── C-MOVE Helper Functions ────────────────────────────────────────────────

func TestCMoveRequest_GetMoveLevelFromIdentifier(t *testing.T) {
	identifier := media.NewEmptyDCMObj()
	identifier.Write(tags.QueryRetrieveLevel, "STUDY")

	req := &dimse.CMoveRequest{
		IdentifierDataSet: identifier,
	}

	level := req.GetMoveLevel()
	if level != "STUDY" {
		t.Errorf("GetMoveLevel: %s want STUDY", level)
	}
}

// ── C-GET ─────────────────────────────────────────────────────────────────────

func TestCGetWriteRQ_WritesCommandAndData(t *testing.T) {
	m := newMockPDU(ctUID)
	q := media.NewEmptyDCMObj()
	q.Write(tags.QueryRetrieveLevel, "STUDY")
	if err := dimse.CGetWriteRQ(m, q, priority.Medium); err != nil {
		t.Fatalf("CGetWriteRQ: %v", err)
	}
	if len(m.written) != 2 {
		t.Fatalf("CGetWriteRQ: want 2 writes, got %d", len(m.written))
	}
	if cf := m.written[0].GetUint16(tags.CommandField); cf != dicomcommand.CGetRequest {
		t.Errorf("CGetWriteRQ: CommandField %04X want CGetRequest", cf)
	}
}

func TestCGetWriteRQ_ErrorMissingSOPClass(t *testing.T) {
	m := newMockPDU("")
	q := media.NewEmptyDCMObj()
	q.Write(tags.QueryRetrieveLevel, "STUDY")
	if err := dimse.CGetWriteRQ(m, q, priority.Medium); err == nil {
		t.Error("CGetWriteRQ: want error when Affected SOP Class UID is missing")
	}
}

func TestCGetReadRSP_PendingThenFinal(t *testing.T) {
	pending := media.NewEmptyDCMObj()
	pending.Write(tags.CommandField, dicomcommand.CGetResponse)
	pending.Write(tags.Status, dicomstatus.Pending)
	pending.Write(tags.CommandDataSetType, 0x0101)
	pending.Write(tags.NumberOfRemainingSuboperations, 2)

	final := media.NewEmptyDCMObj()
	final.Write(tags.CommandField, dicomcommand.CGetResponse)
	final.Write(tags.Status, dicomstatus.Success)
	final.Write(tags.CommandDataSetType, 0x0101)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{pending, final}

	pendingCount := 0
	_, st1, err1 := dimse.CGetReadRSP(m, &pendingCount)
	if err1 != nil {
		t.Fatalf("CGetReadRSP (pending): %v", err1)
	}
	if st1 != dicomstatus.Pending || pendingCount != 2 {
		t.Fatalf("CGetReadRSP pending: status=%04X pending=%d", st1, pendingCount)
	}

	_, st2, err2 := dimse.CGetReadRSP(m, &pendingCount)
	if err2 != nil {
		t.Fatalf("CGetReadRSP (final): %v", err2)
	}
	if st2 != dicomstatus.Success || pendingCount != -1 {
		t.Fatalf("CGetReadRSP final: status=%04X pending=%d", st2, pendingCount)
	}
}

func TestCGetReadRSP_ErrorInvalidCommandDataSetType(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CGetResponse)
	cmd.Write(tags.Status, dicomstatus.Success)
	cmd.Write(tags.CommandDataSetType, 0x1234)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	var pending int
	if _, _, err := dimse.CGetReadRSP(m, &pending); err == nil {
		t.Fatal("CGetReadRSP: want error for invalid CommandDataSetType")
	}
}

func TestCGetReadRSP_ErrorInvalidStatusCode(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CGetResponse)
	cmd.Write(tags.Status, 0x2222)
	cmd.Write(tags.CommandDataSetType, 0x0101)
	cmd.Write(tags.NumberOfRemainingSuboperations, 0)
	cmd.Write(tags.NumberOfCompletedSuboperations, 1)
	cmd.Write(tags.NumberOfFailedSuboperations, 0)
	cmd.Write(tags.NumberOfWarningSuboperations, 0)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	var pending int
	if _, _, err := dimse.CGetReadRSP(m, &pending); err == nil {
		t.Fatal("CGetReadRSP: want error for invalid status code")
	}
}

func TestCGetWriteRSP_WritesResponse(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.Write(tags.CommandField, dicomcommand.CGetRequest)
	rq.Write(tags.MessageID, 7)

	if err := dimse.CGetWriteRSP(m, rq, dicomstatus.Success, 0, 1, 0, 0); err != nil {
		t.Fatalf("CGetWriteRSP: %v", err)
	}
	if len(m.written) != 1 {
		t.Fatalf("CGetWriteRSP: want 1 write, got %d", len(m.written))
	}
	if cf := m.written[0].GetUint16(tags.CommandField); cf != dicomcommand.CGetResponse {
		t.Errorf("CGetWriteRSP: CommandField %04X want CGetResponse", cf)
	}
}

func TestCMoveReadRSP_ErrorInvalidStatusCode(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CMoveResponse)
	cmd.Write(tags.Status, 0x2222)
	cmd.Write(tags.CommandDataSetType, 0x0101)
	cmd.Write(tags.NumberOfRemainingSuboperations, 0)
	cmd.Write(tags.NumberOfCompletedSuboperations, 1)
	cmd.Write(tags.NumberOfFailedSuboperations, 0)
	cmd.Write(tags.NumberOfWarningSuboperations, 0)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	var pending int
	if _, _, err := dimse.CMoveReadRSP(m, &pending); err == nil {
		t.Fatal("CMoveReadRSP: want error for invalid status code")
	}
}

func TestCGetWriteRSP_ErrorNoMessageID(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	if err := dimse.CGetWriteRSP(m, rq, dicomstatus.Success, 0, 0, 0, 0); err == nil {
		t.Error("CGetWriteRSP: want error when MessageID is missing")
	}
}

func TestCGetWriteRSP_ErrorInvalidStatusCode(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.Write(tags.CommandField, dicomcommand.CGetRequest)
	rq.Write(tags.MessageID, 99)

	if err := dimse.CGetWriteRSP(m, rq, 0x2222, 0, 0, 1, 0); err == nil {
		t.Error("CGetWriteRSP: want error for invalid status code")
	}
}

func TestCGetWriteRSP_ErrorCMoveSpecificStatus(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.Write(tags.CommandField, dicomcommand.CGetRequest)
	rq.Write(tags.MessageID, 101)

	if err := dimse.CGetWriteRSP(m, rq, dicomstatus.RefusedMoveDestinationUnknown, 0, 0, 1, 0); err == nil {
		t.Error("CGetWriteRSP: want error for C-MOVE-specific status")
	}
}

// ── N-SERVICE Generic Response ───────────────────────────────────────────────

func TestNWriteRSP_WritesResponse(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.RequestedSOPClassUID, ctUID)
	rq.Write(tags.RequestedSOPInstanceUID, "1.2.3.4.5")
	rq.Write(tags.MessageID, 9)

	if err := dimse.NWriteRSP(m, rq, dicomcommand.NGetResponse, dicomstatus.FailureSOPClassNotSupported, media.NewEmptyDCMObj()); err != nil {
		t.Fatalf("NWriteRSP: %v", err)
	}
	if len(m.written) != 1 {
		t.Fatalf("NWriteRSP: want 1 write, got %d", len(m.written))
	}
	if cf := m.written[0].GetUint16(tags.CommandField); cf != dicomcommand.NGetResponse {
		t.Errorf("NWriteRSP: CommandField %04X want NGetResponse", cf)
	}
}

// ── PS3.7 Status-Code Conformance Matrix ────────────────────────────────────

func TestCFindReadRSP_StatusMatrix(t *testing.T) {
	tests := []struct {
		name            string
		status          uint16
		commandDST      uint16
		expectDataset   bool
		expectReadError bool
	}{
		{name: "pending", status: dicomstatus.Pending, commandDST: 0x0102, expectDataset: true},
		{name: "pending_with_warnings", status: dicomstatus.PendingWithWarnings, commandDST: 0x0102, expectDataset: true},
		{name: "success", status: dicomstatus.Success, commandDST: 0x0101, expectDataset: false},
		{name: "cancel", status: dicomstatus.Cancel, commandDST: 0x0101, expectDataset: false},
		{name: "failure", status: dicomstatus.FailureUnableToProcess, commandDST: 0x0101, expectDataset: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := media.NewEmptyDCMObj()
			cmd.Write(tags.CommandField, dicomcommand.CFindResponse)
			cmd.Write(tags.Status, tc.status)
			cmd.Write(tags.CommandDataSetType, tc.commandDST)

			m := newMockPDU(ctUID)
			if tc.expectDataset {
				ds := media.NewEmptyDCMObj()
				ds.Write(tags.PatientID, "P001")
				m.nextPDUs = []media.DICOMObject{cmd, ds}
			} else {
				m.nextPDUs = []media.DICOMObject{cmd}
			}

			data, st, err := dimse.CFindReadRSP(m)
			if tc.expectReadError {
				if err == nil {
					t.Fatalf("CFindReadRSP: want read error")
				}
				return
			}
			if err != nil {
				t.Fatalf("CFindReadRSP: %v", err)
			}
			if st != tc.status {
				t.Fatalf("CFindReadRSP: status %04X want %04X", st, tc.status)
			}
			if tc.expectDataset && data == nil {
				t.Fatalf("CFindReadRSP: expected dataset for status %04X", tc.status)
			}
			if !tc.expectDataset && data != nil {
				t.Fatalf("CFindReadRSP: unexpected dataset for status %04X", tc.status)
			}
		})
	}
}

func TestCFindReadRSP_RejectsBxxxWarningStatus(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CFindResponse)
	cmd.Write(tags.Status, dicomstatus.WarningSubOperationsCompleteOneOrMoreFailures)
	cmd.Write(tags.CommandDataSetType, 0x0101)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	if _, _, err := dimse.CFindReadRSP(m); err == nil {
		t.Fatal("CFindReadRSP: want error for Bxxx warning status")
	}
}

func TestCGetReadRSP_StatusAndPendingMatrix(t *testing.T) {
	tests := []struct {
		name          string
		status        uint16
		remaining     uint16
		commandDST    uint16
		expectPending int
		expectDataset bool
	}{
		{name: "pending", status: dicomstatus.Pending, remaining: 3, commandDST: 0x0102, expectPending: 3, expectDataset: true},
		{name: "pending_with_warnings", status: dicomstatus.PendingWithWarnings, remaining: 2, commandDST: 0x0101, expectPending: 2, expectDataset: false},
		{name: "success", status: dicomstatus.Success, remaining: 0, commandDST: 0x0101, expectPending: -1, expectDataset: false},
		{name: "cancel", status: dicomstatus.Cancel, remaining: 0, commandDST: 0x0101, expectPending: -1, expectDataset: false},
		{name: "failure", status: dicomstatus.FailureOutOfResourcesUnableToPerformSubOperations, remaining: 0, commandDST: 0x0101, expectPending: -1, expectDataset: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := media.NewEmptyDCMObj()
			cmd.Write(tags.CommandField, dicomcommand.CGetResponse)
			cmd.Write(tags.Status, tc.status)
			cmd.Write(tags.CommandDataSetType, tc.commandDST)
			cmd.Write(tags.NumberOfRemainingSuboperations, tc.remaining)

			m := newMockPDU(ctUID)
			if tc.expectDataset {
				ds := media.NewEmptyDCMObj()
				ds.Write(tags.PatientID, "P001")
				m.nextPDUs = []media.DICOMObject{cmd, ds}
			} else {
				m.nextPDUs = []media.DICOMObject{cmd}
			}

			pending := 99
			data, st, err := dimse.CGetReadRSP(m, &pending)
			if err != nil {
				t.Fatalf("CGetReadRSP: %v", err)
			}
			if st != tc.status {
				t.Fatalf("CGetReadRSP: status %04X want %04X", st, tc.status)
			}
			if pending != tc.expectPending {
				t.Fatalf("CGetReadRSP: pending %d want %d", pending, tc.expectPending)
			}
			if tc.expectDataset && data == nil {
				t.Fatalf("CGetReadRSP: expected dataset for status %04X", tc.status)
			}
			if !tc.expectDataset && data != nil {
				t.Fatalf("CGetReadRSP: unexpected dataset for status %04X", tc.status)
			}
		})
	}
}

func TestCGetReadRSP_RejectsCMoveSpecificStatus(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CGetResponse)
	cmd.Write(tags.Status, dicomstatus.RefusedMoveDestinationUnknown)
	cmd.Write(tags.CommandDataSetType, 0x0101)
	cmd.Write(tags.NumberOfRemainingSuboperations, 0)
	cmd.Write(tags.NumberOfCompletedSuboperations, 0)
	cmd.Write(tags.NumberOfFailedSuboperations, 1)
	cmd.Write(tags.NumberOfWarningSuboperations, 0)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	pending := 1
	if _, _, err := dimse.CGetReadRSP(m, &pending); err == nil {
		t.Fatal("CGetReadRSP: want error for C-MOVE-specific status")
	}
}

func TestCGetReadRSP_RejectsInvalidBxxxWarningStatus(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CGetResponse)
	cmd.Write(tags.Status, dicomstatus.WarningDataSetDoesNotMatchSOPClass)
	cmd.Write(tags.CommandDataSetType, 0x0101)
	cmd.Write(tags.NumberOfRemainingSuboperations, 0)
	cmd.Write(tags.NumberOfCompletedSuboperations, 0)
	cmd.Write(tags.NumberOfFailedSuboperations, 1)
	cmd.Write(tags.NumberOfWarningSuboperations, 0)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	pending := 1
	if _, _, err := dimse.CGetReadRSP(m, &pending); err == nil {
		t.Fatal("CGetReadRSP: want error for non-B000 warning status")
	}
}

func TestCMoveReadRSP_StatusAndPendingMatrix(t *testing.T) {
	tests := []struct {
		name          string
		status        uint16
		remaining     uint16
		completed     uint16
		failed        uint16
		warnings      uint16
		commandDST    uint16
		expectPending int
		expectDataset bool
	}{
		{name: "pending", status: dicomstatus.Pending, remaining: 4, completed: 1, failed: 0, warnings: 0, commandDST: 0x0102, expectPending: 4, expectDataset: true},
		{name: "pending_with_warnings", status: dicomstatus.PendingWithWarnings, remaining: 2, completed: 2, failed: 0, warnings: 1, commandDST: 0x0101, expectPending: 2, expectDataset: false},
		{name: "success", status: dicomstatus.Success, remaining: 0, completed: 5, failed: 0, warnings: 0, commandDST: 0x0101, expectPending: -1, expectDataset: false},
		{name: "cancel", status: dicomstatus.Cancel, remaining: 0, completed: 1, failed: 0, warnings: 0, commandDST: 0x0101, expectPending: -1, expectDataset: false},
		{name: "failure", status: dicomstatus.FailureOutOfResourcesUnableToCalculateMatches, remaining: 0, completed: 0, failed: 3, warnings: 0, commandDST: 0x0101, expectPending: -1, expectDataset: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := media.NewEmptyDCMObj()
			cmd.Write(tags.CommandField, dicomcommand.CMoveResponse)
			cmd.Write(tags.Status, tc.status)
			cmd.Write(tags.CommandDataSetType, tc.commandDST)
			cmd.Write(tags.NumberOfRemainingSuboperations, tc.remaining)
			cmd.Write(tags.NumberOfCompletedSuboperations, tc.completed)
			cmd.Write(tags.NumberOfFailedSuboperations, tc.failed)
			cmd.Write(tags.NumberOfWarningSuboperations, tc.warnings)

			m := newMockPDU(ctUID)
			if tc.expectDataset {
				ds := media.NewEmptyDCMObj()
				ds.Write(tags.PatientID, "P001")
				m.nextPDUs = []media.DICOMObject{cmd, ds}
			} else {
				m.nextPDUs = []media.DICOMObject{cmd}
			}

			pending := 88
			data, st, err := dimse.CMoveReadRSP(m, &pending)
			if err != nil {
				t.Fatalf("CMoveReadRSP: %v", err)
			}
			if st != tc.status {
				t.Fatalf("CMoveReadRSP: status %04X want %04X", st, tc.status)
			}
			if pending != tc.expectPending {
				t.Fatalf("CMoveReadRSP: pending %d want %d", pending, tc.expectPending)
			}
			if tc.expectDataset && data == nil {
				t.Fatalf("CMoveReadRSP: expected dataset for status %04X", tc.status)
			}
			if !tc.expectDataset && data != nil {
				t.Fatalf("CMoveReadRSP: unexpected dataset for status %04X", tc.status)
			}
		})
	}
}

func TestCMoveReadRSP_RejectsInvalidBxxxWarningStatus(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CMoveResponse)
	cmd.Write(tags.Status, dicomstatus.WarningElementsDiscarded)
	cmd.Write(tags.CommandDataSetType, 0x0101)
	cmd.Write(tags.NumberOfRemainingSuboperations, 0)
	cmd.Write(tags.NumberOfCompletedSuboperations, 1)
	cmd.Write(tags.NumberOfFailedSuboperations, 0)
	cmd.Write(tags.NumberOfWarningSuboperations, 1)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}

	pending := 1
	if _, _, err := dimse.CMoveReadRSP(m, &pending); err == nil {
		t.Fatal("CMoveReadRSP: want error for non-B000 warning status")
	}
}

func TestCGetWriteRSP_StatusPropagationMatrix(t *testing.T) {
	tests := []uint16{
		dicomstatus.Success,
		dicomstatus.Pending,
		dicomstatus.PendingWithWarnings,
		dicomstatus.Cancel,
		dicomstatus.FailureUnableToProcess,
	}

	countersForStatus := func(st uint16) (remaining, completed, failed, warnings uint16) {
		switch st {
		case dicomstatus.Success:
			return 0, 2, 0, 0
		case dicomstatus.Pending, dicomstatus.PendingWithWarnings:
			return 1, 1, 0, 0
		default:
			return 0, 0, 1, 0
		}
	}

	for _, st := range tests {
		t.Run(fmt.Sprintf("status_%04X", st), func(t *testing.T) {
			m := newMockPDU(ctUID)
			rq := media.NewEmptyDCMObj()
			rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
			rq.Write(tags.AffectedSOPClassUID, ctUID)
			rq.Write(tags.CommandField, dicomcommand.CGetRequest)
			rq.Write(tags.MessageID, 11)

			remaining, completed, failed, warnings := countersForStatus(st)
			if err := dimse.CGetWriteRSP(m, rq, st, remaining, completed, failed, warnings); err != nil {
				t.Fatalf("CGetWriteRSP: %v", err)
			}
			got := m.written[0].GetUint16(tags.Status)
			if got != st {
				t.Fatalf("CGetWriteRSP: status %04X want %04X", got, st)
			}
		})
	}
}

func TestCMoveWriteRSP_StatusPropagationMatrix(t *testing.T) {
	tests := []uint16{
		dicomstatus.Success,
		dicomstatus.Pending,
		dicomstatus.PendingWithWarnings,
		dicomstatus.Cancel,
		dicomstatus.FailureOutOfResourcesUnableToPerformSubOperations,
	}

	countersForStatus := func(st uint16) (remaining, completed, failed, warnings uint16) {
		switch st {
		case dicomstatus.Success:
			return 0, 3, 0, 0
		case dicomstatus.Pending, dicomstatus.PendingWithWarnings:
			return 2, 1, 0, 0
		default:
			return 0, 0, 2, 0
		}
	}

	for _, st := range tests {
		t.Run(fmt.Sprintf("status_%04X", st), func(t *testing.T) {
			m := newMockPDU(ctUID)
			rq := media.NewEmptyDCMObj()
			rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
			rq.Write(tags.AffectedSOPClassUID, ctUID)
			rq.Write(tags.MessageID, 13)

			remaining, completed, failed, warnings := countersForStatus(st)
			if err := dimse.CMoveWriteRSP(m, rq, st, remaining, completed, failed, warnings); err != nil {
				t.Fatalf("CMoveWriteRSP: %v", err)
			}
			got := m.written[0].GetUint16(tags.Status)
			if got != st {
				t.Fatalf("CMoveWriteRSP: status %04X want %04X", got, st)
			}
		})
	}
}

func TestCMoveWriteRSP_ErrorInvalidStatusCode(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.Write(tags.MessageID, 77)

	if err := dimse.CMoveWriteRSP(m, rq, 0x2222, 0, 0, 1, 0); err == nil {
		t.Error("CMoveWriteRSP: want error for invalid status code")
	}
}

func TestCGetWriteRSP_RejectsInvalidSuboperationCounters(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.Write(tags.AffectedSOPClassUID, ctUID)
	rq.Write(tags.CommandField, dicomcommand.CGetRequest)
	rq.Write(tags.MessageID, 21)

	if err := dimse.CGetWriteRSP(m, rq, dicomstatus.Pending, 0, 1, 0, 0); err == nil {
		t.Fatal("CGetWriteRSP: want error when pending has remaining=0")
	}
	if err := dimse.CGetWriteRSP(m, rq, dicomstatus.Success, 1, 1, 0, 0); err == nil {
		t.Fatal("CGetWriteRSP: want error when final success has remaining>0")
	}
	if err := dimse.CGetWriteRSP(m, rq, dicomstatus.Success, 0, 1, 1, 0); err == nil {
		t.Fatal("CGetWriteRSP: want error when success has failed>0")
	}
}

// TestCMoveReadRSP_AcceptsPendingWithZeroRemaining verifies that the read path
// accepts a Pending response with remaining=0. Some implementations (e.g. DCMTK
// dcmqrscp) send this before the final response, which is non-conformant per
// DICOM PS3.7 but common enough to require interoperability.
func TestCMoveReadRSP_AcceptsPendingWithZeroRemaining(t *testing.T) {
	cmd := media.NewEmptyDCMObj()
	cmd.Write(tags.CommandField, dicomcommand.CMoveResponse)
	cmd.Write(tags.Status, dicomstatus.Pending)
	cmd.Write(tags.CommandDataSetType, 0x0101)
	cmd.Write(tags.NumberOfRemainingSuboperations, 0)
	cmd.Write(tags.NumberOfCompletedSuboperations, 1)
	cmd.Write(tags.NumberOfFailedSuboperations, 0)
	cmd.Write(tags.NumberOfWarningSuboperations, 0)

	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{cmd}
	pending := 5

	_, status, err := dimse.CMoveReadRSP(m, &pending)
	if err != nil {
		t.Fatalf("CMoveReadRSP: unexpected error for Pending+remaining=0: %v", err)
	}
	if status != dicomstatus.Pending {
		t.Errorf("CMoveReadRSP: status %04X want Pending", status)
	}
	if pending != 0 {
		t.Errorf("CMoveReadRSP: pending count %d want 0", pending)
	}
}
