package services

import (
	"context"
	"errors"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

// nServiceMockPDU embeds the C-GET sub-op mock (which already satisfies the full
// network.PDUService interface) and overrides only the read/write paths that
// handleNRequest exercises.
type nServiceMockPDU struct {
	cgetStoreSubopMockPDU
	dataset    media.DICOMObject // returned by NextPDU when a request dataset is read
	nextErr    error             // error returned by NextPDU
	nextCalled bool
	respCmd    media.DICOMObject // captured N-service response command
}

func (m *nServiceMockPDU) NextPDU() (media.DICOMObject, error) {
	m.nextCalled = true
	return m.dataset, m.nextErr
}

func (m *nServiceMockPDU) Write(dco media.DICOMObject, itemType byte) error {
	if itemType == network.PDVCommand {
		m.respCmd = dco
	}
	return nil
}

// nServiceRequest builds a minimal N-service request command carrying the fields
// NWriteRSP requires (a SOP Class UID and a message ID) plus the dataset-type
// flag that controls whether handleNRequest reads a request dataset.
func nServiceRequest(dataSetType uint16) media.DICOMObject {
	dco := media.NewEmptyDCMObj()
	dco.Write(tags.AffectedSOPClassUID, "1.2.840.10008.5.1.4.1.1.4")
	dco.Write(tags.MessageID, uint16(7))
	dco.Write(tags.CommandDataSetType, dataSetType)
	return dco
}

// TestHandleNRequest_DispatchesAndWritesResponse verifies the common N-service
// path: the registered handler is invoked, its status flows into the response,
// and the response carries the requested command field.
func TestHandleNRequest_DispatchesAndWritesResponse(t *testing.T) {
	s := NewSCP(0).(*scp)
	mock := &nServiceMockPDU{}
	dco := nServiceRequest(dicomcommand.DataSetNone)

	var gotCmd media.DICOMObject
	handler := func(_ context.Context, _ network.AssociationRequest, command media.DICOMObject, _ media.DICOMObject) (uint16, media.DICOMObject) {
		gotCmd = command
		return dicomstatus.Success, media.NewEmptyDCMObj()
	}

	if stop := s.handleNRequest(context.Background(), mock, dco, "N-Get", dicomcommand.NGetResponse, handler); stop {
		t.Fatal("handleNRequest returned stop=true on the success path")
	}
	if mock.nextCalled {
		t.Error("NextPDU was called even though CommandDataSetType is DataSetNone")
	}
	if gotCmd == nil {
		t.Error("handler did not receive the request command object")
	}
	if mock.respCmd == nil {
		t.Fatal("no response command was written")
	}
	if got := mock.respCmd.GetUint16(tags.Status); got != dicomstatus.Success {
		t.Errorf("response status = 0x%04X, want Success (0x0000)", got)
	}
	if got := mock.respCmd.GetUint16(tags.CommandField); got != dicomcommand.NGetResponse {
		t.Errorf("response command field = 0x%04X, want NGetResponse (0x%04X)", got, dicomcommand.NGetResponse)
	}
}

// TestHandleNRequest_NoHandlerReturnsUnsupported verifies that, with no handler
// registered, the SCP replies with FailureSOPClassNotSupported.
func TestHandleNRequest_NoHandlerReturnsUnsupported(t *testing.T) {
	s := NewSCP(0).(*scp)
	mock := &nServiceMockPDU{}
	dco := nServiceRequest(dicomcommand.DataSetNone)

	if stop := s.handleNRequest(context.Background(), mock, dco, "N-Get", dicomcommand.NGetResponse, nil); stop {
		t.Fatal("handleNRequest returned stop=true with no handler")
	}
	if mock.respCmd == nil {
		t.Fatal("no response command was written")
	}
	if got := mock.respCmd.GetUint16(tags.Status); got != dicomstatus.FailureSOPClassNotSupported {
		t.Errorf("response status = 0x%04X, want FailureSOPClassNotSupported (0x%04X)", got, dicomstatus.FailureSOPClassNotSupported)
	}
}

// TestHandleNRequest_PassesDataset verifies the request dataset is read (when
// CommandDataSetType is DataSetPresent) and handed to the handler.
func TestHandleNRequest_PassesDataset(t *testing.T) {
	s := NewSCP(0).(*scp)
	ds := media.NewEmptyDCMObj()
	ds.Write(tags.PatientID, "PID-1")
	mock := &nServiceMockPDU{dataset: ds}
	dco := nServiceRequest(dicomcommand.DataSetPresent)

	var gotData media.DICOMObject
	handler := func(_ context.Context, _ network.AssociationRequest, _ media.DICOMObject, data media.DICOMObject) (uint16, media.DICOMObject) {
		gotData = data
		return dicomstatus.Success, nil
	}

	if stop := s.handleNRequest(context.Background(), mock, dco, "N-Set", dicomcommand.NSetResponse, handler); stop {
		t.Fatal("handleNRequest returned stop=true on the success path")
	}
	if !mock.nextCalled {
		t.Error("NextPDU was not called for DataSetPresent")
	}
	if gotData == nil || gotData.GetString(tags.PatientID) != "PID-1" {
		t.Errorf("handler did not receive the request dataset: %v", gotData)
	}
}

// TestHandleNRequest_DatasetReadErrorStops verifies that a failed dataset read
// terminates the association (stop=true), the handler is not invoked, and no
// response is written.
func TestHandleNRequest_DatasetReadErrorStops(t *testing.T) {
	s := NewSCP(0).(*scp)
	mock := &nServiceMockPDU{nextErr: errors.New("read failed")}
	dco := nServiceRequest(dicomcommand.DataSetPresent)

	called := false
	handler := func(_ context.Context, _ network.AssociationRequest, _ media.DICOMObject, _ media.DICOMObject) (uint16, media.DICOMObject) {
		called = true
		return dicomstatus.Success, nil
	}

	if stop := s.handleNRequest(context.Background(), mock, dco, "N-Get", dicomcommand.NGetResponse, handler); !stop {
		t.Fatal("handleNRequest returned stop=false on a dataset read error")
	}
	if !mock.nextCalled {
		t.Error("NextPDU was not called for DataSetPresent")
	}
	if called {
		t.Error("handler was invoked despite the dataset read failing")
	}
	if mock.respCmd != nil {
		t.Error("a response was written despite the dataset read failing")
	}
}
