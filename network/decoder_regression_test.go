package network

import (
	"testing"

	"github.com/innovative-io/io-dicom/media"
)

// TestDescriptionHelpers_UnknownUIDNoNilPanic guards the nil dereference that
// FuzzAssociationACReadDynamic found: sopclass.GetSOPClassFromUID and
// transfersyntax.GetTransferSyntaxFromUID return nil for unknown UIDs, and the
// A-ASSOCIATE log lines dereferenced .Description on the result.
func TestDescriptionHelpers_UnknownUIDNoNilPanic(t *testing.T) {
	if got := sopClassDescription("9.9.9.not.a.real.uid"); got != "" {
		t.Errorf("sopClassDescription(unknown) = %q, want empty string", got)
	}
	if got := transferSyntaxDescription("9.9.9.not.a.real.uid"); got != "" {
		t.Errorf("transferSyntaxDescription(unknown) = %q, want empty string", got)
	}
}

// TestPresentationContextReadDynamic_TruncatedRejected guards the uint16
// underflow that spun presentationContext.ReadDynamic forever on a malformed
// context (found by FuzzAssociationRQRead). The transfer-syntax loop counter
// could wrap past zero back to ~65535 on a truncated item.
//
// The input declares Length=32 but supplies only an abstract-syntax item of
// odd UID length (so the post-abstract "remaining" is 19, not a multiple of 4)
// and no transfer syntax bytes. ReadDynamic must return an error promptly; if
// the underflow regressed, this test would hang and the package -timeout would
// fail the run.
func TestPresentationContextReadDynamic_TruncatedRejected(t *testing.T) {
	data := []byte{
		0x00,       // Reserved1
		0x00, 0x20, // Length = 32
		0x01,             // PresentationContextID
		0x00, 0x00, 0x00, // Reserved2/3/4
		// Abstract syntax item: type, reserved, length=5, 5 UID bytes.
		0x30, 0x00, 0x00, 0x05, '1', '.', '2', '.', '3',
		// (no transfer syntax bytes — buffer is now exhausted)
	}
	pc := NewPresentationContext().(*presentationContext)
	buf := media.NewDICOMBufferFromBytes(data)
	buf.SetBigEndian(true)
	if err := pc.ReadDynamic(buf); err == nil {
		t.Fatal("expected an error decoding a truncated presentation context, got nil")
	}
}

// TestAssociationRQRead_TruncatedRejected feeds a structurally valid header but
// a truncated item body to the full A-ASSOCIATE-RQ decoder; it must return an
// error rather than panic or hang.
func TestAssociationRQRead_TruncatedRejected(t *testing.T) {
	data := make([]byte, 4+16+16+32) // protocol version + reserved + AE titles + reserved
	data = append(data, 0x20)        // a presentation-context item type byte, then nothing
	aarq := newAssociationRequest()
	buf := media.NewDICOMBufferFromBytes(data)
	buf.SetBigEndian(true)
	if err := aarq.Read(buf); err == nil {
		t.Fatal("expected an error decoding a truncated A-ASSOCIATE-RQ, got nil")
	}
}
