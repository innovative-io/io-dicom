package dicomstatus

import "fmt"

var statusDescriptions = map[uint16]string{
	Success:                                           "Success",
	Cancel:                                            "Cancel",
	Pending:                                           "Pending",
	PendingWithWarnings:                               "Pending: warnings",
	FailureProcessingFailure:                          "Failure: processing failure",
	FailureSOPClassNotSupported:                       "Failure: SOP class not supported",
	FailureOutOfResources:                             "Failure: out of resources",
	FailureOutOfResourcesUnableToCalculateMatches:     "Failure: out of resources, unable to calculate matches",
	FailureOutOfResourcesUnableToPerformSubOperations: "Failure: out of resources, unable to perform sub-operations",
	RefusedMoveDestinationUnknown:                     "Refused: move destination unknown",
	FailureIdentifierDoesNotMatchSOPClass:             "Failure: identifier does not match SOP class",
	WarningCoercionOfDataElements:                     "Warning: coercion of data elements",
	WarningElementsDiscarded:                          "Warning: elements discarded",
	WarningDataSetDoesNotMatchSOPClass:                "Warning: dataset does not match SOP class",
}

// Description returns a human-readable description for a DICOM status code.
func Description(status uint16) string {
	if desc, ok := statusDescriptions[status]; ok {
		return desc
	}

	switch status >> 12 {
	case 0xA:
		return "Failure"
	case 0xB:
		return "Warning"
	case 0xC:
		return "Failure: unable to process"
	}

	return fmt.Sprintf("Unknown status 0x%04X", status)
}
