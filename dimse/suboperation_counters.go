package dimse

import (
	"fmt"

	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

func validateSuboperationCounters(status uint16, remaining, completed, failed, warnings int) error {
	if remaining < 0 || completed < 0 || failed < 0 || warnings < 0 {
		return fmt.Errorf("negative sub-operation counters are invalid: remaining=%d completed=%d failed=%d warnings=%d", remaining, completed, failed, warnings)
	}

	if (status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings) && remaining <= 0 {
		return fmt.Errorf("pending response requires remaining sub-operations > 0: remaining=%d", remaining)
	}

	if status != dicomstatus.Pending && status != dicomstatus.PendingWithWarnings && remaining != 0 {
		return fmt.Errorf("final response requires remaining sub-operations == 0: remaining=%d", remaining)
	}

	if status == dicomstatus.Success && (failed != 0 || warnings != 0) {
		return fmt.Errorf("success response requires failed=0 and warnings=0: failed=%d warnings=%d", failed, warnings)
	}

	return nil
}
