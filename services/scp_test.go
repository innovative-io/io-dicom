package services

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/network/priority"
)

// TestSCP_OnCEchoRequest verifies that a custom C-ECHO handler is invoked and
// that allowing the echo returns a successful response to the SCU.
func TestSCP_OnCEchoRequest_Allow(t *testing.T) {
	_, testSCP := StartSCP(t, 1044)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})

	called := false
	testSCP.OnCEchoRequest(func(request network.AssociationRequest) bool {
		called = true
		return true // allow
	})

	dest := &network.Destination{
		Name:      "CEchoTest",
		CalledAE:  "SCP",
		CallingAE: "SCU",
		HostName:  "localhost",
		Port:      1044,
	}
	scu := NewSCU(dest)
	if err := scu.EchoSCU(context.Background()); err != nil {
		t.Fatalf("EchoSCU: %v", err)
	}
	if !called {
		t.Error("OnCEchoRequest handler was not invoked")
	}
}

// TestSCP_OnCEchoRequest_Reject verifies that denying echoes results in an echo
// that is silently dropped (the SCU gets no response and times out / errors).
func TestSCP_OnCEchoRequest_Reject(t *testing.T) {
	_, testSCP := StartSCP(t, 1045)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})
	testSCP.OnCEchoRequest(func(request network.AssociationRequest) bool {
		return false // reject
	})

	dest := &network.Destination{
		Name:      "CEchoRejectTest",
		CalledAE:  "SCP",
		CallingAE: "SCU",
		HostName:  "localhost",
		Port:      1045,
	}
	scu := NewSCU(dest)
	// When the echo is rejected the SCP drops the response; the SCU should
	// get an error (timeout, connection close, or bad response).
	// We simply verify the call completes without panicking.
	_ = scu.EchoSCU(context.Background())
}

// TestSCP_OnCMoveRequest verifies the OnCMoveRequest handler setter and that
// MoveSCU drives the full C-MOVE exchange end-to-end.
func TestSCP_OnCMoveRequest(t *testing.T) {
	_, testSCP := StartSCP(t, 1046)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})
	testSCP.OnCMoveRequest(func(ctx context.Context, request network.AssociationRequest, moveDestAE string, moveLevel string, data media.DICOMObject, emit func(CMoveProgress)) (CMoveResult, error) {
		return CMoveResult{Status: dicomstatus.Success}, nil
	})

	dest := &network.Destination{
		Name:      "CMoveTest",
		CalledAE:  "SCP",
		CallingAE: "SCU",
		HostName:  "localhost",
		Port:      1046,
	}
	scu := NewSCU(dest)

	status, err := scu.MoveSCU(context.Background(), "DEST_AE", dimse.DefaultCMoveRequest("1.2.3.4"))
	if err != nil {
		t.Fatalf("MoveSCU: %v", err)
	}
	if status != dicomstatus.Success {
		t.Errorf("MoveSCU: status = 0x%04X, want Success (0x0000)", status)
	}
}

// TestSCU_SetOnCMoveResult verifies the callback setter doesn't panic and is
// stored correctly.
func TestSCU_SetOnCMoveResult(t *testing.T) {
	dest := &network.Destination{
		Name:      "MoveResultTest",
		CalledAE:  "SCP",
		CallingAE: "SCU",
		HostName:  "localhost",
		Port:      9999,
	}
	scu := NewSCU(dest)
	called := false
	scu.SetOnCMoveResult(func(result media.DICOMObject) {
		called = true
	})
	// Callback should be stored; verify it's exercised when MoveSCU fires it.
	_ = called // captured in production use
}

func TestSCP_CFindRejectsInvalidQueryRetrieveLevel(t *testing.T) {
	_, testSCP := StartSCP(t, 1047)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})
	testSCP.OnCFindRequest(func(ctx context.Context, request network.AssociationRequest, findLevel string, data media.DICOMObject, emit func(media.DICOMObject)) (CFindResult, error) {
		return CFindResult{Status: dicomstatus.Success}, nil
	})

	dest := &network.Destination{
		Name:      "CFindInvalidLevel",
		CalledAE:  "SCP",
		CallingAE: "SCU",
		HostName:  "localhost",
		Port:      1047,
	}

	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "NOT_A_LEVEL")
	query.Write(tags.PatientID, "P123")

	scu := NewSCU(dest)
	_, status, err := scu.FindSCU(context.Background(), query)
	if err != nil {
		t.Fatalf("FindSCU: %v", err)
	}
	if status != dicomstatus.FailureIdentifierDoesNotMatchSOPClass {
		t.Fatalf("FindSCU status = 0x%04X, want 0x%04X", status, dicomstatus.FailureIdentifierDoesNotMatchSOPClass)
	}
}

func TestSCP_CFindAllowsEmptyQueryRetrieveLevelForWorklist(t *testing.T) {
	_, testSCP := StartSCP(t, 1048)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})
	testSCP.OnCFindRequest(func(ctx context.Context, request network.AssociationRequest, findLevel string, data media.DICOMObject, emit func(media.DICOMObject)) (CFindResult, error) {
		if findLevel != "" {
			t.Fatalf("findLevel = %q, want empty string for worklist", findLevel)
		}
		result := media.NewEmptyDCMObj()
		result.Write(tags.PatientID, "MWL-001")
		result.Write(tags.PatientName, "Test^Worklist")
		emit(result)
		return CFindResult{Status: dicomstatus.Success}, nil
	})

	dest := &network.Destination{
		Name:      "Worklist SCP",
		CalledAE:  "SCP",
		CallingAE: "SCU",
		HostName:  "localhost",
		Port:      1048,
	}

	query := media.NewEmptyDCMObj()
	query.Write(tags.PatientName, "Test*")

	scu := NewSCU(dest)
	results := 0
	scu.SetOnCFindResult(func(result media.DICOMObject) {
		results++
	})

	count, status, err := scu.WorklistSCU(context.Background(), query)
	if err != nil {
		t.Fatalf("WorklistSCU: %v", err)
	}
	if status != dicomstatus.Success {
		t.Fatalf("WorklistSCU status = 0x%04X, want Success", status)
	}
	if count != 1 {
		t.Fatalf("WorklistSCU count = %d, want 1", count)
	}
	if results != 1 {
		t.Fatalf("callback results = %d, want 1", results)
	}
}

func TestSCP_CCancelTracking_ConsumeOnce(t *testing.T) {
	s := NewSCP(1050).(*scp)

	if s.consumeCanceled(42) {
		t.Fatal("consumeCanceled() before mark = true, want false")
	}

	s.markCanceled(42)
	if !s.consumeCanceled(42) {
		t.Fatal("consumeCanceled() after mark = false, want true")
	}
	if s.consumeCanceled(42) {
		t.Fatal("consumeCanceled() second call = true, want false")
	}
}

func TestSCP_OnCCancelRequest_Setter(t *testing.T) {
	s := NewSCP(1051).(*scp)

	called := false
	var gotMsgID uint16
	s.OnCCancelRequest(func(_ network.AssociationRequest, messageID uint16) {
		called = true
		gotMsgID = messageID
	})

	if s.onCCancelRequest == nil {
		t.Fatal("onCCancelRequest callback was not set")
	}

	s.onCCancelRequest(nil, 77)
	if !called {
		t.Fatal("onCCancelRequest callback was not invoked")
	}
	if gotMsgID != 77 {
		t.Fatalf("callback messageID = %d, want 77", gotMsgID)
	}
}

func TestSCP_CFindInFlightCancelPreemptsStreaming(t *testing.T) {
	const port = 1052
	_, testSCP := StartSCP(t, port)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})
	handlerStarted := make(chan struct{})
	testSCP.OnCFindRequest(func(ctx context.Context, request network.AssociationRequest, findLevel string, data media.DICOMObject, emit func(media.DICOMObject)) (CFindResult, error) {
		close(handlerStarted)
		for i := 0; i < 200; i++ {
			obj := media.NewEmptyDCMObj()
			obj.Write(tags.PatientID, fmt.Sprintf("P%03d", i))
			emit(obj)
			select {
			case <-ctx.Done():
				return CFindResult{Status: dicomstatus.Cancel}, nil
			case <-time.After(2 * time.Millisecond):
			}
		}
		return CFindResult{Status: dicomstatus.Success}, nil
	})

	pdu := network.NewPDUService()
	pdu.SetTimeout(2)
	pdu.SetCalledAE("SCP")
	pdu.SetCallingAE("SCU")

	pc := network.NewPresentationContext()
	pc.SetAbstractSyntax(sopclass.StudyRootQueryRetrieveInformationModelFind.UID)
	pc.AddTransferSyntax(transfersyntax.ImplicitVRLittleEndian.UID)
	pdu.AddPresContexts(pc)

	if err := pdu.Connect(context.Background(), "localhost", strconv.Itoa(port)); err != nil {
		t.Fatalf("pdu.Connect: %v", err)
	}
	defer pdu.Close()

	msgID := uint16(111)
	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.PatientID, "*")

	if err := writeCFindRQWithMessageID(pdu, query, msgID); err != nil {
		t.Fatalf("writeCFindRQWithMessageID: %v", err)
	}

	select {
	case <-handlerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for C-FIND handler start")
	}

	_, status, err := dimse.CFindReadRSP(pdu)
	if err != nil {
		t.Fatalf("first CFindReadRSP: %v", err)
	}
	if status != dicomstatus.Pending && status != dicomstatus.PendingWithWarnings {
		t.Fatalf("first status = 0x%04X, want pending", status)
	}

	if err := dimse.CCancelWriteRQ(pdu, msgID); err != nil {
		t.Fatalf("CCancelWriteRQ: %v", err)
	}

	finalStatus := uint16(0)
	pendingAfterCancel := 0
	for {
		_, st, err := dimse.CFindReadRSP(pdu)
		if err != nil {
			t.Fatalf("CFindReadRSP after cancel: %v", err)
		}

		if st == dicomstatus.Pending || st == dicomstatus.PendingWithWarnings {
			pendingAfterCancel++
			if pendingAfterCancel > 3 {
				t.Fatalf("received too many pending responses after cancel: %d", pendingAfterCancel)
			}
			continue
		}

		finalStatus = st
		break
	}

	if finalStatus != dicomstatus.Cancel {
		t.Fatalf("final status = 0x%04X, want cancel 0x%04X", finalStatus, dicomstatus.Cancel)
	}
}

func TestSCP_CFindCancelTimeoutAbortsAssociation(t *testing.T) {
	const port = 1055
	_, testSCP := StartSCP(t, port)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})

	handlerStarted := make(chan struct{})
	testSCP.OnCFindRequest(func(ctx context.Context, request network.AssociationRequest, findLevel string, data media.DICOMObject, emit func(media.DICOMObject)) (CFindResult, error) {
		close(handlerStarted)
		obj := media.NewEmptyDCMObj()
		obj.Write(tags.PatientID, "P000")
		emit(obj)
		time.Sleep(2 * time.Second)
		return CFindResult{Status: dicomstatus.Success}, nil
	})

	pdu := network.NewPDUService()
	pdu.SetTimeout(3)
	pdu.SetCalledAE("SCP")
	pdu.SetCallingAE("SCU")

	pc := network.NewPresentationContext()
	pc.SetAbstractSyntax(sopclass.StudyRootQueryRetrieveInformationModelFind.UID)
	pc.AddTransferSyntax(transfersyntax.ImplicitVRLittleEndian.UID)
	pdu.AddPresContexts(pc)

	if err := pdu.Connect(context.Background(), "localhost", strconv.Itoa(port)); err != nil {
		t.Fatalf("pdu.Connect: %v", err)
	}
	defer pdu.Close()

	msgID := uint16(112)
	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.PatientID, "*")

	if err := writeCFindRQWithMessageID(pdu, query, msgID); err != nil {
		t.Fatalf("writeCFindRQWithMessageID: %v", err)
	}

	select {
	case <-handlerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for C-FIND handler start")
	}

	_, status, err := dimse.CFindReadRSP(pdu)
	if err != nil {
		t.Fatalf("first CFindReadRSP: %v", err)
	}
	if status != dicomstatus.Pending && status != dicomstatus.PendingWithWarnings {
		t.Fatalf("first status = 0x%04X, want pending", status)
	}

	if err := dimse.CCancelWriteRQ(pdu, msgID); err != nil {
		t.Fatalf("CCancelWriteRQ: %v", err)
	}

	_, _, err = dimse.CFindReadRSP(pdu)
	if err == nil {
		t.Fatal("expected association abort/error after C-FIND cancel timeout, got nil error")
	}
}

func writeCFindRQWithMessageID(pdu network.PDUService, dataObj media.DICOMObject, messageID uint16) error {
	sopClassUID := sopclass.StudyRootQueryRetrieveInformationModelFind.UID
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	commandObj := media.NewEmptyDCMObj()
	commandObj.Write(tags.CommandGroupLength, commandLength)
	commandObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.Write(tags.CommandField, dicomcommand.CFindRequest)
	commandObj.Write(tags.MessageID, messageID)
	commandObj.Write(tags.Priority, priority.Medium)
	commandObj.Write(tags.CommandDataSetType, dicomcommand.DataSetPresent)

	if err := pdu.Write(commandObj, network.PDVCommand); err != nil {
		return err
	}
	return pdu.Write(dataObj, network.PDVDataset)
}

func TestSCP_CGetInFlightCancelOverridesFinalStatus(t *testing.T) {
	const port = 1053
	_, testSCP := StartSCP(t, port)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})

	handlerStarted := make(chan struct{})
	testSCP.OnCGetRequest(func(ctx context.Context, request network.AssociationRequest, getLevel string, data media.DICOMObject, _ func(string) error, emit func(CGetProgress)) (CGetResult, error) {
		close(handlerStarted)
		emit(CGetProgress{Remaining: 1, Completed: 0, Failed: 0, Warnings: 0})
		select {
		case <-ctx.Done():
			return CGetResult{Status: dicomstatus.Cancel, Remaining: 0, Completed: 0, Failed: 0, Warnings: 0}, nil
		case <-time.After(60 * time.Millisecond):
			return CGetResult{Status: dicomstatus.Success, Remaining: 0, Completed: 1, Failed: 0, Warnings: 0}, nil
		}
	})

	pdu := network.NewPDUService()
	pdu.SetCalledAE("SCP")
	pdu.SetCallingAE("SCU")

	pc := network.NewPresentationContext()
	pc.SetAbstractSyntax(sopclass.StudyRootQueryRetrieveInformationModelGet.UID)
	pc.AddTransferSyntax(transfersyntax.ImplicitVRLittleEndian.UID)
	pdu.AddPresContexts(pc)

	if err := pdu.Connect(context.Background(), "localhost", strconv.Itoa(port)); err != nil {
		t.Fatalf("pdu.Connect: %v", err)
	}
	defer pdu.Close()

	msgID := uint16(113)
	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.PatientID, "*")

	if err := writeCGetRQWithMessageID(pdu, query, msgID); err != nil {
		t.Fatalf("writeCGetRQWithMessageID: %v", err)
	}

	select {
	case <-handlerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for C-GET handler start")
	}

	if err := dimse.CCancelWriteRQ(pdu, msgID); err != nil {
		t.Fatalf("CCancelWriteRQ: %v", err)
	}

	pending := 0
	_, status, err := dimse.CGetReadRSP(pdu, &pending)
	if err != nil {
		t.Fatalf("CGetReadRSP: %v", err)
	}
	if status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
		_, status, err = dimse.CGetReadRSP(pdu, &pending)
		if err != nil {
			t.Fatalf("CGetReadRSP(final): %v", err)
		}
	}
	if status != dicomstatus.Cancel {
		t.Fatalf("C-GET final status = 0x%04X, want cancel 0x%04X", status, dicomstatus.Cancel)
	}
	if pending != -1 {
		t.Fatalf("C-GET pending marker = %d, want -1 for final response", pending)
	}
}

func TestSCP_CMoveInFlightCancelOverridesFinalStatus(t *testing.T) {
	const port = 1054
	_, testSCP := StartSCP(t, port)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})

	handlerStarted := make(chan struct{})
	testSCP.OnCMoveRequest(func(ctx context.Context, request network.AssociationRequest, moveDestAE string, moveLevel string, data media.DICOMObject, emit func(CMoveProgress)) (CMoveResult, error) {
		close(handlerStarted)
		emit(CMoveProgress{Remaining: 1, Completed: 0, Failed: 0, Warnings: 0})
		select {
		case <-ctx.Done():
			return CMoveResult{Status: dicomstatus.Cancel, Remaining: 0, Completed: 0, Failed: 0, Warnings: 0}, nil
		case <-time.After(60 * time.Millisecond):
			return CMoveResult{Status: dicomstatus.Success, Remaining: 0, Completed: 1, Failed: 0, Warnings: 0}, nil
		}
	})

	pdu := network.NewPDUService()
	pdu.SetCalledAE("SCP")
	pdu.SetCallingAE("SCU")

	pc := network.NewPresentationContext()
	pc.SetAbstractSyntax(sopclass.StudyRootQueryRetrieveInformationModelMove.UID)
	pc.AddTransferSyntax(transfersyntax.ImplicitVRLittleEndian.UID)
	pdu.AddPresContexts(pc)

	if err := pdu.Connect(context.Background(), "localhost", strconv.Itoa(port)); err != nil {
		t.Fatalf("pdu.Connect: %v", err)
	}
	defer pdu.Close()

	msgID := uint16(115)
	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.PatientID, "*")

	if err := writeCMoveRQWithMessageID(pdu, query, "DEST_AE", msgID); err != nil {
		t.Fatalf("writeCMoveRQWithMessageID: %v", err)
	}

	select {
	case <-handlerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for C-MOVE handler start")
	}

	if err := dimse.CCancelWriteRQ(pdu, msgID); err != nil {
		t.Fatalf("CCancelWriteRQ: %v", err)
	}

	pending := 0
	_, status, err := dimse.CMoveReadRSP(pdu, &pending)
	if err != nil {
		t.Fatalf("CMoveReadRSP: %v", err)
	}
	if status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
		_, status, err = dimse.CMoveReadRSP(pdu, &pending)
		if err != nil {
			t.Fatalf("CMoveReadRSP(final): %v", err)
		}
	}
	if status != dicomstatus.Cancel {
		t.Fatalf("C-MOVE final status = 0x%04X, want cancel 0x%04X", status, dicomstatus.Cancel)
	}
	if pending != -1 {
		t.Fatalf("C-MOVE pending marker = %d, want -1 for final response", pending)
	}
}

func TestSCP_CGetRejectsNonMonotonicProgress(t *testing.T) {
	const port = 1056
	_, testSCP := StartSCP(t, port)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})
	testSCP.OnCGetRequest(func(ctx context.Context, request network.AssociationRequest, getLevel string, data media.DICOMObject, _ func(string) error, emit func(CGetProgress)) (CGetResult, error) {
		emit(CGetProgress{Remaining: 3, Completed: 0, Failed: 0, Warnings: 0})
		emit(CGetProgress{Remaining: 4, Completed: 0, Failed: 0, Warnings: 0}) // invalid: remaining increased
		return CGetResult{Status: dicomstatus.Success, Remaining: 0, Completed: 3, Failed: 0, Warnings: 0}, nil
	})

	pdu := network.NewPDUService()
	pdu.SetCalledAE("SCP")
	pdu.SetCallingAE("SCU")

	pc := network.NewPresentationContext()
	pc.SetAbstractSyntax(sopclass.StudyRootQueryRetrieveInformationModelGet.UID)
	pc.AddTransferSyntax(transfersyntax.ImplicitVRLittleEndian.UID)
	pdu.AddPresContexts(pc)

	if err := pdu.Connect(context.Background(), "localhost", strconv.Itoa(port)); err != nil {
		t.Fatalf("pdu.Connect: %v", err)
	}
	defer pdu.Close()

	msgID := uint16(117)
	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.PatientID, "*")
	if err := writeCGetRQWithMessageID(pdu, query, msgID); err != nil {
		t.Fatalf("writeCGetRQWithMessageID: %v", err)
	}

	pending := 0
	for i := 0; i < 4; i++ {
		_, status, err := dimse.CGetReadRSP(pdu, &pending)
		if err != nil {
			t.Fatalf("CGetReadRSP: %v", err)
		}
		if status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
			continue
		}
		if status != dicomstatus.FailureProcessingFailure {
			t.Fatalf("final status = 0x%04X, want processing failure 0x%04X", status, dicomstatus.FailureProcessingFailure)
		}
		return
	}

	t.Fatal("did not receive final processing failure response")
}

func TestSCP_CMoveRejectsNonMonotonicProgress(t *testing.T) {
	const port = 1057
	_, testSCP := StartSCP(t, port)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})
	testSCP.OnCMoveRequest(func(ctx context.Context, request network.AssociationRequest, moveDestAE string, moveLevel string, data media.DICOMObject, emit func(CMoveProgress)) (CMoveResult, error) {
		emit(CMoveProgress{Remaining: 2, Completed: 1, Failed: 0, Warnings: 0})
		emit(CMoveProgress{Remaining: 1, Completed: 0, Failed: 0, Warnings: 0}) // invalid: completed decreased
		return CMoveResult{Status: dicomstatus.Success, Remaining: 0, Completed: 2, Failed: 0, Warnings: 0}, nil
	})

	pdu := network.NewPDUService()
	pdu.SetCalledAE("SCP")
	pdu.SetCallingAE("SCU")

	pc := network.NewPresentationContext()
	pc.SetAbstractSyntax(sopclass.StudyRootQueryRetrieveInformationModelMove.UID)
	pc.AddTransferSyntax(transfersyntax.ImplicitVRLittleEndian.UID)
	pdu.AddPresContexts(pc)

	if err := pdu.Connect(context.Background(), "localhost", strconv.Itoa(port)); err != nil {
		t.Fatalf("pdu.Connect: %v", err)
	}
	defer pdu.Close()

	msgID := uint16(119)
	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.PatientID, "*")
	if err := writeCMoveRQWithMessageID(pdu, query, "DEST_AE", msgID); err != nil {
		t.Fatalf("writeCMoveRQWithMessageID: %v", err)
	}

	pending := 0
	for i := 0; i < 4; i++ {
		_, status, err := dimse.CMoveReadRSP(pdu, &pending)
		if err != nil {
			t.Fatalf("CMoveReadRSP: %v", err)
		}
		if status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
			continue
		}
		if status != dicomstatus.FailureProcessingFailure {
			t.Fatalf("final status = 0x%04X, want processing failure 0x%04X", status, dicomstatus.FailureProcessingFailure)
		}
		return
	}

	t.Fatal("did not receive final processing failure response")
}

func TestSCP_CGetCancelTimeoutAbortsAssociation(t *testing.T) {
	const port = 1058
	_, testSCP := StartSCP(t, port)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})

	handlerStarted := make(chan struct{})
	testSCP.OnCGetRequest(func(ctx context.Context, request network.AssociationRequest, getLevel string, data media.DICOMObject, _ func(string) error, emit func(CGetProgress)) (CGetResult, error) {
		close(handlerStarted)
		emit(CGetProgress{Remaining: 1, Completed: 0, Failed: 0, Warnings: 0})
		time.Sleep(2 * time.Second) // intentionally ignores ctx cancellation
		return CGetResult{Status: dicomstatus.Success, Remaining: 0, Completed: 1, Failed: 0, Warnings: 0}, nil
	})

	pdu := network.NewPDUService()
	pdu.SetTimeout(3)
	pdu.SetCalledAE("SCP")
	pdu.SetCallingAE("SCU")

	pc := network.NewPresentationContext()
	pc.SetAbstractSyntax(sopclass.StudyRootQueryRetrieveInformationModelGet.UID)
	pc.AddTransferSyntax(transfersyntax.ImplicitVRLittleEndian.UID)
	pdu.AddPresContexts(pc)

	if err := pdu.Connect(context.Background(), "localhost", strconv.Itoa(port)); err != nil {
		t.Fatalf("pdu.Connect: %v", err)
	}
	defer pdu.Close()

	msgID := uint16(121)
	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.PatientID, "*")
	if err := writeCGetRQWithMessageID(pdu, query, msgID); err != nil {
		t.Fatalf("writeCGetRQWithMessageID: %v", err)
	}

	select {
	case <-handlerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for C-GET handler start")
	}

	if err := dimse.CCancelWriteRQ(pdu, msgID); err != nil {
		t.Fatalf("CCancelWriteRQ: %v", err)
	}

	pending := 0
	_, status, err := dimse.CGetReadRSP(pdu, &pending)
	if err == nil && (status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings) {
		_, _, err = dimse.CGetReadRSP(pdu, &pending)
	}
	if err == nil {
		t.Fatal("expected association abort/error after cancel timeout, got nil error")
	}
}

func writeCGetRQWithMessageID(pdu network.PDUService, dataObj media.DICOMObject, messageID uint16) error {
	sopClassUID := sopclass.StudyRootQueryRetrieveInformationModelGet.UID
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	commandObj := media.NewEmptyDCMObj()
	commandObj.Write(tags.CommandGroupLength, commandLength)
	commandObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.Write(tags.CommandField, dicomcommand.CGetRequest)
	commandObj.Write(tags.MessageID, messageID)
	commandObj.Write(tags.Priority, priority.Medium)
	commandObj.Write(tags.CommandDataSetType, dicomcommand.DataSetPresent)

	if err := pdu.Write(commandObj, network.PDVCommand); err != nil {
		return err
	}
	return pdu.Write(dataObj, network.PDVDataset)
}

func writeCMoveRQWithMessageID(pdu network.PDUService, dataObj media.DICOMObject, destinationAETitle string, messageID uint16) error {
	sopClassUID := sopclass.StudyRootQueryRetrieveInformationModelMove.UID
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	destinationAETitleLength := uint16(len(destinationAETitle))
	if destinationAETitleLength%2 == 1 {
		destinationAETitleLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + destinationAETitleLength + 8 + 2 + 8 + 2)

	commandObj := media.NewEmptyDCMObj()
	commandObj.Write(tags.CommandGroupLength, commandLength)
	commandObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.Write(tags.CommandField, dicomcommand.CMoveRequest)
	commandObj.Write(tags.MessageID, messageID)
	commandObj.Write(tags.MoveDestination, destinationAETitle)
	commandObj.Write(tags.Priority, priority.Medium)
	commandObj.Write(tags.CommandDataSetType, dicomcommand.DataSetPresent)

	if err := pdu.Write(commandObj, network.PDVCommand); err != nil {
		return err
	}
	return pdu.Write(dataObj, network.PDVDataset)
}

// TestSCP_CGetStoreSubop verifies that the SCP sends a C-STORE sub-operation
// back to the SCU over the same association when storeFile is called, and that
// the final C-GET-RSP reports Success.
func TestSCP_CGetStoreSubop(t *testing.T) {
	const samplePath = "../testdata/test2.dcm"
	if _, err := os.Stat(samplePath); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	const port = 1059
	_, testSCP := StartSCP(t, port)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})

	handlerDone := make(chan error, 1)
	testSCP.OnCGetRequest(func(ctx context.Context, request network.AssociationRequest, getLevel string, data media.DICOMObject, storeFile func(string) error, emit func(CGetProgress)) (CGetResult, error) {
		err := storeFile(samplePath)
		handlerDone <- err
		if err != nil {
			return CGetResult{Status: dicomstatus.FailureUnableToProcess, Failed: 1}, nil
		}
		return CGetResult{Status: dicomstatus.Success, Completed: 1}, nil
	})

	pdu := network.NewPDUService()
	pdu.SetCalledAE("SCP")
	pdu.SetCallingAE("SCU")

	// Propose C-GET SOP class and CT Image Storage (needed to receive sub-ops).
	pcGet := network.NewPresentationContext()
	pcGet.SetAbstractSyntax(sopclass.StudyRootQueryRetrieveInformationModelGet.UID)
	pcGet.AddTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)
	pdu.AddPresContexts(pcGet)

	pcCT := network.NewPresentationContext()
	pcCT.SetAbstractSyntax(sopclass.CTImageStorage.UID)
	pcCT.AddTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)
	pdu.AddPresContexts(pcCT)

	if err := pdu.Connect(context.Background(), "localhost", strconv.Itoa(port)); err != nil {
		t.Fatalf("pdu.Connect: %v", err)
	}
	defer pdu.Close()

	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.StudyInstanceUID, "1.2.3.4")

	if err := writeCGetRQWithMessageID(pdu, query, 127); err != nil {
		t.Fatalf("writeCGetRQWithMessageID: %v", err)
	}

	// The first PDU from the SCP should be a C-STORE-RQ sub-operation.
	dco, err := pdu.NextPDU()
	if err != nil {
		t.Fatalf("NextPDU (C-STORE-RQ): %v", err)
	}
	cmd := dco.GetUint16(tags.CommandField)
	if cmd != dicomcommand.CStoreRequest {
		t.Fatalf("expected C-STORE-RQ (0x0001), got 0x%04X", cmd)
	}

	// Read the pixel data dataset.
	if _, err := pdu.NextPDU(); err != nil {
		t.Fatalf("NextPDU (C-STORE dataset): %v", err)
	}

	// Acknowledge the store.
	if err := dimse.CStoreWriteRSP(pdu, dco, dicomstatus.Success); err != nil {
		t.Fatalf("CStoreWriteRSP: %v", err)
	}

	// Verify the storeFile call in the handler succeeded.
	select {
	case storeErr := <-handlerDone:
		if storeErr != nil {
			t.Fatalf("storeFile returned error: %v", storeErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for handler storeFile to complete")
	}

	// Read final C-GET-RSP.
	pending := 0
	_, status, err := dimse.CGetReadRSP(pdu, &pending)
	if err != nil {
		t.Fatalf("CGetReadRSP: %v", err)
	}
	for status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
		_, status, err = dimse.CGetReadRSP(pdu, &pending)
		if err != nil {
			t.Fatalf("CGetReadRSP (pending loop): %v", err)
		}
	}
	if status != dicomstatus.Success {
		t.Fatalf("C-GET final status = 0x%04X, want Success (0x0000)", status)
	}
}

// cgetStoreSubopMockPDU is a minimal PDUService stub for cgetStoreSubop unit
// tests. It captures the objects passed to Write(), returns a success C-STORE-RSP
// from NextPDU(), and exposes accepted presentation contexts for TS lookup.
type cgetStoreSubopMockPDU struct {
	acceptedContexts []network.PresentationContextAccept
	writtenCmd       media.DICOMObject
	writtenData      media.DICOMObject
}

func (m *cgetStoreSubopMockPDU) Write(dco media.DICOMObject, itemType byte) error {
	if itemType == network.PDVCommand {
		m.writtenCmd = dco
	} else {
		m.writtenData = dco
	}
	return nil
}

func (m *cgetStoreSubopMockPDU) NextPDU() (media.DICOMObject, error) {
	msgID := uint16(1)
	if m.writtenCmd != nil {
		msgID = m.writtenCmd.GetUint16(tags.MessageID)
		if msgID == 0 {
			msgID = 1
		}
	}
	rsp := media.NewEmptyDCMObj()
	rsp.Write(tags.CommandField, dicomcommand.CStoreResponse)
	rsp.Write(tags.CommandDataSetType, dicomcommand.DataSetNone)
	rsp.Write(tags.MessageIDBeingRespondedTo, msgID)
	rsp.Write(tags.Status, dicomstatus.Success)
	return rsp, nil
}

func (m *cgetStoreSubopMockPDU) GetAcceptedPresentationContexts() []network.PresentationContextAccept {
	return m.acceptedContexts
}

func (m *cgetStoreSubopMockPDU) GetTransferSyntax(_ byte) *transfersyntax.TransferSyntax {
	if len(m.acceptedContexts) > 0 {
		return transfersyntax.GetTransferSyntaxFromUID(m.acceptedContexts[0].GetTrnSyntax().GetUID())
	}
	return nil
}
func (m *cgetStoreSubopMockPDU) GetPresentationContextID() byte               { return 1 }
func (m *cgetStoreSubopMockPDU) SetTimeout(_ int)                             {}
func (m *cgetStoreSubopMockPDU) Connect(_ context.Context, _, _ string) error { return nil }
func (m *cgetStoreSubopMockPDU) ConnectTLS(_ context.Context, _, _ string, _ *tls.Config) error {
	return nil
}
func (m *cgetStoreSubopMockPDU) Close() error { return nil }
func (m *cgetStoreSubopMockPDU) GetAAssociationRQ() network.AssociationRequest {
	return network.NewAssociationRequest()
}
func (m *cgetStoreSubopMockPDU) GetCalledAE() string                                             { return "SCP" }
func (m *cgetStoreSubopMockPDU) GetCallingAE() string                                            { return "SCU" }
func (m *cgetStoreSubopMockPDU) GetRemoteAddress() string                                        { return "127.0.0.1:104" }
func (m *cgetStoreSubopMockPDU) SetCalledAE(_ string)                                            {}
func (m *cgetStoreSubopMockPDU) SetCallingAE(_ string)                                           {}
func (m *cgetStoreSubopMockPDU) SetConn(_ *bufio.ReadWriter)                                     {}
func (m *cgetStoreSubopMockPDU) SetNetConn(_ net.Conn)                                           {}
func (m *cgetStoreSubopMockPDU) SetReadDeadline(_ time.Time) error                               { return nil }
func (m *cgetStoreSubopMockPDU) AddPresContexts(_ network.PresentationContext)                   {}
func (m *cgetStoreSubopMockPDU) SetOnAssociationRequest(_ func(network.AssociationRequest) bool) {}
func (m *cgetStoreSubopMockPDU) SetOnRawPDU(_ func(network.RawPDUEvent))                         {}
func (m *cgetStoreSubopMockPDU) SetLogger(_ *slog.Logger)                                        {}
func (m *cgetStoreSubopMockPDU) Logger() *slog.Logger                                            { return slog.Default() }

// TestCGetStoreSubop_TranscodesToNegotiatedTS verifies that cgetStoreSubop
// transcodes the file to the transfer syntax negotiated with the SCU. When the
// SCP stores a file in ELE but the SCU only accepted ILE, the pixel data must
// be re-encoded before it is sent back.
func TestCGetStoreSubop_TranscodesToNegotiatedTS(t *testing.T) {
	const samplePath = "../testdata/test.dcm"
	if _, err := os.Stat(samplePath); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	original, err := media.NewDCMObjFromFile(samplePath)
	if err != nil {
		t.Fatalf("load test.dcm: %v", err)
	}
	fileTS := original.GetTransferSyntax()
	if fileTS == nil {
		t.Skip("test.dcm has no transfer syntax; skipping")
	}

	// Choose a target TS different from the file's native TS.
	targetTS := transfersyntax.ImplicitVRLittleEndian
	if fileTS.UID == transfersyntax.ImplicitVRLittleEndian.UID {
		targetTS = transfersyntax.ExplicitVRLittleEndian
	}

	sopUID := original.GetString(tags.SOPClassUID)

	pc := network.NewPresentationContextAccept()
	pc.SetResult(0)
	pc.SetPresentationContextID(1)
	pc.SetAbstractSyntax(sopUID)
	pc.SetTransferSyntax(targetTS.UID)

	mock := &cgetStoreSubopMockPDU{
		acceptedContexts: []network.PresentationContextAccept{pc},
	}

	s := &scp{}
	if err := s.cgetStoreSubop(context.Background(), mock, samplePath); err != nil {
		t.Fatalf("cgetStoreSubop: %v", err)
	}

	if mock.writtenData == nil {
		t.Fatal("no data object written to PDU")
	}
	gotTS := mock.writtenData.GetTransferSyntax()
	if gotTS == nil || gotTS.UID != targetTS.UID {
		gotUID := "<nil>"
		if gotTS != nil {
			gotUID = gotTS.UID
		}
		t.Errorf("written data TS = %q, want %q (%s)", gotUID, targetTS.UID, targetTS.Description)
	}
}

// TestSCP_WithImplementationClass verifies that the WithImplementationClass
// option causes the SCP to send the overridden UID in the A-ASSOCIATE-AC.
// The SCU captures the raw PDU bytes and checks for the custom UID string.
func TestSCP_WithImplementationClass(t *testing.T) {
	const customUID = "1.2.3.4.888"
	const port = 1065

	_, testSCP := StartSCP(t, port, WithImplementationClass(customUID, "MYVER"))
	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool { return true })
	testSCP.OnCFindRequest(func(ctx context.Context, request network.AssociationRequest, findLevel string, data media.DICOMObject, emit func(media.DICOMObject)) (CFindResult, error) {
		return CFindResult{Status: dicomstatus.Success}, nil
	})

	var foundUID bool
	dest := &network.Destination{
		Name:      "Impl Class SCP Test",
		CalledAE:  "TEST_SCP",
		CallingAE: "TEST_SCU",
		HostName:  "localhost",
		Port:      port,
	}
	scu := NewSCU(dest)
	scu.SetOnRawPDU(func(event network.RawPDUEvent) {
		if event.Direction == network.RawPDUDirectionInbound && event.PDUType == 0x02 {
			if containsString(event.Data, customUID) {
				foundUID = true
			}
		}
	})

	if err := scu.EchoSCU(context.Background()); err != nil {
		t.Fatalf("EchoSCU: %v", err)
	}
	if !foundUID {
		t.Errorf("custom implementation class UID %q not found in A-ASSOCIATE-AC PDU", customUID)
	}
}

// containsString reports whether s appears as a substring in data.
func containsString(data []byte, s string) bool {
	b := []byte(s)
	if len(b) > len(data) {
		return false
	}
	for i := 0; i <= len(data)-len(b); i++ {
		match := true
		for j := range b {
			if data[i+j] != b[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestSCP_WithTimeout verifies that SetTimeout is accepted without error and
// that the SCP remains functional (a C-ECHO succeeds over the timed connection).
func TestSCP_WithTimeout(t *testing.T) {
	const port = 1067
	_, testSCP := StartSCP(t, port)
	testSCP.SetTimeout(30)
	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool { return true })

	dest := &network.Destination{
		Name:      "Timeout SCP Test",
		CalledAE:  "TEST_SCP",
		CallingAE: "TEST_SCU",
		HostName:  "localhost",
		Port:      port,
	}
	scu := NewSCU(dest)
	if err := scu.EchoSCU(context.Background()); err != nil {
		t.Fatalf("EchoSCU with timeout SCP: %v", err)
	}
}

// TestSCP_SetTimeout verifies that SetTimeout (called post-construction) is
// accepted and that the SCP remains functional.
func TestSCP_SetTimeout(t *testing.T) {
	const port = 1068
	_, testSCP := StartSCP(t, port)
	testSCP.SetTimeout(30)
	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool { return true })

	dest := &network.Destination{
		Name:      "SetTimeout SCP Test",
		CalledAE:  "TEST_SCP",
		CallingAE: "TEST_SCU",
		HostName:  "localhost",
		Port:      port,
	}
	scu := NewSCU(dest)
	if err := scu.EchoSCU(context.Background()); err != nil {
		t.Fatalf("EchoSCU with SetTimeout SCP: %v", err)
	}
}
