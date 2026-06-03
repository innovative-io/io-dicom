package uuids

import (
	"testing"

	implementation "github.com/innovative-io/io-dicom/internal/implclass"
)

func Test_hash32(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want uint32
	}{
		{
			name: "Should hash a string to uint32",
			args: args{text: "Some strange text that should be hashed"},
			want: 3337973406,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hash32(tt.args.text); got != tt.want {
				t.Errorf("hash32() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateStudyUID(t *testing.T) {
	type args struct {
		patName string
		patID   string
		accNum  string
		stDate  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Should generate study instance UID",
			args: args{
				patName: "test",
				patID:   "000000",
				accNum:  "00000000",
				stDate:  "2020-01-31",
			},
			want: "1.2.826.0.1.3680043.10.90.999.3951767102",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateStudyUID(tt.args.patName, tt.args.patID, tt.args.accNum, tt.args.stDate); got != tt.want {
				t.Errorf("CreateStudyUID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateSeriesUID(t *testing.T) {
	type args struct {
		RootUID      string
		Modality     string
		SeriesNumber string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Should generate series UID",
			args: args{
				RootUID:      implementation.GetImplementationClassUID(),
				Modality:     "MR",
				SeriesNumber: "1",
			},
			want: "1.2.826.0.1.3680043.10.90.999.2335432029",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateSeriesUID(tt.args.RootUID, tt.args.Modality, tt.args.SeriesNumber); got != tt.want {
				t.Errorf("CreateSeriesUID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateInstanceUID(t *testing.T) {
	type args struct {
		RootUID    string
		InstNumber string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Should generate instance UID",
			args: args{
				RootUID:    implementation.GetImplementationClassUID(),
				InstNumber: "1",
			},
			want: "1.2.826.0.1.3680043.10.90.999.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateInstanceUID(tt.args.RootUID, tt.args.InstNumber); got != tt.want {
				t.Errorf("CreateInstanceUID() = %v, want %v", got, tt.want)
			}
		})
	}
}
