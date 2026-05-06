package dimse

import (
	"fmt"

	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

func validateSuboperationCounters(status uint16, remaining, completed, failed, warnings uint16) error {
	if (status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings) && remaining == 0 {
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
