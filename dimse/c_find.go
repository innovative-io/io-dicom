package dimse

import (
	"errors"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

// CFindWriteRQ CFind request write
func CFindWriteRQ(pdu network.PDUService, dataObj media.DICOMObject, pri uint16) error {
	commandObj := media.NewEmptyDCMObj()

	sopClassUID := sopClassUID(pdu)
	if sopClassUID == "" {
		return errors.New("CFindWriteRQ: AffectedSOPClassUID is required")
	}
	sopClassUIDLength := paddedLen(sopClassUID)

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	commandObj.Write(tags.CommandGroupLength, commandLength)
	commandObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.Write(tags.CommandField, dicomcommand.CFindRequest)
	commandObj.Write(tags.MessageID, network.Uniq16odd())
	commandObj.Write(tags.Priority, pri)
	commandObj.Write(tags.CommandDataSetType, dicomcommand.DataSetPresent)

	if err := pdu.Write(commandObj, network.PDVCommand); err != nil {
		return err
	}
	return pdu.Write(dataObj, network.PDVDataset)
}

// CFindReadRSP CFind response read
func CFindReadRSP(pdu network.PDUService) (media.DICOMObject, uint16, error) {
	responseCommandObj, err := pdu.NextPDU()
	if err != nil {
		return nil, dicomstatus.FailureProcessingFailure, err
	}

	// Is this a C-Find RSP?
	if responseCommandObj.GetUint16(tags.CommandField) == dicomcommand.CFindResponse {
		status := responseCommandObj.GetUint16(tags.Status)
		if err := validateQROperationStatus(status, "CFindReadRSP"); err != nil {
			return nil, dicomstatus.FailureProcessingFailure, err
		}
		commandDataSetType := responseCommandObj.GetUint16(tags.CommandDataSetType)

		if commandDataSetType != dicomcommand.DataSetNone && commandDataSetType != dicomcommand.DataSetPresent {
			return nil, dicomstatus.FailureProcessingFailure,
				errors.New("CFindReadRSP: invalid CommandDataSetType (must be DataSetNone or DataSetPresent)")
		}

		if (status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings) && commandDataSetType == dicomcommand.DataSetNone {
			return nil, dicomstatus.FailureProcessingFailure,
				errors.New("CFindReadRSP: pending response must include an identifier dataset")
		}

		if status != dicomstatus.Pending && status != dicomstatus.PendingWithWarnings && commandDataSetType == dicomcommand.DataSetPresent {
			return nil, dicomstatus.FailureProcessingFailure,
				errors.New("CFindReadRSP: final response must not include an identifier dataset")
		}

		if commandDataSetType == dicomcommand.DataSetPresent {
			responseDataObj, err := pdu.NextPDU()
			if err != nil {
				return nil, dicomstatus.FailureProcessingFailure, err
			}
			return responseDataObj, status, nil
		}
		return nil, status, nil
	}
	return nil, dicomstatus.FailureProcessingFailure, errors.New("CFindReadRSP: unknown response command")
}

// CFindWriteRSP CFind response write
func CFindWriteRSP(pdu network.PDUService, requestCommandObj media.DICOMObject, responseDataObj media.DICOMObject, status uint16) error {
	if requestCommandObj == nil {
		return errors.New("CFindWriteRSP: requestCommandObj cannot be nil")
	}

	responseCommandObj := media.NewEmptyDCMObj()

	responseCommandObj.SetTransferSyntax(requestCommandObj.GetTransferSyntax())

	leDSType := dicomcommand.DataSetNone
	if responseDataObj.TagCount() > 0 {
		leDSType = dicomcommand.DataSetPresent
	}

	isPending := status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings
	if err := validateQROperationStatus(status, "CFindWriteRSP"); err != nil {
		return err
	}
	if isPending && leDSType != dicomcommand.DataSetPresent {
		return errors.New("CFindWriteRSP: pending response must include an identifier dataset")
	}
	if !isPending && leDSType == dicomcommand.DataSetPresent {
		return errors.New("CFindWriteRSP: final response must not include an identifier dataset")
	}

	sopClassUID := requestCommandObj.GetString(tags.AffectedSOPClassUID)
	if sopClassUID == "" {
		return errors.New("CFindWriteRSP: AffectedSOPClassUID is required")
	}

	messageID := requestCommandObj.GetUint16(tags.MessageID)
	if messageID == 0 {
		return errors.New("CFindWriteRSP: MessageID is required")
	}

	sopClassUIDLength := paddedLen(sopClassUID)

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	responseCommandObj.Write(tags.CommandGroupLength, commandLength)
	responseCommandObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	responseCommandObj.Write(tags.CommandField, dicomcommand.CFindResponse)
	responseCommandObj.Write(tags.MessageIDBeingRespondedTo, messageID)
	responseCommandObj.Write(tags.CommandDataSetType, leDSType)
	responseCommandObj.Write(tags.Status, status)

	if err := pdu.Write(responseCommandObj, network.PDVCommand); err != nil {
		return err
	}

	if responseDataObj.TagCount() > 0 {
		return pdu.Write(responseDataObj, network.PDVDataset)
	}
	return nil
}
