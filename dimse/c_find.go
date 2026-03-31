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

// CFindReadRQ CFind request read
func CFindReadRQ(pdu network.PDUService) (media.DICOMObject, error) {
	return pdu.NextPDU()
}

// CFindWriteRQ CFind request write
func CFindWriteRQ(pdu network.PDUService, dataObj media.DICOMObject) error {
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

	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CFindRequest)
	commandObj.WriteUint16(tags.MessageID, network.Uniq16odd())
	commandObj.WriteUint16(tags.Priority, priority.Medium)
	commandObj.WriteUint16(tags.CommandDataSetType, 0x0102)

	if err := pdu.Write(commandObj, 0x01); err != nil {
		return err
	}
	return pdu.Write(dataObj, 0x00)
}

// CFindReadRSP CFind response read
func CFindReadRSP(pdu network.PDUService) (media.DICOMObject, uint16, error) {
	responseCommandObj, err := pdu.NextPDU()
	if err != nil {
		return nil, dicomstatus.FailureUnableToProcess, err
	}

	// Is this a C-Find RSP?
	if responseCommandObj.GetUShort(tags.CommandField) == dicomcommand.CFindResponse {
		if responseCommandObj.GetUShort(tags.CommandDataSetType) != 0x0101 {
			responseDataObj, err := pdu.NextPDU()
			if err != nil {
				return nil, dicomstatus.FailureUnableToProcess, err
			}
			return responseDataObj, responseCommandObj.GetUShort(tags.Status), nil
		}
		return nil, responseCommandObj.GetUShort(tags.Status), nil
	}
	return nil, dicomstatus.FailureUnableToProcess, errors.New("CFindReadRSP, unknown error")
}

// CFindWriteRSP CFind response write
func CFindWriteRSP(pdu network.PDUService, requestCommandObj media.DICOMObject, responseDataObj media.DICOMObject, status uint16) error {
	responseCommandObj := media.NewEmptyDCMObj()

	responseCommandObj.SetTransferSyntax(requestCommandObj.GetTransferSyntax())

	leDSType := uint16(0x0101)
	if responseDataObj.TagCount() > 0 {
		leDSType = 0x0102
	}

	sopClassUID := requestCommandObj.GetString(tags.AffectedSOPClassUID)
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength > 0 {
		if sopClassUIDLength%2 == 1 {
			sopClassUIDLength++
		}

		commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

		responseCommandObj.WriteUint32(tags.CommandGroupLength, commandLength)
		responseCommandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
		responseCommandObj.WriteUint16(tags.CommandField, dicomcommand.CFindResponse)
		messageID := requestCommandObj.GetUShort(tags.MessageID)
		responseCommandObj.WriteUint16(tags.MessageIDBeingRespondedTo, messageID)
		responseCommandObj.WriteUint16(tags.CommandDataSetType, leDSType)
		responseCommandObj.WriteUint16(tags.Status, status)

		if err := pdu.Write(responseCommandObj, 0x01); err != nil {
			return err
		}

		if responseDataObj.TagCount() > 0 {
			return pdu.Write(responseDataObj, 0x00)
		}
	}
	return errors.New("CFindReadRSP, unknown error")
}
