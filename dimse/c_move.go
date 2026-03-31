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

// CMoveReadRQ CMove request read
func CMoveReadRQ(pdu network.PDUService) (media.DICOMObject, error) {
	return pdu.NextPDU()
}

// CMoveWriteRQ CMove request write
func CMoveWriteRQ(pdu network.PDUService, dataObj media.DICOMObject, destinationAETitle string) error {
	commandObj := media.NewEmptyDCMObj()

	destinationAETitleLength := uint16(len(destinationAETitle))
	if destinationAETitleLength%2 == 1 {
		destinationAETitleLength++
	}

	sopClassUID := ""
	for _, presContext := range pdu.GetAAssociationRQ().GetPresContexts() {
		sopClassUID = presContext.GetAbstractSyntax().GetUID()
	}
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + destinationAETitleLength + 8 + 2 + 8 + 2)

	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CMoveRequest)
	commandObj.WriteUint16(tags.MessageID, network.Uniq16odd())
	commandObj.WriteString(tags.MoveDestination, destinationAETitle)
	commandObj.WriteUint16(tags.Priority, priority.Medium)
	commandObj.WriteUint16(tags.CommandDataSetType, 0x0102)

	if err := pdu.Write(commandObj, 0x01); err != nil {
		return err
	}
	return pdu.Write(dataObj, 0x00)
}

// CMoveReadRSP CMove response read
func CMoveReadRSP(pdu network.PDUService, pending *int) (media.DICOMObject, uint16, error) {
	status := dicomstatus.FailureUnableToProcess
	responseCommandObj, err := pdu.NextPDU()
	if err != nil {
		return nil, dicomstatus.FailureUnableToProcess, err
	}

	if responseCommandObj.GetUShort(tags.CommandField) == dicomcommand.CMoveResponse {
		if responseCommandObj.GetUShort(tags.CommandDataSetType) != 0x0101 {
			responseDataObj, err := pdu.NextPDU()
			if err != nil {
				return nil, dicomstatus.FailureUnableToProcess, err
			}
			status = responseCommandObj.GetUShort(tags.Status)
			*pending = int(responseCommandObj.GetUShort(tags.NumberOfRemainingSuboperations))
			return responseDataObj, status, nil
		}
		status = responseCommandObj.GetUShort(tags.Status)
		*pending = -1
	}

	return nil, status, nil
}

// CMoveWriteRSP CMove response write
func CMoveWriteRSP(pdu network.PDUService, requestCommandObj media.DICOMObject, status uint16, pending uint16) error {
	responseCommandObj := media.NewEmptyDCMObj()

	responseCommandObj.SetTransferSyntax(requestCommandObj.GetTransferSyntax())

	sopClassUID := requestCommandObj.GetString(tags.AffectedSOPClassUID)
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength > 0 {
		if sopClassUIDLength%2 == 1 {
			sopClassUIDLength++
		}

		commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2)

		responseCommandObj.WriteUint32(tags.CommandGroupLength, commandLength)
		responseCommandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
		responseCommandObj.WriteUint16(tags.CommandField, dicomcommand.CMoveResponse)
		messageID := requestCommandObj.GetUShort(tags.MessageID)
		responseCommandObj.WriteUint16(tags.MessageIDBeingRespondedTo, messageID)
		responseCommandObj.WriteUint16(tags.CommandDataSetType, 0x101)
		responseCommandObj.WriteUint16(tags.Status, status)
		responseCommandObj.WriteUint16(tags.NumberOfRemainingSuboperations, pending)

		return pdu.Write(responseCommandObj, 0x01)
	}
	return errors.New("CMoveWriteRSP, unknown error")
}
