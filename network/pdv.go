package network

// ProtocolVersionCurrent is the DICOM protocol version advertised in
// A-ASSOCIATE-RQ/AC PDUs. Per PS3.8 Section 9.3.2, this is always 0x0001.
const ProtocolVersionCurrent uint16 = 0x0001

// PDVCommand is the PDV Message Control Header byte value for a command PDU fragment.
// Per DICOM PS3.8 Annex F Table F.1-3, bit 0 = 1 indicates the PDV carries a command.
const PDVCommand byte = 0x01

// PDVDataset is the PDV Message Control Header byte value for a dataset PDU fragment.
// Per DICOM PS3.8 Annex F Table F.1-3, bit 0 = 0 indicates the PDV carries a data set.
const PDVDataset byte = 0x00

// PDVLastFragment is the MsgHeader bit mask for the "last fragment" flag.
// Per DICOM PS3.8 Annex F Table F.1-3, bit 1 = 1 indicates this is the last PDV fragment.
const PDVLastFragment byte = 0x02

// PDVTypeMask is used to clear the last-fragment bit while preserving the command/dataset bit.
const PDVTypeMask byte = 0x01

type presentationDataValue struct {
	Length                uint32
	PresentationContextID byte
	MsgHeader             byte
}
