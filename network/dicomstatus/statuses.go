package dicomstatus

// Success - 0x0000
const Success uint16 = 0x0000

// Cancel - 0xFE00
// Returned for canceled operations (for example C-FIND/C-MOVE/C-GET).
const Cancel uint16 = 0xFE00

// Pending - 0xFF00
const Pending uint16 = 0xFF00

// PendingWithWarnings - 0xFF01
// For C-FIND/C-GET/C-MOVE this indicates pending with warning semantics.
const PendingWithWarnings uint16 = 0xFF01

// FailureProcessingFailure - 0x0110
const FailureProcessingFailure uint16 = 0x0110

// FailureSOPClassNotSupported - 0x0122
const FailureSOPClassNotSupported uint16 = 0x0122

// FailureOutOfResources - 0xA700
// Generic out-of-resources status (context specific by service).
const FailureOutOfResources uint16 = 0xA700

// FailureOutOfResourcesUnableToCalculateMatches - 0xA701
// Query/Retrieve specific (e.g. C-MOVE/C-GET).
const FailureOutOfResourcesUnableToCalculateMatches uint16 = 0xA701

// FailureOutOfResourcesUnableToPerformSubOperations - 0xA702
// Query/Retrieve specific (e.g. C-MOVE/C-GET).
const FailureOutOfResourcesUnableToPerformSubOperations uint16 = 0xA702

// RefusedMoveDestinationUnknown - 0xA801
// C-MOVE specific: requested Move Destination AE is unknown.
const RefusedMoveDestinationUnknown uint16 = 0xA801

// FailureIdentifierDoesNotMatchSOPClass - 0xA900
// Used by C-FIND/C-MOVE/C-GET/C-STORE depending on context.
const FailureIdentifierDoesNotMatchSOPClass uint16 = 0xA900

// FailureUnableToProcess - 0xC000
// Represents the service-specific "unable to process" failure range (0xCxxx).
const FailureUnableToProcess uint16 = 0xC000

// WarningCoercionOfDataElements - 0xB000
// C-STORE warning: one or more data elements were coerced.
const WarningCoercionOfDataElements uint16 = 0xB000

// WarningElementsDiscarded - 0xB006
// C-STORE warning: one or more elements were discarded.
const WarningElementsDiscarded uint16 = 0xB006

// WarningDataSetDoesNotMatchSOPClass - 0xB007
// C-STORE warning: data set does not match SOP class.
const WarningDataSetDoesNotMatchSOPClass uint16 = 0xB007

// WarningSubOperationsCompleteOneOrMoreFailures - 0xB000
// C-MOVE/C-GET warning: completed with one or more failed/warning sub-ops.
const WarningSubOperationsCompleteOneOrMoreFailures uint16 = 0xB000

// FailureServiceSpecificMin - 0x0100
// Lower bound of the service-specific failure status code range used across multiple
// DICOM services (e.g. C-STORE PS3.7 Table C.4-1, Query/Retrieve).
const FailureServiceSpecificMin uint16 = 0x0100

// FailureServiceSpecificMax - 0x01FF
// Upper bound of the service-specific failure status code range.
const FailureServiceSpecificMax uint16 = 0x01FF

// HighNibbleFailureRefused is the high nibble (bits 15-12) value identifying
// refused/out-of-resources failure status codes (0xAxxx per PS3.7).
const HighNibbleFailureRefused uint16 = 0xA

// HighNibbleWarning is the high nibble value identifying warning status codes
// (0xBxxx per PS3.7).
const HighNibbleWarning uint16 = 0xB

// HighNibbleFailureCannotUnderstand is the high nibble value identifying
// "cannot understand" / unable-to-process failure status codes (0xCxxx per PS3.7).
const HighNibbleFailureCannotUnderstand uint16 = 0xC
