package network

import (
	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

// sopClassDescription returns the human-readable description for a SOP class
// UID, or "" when the UID is unknown. GetSOPClassFromUID returns nil for
// unrecognized UIDs, so this guard prevents a nil dereference when logging the
// description of an attacker-supplied or malformed application/abstract syntax.
func sopClassDescription(uid string) string {
	if sc := sopclass.GetSOPClassFromUID(uid); sc != nil {
		return sc.Description
	}
	return ""
}

// transferSyntaxDescription returns the description for a transfer syntax UID,
// or "" when the UID is unknown (GetTransferSyntaxFromUID returns nil).
func transferSyntaxDescription(uid string) string {
	if ts := transfersyntax.GetTransferSyntaxFromUID(uid); ts != nil {
		return ts.Description
	}
	return ""
}
