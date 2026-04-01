package dicomcommand

// CStoreRequest = 0x0001
const CStoreRequest = 0x0001

// CStoreResponse = 0x8001
const CStoreResponse = 0x8001

// CGetRequest = 0x0010
const CGetRequest = 0x0010

// CGetResponse = 0x8010
const CGetResponse = 0x8010

// CFindRequest = 0x0020
const CFindRequest = 0x0020

// CFindResponse = 0x8020
const CFindResponse = 0x8020

// CMoveRequest = 0x0021
const CMoveRequest = 0x0021

// CMoveResponse = 0x8021
const CMoveResponse = 0x8021

// CEchoRequest = 0x0030
const CEchoRequest = 0x0030

// CEchoResponse = 0x8030
const CEchoResponse = 0x8030

// NEventReportRequest = 0x0100
const NEventReportRequest = 0x0100

// NEventReportResponse = 0x8100
const NEventReportResponse = 0x8100

// NGetRequest = 0x0110
const NGetRequest = 0x0110

// NGetResponse = 0x8110
const NGetResponse = 0x8110

// NSetRequest = 0x0120
const NSetRequest = 0x0120

// NSetResponse = 0x8120
const NSetResponse = 0x8120

// NActionRequest = 0x0130
const NActionRequest = 0x0130

// NActionResponse = 0x8130
const NActionResponse = 0x8130

// NCreateRequest = 0x0140
const NCreateRequest = 0x0140

// NCreateResponse = 0x8140
const NCreateResponse = 0x8140

// NDeleteRequest = 0x0150
const NDeleteRequest = 0x0150

// NDeleteResponse = 0x8150
const NDeleteResponse = 0x8150

// CCancelRequest = 0x0FFF
const CCancelRequest = 0x0FFF

// DataSetNone - 0x0101
// CommandDataSetType value indicating no dataset is included in the message.
const DataSetNone uint16 = 0x0101

// DataSetPresent - 0x0102
// CommandDataSetType value indicating a dataset follows the command.
const DataSetPresent uint16 = 0x0102
