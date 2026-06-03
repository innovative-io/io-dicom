package network

import (
	"bufio"
	"bytes"
	"io"
	"testing"

	"github.com/innovative-io/io-dicom/media"
)

// FuzzReadIncomingPDU drives the raw PDU reader with arbitrary bytes. The reader
// parses an attacker-controlled 32-bit length and allocates a receive buffer, so
// the contract is: never panic and never allocate beyond maxIncomingPDULength.
// Run with: go test -run=^$ -fuzz=FuzzReadIncomingPDU ./network
func FuzzReadIncomingPDU(f *testing.F) {
	f.Add([]byte{})
	// Well-formed-ish small PDU header (type, reserved, length=6) + 6 body bytes.
	f.Add([]byte{0x04, 0x00, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02})
	// Header advertising a near-4 GiB length — must be rejected before allocating.
	f.Add([]byte{0x01, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		pdu := NewPDUService().(*pduService)
		r := bufio.NewReader(bytes.NewReader(data))
		w := bufio.NewWriter(io.Discard)
		pdu.SetConn(bufio.NewReadWriter(r, w))
		// A returned error is fine; a panic or unbounded allocation is not.
		_, _, _ = pdu.readIncomingPDU()
	})
}

// FuzzPDataTFReadDynamic targets the P-DATA-TF / PDV decode path, where malformed
// PDV length fields previously underflowed uint32 arithmetic. The contract is:
// never panic and never loop unboundedly on arbitrary input.
// Run with: go test -run=^$ -fuzz=FuzzPDataTFReadDynamic ./network
func FuzzPDataTFReadDynamic(f *testing.F) {
	f.Add([]byte{})
	// Reserved1 + Length(10 BE) + pdv.Length(0xFFFFFFFF) + PCID + MsgHeader.
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x0A, 0xFF, 0xFF, 0xFF, 0xFF, 0x01, 0x02})
	// A small, structurally plausible PDV.
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x04, 0x01, 0x03, 0xAA, 0xBB})

	f.Fuzz(func(t *testing.T, data []byte) {
		pd := &PresentationDataTransfer{Buffer: media.NewDICOMBuffer()}
		buf := media.NewDICOMBufferFromBytes(data)
		buf.SetBigEndian(true)
		_ = pd.ReadDynamic(buf)
	})
}
