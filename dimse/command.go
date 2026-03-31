package dimse

import "github.com/innovative-io/io-dicom/network"

// sopClassUID returns the SOP class UID for the active negotiated presentation
// context. It falls back to the first accepted context if the active PCID is
// not found, and to the empty string when no contexts are accepted.
func sopClassUID(pdu network.PDUService) string {
	pcid := pdu.GetPresentationContextID()

	for _, pc := range pdu.GetAAssociationRQ().GetPresContexts() {
		if pc.GetPresentationContextID() == pcid {
			return pc.GetAbstractSyntax().GetUID()
		}
	}

	// Fallback: return the first proposed abstract syntax (covers the common
	// single-context SCU case where PCID may not yet be set).
	if pcs := pdu.GetAAssociationRQ().GetPresContexts(); len(pcs) > 0 {
		return pcs[0].GetAbstractSyntax().GetUID()
	}

	return ""
}
