package services

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

func Test_scu_EchoSCU(t *testing.T) {
	_, testSCP := StartSCP(t, 1040)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	type fields struct {
		destination *network.Destination
	}
	type args struct {
		timeout int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Should have C-Echo Success",
			fields: fields{
				destination: &network.Destination{
					Name:      "Test Destination",
					CalledAE:  "TEST_SCP",
					CallingAE: "TEST_SCU",
					HostName:  "localhost",
					Port:      1040,
					IsTLS:     false,
				},
			},
			args: args{
				timeout: 0,
			},
			wantErr: false,
		},
		{
			name: "Should not have C-Echo Success",
			fields: fields{
				destination: &network.Destination{
					Name:      "Test Destination",
					CalledAE:  "TEST_SCP2",
					CallingAE: "TEST_SCU",
					HostName:  "localhost",
					Port:      1040,
					IsTLS:     false,
				},
			},
			args: args{
				timeout: 0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewSCU(tt.fields.destination)
			if err := d.EchoSCU(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("scu.EchoSCU() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_scu_FindSCU(t *testing.T) {
	_, testSCP := StartSCP(t, 1041)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	testSCP.OnCFindRequest(func(ctx context.Context, request network.AssociationRequest, findLevel string, data media.DICOMObject, emit func(media.DICOMObject)) (CFindResult, error) {
		return CFindResult{Status: dicomstatus.Success}, nil
	})

	type fields struct {
		destination *network.Destination
	}
	type args struct {
		Query   media.DICOMObject
		timeout int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    uint16
		wantErr bool
	}{
		{
			name: "Should C-Find All",
			fields: fields{
				destination: &network.Destination{
					Name:      "Test Destination",
					CalledAE:  "TEST_SCP",
					CallingAE: "TEST_SCU",
					HostName:  "localhost",
					Port:      1041,
					IsTLS:     false,
				},
			},
			args: args{
				Query:   dimse.DefaultCFindRequest(),
				timeout: 0,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.Query.Write(tags.StudyDate, "20150617")
			d := NewSCU(tt.fields.destination)
			d.SetOnCFindResult(func(result media.DICOMObject) {
				result.DumpTags(io.Discard)
			})

			_, status, err := d.FindSCU(context.Background(), tt.args.Query)
			if (err != nil) != tt.wantErr {
				t.Errorf("scu.FindSCU() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if status != tt.want {
				t.Errorf("scu.FindSCU() = %v, want %v", status, tt.want)
			}
		})
	}
}

func Test_scu_StoreSCU(t *testing.T) {
	if _, err := os.Stat("../testdata/test.dcm"); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	_, testSCP := StartSCP(t, 1042)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	testSCP.OnCStoreRequest(func(ctx context.Context, request network.AssociationRequest, data media.DICOMObject) uint16 {
		data.DumpTags(io.Discard)
		return dicomstatus.Success
	})

	type fields struct {
		destination *network.Destination
	}
	type args struct {
		FileName string
		timeout  int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Should store DICOM file",
			fields: fields{
				destination: &network.Destination{
					Name:      "Test Destination",
					CalledAE:  "TEST_SCP",
					CallingAE: "TEST_SCU",
					HostName:  "localhost",
					Port:      1042,
					IsTLS:     false,
				},
			},
			args: args{
				FileName: "../testdata/test.dcm",
				timeout:  0,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewSCU(tt.fields.destination)
			if err := d.StoreSCU(context.Background(), tt.args.FileName); (err != nil) != tt.wantErr {
				t.Errorf("scu.StoreSCU() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_scu_StoreObjectSCU(t *testing.T) {
	if _, err := os.Stat("../testdata/test.dcm"); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	_, testSCP := StartSCP(t, 1043)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	var received media.DICOMObject
	testSCP.OnCStoreRequest(func(ctx context.Context, request network.AssociationRequest, data media.DICOMObject) uint16 {
		received = data
		return dicomstatus.Success
	})

	obj, err := media.NewDCMObjFromFile("../testdata/test.dcm")
	if err != nil {
		t.Fatalf("NewDCMObjFromFile: %v", err)
	}

	dest := &network.Destination{
		Name:      "StoreObject Test",
		CalledAE:  "TEST_SCP",
		CallingAE: "TEST_SCU",
		HostName:  "localhost",
		Port:      1043,
	}
	d := NewSCU(dest)
	if err := d.StoreObjectSCU(context.Background(), obj); err != nil {
		t.Fatalf("StoreObjectSCU: %v", err)
	}
	if received == nil {
		t.Fatal("SCP did not receive the C-STORE request")
	}
}

func Test_scu_GetSCU(t *testing.T) {
	_, testSCP := StartSCP(t, 1060)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	testSCP.OnCGetRequest(func(ctx context.Context, request network.AssociationRequest, getLevel string, data media.DICOMObject, _ func(string) error, emit func(CGetProgress)) (CGetResult, error) {
		return CGetResult{Status: dicomstatus.Success, Completed: 0}, nil
	})

	dest := &network.Destination{
		Name:      "Test Destination",
		CalledAE:  "TEST_SCP",
		CallingAE: "TEST_SCU",
		HostName:  "localhost",
		Port:      1060,
	}
	d := NewSCU(dest)

	received := 0
	d.SetOnCGetStore(func(data media.DICOMObject) uint16 {
		received++
		return dicomstatus.Success
	})

	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.StudyInstanceUID, "1.2.3.4")

	status, err := d.GetSCU(context.Background(), query)
	if err != nil {
		t.Fatalf("GetSCU: %v", err)
	}
	if status != dicomstatus.Success {
		t.Fatalf("GetSCU status = 0x%04X, want Success (0x0000)", status)
	}
	if received != 0 {
		t.Fatalf("received = %d, want 0 (no sub-ops sent)", received)
	}
}

func Test_scu_GetSCUReceivesStoreSuboperations(t *testing.T) {
	const samplePath = "../testdata/test2.dcm"
	if _, err := os.Stat(samplePath); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	_, testSCP := StartSCP(t, 1061)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	testSCP.OnCGetRequest(func(ctx context.Context, request network.AssociationRequest, getLevel string, data media.DICOMObject, storeFile func(string) error, emit func(CGetProgress)) (CGetResult, error) {
		if err := storeFile(samplePath); err != nil {
			return CGetResult{Status: dicomstatus.FailureUnableToProcess, Failed: 1}, nil
		}
		return CGetResult{Status: dicomstatus.Success, Completed: 1}, nil
	})

	dest := &network.Destination{
		Name:      "Test Destination",
		CalledAE:  "TEST_SCP",
		CallingAE: "TEST_SCU",
		HostName:  "localhost",
		Port:      1061,
	}
	d := NewSCU(dest)

	received := 0
	d.SetOnCGetStore(func(data media.DICOMObject) uint16 {
		received++
		return dicomstatus.Success
	})

	query := media.NewEmptyDCMObj()
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.StudyInstanceUID, "1.2.3.4")

	status, err := d.GetSCU(context.Background(), query)
	if err != nil {
		t.Fatalf("GetSCU: %v", err)
	}
	if status != dicomstatus.Success {
		t.Fatalf("GetSCU status = 0x%04X, want Success (0x0000)", status)
	}
	if received != 1 {
		t.Fatalf("received = %d, want 1", received)
	}
}

func StartSCP(t testing.TB, port int, opts ...SCPOption) (func(t testing.TB), SCP) {
	testSCP := NewSCP(port, opts...)
	go func() {
		if err := testSCP.Start(context.Background()); err != nil {
			t.Logf("SCP stopped: %v", err)
		}
	}()

	// Poll until the SCP port is accepting connections.
	addr := fmt.Sprintf("localhost:%d", port)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			// Give the SCP goroutine a moment to reach Accept() again
			// after handling our probe connection.
			time.Sleep(20 * time.Millisecond)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cleanup := func(t testing.TB) {
		if err := testSCP.Stop(); err != nil {
			t.Errorf("failed to stop SCP: %v", err)
		}
	}
	t.Cleanup(func() { cleanup(t) })
	return cleanup, testSCP
}

// Test_scu_writeStoreRQ_TranscodesOnMismatch verifies that writeStoreRQ
// transcodes the DICOMObject to the negotiated transfer syntax when it differs
// from the file's native syntax (e.g. EVLE file sent to a peer that only
// accepted ImplicitVRLittleEndian).
func Test_scu_writeStoreRQ_TranscodesOnMismatch(t *testing.T) {
	if _, err := os.Stat("../testdata/test.dcm"); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	DDO, err := media.NewDCMObjFromFile("../testdata/test.dcm")
	if err != nil {
		t.Fatalf("load test.dcm: %v", err)
	}
	if DDO.GetTransferSyntax().UID != transfersyntax.ExplicitVRLittleEndian.UID {
		t.Skipf("test.dcm is not EVLE (got %s); skipping transcoding test", DDO.GetTransferSyntax().UID)
	}

	// Simulate a remote SCP that negotiated ImplicitVRLittleEndian.
	mock := &storeMockPDU{ts: transfersyntax.ImplicitVRLittleEndian}
	d := &scu{}
	if err := d.writeStoreRQ(context.Background(), mock, DDO); err != nil {
		t.Fatalf("writeStoreRQ() error = %v", err)
	}

	// The DICOMObject must have been transcoded to the negotiated syntax.
	if got := DDO.GetTransferSyntax().UID; got != transfersyntax.ImplicitVRLittleEndian.UID {
		t.Errorf("after writeStoreRQ, DDO.GetTransferSyntax() = %q, want ImplicitVRLittleEndian", got)
	}
}

// storeMockPDU is a minimal network.PDUService stub for writeStoreRQ unit tests.
// Only GetPresentationContextID, GetTransferSyntax, GetAAssociationRQ, and Write
// are meaningful; all other methods are no-ops.
type storeMockPDU struct {
	ts      *transfersyntax.TransferSyntax
	written []media.DICOMObject
}

func (m *storeMockPDU) GetTransferSyntax(_ byte) *transfersyntax.TransferSyntax { return m.ts }
func (m *storeMockPDU) GetPresentationContextID() byte                          { return 1 }
func (m *storeMockPDU) Write(dco media.DICOMObject, _ byte) error {
	m.written = append(m.written, dco)
	return nil
}
func (m *storeMockPDU) SetTimeout(_ int)                                               {}
func (m *storeMockPDU) Connect(_ context.Context, _, _ string) error                   { return nil }
func (m *storeMockPDU) ConnectTLS(_ context.Context, _, _ string, _ *tls.Config) error { return nil }
func (m *storeMockPDU) Close() error                                                   { return nil }
func (m *storeMockPDU) GetAAssociationRQ() network.AssociationRequest {
	return network.NewAssociationRequest()
}
func (m *storeMockPDU) GetCalledAE() string                           { return "CALLED" }
func (m *storeMockPDU) GetCallingAE() string                          { return "CALLING" }
func (m *storeMockPDU) GetRemoteAddress() string                      { return "127.0.0.1:104" }
func (m *storeMockPDU) SetCalledAE(_ string)                          {}
func (m *storeMockPDU) SetCallingAE(_ string)                         {}
func (m *storeMockPDU) SetConn(_ *bufio.ReadWriter)                   {}
func (m *storeMockPDU) SetNetConn(_ net.Conn)                         {}
func (m *storeMockPDU) NextPDU() (media.DICOMObject, error)           { return nil, nil }
func (m *storeMockPDU) AddPresContexts(_ network.PresentationContext) {}
func (m *storeMockPDU) GetAcceptedPresentationContexts() []network.PresentationContextAccept {
	return nil
}
func (m *storeMockPDU) SetOnAssociationRequest(_ func(network.AssociationRequest) bool) {}
func (m *storeMockPDU) SetOnRawPDU(_ func(network.RawPDUEvent))                         {}
func (m *storeMockPDU) SetLogger(_ *slog.Logger)                                        {}
func (m *storeMockPDU) Logger() *slog.Logger                                            { return slog.Default() }

// Test_scu_BeginStoreSession_SendsMultipleFilesOnOneAssociation verifies that
// BeginStoreSession opens exactly one association and that all files in the
// batch are delivered to the SCP over that single connection.
func Test_scu_BeginStoreSession_SendsMultipleFilesOnOneAssociation(t *testing.T) {
	const samplePath = "../testdata/test.dcm"
	if _, err := os.Stat(samplePath); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	_, testSCP := StartSCP(t, 1062)
	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	var mu sync.Mutex
	received := 0
	testSCP.OnCStoreRequest(func(ctx context.Context, request network.AssociationRequest, data media.DICOMObject) uint16 {
		mu.Lock()
		received++
		mu.Unlock()
		return dicomstatus.Success
	})

	dest := &network.Destination{
		Name:      "BatchStore Test",
		CalledAE:  "TEST_SCP",
		CallingAE: "TEST_SCU",
		HostName:  "localhost",
		Port:      1062,
	}
	d := NewSCU(dest)

	session, err := d.BeginStoreSession(context.Background())
	if err != nil {
		t.Fatalf("BeginStoreSession: %v", err)
	}
	defer session.Close()

	const n = 3
	for i := 0; i < n; i++ {
		if err := session.StorePath(context.Background(), samplePath); err != nil {
			t.Fatalf("StorePath[%d]: %v", i, err)
		}
	}
	session.Close()

	mu.Lock()
	got := received
	mu.Unlock()
	if got != n {
		t.Errorf("SCP received %d C-STORE requests, want %d", got, n)
	}
}

// Test_scu_BeginStoreSession_StoreObject verifies that Store (object variant)
// works correctly within a session.
func Test_scu_BeginStoreSession_StoreObject(t *testing.T) {
	const samplePath = "../testdata/test.dcm"
	if _, err := os.Stat(samplePath); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	_, testSCP := StartSCP(t, 1063)
	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	var received media.DICOMObject
	testSCP.OnCStoreRequest(func(ctx context.Context, request network.AssociationRequest, data media.DICOMObject) uint16 {
		received = data
		return dicomstatus.Success
	})

	obj, err := media.NewDCMObjFromFile(samplePath)
	if err != nil {
		t.Fatalf("NewDCMObjFromFile: %v", err)
	}

	dest := &network.Destination{
		Name:      "BatchStore Object Test",
		CalledAE:  "TEST_SCP",
		CallingAE: "TEST_SCU",
		HostName:  "localhost",
		Port:      1063,
	}
	session, err := NewSCU(dest).BeginStoreSession(context.Background())
	if err != nil {
		t.Fatalf("BeginStoreSession: %v", err)
	}
	defer session.Close()

	if err := session.Store(context.Background(), obj); err != nil {
		t.Fatalf("Store: %v", err)
	}
	session.Close()

	if received == nil {
		t.Fatal("SCP did not receive the C-STORE request")
	}
}

// Test_scu_SetImplementationClass verifies that SetImplementationClass causes
// the SCU to send the overridden UID in the A-ASSOCIATE-RQ.
func Test_scu_SetImplementationClass(t *testing.T) {
	_, testSCP := StartSCP(t, 1064)
	testSCP.OnCFindRequest(func(ctx context.Context, request network.AssociationRequest, findLevel string, data media.DICOMObject, emit func(media.DICOMObject)) (CFindResult, error) {
		return CFindResult{Status: dicomstatus.Success}, nil
	})

	const wantUID = "1.2.3.4.999"
	const wantVer = "CUSTOM-TEST"

	var gotUID string
	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		gotUID = request.GetImplementationClass().GetUID()
		return request.GetCalledAE() == "TEST_SCP"
	})

	dest := &network.Destination{
		Name:      "Impl Class Test",
		CalledAE:  "TEST_SCP",
		CallingAE: "TEST_SCU",
		HostName:  "localhost",
		Port:      1064,
	}
	d := NewSCU(dest)
	d.SetImplementationClass(wantUID, wantVer)
	if err := d.EchoSCU(context.Background()); err != nil {
		t.Fatalf("EchoSCU: %v", err)
	}
	if gotUID != wantUID {
		t.Errorf("implementation class UID = %q, want %q", gotUID, wantUID)
	}
}

// Test_scu_WithTimeout verifies that an SCU constructed with WithTimeout
// successfully completes an association when the remote SCP responds within
// the configured window. It also exercises the SetTimeout setter path.
func Test_scu_WithTimeout(t *testing.T) {
	_, testSCP := StartSCP(t, 1066)
	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	dest := &network.Destination{
		Name:      "Timeout Test",
		CalledAE:  "TEST_SCP",
		CallingAE: "TEST_SCU",
		HostName:  "localhost",
		Port:      1066,
	}

	// Via SCUOption at construction time.
	if err := NewSCU(dest, WithTimeout(30)).EchoSCU(context.Background()); err != nil {
		t.Fatalf("EchoSCU with WithTimeout option: %v", err)
	}

	// Via SetTimeout setter after construction.
	d := NewSCU(dest)
	d.SetTimeout(30)
	if err := d.EchoSCU(context.Background()); err != nil {
		t.Fatalf("EchoSCU with SetTimeout: %v", err)
	}
}
