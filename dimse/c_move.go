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

// CMoveRequest represents a parsed C-MOVE request per DICOM PS3.7 §C.4.2.1.
// It encapsulates the command fields and optional identifier dataset.
type CMoveRequest struct {
	// Command fields (from C-MOVE-RQ)
	AffectedSOPClassUID string
	MessageID           uint16
	Priority            uint16
	MoveDestination     string
	CommandDataSetType  uint16

	// Identifier dataset (optional, follows the command)
	IdentifierDataSet media.DICOMObject

	// Request for internal tracking
	requestCommandObj media.DICOMObject
}

// CMoveReadRQ reads a C-MOVE-RQ from the PDU service and returns a parsed CMoveRequest.
// Per DICOM PS3.7 §C.4.2.1, the C-MOVE-RQ command must include:
//   - (0000,0000) Command Group Length
//   - (0000,0002) Affected SOP Class UID
//   - (0000,0100) Command Field = 0x0021 (C-MOVE Request)
//   - (0000,0110) Message ID
//   - (0000,0600) Move Destination
//   - (0000,0700) Priority (optional, defaults to Medium)
//   - (0000,0800) Command Data Set Type
//
// If CommandDataSetType indicates identifier present (DataSetPresent), the identifier
// dataset is read from the next PDU.
func CMoveReadRQ(pdu network.PDUService) (*CMoveRequest, error) {
	commandObj, err := pdu.NextPDU()
	if err != nil {
		return nil, fmt.Errorf("CMoveReadRQ: failed to read command PDU: %w", err)
	}

	if commandObj.GetUint16(tags.CommandField) != dicomcommand.CMoveRequest {
		return nil, errors.New("CMoveReadRQ: CommandField is not C-MOVE Request")
	}

	req := &CMoveRequest{
		AffectedSOPClassUID: commandObj.GetString(tags.AffectedSOPClassUID),
		MessageID:           commandObj.GetUint16(tags.MessageID),
		Priority:            commandObj.GetUint16(tags.Priority),
		MoveDestination:     commandObj.GetString(tags.MoveDestination),
		CommandDataSetType:  commandObj.GetUint16(tags.CommandDataSetType),
		requestCommandObj:   commandObj,
	}

	// Validate required fields per PS3.7 §C.4.2.1.1
	if err := validateCMoveRequest(req); err != nil {
		return nil, err
	}

	// If CommandDataSetType indicates identifier dataset present (any value != 0x0101),
	// read it from the next PDU per DICOM PS3.7 §9.3.1.
	if req.CommandDataSetType != dicomcommand.DataSetNone {
		identifierDataSet, err := pdu.NextPDU()
		if err != nil {
			return nil, fmt.Errorf("CMoveReadRQ: failed to read identifier dataset: %w", err)
		}
		req.IdentifierDataSet = identifierDataSet
	}

	return req, nil
}

// validateCMoveRequest validates that a CMoveRequest has all required fields
// per DICOM PS3.7 §C.4.2.1.1.
func validateCMoveRequest(req *CMoveRequest) error {
	if req.AffectedSOPClassUID == "" {
		return errors.New("CMoveReadRQ: Affected SOP Class UID is required")
	}
	if req.MessageID == 0 {
		return errors.New("CMoveReadRQ: Message ID is required")
	}
	if req.MoveDestination == "" {
		return errors.New("CMoveReadRQ: Move Destination is required")
	}
	// Per DICOM PS3.7 §9.3.1, 0x0101 means no dataset; any other value means dataset present.
	// Accept all conformant implementations (e.g. DCMTK uses 0x0001 for DataSetPresent).
	return nil
}

// GetMoveLevel returns the Query/Retrieve level from the identifier dataset.
// Returns empty string if not present or if identifier dataset is nil.
func (r *CMoveRequest) GetMoveLevel() string {
	if r.IdentifierDataSet == nil {
		return ""
	}
	return r.IdentifierDataSet.GetString(tags.QueryRetrieveLevel)
}

// CMoveWriteRQ writes a C-MOVE-RQ to the PDU service with explicit priority.
// Per DICOM PS3.7 §C.4.2.1, the request includes:
//   - Affected SOP Class UID from the negotiated presentation context
//   - Message ID (generated automatically)
//   - Move Destination (the destination AE title)
//   - Priority (explicit parameter)
//   - Command Data Set Type (DataSetPresent to indicate identifier follows)
//
// If dataObj contains tags, it is sent as the identifier dataset; otherwise
// CommandDataSetType is set to DataSetNone (no dataset).
func CMoveWriteRQ(pdu network.PDUService, dataObj media.DICOMObject, destinationAETitle string, pri uint16) error {
	if destinationAETitle == "" {
		return errors.New("CMoveWriteRQ: destination AE title cannot be empty")
	}

	commandObj := media.NewEmptyDCMObj()
	commandObj.SetTransferSyntax(dataObj.GetTransferSyntax())

	destinationAETitleLength := paddedLen(destinationAETitle)

	sopClassUID := sopClassUID(pdu)
	if sopClassUID == "" {
		return errors.New("CMoveWriteRQ: unable to determine SOP Class UID from presentation context")
	}

	sopClassUIDLength := paddedLen(sopClassUID)

	// Determine if identifier dataset is present
	commandDataSetType := dicomcommand.DataSetNone
	if dataObj != nil && dataObj.TagCount() > 0 {
		commandDataSetType = dicomcommand.DataSetPresent
	}

	// Calculate command length per PS3.7 Table C.4-2:
	// - Affected SOP Class UID (8 + length)
	// - Command Field (8 + 2)
	// - Message ID (8 + 2)
	// - Move Destination (8 + length)
	// - Priority (8 + 2)
	// - Command Data Set Type (8 + 2)
	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + destinationAETitleLength + 8 + 2 + 8 + 2)

	commandObj.Write(tags.CommandGroupLength, commandLength)
	commandObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	commandObj.Write(tags.CommandField, dicomcommand.CMoveRequest)
	commandObj.Write(tags.MessageID, network.Uniq16odd())
	commandObj.Write(tags.MoveDestination, destinationAETitle)
	commandObj.Write(tags.Priority, pri)
	commandObj.Write(tags.CommandDataSetType, commandDataSetType)

	if err := pdu.Write(commandObj, network.PDVCommand); err != nil {
		return err
	}

	// If identifier dataset is present, send it
	if commandDataSetType == dicomcommand.DataSetPresent && dataObj != nil {
		return pdu.Write(dataObj, network.PDVDataset)
	}
	return nil
}

// CMoveReadRSP reads a C-MOVE-RSP from the PDU service and returns:
//   - The received data object (may be nil if no dataset)
//   - The status code
//   - An error if the response is malformed
//
// The pending parameter is updated with the number of remaining suboperations
// when a Pending status (0xFF00) is returned. For final responses, pending is set to -1.
//
// Per DICOM PS3.7 §C.4.2.1.9, all responses must include the four sub-operation
// count fields (Remaining, Completed, Failed, Warning).
func CMoveReadRSP(pdu network.PDUService, pending *int) (media.DICOMObject, uint16, error) {
	responseCommandObj, err := pdu.NextPDU()
	if err != nil {
		return nil, dicomstatus.FailureProcessingFailure, fmt.Errorf("CMoveReadRSP: failed to read response PDU: %w", err)
	}

	if got := responseCommandObj.GetUint16(tags.CommandField); got != dicomcommand.CMoveResponse {
		return nil, dicomstatus.FailureProcessingFailure,
			fmt.Errorf("CMoveReadRSP: expected %s (0x%04X), got %s (0x%04X)",
				dicomcommand.Description(dicomcommand.CMoveResponse), dicomcommand.CMoveResponse,
				dicomcommand.Description(got), got)
	}

	status := responseCommandObj.GetUint16(tags.Status)
	if err := validateQROperationStatus(status, "CMoveReadRSP"); err != nil {
		return nil, dicomstatus.FailureProcessingFailure, err
	}
	commandDataSetType := responseCommandObj.GetUint16(tags.CommandDataSetType)
	// Per DICOM PS3.7 §9.3.1, 0x0101 means no dataset; any other value means dataset present.
	hasDataSet := commandDataSetType != dicomcommand.DataSetNone

	// Extract sub-operation counts per PS3.7 §C.4.2.1.9
	remaining := responseCommandObj.GetUint16(tags.NumberOfRemainingSuboperations)
	completed := responseCommandObj.GetUint16(tags.NumberOfCompletedSuboperations)
	failed := responseCommandObj.GetUint16(tags.NumberOfFailedSuboperations)
	warnings := responseCommandObj.GetUint16(tags.NumberOfWarningSuboperations)

	// Update pending count
	if err := validateSuboperationCounters(status, remaining, completed, failed, warnings, false); err != nil {
		return nil, dicomstatus.FailureProcessingFailure, fmt.Errorf("CMoveReadRSP: invalid sub-operation counters: %w", err)
	}

	if status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
		if pending != nil {
			*pending = int(remaining)
		}
	} else {
		if pending != nil {
			*pending = -1
		}
	}

	// If CommandDataSetType indicates dataset present, read it
	if hasDataSet {
		responseDataObj, err := pdu.NextPDU()
		if err != nil {
			return nil, status, fmt.Errorf("CMoveReadRSP: failed to read response dataset: %w", err)
		}
		return responseDataObj, status, nil
	}

	// Return nil for both completed and failed status codes
	if status != dicomstatus.Pending && status != dicomstatus.PendingWithWarnings {
		if remaining != 0 || completed == 0 && failed == 0 && warnings == 0 {
			// Final response - log sub-operation summary
		}
	}

	return nil, status, nil
}

// CMoveResponseStats encapsulates the sub-operation counts in a C-MOVE-RSP.
type CMoveResponseStats struct {
	Remaining int
	Completed int
	Failed    int
	Warnings  int
}

// GetCMoveResponseStats extracts all sub-operation counts from a C-MOVE-RSP command.
// This helper function is useful when you need detailed progress tracking.
func GetCMoveResponseStats(responseCommandObj media.DICOMObject) CMoveResponseStats {
	return CMoveResponseStats{
		Remaining: int(responseCommandObj.GetUint16(tags.NumberOfRemainingSuboperations)),
		Completed: int(responseCommandObj.GetUint16(tags.NumberOfCompletedSuboperations)),
		Failed:    int(responseCommandObj.GetUint16(tags.NumberOfFailedSuboperations)),
		Warnings:  int(responseCommandObj.GetUint16(tags.NumberOfWarningSuboperations)),
	}
}

// CMoveWriteRSP writes a C-MOVE-RSP to the PDU service per DICOM PS3.7 §C.4.2.1.9.
// The response must include all four sub-operation count fields (Remaining, Completed, Failed, Warning).
//
// Parameters:
//   - pdu: PDU service to write to
//   - requestCommandObj: The original C-MOVE-RQ command object (for reference fields)
//   - status: The response status code (e.g., dicomstatus.Success, Pending, FailureXXX)
//   - remaining: Number of remaining suboperations
//   - completed: Number of completed suboperations
//   - failed: Number of failed suboperations
//   - warnings: Number of warnings
//
// Per PS3.7 §C.4.2.1.9, all four counts must be present in every response.
func CMoveWriteRSP(pdu network.PDUService, requestCommandObj media.DICOMObject, status uint16, remaining, completed, failed, warnings uint16) error {
	if requestCommandObj == nil {
		return errors.New("CMoveWriteRSP: requestCommandObj cannot be nil")
	}

	responseCommandObj := media.NewEmptyDCMObj()
	responseCommandObj.SetTransferSyntax(requestCommandObj.GetTransferSyntax())

	sopClassUID := requestCommandObj.GetString(tags.AffectedSOPClassUID)
	if sopClassUID == "" {
		return errors.New("CMoveWriteRSP: Affected SOP Class UID is required")
	}

	sopClassUIDLength := paddedLen(sopClassUID)

	if err := validateSuboperationCounters(status, remaining, completed, failed, warnings, true); err != nil {
		return fmt.Errorf("CMoveWriteRSP: %w", err)
	}
	if err := validateQROperationStatus(status, "CMoveWriteRSP"); err != nil {
		return err
	}

	// commandLength includes all fields after CommandGroupLength per PS3.7 Table C.4-3:
	// - Affected SOP Class UID (8 + length)
	// - Command Field (8 + 2)
	// - Message ID Being Responded To (8 + 2)
	// - Command Data Set Type (8 + 2)
	// - Status (8 + 2)
	// - Number of Remaining Suboperations (8 + 2)
	// - Number of Completed Suboperations (8 + 2)
	// - Number of Failed Suboperations (8 + 2)
	// - Number of Warning Suboperations (8 + 2)
	commandLength := uint32(8 + sopClassUIDLength + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 8 + 2)

	responseCommandObj.Write(tags.CommandGroupLength, commandLength)
	responseCommandObj.Write(tags.AffectedSOPClassUID, sopClassUID)
	responseCommandObj.Write(tags.CommandField, dicomcommand.CMoveResponse)
	messageID := requestCommandObj.GetUint16(tags.MessageID)
	responseCommandObj.Write(tags.MessageIDBeingRespondedTo, messageID)
	responseCommandObj.Write(tags.CommandDataSetType, dicomcommand.DataSetNone) // No dataset in response
	responseCommandObj.Write(tags.Status, status)
	responseCommandObj.Write(tags.NumberOfRemainingSuboperations, remaining)
	responseCommandObj.Write(tags.NumberOfCompletedSuboperations, completed)
	responseCommandObj.Write(tags.NumberOfFailedSuboperations, failed)
	responseCommandObj.Write(tags.NumberOfWarningSuboperations, warnings)

	return pdu.Write(responseCommandObj, network.PDVCommand)
}
