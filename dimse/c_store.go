package dimse

import (
	"errors"
	"fmt"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/network/priority"
)

func isValidCStoreStatus(status uint16) bool {
	if status == dicomstatus.Success ||
		status == dicomstatus.WarningCoercionOfDataElements ||
		status == dicomstatus.WarningElementsDiscarded ||
		status == dicomstatus.WarningDataSetDoesNotMatchSOPClass ||
		status == dicomstatus.FailureProcessingFailure ||
		status == dicomstatus.FailureSOPClassNotSupported {
		return true
	}

	if status >= dicomstatus.FailureServiceSpecificMin && status <= dicomstatus.FailureServiceSpecificMax {
		return true
	}

	h := status >> 12
	return h == dicomstatus.HighNibbleFailureRefused || h == dicomstatus.HighNibbleFailureCannotUnderstand
}

func validateCStoreStatus(status uint16, op string) error {
	if !isValidCStoreStatus(status) {
		return fmt.Errorf("%s: invalid C-STORE status code %s (0x%04X)", op, dicomstatus.Description(status), status)
	}
	return nil
}

// CStoreWriteRQ CStore request write
func CStoreWriteRQ(pdu network.PDUService, dataObj media.DICOMObject) error {
	commandObj := media.NewEmptyDCMObj()

	sopClassUID := sopClassUID(pdu)
	if sopClassUID == "" {
		return errors.New("CStoreWriteRQ: AffectedSOPClassUID is required")
	}
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	// AffectedSOPInstanceUID (0000,1000) is required in C-STORE-RQ per PS3.7 §C.3.1.
	// Always include it in CommandGroupLength regardless of even/odd length.
	sopInstanceUID := dataObj.GetString(tags.SOPInstanceUID)
	if sopInstanceUID == "" {
		return errors.New("CStoreWriteRQ: SOPInstanceUID is required")
	}
	sopInstanceUIDLength := uint32(len(sopInstanceUID))
	if sopInstanceUIDLength%2 == 1 {
		sopInstanceUIDLength++
	}
	commandLength = commandLength + 8 + sopInstanceUIDLength

	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CStoreRequest)
	commandObj.WriteUint16(tags.MessageID, network.Uniq16odd())
	commandObj.WriteUint16(tags.Priority, priority.Medium)
	commandObj.WriteUint16(tags.CommandDataSetType, dicomcommand.DataSetPresent)
	commandObj.WriteString(tags.AffectedSOPInstanceUID, sopInstanceUID)

	if err := pdu.Write(commandObj, network.PDVCommand); err != nil {
		return err
	}
	return pdu.Write(dataObj, network.PDVDataset)
}

// CStoreReadRSP CStore response read
func CStoreReadRSP(pdu network.PDUService) (uint16, error) {
	dco, err := pdu.NextPDU()
	if err != nil {
		return dicomstatus.FailureProcessingFailure, err
	}
	// Is this a C-Store RSP?
	if dco.GetUShort(tags.CommandField) == dicomcommand.CStoreResponse {
		if dco.GetUShort(tags.CommandDataSetType) != dicomcommand.DataSetNone {
			return dicomstatus.FailureProcessingFailure, errors.New("CStoreReadRSP: CommandDataSetType must be DataSetNone")
		}
		if dco.GetUShort(tags.MessageIDBeingRespondedTo) == 0 {
			return dicomstatus.FailureProcessingFailure, errors.New("CStoreReadRSP: MessageIDBeingRespondedTo is required")
		}
		status := dco.GetUShort(tags.Status)
		if err := validateCStoreStatus(status, "CStoreReadRSP"); err != nil {
			return dicomstatus.FailureProcessingFailure, err
		}
		return status, nil
	}
	return dicomstatus.FailureProcessingFailure, errors.New("CStoreReadRSP: unknown response command")
}

// CStoreWriteRSP CStore response write
func CStoreWriteRSP(pdu network.PDUService, requestCommandObj media.DICOMObject, status uint16) error {
	if requestCommandObj == nil {
		return errors.New("CStoreWriteRSP: requestCommandObj cannot be nil")
	}

	responseCommandObj := media.NewEmptyDCMObj()

	responseCommandObj.SetTransferSyntax(requestCommandObj.GetTransferSyntax())
	sopClassUID := requestCommandObj.GetString(tags.AffectedSOPClassUID)
	if sopClassUID == "" {
		return errors.New("CStoreWriteRSP: AffectedSOPClassUID is required")
	}

	sopInstanceUID := requestCommandObj.GetString(tags.AffectedSOPInstanceUID)
	if sopInstanceUID == "" {
		return errors.New("CStoreWriteRSP: AffectedSOPInstanceUID is required")
	}

	messageID := requestCommandObj.GetUShort(tags.MessageID)
	if messageID == 0 {
		return errors.New("CStoreWriteRSP: MessageID is required")
	}

	if err := validateCStoreStatus(status, "CStoreWriteRSP"); err != nil {
		return err
	}

	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	// sopInstanceUIDLength must be computed from sopInstanceUID, not sopClassUID.
	sopInstanceUIDLength := uint16(len(sopInstanceUID))
	if sopInstanceUIDLength%2 == 1 {
		sopInstanceUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2 + 8 + sopInstanceUIDLength)

	responseCommandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	responseCommandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	responseCommandObj.WriteUint16(tags.CommandField, dicomcommand.CStoreResponse)
	responseCommandObj.WriteUint16(tags.MessageIDBeingRespondedTo, messageID)
	responseCommandObj.WriteUint16(tags.CommandDataSetType, dicomcommand.DataSetNone)
	responseCommandObj.WriteUint16(tags.Status, status)
	responseCommandObj.WriteString(tags.AffectedSOPInstanceUID, sopInstanceUID)
	return pdu.Write(responseCommandObj, network.PDVCommand)
}
