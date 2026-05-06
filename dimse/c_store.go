package dimse

import (
	"errors"
	"fmt"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
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
func CStoreWriteRQ(pdu network.PDUService, dataObj media.DICOMObject, pri uint16) error {
	commandObj := media.NewEmptyDCMObj()

	// Prefer the data object's own SOP class UID so that C-STORE sub-operations
	// sent by a C-GET SCP use the correct SOP class rather than the active PDU context.
	scUID := dataObj.GetString(tags.SOPClassUID)
	if scUID == "" {
		scUID = sopClassUID(pdu)
	}
	if scUID == "" {
		return errors.New("CStoreWriteRQ: AffectedSOPClassUID is required")
	}
	sopClassUIDLength := paddedLen(scUID)

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	// AffectedSOPInstanceUID (0000,1000) is required in C-STORE-RQ per PS3.7 §C.3.1.
	// Always include it in CommandGroupLength regardless of even/odd length.
	sopInstanceUID := dataObj.GetString(tags.SOPInstanceUID)
	if sopInstanceUID == "" {
		return errors.New("CStoreWriteRQ: SOPInstanceUID is required")
	}
	sopInstanceUIDLength := uint32(paddedLen(sopInstanceUID))
	commandLength = commandLength + 8 + sopInstanceUIDLength

	commandObj.Write(tags.CommandGroupLength, commandLength)
	commandObj.Write(tags.AffectedSOPClassUID, scUID)
	commandObj.Write(tags.CommandField, dicomcommand.CStoreRequest)
	commandObj.Write(tags.MessageID, network.Uniq16odd())
	commandObj.Write(tags.Priority, pri)
	commandObj.Write(tags.CommandDataSetType, dicomcommand.DataSetPresent)
	commandObj.Write(tags.AffectedSOPInstanceUID, sopInstanceUID)

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
	if dco.GetUint16(tags.CommandField) == dicomcommand.CStoreResponse {
		if dco.GetUint16(tags.CommandDataSetType) != dicomcommand.DataSetNone {
			return dicomstatus.FailureProcessingFailure, errors.New("CStoreReadRSP: CommandDataSetType must be DataSetNone")
		}
		if dco.GetUint16(tags.MessageIDBeingRespondedTo) == 0 {
			return dicomstatus.FailureProcessingFailure, errors.New("CStoreReadRSP: MessageIDBeingRespondedTo is required")
		}
		status := dco.GetUint16(tags.Status)
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

	messageID := requestCommandObj.GetUint16(tags.MessageID)
	if messageID == 0 {
		return errors.New("CStoreWriteRSP: MessageID is required")
	}

	if err := validateCStoreStatus(status, "CStoreWriteRSP"); err != nil {
		return err
	}

	sopClassUIDLength := paddedLen(sopClassUID)

	// sopInstanceUIDLength must be computed from sopInstanceUID, not sopClassUID.
	sopInstanceUIDLength := paddedLen(sopInstanceUID)

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2 + 8 + sopInstanceUIDLength)

	responseCommandObj.Write(tags.CommandGroupLength, commandLength)
	responseCommandObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	responseCommandObj.Write(tags.CommandField, dicomcommand.CStoreResponse)
	responseCommandObj.Write(tags.MessageIDBeingRespondedTo, messageID)
	responseCommandObj.Write(tags.CommandDataSetType, dicomcommand.DataSetNone)
	responseCommandObj.Write(tags.Status, status)
	responseCommandObj.Write(tags.AffectedSOPInstanceUID, sopInstanceUID)
	return pdu.Write(responseCommandObj, network.PDVCommand)
}
