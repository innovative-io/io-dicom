package network

import (
	"testing"

	"github.com/innovative-io/io-dicom/media"
)

// FuzzAssociationRQRead drives the A-ASSOCIATE-RQ decoder (the SCP-side parse of
// an incoming association request) with arbitrary bytes. Decoding attacker-
// controlled PDU items must never panic or hang — an error return is fine.
// Run with: go test -run=^$ -fuzz=FuzzAssociationRQRead ./network
func FuzzAssociationRQRead(f *testing.F) {
	f.Add([]byte{})
	// Protocol version + reserved + 16+16 AE + 32 reserved, then a truncated item.
	f.Add([]byte{0x00, 0x01, 0x00, 0x00})
	f.Add(make([]byte, 4+16+16+32+1))

	f.Fuzz(func(t *testing.T, data []byte) {
		aarq := newAssociationRequest()
		buf := media.NewDICOMBufferFromBytes(data)
		buf.SetBigEndian(true)
		_ = aarq.Read(buf)
	})
}

// FuzzAssociationACReadDynamic drives the A-ASSOCIATE-AC decoder (the SCU-side
// parse of the peer's acceptance) with arbitrary bytes.
// Run with: go test -run=^$ -fuzz=FuzzAssociationACReadDynamic ./network
func FuzzAssociationACReadDynamic(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x10})
	f.Add(make([]byte, 1+4+2+2+16+16+32+1))

	f.Fuzz(func(t *testing.T, data []byte) {
		aaac := newAssociationAccept()
		buf := media.NewDICOMBufferFromBytes(data)
		buf.SetBigEndian(true)
		_ = aaac.ReadDynamic(buf)
	})
}
