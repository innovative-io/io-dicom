package dicomstatus

import "testing"

func TestDescription_KnownStatuses(t *testing.T) {
	tests := []struct {
		status uint16
		want   string
	}{
		{Success, "Success"},
		{Cancel, "Cancel"},
		{PendingWithWarnings, "Pending: warnings"},
		{RefusedMoveDestinationUnknown, "Refused: move destination unknown"},
		{WarningElementsDiscarded, "Warning: elements discarded"},
	}

	for _, tc := range tests {
		if got := Description(tc.status); got != tc.want {
			t.Fatalf("Description(0x%04X) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestDescription_RangeFallbacks(t *testing.T) {
	tests := []struct {
		status uint16
		want   string
	}{
		{0xA9FF, "Failure"},
		{0xB0FF, "Warning"},
		{FailureUnableToProcess + 0x12, "Failure: unable to process"},
	}

	for _, tc := range tests {
		if got := Description(tc.status); got != tc.want {
			t.Fatalf("Description(0x%04X) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestDescription_Unknown(t *testing.T) {
	if got := Description(0x1234); got != "Unknown status 0x1234" {
		t.Fatalf("Description(0x1234) = %q, want %q", got, "Unknown status 0x1234")
	}
}
