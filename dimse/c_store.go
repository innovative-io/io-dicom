package dimse

import (
	"errors"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/network/priority"
)

// CStoreReadRQ CStore request read
func CStoreReadRQ(pdu network.PDUService, commandObj media.DICOMObject) (media.DICOMObject, error) {
	return pdu.NextPDU()
}

// CStoreWriteRQ CStore request write
func CStoreWriteRQ(pdu network.PDUService, dataObj media.DICOMObject) error {
	commandObj := media.NewEmptyDCMObj()

	sopClassUID := ""
	for _, presContext := range pdu.GetAAssociationRQ().GetPresContexts() {
		sopClassUID = presContext.GetAbstractSyntax().GetUID()
	}
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	sopInstanceUID := dataObj.GetString(tags.SOPInstanceUID)
	sopInstanceUIDLength := uint32(len(sopInstanceUID))
	if sopInstanceUIDLength%2 == 1 {
		sopInstanceUIDLength++
		commandLength = commandLength + 8 + sopInstanceUIDLength
	}

	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CStoreRequest)
	commandObj.WriteUint16(tags.MessageID, network.Uniq16odd())
	commandObj.WriteUint16(tags.Priority, priority.Medium)
	commandObj.WriteUint16(tags.CommandDataSetType, 0x0102)

	if sopInstanceUIDLength > 0 {
		commandObj.WriteString(tags.AffectedSOPInstanceUID, sopInstanceUID)
	}

	if err := pdu.Write(commandObj, 0x01); err != nil {
		return err
	}
	return pdu.Write(dataObj, 0x00)
}

// CStoreReadRSP CStore response read
func CStoreReadRSP(pdu network.PDUService) (uint16, error) {
	dco, err := pdu.NextPDU()
	if err != nil {
		return dicomstatus.FailureUnableToProcess, err
	}
	// Is this a C-Store RSP?
	if dco.GetUShort(tags.CommandField) == dicomcommand.CStoreResponse {
		return dco.GetUShort(tags.Status), nil
	}
	return dicomstatus.FailureUnableToProcess, errors.New("CStoreReadRSP, unknown error")
}

// CStoreWriteRSP CStore response write
func CStoreWriteRSP(pdu network.PDUService, requestCommandObj media.DICOMObject, status uint16) error {
	responseCommandObj := media.NewEmptyDCMObj()

	responseCommandObj.SetTransferSyntax(requestCommandObj.GetTransferSyntax())
	sopClassUID := requestCommandObj.GetString(tags.AffectedSOPClassUID)
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength > 0 {
		if sopClassUIDLength%2 == 1 {
			sopClassUIDLength++
		}

		sopInstanceUID := requestCommandObj.GetString(tags.AffectedSOPInstanceUID)
		sopInstanceUIDLength := uint16(len(sopClassUID))
		if sopInstanceUIDLength > 0 {
			if sopInstanceUIDLength%2 == 1 {
				sopInstanceUIDLength++
			}

			commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2 + 8 + sopInstanceUIDLength)

			responseCommandObj.WriteUint32(tags.CommandGroupLength, commandLength)
			responseCommandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
			responseCommandObj.WriteUint16(tags.CommandField, dicomcommand.CStoreResponse)
			messageID := requestCommandObj.GetUShort(tags.MessageID)
			responseCommandObj.WriteUint16(tags.MessageIDBeingRespondedTo, messageID)
			responseCommandObj.WriteUint16(tags.CommandDataSetType, 0x0101)
			responseCommandObj.WriteUint16(tags.Status, status)
			responseCommandObj.WriteString(tags.AffectedSOPInstanceUID, sopInstanceUID)
			return pdu.Write(responseCommandObj, 0x01)
		}
	}
	return errors.New("CStoreWriteRSP, unknown error")
}
