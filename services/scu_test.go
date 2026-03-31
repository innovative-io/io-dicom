package services

import (
	"context"
	"fmt"
	"net"
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
			if err := d.EchoSCU(tt.args.timeout); (err != nil) != tt.wantErr {
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

	testSCP.OnCFindRequest(func(request network.AssociationRequest, findLevel string, data media.DICOMObject) ([]media.DICOMObject, uint16) {
		return make([]media.DICOMObject, 0), dicomstatus.Success
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
				result.DumpTags()
			})

			_, status, err := d.FindSCU(tt.args.Query, tt.args.timeout)
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
	_, testSCP := StartSCP(t, 1042)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TEST_SCP"
	})

	testSCP.OnCStoreRequest(func(request network.AssociationRequest, data media.DICOMObject) uint16 {
		data.DumpTags()
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
			if err := d.StoreSCU(tt.args.FileName, tt.args.timeout); (err != nil) != tt.wantErr {
				t.Errorf("scu.StoreSCU() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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
