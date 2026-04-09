package services

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/utils"
)

func Test_scu_EchoSCU(t *testing.T) {
	_, testSCP := StartSCP(t, 1040)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	media.InitDict()

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
					IsCFind:   false,
					IsCMove:   false,
					IsCStore:  false,
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
					IsCFind:   false,
					IsCMove:   false,
					IsCStore:  false,
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
			if err := d.EchoSCU(context.Background(), tt.args.timeout); (err != nil) != tt.wantErr {
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

	media.InitDict()

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
					IsCFind:   true,
					IsCMove:   true,
					IsCStore:  true,
					IsTLS:     false,
				},
			},
			args: args{
				Query:   utils.DefaultCFindRequest(),
				timeout: 0,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.Query.WriteString(tags.StudyDate, "20150617")
			d := NewSCU(tt.fields.destination)
			d.SetOnCFindResult(func(result media.DICOMObject) {
				result.DumpTags(io.Discard)
			})

			_, status, err := d.FindSCU(context.Background(), tt.args.Query, tt.args.timeout)
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
	if _, err := os.Stat("../samples/test.dcm"); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}

	_, testSCP := StartSCP(t, 1042)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	testSCP.OnCStoreRequest(func(request network.AssociationRequest, data media.DICOMObject) uint16 {
		data.DumpTags(io.Discard)
		return dicomstatus.Success
	})

	media.InitDict()

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
					IsCFind:   true,
					IsCMove:   true,
					IsCStore:  true,
					IsTLS:     false,
				},
			},
			args: args{
				FileName: "../samples/test.dcm",
				timeout:  0,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewSCU(tt.fields.destination)
			if err := d.StoreSCU(context.Background(), tt.args.FileName, tt.args.timeout); (err != nil) != tt.wantErr {
				t.Errorf("scu.StoreSCU() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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

	media.InitDict()

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
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.StudyInstanceUID, "1.2.3.4")

	status, err := d.GetSCU(context.Background(), query, 0)
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

func StartSCP(t testing.TB, port int) (func(t testing.TB), SCP) {
	testSCP := NewSCP(port)
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
