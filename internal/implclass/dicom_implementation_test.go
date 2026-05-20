package implclass

import (
	"reflect"
	"testing"
)

func TestSetDefaultImplementation(t *testing.T) {
	tests := []struct {
		name string
		want Implementation
	}{
		{
			name: "Should set default dicomImplementation",
			want: &dicomImplementation{
				classUID: "1.2.826.0.1.3680043.10.90.999",
				version:  "IO-Dicom",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SetDefaultImplementation(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetDefaultImplementation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetImplementation(t *testing.T) {
	type args struct {
		classUID string
		version  string
	}
	tests := []struct {
		name string
		args args
		want Implementation
	}{
		{
			name: "Should set dicomImplementation",
			args: args{
				classUID: "1.2.826.0.1.3680043.10.90.999",
				version:  "Innovative IO",
			},
			want: &dicomImplementation{
				classUID: "1.2.826.0.1.3680043.10.90.999",
				version:  "Innovative IO",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SetImplementation(tt.args.classUID, tt.args.version); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetImplementation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetImpClassUID(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Should get default class UID",
			want: "1.2.826.0.1.3680043.10.90.999",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetImplementationClassUID(); got != tt.want {
				t.Errorf("GetImplementationClassUID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetImpVersion(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Should get default name",
			want: "Innovative IO",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetImplementationVersion(); got != tt.want {
				t.Errorf("GetImplementationVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_implementation_GetClassUID(t *testing.T) {
	type fields struct {
		classUID string
		version  string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Should get default class UID",
			fields: fields{
				classUID: "1.2.826.0.1.3680043.10.90.999",
				version:  "Innovative IO",
			},
			want: "1.2.826.0.1.3680043.10.90.999",
		},
		{
			name: "Should get default class UID",
			fields: fields{
				classUID: "",
				version:  "",
			},
			want: "1.2.826.0.1.3680043.10.90.999",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &dicomImplementation{
				classUID: tt.fields.classUID,
				version:  tt.fields.version,
			}
			if got := i.GetClassUID(); got != tt.want {
				t.Errorf("implementation.GetClassUID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_implementation_GetVersion(t *testing.T) {
	type fields struct {
		classUID string
		version  string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Should get default name",
			fields: fields{
				classUID: "1.2.826.0.1.3680043.10.90.999",
				version:  "Innovative IO",
			},
			want: "Innovative IO",
		},
		{
			name: "Should get default name",
			fields: fields{
				classUID: "",
				version:  "",
			},
			want: "IO-Dicom",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &dicomImplementation{
				classUID: tt.fields.classUID,
				version:  tt.fields.version,
			}
			if got := i.GetVersion(); got != tt.want {
				t.Errorf("implementation.GetVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
