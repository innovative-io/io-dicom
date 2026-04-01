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
