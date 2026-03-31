package dimse_test

import (
	"bufio"
	"crypto/tls"
	"errors"
	"net"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
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
	media.InitDict()
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
func (m *mockPDU) Connect(_, _ string) error                                       { return nil }
func (m *mockPDU) ConnectTLS(_, _ string, _ *tls.Config) error                     { return nil }
func (m *mockPDU) Close()                                                          {}
func (m *mockPDU) GetCalledAE() string                                             { return "CALLED" }
func (m *mockPDU) GetCallingAE() string                                            { return "CALLING" }
func (m *mockPDU) SetCalledAE(_ string)                                            {}
func (m *mockPDU) SetCallingAE(_ string)                                           {}
func (m *mockPDU) SetConn(_ *bufio.ReadWriter)                                     {}
func (m *mockPDU) SetNetConn(_ net.Conn)                                           {}
func (m *mockPDU) AddPresContexts(_ network.PresentationContext)                   {}
func (m *mockPDU) SetOnAssociationRequest(_ func(network.AssociationRequest) bool) {}

const ctUID = "1.2.840.10008.5.1.4.1.1.2"

func echoRQObj() media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.WriteUint16(tags.CommandField, dicomcommand.CEchoRequest)
	obj.WriteUint16(tags.MessageID, 1)
	obj.WriteUint16(tags.CommandDataSetType, 0x0101)
	return obj
}

func findRQObj(queryLevel string) media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.WriteUint16(tags.CommandField, dicomcommand.CFindRequest)
	obj.WriteUint16(tags.MessageID, 1)
	obj.WriteUint16(tags.CommandDataSetType, 0x0102)
	obj.WriteString(tags.QueryRetrieveLevel, queryLevel)
	return obj
}

func dicomDataObj(sopClassUID, sopInstanceUID string) media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.WriteString(tags.SOPClassUID, sopClassUID)
	obj.WriteString(tags.SOPInstanceUID, sopInstanceUID)
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
	obj.WriteUint16(tags.CommandField, dicomcommand.CStoreRequest)
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
	if cf := w.GetUShort(tags.CommandField); cf != dicomcommand.CEchoRequest {
		t.Errorf("CEchoWriteRQ: CommandField %04X want %04X", cf, dicomcommand.CEchoRequest)
	}
	if cdt := w.GetUShort(tags.CommandDataSetType); cdt != 0x0101 {
		t.Errorf("CEchoWriteRQ: CommandDataSetType %04X want 0101", cdt)
	}
}

func TestCEchoReadRSP_Success(t *testing.T) {
	rsp := media.NewEmptyDCMObj()
	rsp.WriteUint16(tags.CommandField, dicomcommand.CEchoResponse)
	rsp.WriteUint16(tags.Status, dicomstatus.Success)
	m := newMockPDU(ctUID)
	m.nextPDUs = []media.DICOMObject{rsp}
	if err := dimse.CEchoReadRSP(m); err != nil {
		t.Errorf("CEchoReadRSP: %v", err)
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
	rq.WriteString(tags.AffectedSOPClassUID, ctUID)
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	if err := dimse.CEchoWriteRSP(m, rq); err != nil {
		t.Fatalf("CEchoWriteRSP: %v", err)
	}
	if len(m.written) == 0 {
		t.Fatal("CEchoWriteRSP: nothing written")
	}
	rsp := m.written[0]
	if cf := rsp.GetUShort(tags.CommandField); cf != dicomcommand.CEchoResponse {
		t.Errorf("CEchoWriteRSP: CommandField %04X want CEchoResponse", cf)
	}
	if st := rsp.GetUShort(tags.Status); st != dicomstatus.Success {
		t.Errorf("CEchoWriteRSP: Status %04X want Success", st)
	}
}

func TestCEchoWriteRSP_ErrorWhenNoSOPClass(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CEchoWriteRSP(m, media.NewEmptyDCMObj()); err == nil {
		t.Error("CEchoWriteRSP: want error when AffectedSOPClassUID absent")
	}
}

// ── C-FIND ────────────────────────────────────────────────────────────────────

func TestCFindWriteRQ_WritesCommandAndData(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CFindWriteRQ(m, findRQObj("STUDY")); err != nil {
		t.Fatalf("CFindWriteRQ: %v", err)
	}
	if len(m.written) != 2 {
		t.Fatalf("CFindWriteRQ: want 2 writes, got %d", len(m.written))
	}
	if cf := m.written[0].GetUShort(tags.CommandField); cf != dicomcommand.CFindRequest {
		t.Errorf("CFindWriteRQ: CommandField %04X want CFindRequest", cf)
	}
}

func TestCFindReadRSP_PendingThenFinal(t *testing.T) {
	pending := media.NewEmptyDCMObj()
	pending.WriteUint16(tags.CommandField, dicomcommand.CFindResponse)
	pending.WriteUint16(tags.Status, dicomstatus.Pending)
	pending.WriteUint16(tags.CommandDataSetType, 0x0001)

	dataset := media.NewEmptyDCMObj()
	dataset.WriteString(tags.PatientID, "P001")

	final := media.NewEmptyDCMObj()
	final.WriteUint16(tags.CommandField, dicomcommand.CFindResponse)
	final.WriteUint16(tags.Status, dicomstatus.Success)
	final.WriteUint16(tags.CommandDataSetType, 0x0101)

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

func TestCFindWriteRSP_Pending(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := findRQObj("STUDY")
	rq.WriteString(tags.AffectedSOPClassUID, ctUID)
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	result := media.NewEmptyDCMObj()
	result.WriteString(tags.PatientID, "P001")
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
	rq.WriteString(tags.AffectedSOPClassUID, ctUID)
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

// ── C-STORE ───────────────────────────────────────────────────────────────────

func TestCStoreWriteRQ_WritesCommandAndData(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CStoreWriteRQ(m, dicomDataObj(ctUID, "1.2.3.4.5.6")); err != nil {
		t.Fatalf("CStoreWriteRQ: %v", err)
	}
	if len(m.written) != 2 {
		t.Fatalf("CStoreWriteRQ: want 2 writes, got %d", len(m.written))
	}
	if cf := m.written[0].GetUShort(tags.CommandField); cf != dicomcommand.CStoreRequest {
		t.Errorf("CStoreWriteRQ: CommandField %04X want CStoreRequest", cf)
	}
}

func TestCStoreWriteRQ_CommandGroupLengthNonZeroForEvenUID(t *testing.T) {
	evenUID := "1.2.840.10008.5.1.4.1"
	m := newMockPDU(ctUID)
	if err := dimse.CStoreWriteRQ(m, dicomDataObj(ctUID, evenUID)); err != nil {
		t.Fatalf("CStoreWriteRQ (even UID): %v", err)
	}
	if m.written[0].GetUInt(tags.CommandGroupLength) == 0 {
		t.Error("CStoreWriteRQ: CommandGroupLength must not be zero for even-length UID")
	}
}

func TestCStoreWriteRQ_AffectedSOPInstanceUIDAlwaysPresent(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CStoreWriteRQ(m, dicomDataObj(ctUID, "1.2.3")); err != nil {
		t.Fatalf("CStoreWriteRQ: %v", err)
	}
	if uid := m.written[0].GetString(tags.AffectedSOPInstanceUID); uid == "" {
		t.Error("CStoreWriteRQ: AffectedSOPInstanceUID must always be present (PS3.7 §C.3.1)")
	}
}

func TestCStoreReadRSP_Success(t *testing.T) {
	rsp := media.NewEmptyDCMObj()
	rsp.WriteUint16(tags.CommandField, dicomcommand.CStoreResponse)
	rsp.WriteUint16(tags.Status, dicomstatus.Success)
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

func TestCStoreWriteRSP_WritesResponse(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.WriteString(tags.AffectedSOPClassUID, ctUID)
	rq.WriteString(tags.AffectedSOPInstanceUID, "1.2.3.4")
	rq.WriteUint16(tags.MessageID, 42)
	if err := dimse.CStoreWriteRSP(m, rq, dicomstatus.Success); err != nil {
		t.Fatalf("CStoreWriteRSP: %v", err)
	}
	if len(m.written) == 0 {
		t.Fatal("CStoreWriteRSP: nothing written")
	}
	rsp := m.written[0]
	if cf := rsp.GetUShort(tags.CommandField); cf != dicomcommand.CStoreResponse {
		t.Errorf("CStoreWriteRSP: CommandField %04X want CStoreResponse", cf)
	}
	if st := rsp.GetUShort(tags.Status); st != dicomstatus.Success {
		t.Errorf("CStoreWriteRSP: Status %04X want Success", st)
	}
}

func TestCStoreWriteRSP_CommandGroupLengthUsesInstanceUID(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.WriteString(tags.AffectedSOPClassUID, ctUID) // 25 chars
	rq.WriteString(tags.AffectedSOPInstanceUID, "1.2.3")
	rq.WriteUint16(tags.MessageID, 1)
	if err := dimse.CStoreWriteRSP(m, rq, dicomstatus.Success); err != nil {
		t.Fatalf("CStoreWriteRSP: %v", err)
	}
	if m.written[0].GetUInt(tags.CommandGroupLength) == 0 {
		t.Error("CStoreWriteRSP: CommandGroupLength must not be zero")
	}
}

func TestCStoreWriteRSP_ErrorNoSOPClass(t *testing.T) {
	m := newMockPDU(ctUID)
	if err := dimse.CStoreWriteRSP(m, media.NewEmptyDCMObj(), dicomstatus.Success); err == nil {
		t.Error("CStoreWriteRSP: want error when AffectedSOPClassUID absent")
	}
}

// ── C-MOVE ────────────────────────────────────────────────────────────────────

func TestCMoveWriteRQ_WritesCommandAndData(t *testing.T) {
	m := newMockPDU(ctUID)
	q := media.NewEmptyDCMObj()
	q.WriteString(tags.QueryRetrieveLevel, "STUDY")
	if err := dimse.CMoveWriteRQ(m, q, "DEST_AE"); err != nil {
		t.Fatalf("CMoveWriteRQ: %v", err)
	}
	if len(m.written) != 2 {
		t.Fatalf("CMoveWriteRQ: want 2 writes, got %d", len(m.written))
	}
	if cf := m.written[0].GetUShort(tags.CommandField); cf != dicomcommand.CMoveRequest {
		t.Errorf("CMoveWriteRQ: CommandField %04X want CMoveRequest", cf)
	}
}

func TestCMoveWriteRSP_AllSubopCountsPresent(t *testing.T) {
	m := newMockPDU(ctUID)
	rq := media.NewEmptyDCMObj()
	rq.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	rq.WriteString(tags.AffectedSOPClassUID, ctUID)
	rq.WriteUint16(tags.MessageID, 1)
	if err := dimse.CMoveWriteRSP(m, rq, dicomstatus.Success, 0, 3, 1, 0); err != nil {
		t.Fatalf("CMoveWriteRSP: %v", err)
	}
	rsp := m.written[0]
	if cf := rsp.GetUShort(tags.CommandField); cf != dicomcommand.CMoveResponse {
		t.Errorf("CMoveWriteRSP: CommandField %04X want CMoveResponse", cf)
	}
	_ = rsp.GetUShort(tags.NumberOfRemainingSuboperations)
	_ = rsp.GetUShort(tags.NumberOfCompletedSuboperations)
	_ = rsp.GetUShort(tags.NumberOfFailedSuboperations)
	_ = rsp.GetUShort(tags.NumberOfWarningSuboperations)
}

func TestCMoveReadRSP_PendingThenFinal(t *testing.T) {
	pending := media.NewEmptyDCMObj()
	pending.WriteUint16(tags.CommandField, dicomcommand.CMoveResponse)
	pending.WriteUint16(tags.Status, dicomstatus.Pending)
	pending.WriteUint16(tags.CommandDataSetType, 0x0001)
	pending.WriteUint16(tags.NumberOfRemainingSuboperations, 2)

	dataset := media.NewEmptyDCMObj()

	final := media.NewEmptyDCMObj()
	final.WriteUint16(tags.CommandField, dicomcommand.CMoveResponse)
	final.WriteUint16(tags.Status, dicomstatus.Success)
	final.WriteUint16(tags.CommandDataSetType, 0x0101)

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

	_, st2, err2 := dimse.CMoveReadRSP(m, &cnt)
	if err2 != nil {
		t.Fatalf("CMoveReadRSP (final): %v", err2)
	}
	if st2 != dicomstatus.Success {
		t.Errorf("CMoveReadRSP: final status %04X want Success", st2)
	}
}

func TestCMoveReadRSP_ErrorOnNoPDU(t *testing.T) {
	m := newMockPDU(ctUID)
	var c int
	if _, _, err := dimse.CMoveReadRSP(m, &c); err == nil {
		t.Error("CMoveReadRSP: want error when NextPDU fails")
	}
}
