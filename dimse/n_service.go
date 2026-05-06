package dimse

import (
	"errors"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
)

// NWriteRSP writes a generic N-service response command and optional dataset.
// requestCommandField must be one of the N-service request command values.
func NWriteRSP(pdu network.PDUService, requestCommandObj media.DICOMObject, responseCommandField uint16, status uint16, responseDataObj media.DICOMObject) error {
	if requestCommandObj == nil {
		return errors.New("NWriteRSP: requestCommandObj cannot be nil")
	}

	responseCommandObj := media.NewEmptyDCMObj()
	responseCommandObj.SetTransferSyntax(requestCommandObj.GetTransferSyntax())

	sopClassUID := requestCommandObj.GetString(tags.RequestedSOPClassUID)
	if sopClassUID == "" {
		sopClassUID = requestCommandObj.GetString(tags.AffectedSOPClassUID)
	}
	if sopClassUID == "" {
		return errors.New("NWriteRSP: SOP Class UID is required")
	}

	sopInstanceUID := requestCommandObj.GetString(tags.RequestedSOPInstanceUID)
	if sopInstanceUID == "" {
		sopInstanceUID = requestCommandObj.GetString(tags.AffectedSOPInstanceUID)
	}

	sopClassUIDLength := uint16(len(sopClassUID))
	if sopClassUIDLength%2 == 1 {
		sopClassUIDLength++
	}
	sopInstanceUIDLength := uint16(len(sopInstanceUID))
	if sopInstanceUIDLength%2 == 1 {
		sopInstanceUIDLength++
	}

	leDSType := dicomcommand.DataSetNone
	if responseDataObj != nil && responseDataObj.TagCount() > 0 {
		leDSType = dicomcommand.DataSetPresent
	}

	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2)
	if sopInstanceUID != "" {
		commandLength += 8 + uint32(sopInstanceUIDLength)
	}

	responseCommandObj.Write(tags.CommandGroupLength, commandLength)
	responseCommandObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	responseCommandObj.Write(tags.CommandField, responseCommandField)
	responseCommandObj.Write(tags.MessageIDBeingRespondedTo, requestCommandObj.GetUint16(tags.MessageID))
	responseCommandObj.Write(tags.CommandDataSetType, leDSType)
	responseCommandObj.Write(tags.Status, status)
	if sopInstanceUID != "" {
		responseCommandObj.Write(tags.AffectedSOPInstanceUID, sopInstanceUID)
	}

	if err := pdu.Write(responseCommandObj, network.PDVCommand); err != nil {
		return err
	}
	if leDSType == dicomcommand.DataSetPresent {
		return pdu.Write(responseDataObj, network.PDVDataset)
	}
	return nil
}
