package dimse

import (
	"fmt"

	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

// validateSuboperationCounters checks sub-operation counter invariants.
// When strict is true, Pending responses must have remaining > 0 (enforced
// for our own outgoing responses per DICOM PS3.7).
// When strict is false, Pending+remaining=0 is accepted (some implementations,
// e.g. DCMTK dcmqrscp, send this before the final response).
func validateSuboperationCounters(status uint16, remaining, completed, failed, warnings uint16, strict bool) error {
	if strict && (status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings) && remaining == 0 {
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
