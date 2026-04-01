package dimse

import (
	"fmt"

	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

func isValidQRStatus(status uint16) bool {
	if status == dicomstatus.Success ||
		status == dicomstatus.Pending ||
		status == dicomstatus.PendingWithWarnings ||
		status == dicomstatus.Cancel {
		return true
	}

	if status >= dicomstatus.FailureServiceSpecificMin && status <= dicomstatus.FailureServiceSpecificMax {
		return true
	}

	h := status >> 12
	return h == dicomstatus.HighNibbleFailureRefused || h == dicomstatus.HighNibbleWarning || h == dicomstatus.HighNibbleFailureCannotUnderstand
}

func validateQRStatus(status uint16, op string) error {
	if !isValidQRStatus(status) {
		return fmt.Errorf("%s: invalid Query/Retrieve status code %s (0x%04X)", op, dicomstatus.Description(status), status)
	}
	return nil
}

func validateQROperationStatus(status uint16, op string) error {
	if err := validateQRStatus(status, op); err != nil {
		return err
	}

	// C-FIND does not define Bxxx warning final statuses.
	if op == "CFindReadRSP" || op == "CFindWriteRSP" {
		if status>>12 == 0xB {
			return fmt.Errorf("%s: status code %s (0x%04X) is not valid for C-FIND", op, dicomstatus.Description(status), status)
		}
	}

	// C-GET/C-MOVE warning final status is B000 only.
	if op == "CGetReadRSP" || op == "CGetWriteRSP" || op == "CMoveReadRSP" || op == "CMoveWriteRSP" {
		if status>>12 == 0xB && status != dicomstatus.WarningSubOperationsCompleteOneOrMoreFailures {
			return fmt.Errorf("%s: status code %s (0x%04X) is not valid for %s", op, dicomstatus.Description(status), status, op[:4])
		}
	}

	// 0xA801 (Move Destination unknown) is C-MOVE specific.
	if (op == "CFindReadRSP" || op == "CFindWriteRSP" || op == "CGetReadRSP" || op == "CGetWriteRSP") &&
		status == dicomstatus.RefusedMoveDestinationUnknown {
		return fmt.Errorf("%s: status code %s (0x%04X) is C-MOVE specific", op, dicomstatus.Description(status), status)
	}

	return nil
}
