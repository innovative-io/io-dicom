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
	return commandObj.GetUint16(tags.CommandField) == dicomcommand.CEchoRequest
}

// CEchoWriteRQ CEcho request write
func CEchoWriteRQ(pdu network.PDUService) error {
	commandObj := media.NewEmptyDCMObj()

	sopClassUID := sopClassUID(pdu)
	if sopClassUID == "" {
		return errors.New("CEchoWriteRQ: AffectedSOPClassUID is required")
	}
	sopClassUIDLength := paddedLen(sopClassUID)

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	commandObj.Write(tags.CommandGroupLength, commandLength)
	commandObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.Write(tags.CommandField, dicomcommand.CEchoRequest)
	commandObj.Write(tags.MessageID, network.Uniq16odd())
	commandObj.Write(tags.CommandDataSetType, dicomcommand.DataSetNone)

	return pdu.Write(commandObj, network.PDVCommand)
}

// CEchoReadRSP CEcho response read
func CEchoReadRSP(pdu network.PDUService) error {
	dco, err := pdu.NextPDU()
	if err != nil {
		return errors.New("CEchoReadRSP: failed to read response PDU")
	}
	if dco.GetUint16(tags.CommandField) == dicomcommand.CEchoResponse {
		if dco.GetUint16(tags.CommandDataSetType) != dicomcommand.DataSetNone {
			return errors.New("CEchoReadRSP: CommandDataSetType must be DataSetNone")
		}
		if dco.GetUint16(tags.MessageIDBeingRespondedTo) == 0 {
			return errors.New("CEchoReadRSP: MessageIDBeingRespondedTo is required")
		}
		if dco.GetUint16(tags.Status) == dicomstatus.Success {
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

	messageID := commandObj.GetUint16(tags.MessageID)
	if messageID == 0 {
		return errors.New("CEchoWriteRSP: MessageID is required")
	}

	sopClassUIDLength := paddedLen(sopClassUID)

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2)

	responseObj.Write(tags.CommandGroupLength, commandLength)
	responseObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	responseObj.Write(tags.CommandField, dicomcommand.CEchoResponse)
	responseObj.Write(tags.MessageIDBeingRespondedTo, messageID)
	responseObj.Write(tags.CommandDataSetType, dicomcommand.DataSetNone)
	responseObj.Write(tags.Status, dicomstatus.Success)
	return pdu.Write(responseObj, network.PDVCommand)
}
