package dicomcommand

import "fmt"

var commandDescriptions = map[uint16]string{
	CStoreRequest:        "C-STORE-RQ",
	CStoreResponse:       "C-STORE-RSP",
	CGetRequest:          "C-GET-RQ",
	CGetResponse:         "C-GET-RSP",
	CFindRequest:         "C-FIND-RQ",
	CFindResponse:        "C-FIND-RSP",
	CMoveRequest:         "C-MOVE-RQ",
	CMoveResponse:        "C-MOVE-RSP",
	CEchoRequest:         "C-ECHO-RQ",
	CEchoResponse:        "C-ECHO-RSP",
	NEventReportRequest:  "N-EVENT-REPORT-RQ",
	NEventReportResponse: "N-EVENT-REPORT-RSP",
	NGetRequest:          "N-GET-RQ",
	NGetResponse:         "N-GET-RSP",
	NSetRequest:          "N-SET-RQ",
	NSetResponse:         "N-SET-RSP",
	NActionRequest:       "N-ACTION-RQ",
	NActionResponse:      "N-ACTION-RSP",
	NCreateRequest:       "N-CREATE-RQ",
	NCreateResponse:      "N-CREATE-RSP",
	NDeleteRequest:       "N-DELETE-RQ",
	NDeleteResponse:      "N-DELETE-RSP",
	CCancelRequest:       "C-CANCEL-RQ",
}

// Description returns a human-readable name for a DICOM command field value.
func Description(command uint16) string {
	if desc, ok := commandDescriptions[command]; ok {
		return desc
	}
	return fmt.Sprintf("Unknown command 0x%04X", command)
}
