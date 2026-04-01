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

// CGetWriteRQ writes a C-GET-RQ command and identifier dataset.
func CGetWriteRQ(pdu network.PDUService, dataObj media.DICOMObject) error {
	commandObj := media.NewEmptyDCMObj()

	sopClassUID := sopClassUID(pdu)
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CGetRequest)
	commandObj.WriteUint16(tags.MessageID, network.Uniq16odd())
	commandObj.WriteUint16(tags.Priority, priority.Medium)
	commandObj.WriteUint16(tags.CommandDataSetType, 0x0102)

	if err := pdu.Write(commandObj, 0x01); err != nil {
		return err
	}
	return pdu.Write(dataObj, 0x00)
}

// CGetReadRSP reads a C-GET-RSP and updates pending with remaining sub-ops.
func CGetReadRSP(pdu network.PDUService, pending *int) (media.DICOMObject, uint16, error) {
	responseCommandObj, err := pdu.NextPDU()
	if err != nil {
		return nil, dicomstatus.FailureProcessingFailure, fmt.Errorf("CGetReadRSP: failed to read response PDU: %w", err)
	}

	if responseCommandObj.GetUShort(tags.CommandField) != dicomcommand.CGetResponse {
		return nil, dicomstatus.FailureProcessingFailure,
			fmt.Errorf("CGetReadRSP: expected C-GET Response (0x%04X), got 0x%04X",
				dicomcommand.CGetResponse, responseCommandObj.GetUShort(tags.CommandField))
	}

	status := responseCommandObj.GetUShort(tags.Status)
	commandDataSetType := responseCommandObj.GetUShort(tags.CommandDataSetType)
	remaining := int(responseCommandObj.GetUShort(tags.NumberOfRemainingSuboperations))

	if status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
		if pending != nil {
			*pending = remaining
		}
	} else if pending != nil {
		*pending = -1
	}

	if commandDataSetType == 0x0102 {
		responseDataObj, err := pdu.NextPDU()
		if err != nil {
			return nil, status, fmt.Errorf("CGetReadRSP: failed to read response dataset: %w", err)
		}
		return responseDataObj, status, nil
	}

	return nil, status, nil
}

// CGetWriteRSP writes a C-GET-RSP with required sub-operation counters.
func CGetWriteRSP(pdu network.PDUService, requestCommandObj media.DICOMObject, status uint16, remaining, completed, failed, warnings uint16) error {
	if requestCommandObj == nil {
		return errors.New("CGetWriteRSP: requestCommandObj cannot be nil")
	}

	responseCommandObj := media.NewEmptyDCMObj()
	responseCommandObj.SetTransferSyntax(requestCommandObj.GetTransferSyntax())

	sopClassUID := requestCommandObj.GetString(tags.AffectedSOPClassUID)
	if sopClassUID == "" {
		return errors.New("CGetWriteRSP: Affected SOP Class UID is required")
	}

	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2)

	responseCommandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	responseCommandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	responseCommandObj.WriteUint16(tags.CommandField, dicomcommand.CGetResponse)
	responseCommandObj.WriteUint16(tags.MessageIDBeingRespondedTo, requestCommandObj.GetUShort(tags.MessageID))
	responseCommandObj.WriteUint16(tags.CommandDataSetType, 0x0101)
	responseCommandObj.WriteUint16(tags.Status, status)
	responseCommandObj.WriteUint16(tags.NumberOfRemainingSuboperations, remaining)
	responseCommandObj.WriteUint16(tags.NumberOfCompletedSuboperations, completed)
	responseCommandObj.WriteUint16(tags.NumberOfFailedSuboperations, failed)
	responseCommandObj.WriteUint16(tags.NumberOfWarningSuboperations, warnings)

	return pdu.Write(responseCommandObj, 0x01)
}
