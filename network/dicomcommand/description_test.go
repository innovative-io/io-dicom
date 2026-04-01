package dicomcommand

import "testing"

func TestDescription(t *testing.T) {
	tests := []struct {
		name    string
		command uint16
		want    string
	}{
		{"CStoreRequest", CStoreRequest, "C-STORE-RQ"},
		{"CStoreResponse", CStoreResponse, "C-STORE-RSP"},
		{"CGetRequest", CGetRequest, "C-GET-RQ"},
		{"CGetResponse", CGetResponse, "C-GET-RSP"},
		{"CFindRequest", CFindRequest, "C-FIND-RQ"},
		{"CFindResponse", CFindResponse, "C-FIND-RSP"},
		{"CMoveRequest", CMoveRequest, "C-MOVE-RQ"},
		{"CMoveResponse", CMoveResponse, "C-MOVE-RSP"},
		{"CEchoRequest", CEchoRequest, "C-ECHO-RQ"},
		{"CEchoResponse", CEchoResponse, "C-ECHO-RSP"},
		{"NEventReportRequest", NEventReportRequest, "N-EVENT-REPORT-RQ"},
		{"NEventReportResponse", NEventReportResponse, "N-EVENT-REPORT-RSP"},
		{"NGetRequest", NGetRequest, "N-GET-RQ"},
		{"NGetResponse", NGetResponse, "N-GET-RSP"},
		{"NSetRequest", NSetRequest, "N-SET-RQ"},
		{"NSetResponse", NSetResponse, "N-SET-RSP"},
		{"NActionRequest", NActionRequest, "N-ACTION-RQ"},
		{"NActionResponse", NActionResponse, "N-ACTION-RSP"},
		{"NCreateRequest", NCreateRequest, "N-CREATE-RQ"},
		{"NCreateResponse", NCreateResponse, "N-CREATE-RSP"},
		{"NDeleteRequest", NDeleteRequest, "N-DELETE-RQ"},
		{"NDeleteResponse", NDeleteResponse, "N-DELETE-RSP"},
		{"CCancelRequest", CCancelRequest, "C-CANCEL-RQ"},
		{"unknown", 0xDEAD, "Unknown command 0xDEAD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Description(tt.command); got != tt.want {
				t.Errorf("Description(0x%04X) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}
