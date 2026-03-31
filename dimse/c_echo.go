package dimse

import (
	"errors"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

// CEchoReadRQ CEcho request read
func CEchoReadRQ(commandObj media.DICOMObject) bool {
	return commandObj.GetUShort(tags.CommandField) == dicomcommand.CEchoRequest
}

// CEchoWriteRQ CEcho request write
func CEchoWriteRQ(pdu network.PDUService) error {
	commandObj := media.NewEmptyDCMObj()

	sopClassUID := sopClassUID(pdu)
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CEchoRequest)
	commandObj.WriteUint16(tags.MessageID, network.Uniq16odd())
	commandObj.WriteUint16(tags.CommandDataSetType, 0x0101)

	return pdu.Write(commandObj, 0x01)
}

// CEchoReadRSP CEcho response read
func CEchoReadRSP(pdu network.PDUService) error {
	dco, err := pdu.NextPDU()
	if err != nil {
		return errors.New("CEchoReadRSP, failed pdu.Read(&DCO)")
	}
	if dco.GetUShort(tags.CommandField) == dicomcommand.CEchoResponse {
		if dco.GetUShort(tags.Status) == dicomstatus.Success {
			return nil
		}
	}
	return nil
}

// CEchoWriteRSP CEcho response write
func CEchoWriteRSP(pdu network.PDUService, commandObj media.DICOMObject) error {
	responseObj := media.NewEmptyDCMObj()

	responseObj.SetTransferSyntax(commandObj.GetTransferSyntax())
	sopClassUID := commandObj.GetString(tags.AffectedSOPClassUID)
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength > 0 {
		if sopClassUIDLength%2 == 1 {
			sopClassUIDLength++
		}

		commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

		responseObj.WriteUint32(tags.CommandGroupLength, commandLength)
		responseObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
		responseObj.WriteUint16(tags.CommandField, dicomcommand.CEchoResponse)
		messageID := commandObj.GetUShort(tags.MessageID)
		responseObj.WriteUint16(tags.MessageIDBeingRespondedTo, messageID)
		commandDataSetType := commandObj.GetUShort(tags.CommandDataSetType)
		responseObj.WriteUint16(tags.CommandDataSetType, commandDataSetType)
		responseObj.WriteUint16(tags.Status, dicomstatus.Success)
		return pdu.Write(responseObj, 0x01)
	}
	return errors.New("CEchoReadRSP, unknown error")
}
