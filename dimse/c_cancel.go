package dimse

import (
	"errors"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
)

// CCancelWriteRQ writes a C-CANCEL-RQ command per DICOM PS3.7 C.4.1/C.4.2/C.4.3.
// It targets an in-progress operation identified by messageIDBeingRespondedTo.
func CCancelWriteRQ(pdu network.PDUService, messageIDBeingRespondedTo uint16) error {
	if messageIDBeingRespondedTo == 0 {
		return errors.New("CCancelWriteRQ: MessageIDBeingRespondedTo is required")
	}

	sopClassUID := sopClassUID(pdu)
	if sopClassUID == "" {
		return errors.New("CCancelWriteRQ: AffectedSOPClassUID is required")
	}

	sopClassUIDLength := paddedLen(sopClassUID)

	// Command Group Length payload fields:
	// Affected SOP Class UID, Command Field, Message ID,
	// Message ID Being Responded To, Command Data Set Type.
	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2)

	commandObj := media.NewEmptyDCMObj()
	commandObj.WriteUint32(tags.CommandGroupLength, commandLength)
	commandObj.WriteString(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.WriteUint16(tags.CommandField, dicomcommand.CCancelRequest)
	commandObj.WriteUint16(tags.MessageID, network.Uniq16odd())
	commandObj.WriteUint16(tags.MessageIDBeingRespondedTo, messageIDBeingRespondedTo)
	commandObj.WriteUint16(tags.CommandDataSetType, dicomcommand.DataSetNone)

	return pdu.Write(commandObj, network.PDVCommand)
}
