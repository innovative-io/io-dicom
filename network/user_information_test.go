package network

import (
	"reflect"
	"testing"
)

func TestNewUserInformation(t *testing.T) {
	tests := []struct {
		name string
		want UserInformation
	}{
		{
			name: "Should get UserInformation",
			want: &userInformation{
				ItemType:      0x50,
				MaxSubLength:  NewMaximumPDULength(),
				AsyncOpWindow: NewAsyncOperationWindow(),
				SCPSCURole:    NewRoleSelect(),
				ImpClass:      uidItem{itemType: 0x52},
				ImpVersion:    uidItem{itemType: 0x55},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewUserInformation(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewUserInformation() = %v, want %v", got, tt.want)
			}
		})
	}
}
