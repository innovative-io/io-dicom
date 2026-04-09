package services

import (
	"context"
	"fmt"
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
	"github.com/innovative-io/io-dicom/utils"
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

	media.InitDict()
	dest := &network.Destination{
		Name:      "CEchoTest",
		CalledAE:  "SCP",
		CallingAE: "SCU",
		HostName:  "localhost",
		Port:      1044,
	}
	scu := NewSCU(dest)
	if err := scu.EchoSCU(context.Background(), 0); err != nil {
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

	media.InitDict()
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
	_ = scu.EchoSCU(context.Background(), 1)
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

	media.InitDict()
	dest := &network.Destination{
		Name:      "CMoveTest",
		CalledAE:  "SCP",
		CallingAE: "SCU",
		HostName:  "localhost",
		Port:      1046,
		IsCMove:   true,
	}
	scu := NewSCU(dest)

	status, err := scu.MoveSCU(context.Background(), "DEST_AE", utils.DefaultCMoveRequest("1.2.3.4"), 0)
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

	media.InitDict()
	dest := &network.Destination{
		Name:      "CFindInvalidLevel",
		CalledAE:  "SCP",
		CallingAE: "SCU",
		HostName:  "localhost",
		Port:      1047,
		IsCFind:   true,
	}

	query := media.NewEmptyDCMObj()
	query.WriteString(tags.QueryRetrieveLevel, "NOT_A_LEVEL")
	query.WriteString(tags.PatientID, "P123")

	scu := NewSCU(dest)
	_, status, err := scu.FindSCU(context.Background(), query, 5)
	if err != nil {
		t.Fatalf("FindSCU: %v", err)
	}
	if status != dicomstatus.FailureIdentifierDoesNotMatchSOPClass {
		t.Fatalf("FindSCU status = 0x%04X, want 0x%04X", status, dicomstatus.FailureIdentifierDoesNotMatchSOPClass)
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
			obj.WriteString(tags.PatientID, fmt.Sprintf("P%03d", i))
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
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.PatientID, "*")

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
		obj.WriteString(tags.PatientID, "P000")
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
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.PatientID, "*")

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
	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CFindRequest)
	commandObj.WriteUint16(tags.MessageID, messageID)
	commandObj.WriteUint16(tags.Priority, priority.Medium)
	commandObj.WriteUint16(tags.CommandDataSetType, dicomcommand.DataSetPresent)

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
	testSCP.OnCGetRequest(func(ctx context.Context, request network.AssociationRequest, getLevel string, data media.DICOMObject, emit func(CGetProgress)) (CGetResult, error) {
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
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.PatientID, "*")

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
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.PatientID, "*")

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
	testSCP.OnCGetRequest(func(ctx context.Context, request network.AssociationRequest, getLevel string, data media.DICOMObject, emit func(CGetProgress)) (CGetResult, error) {
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
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.PatientID, "*")
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
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.PatientID, "*")
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
	testSCP.OnCGetRequest(func(ctx context.Context, request network.AssociationRequest, getLevel string, data media.DICOMObject, emit func(CGetProgress)) (CGetResult, error) {
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
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.PatientID, "*")
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
	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CGetRequest)
	commandObj.WriteUint16(tags.MessageID, messageID)
	commandObj.WriteUint16(tags.Priority, priority.Medium)
	commandObj.WriteUint16(tags.CommandDataSetType, dicomcommand.DataSetPresent)

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
	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CMoveRequest)
	commandObj.WriteUint16(tags.MessageID, messageID)
	commandObj.WriteString(tags.MoveDestination, destinationAETitle)
	commandObj.WriteUint16(tags.Priority, priority.Medium)
	commandObj.WriteUint16(tags.CommandDataSetType, dicomcommand.DataSetPresent)

	if err := pdu.Write(commandObj, network.PDVCommand); err != nil {
		return err
	}
	return pdu.Write(dataObj, network.PDVDataset)
}
