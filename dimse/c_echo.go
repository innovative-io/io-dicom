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
	if sopClassUID == "" {
		return errors.New("CEchoWriteRQ: AffectedSOPClassUID is required")
	}
	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CEchoRequest)
	commandObj.WriteUint16(tags.MessageID, network.Uniq16odd())
	commandObj.WriteUint16(tags.CommandDataSetType, dicomcommand.DataSetNone)

	return pdu.Write(commandObj, 0x01)
}

// CEchoReadRSP CEcho response read
func CEchoReadRSP(pdu network.PDUService) error {
	dco, err := pdu.NextPDU()
	if err != nil {
		return errors.New("CEchoReadRSP: failed to read response PDU")
	}
	if dco.GetUShort(tags.CommandField) == dicomcommand.CEchoResponse {
		if dco.GetUShort(tags.CommandDataSetType) != dicomcommand.DataSetNone {
			return errors.New("CEchoReadRSP: CommandDataSetType must be DataSetNone")
		}
		if dco.GetUShort(tags.MessageIDBeingRespondedTo) == 0 {
			return errors.New("CEchoReadRSP: MessageIDBeingRespondedTo is required")
		}
		if dco.GetUShort(tags.Status) == dicomstatus.Success {
			return nil
		}
		return errors.New("CEchoReadRSP: received non-success status")
	}
	return errors.New("CEchoReadRSP: unknown response command")
}

// CEchoWriteRSP CEcho response write
func CEchoWriteRSP(pdu network.PDUService, commandObj media.DICOMObject) error {
	if commandObj == nil {
		return errors.New("CEchoWriteRSP: commandObj cannot be nil")
	}

	responseObj := media.NewEmptyDCMObj()

	responseObj.SetTransferSyntax(commandObj.GetTransferSyntax())
	sopClassUID := commandObj.GetString(tags.AffectedSOPClassUID)
	if sopClassUID == "" {
		return errors.New("CEchoWriteRSP: AffectedSOPClassUID is required")
	}

	messageID := commandObj.GetUShort(tags.MessageID)
	if messageID == 0 {
		return errors.New("CEchoWriteRSP: MessageID is required")
	}

	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	responseObj.WriteUint32(tags.CommandGroupLength, commandLength)
	responseObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	responseObj.WriteUint16(tags.CommandField, dicomcommand.CEchoResponse)
	responseObj.WriteUint16(tags.MessageIDBeingRespondedTo, messageID)
	responseObj.WriteUint16(tags.CommandDataSetType, dicomcommand.DataSetNone)
	responseObj.WriteUint16(tags.Status, dicomstatus.Success)
	return pdu.Write(responseObj, 0x01)
}
