package network

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/innovative-io/io-dicom/media"
)

// pduWithReader builds a pduService whose read side is fed from the given bytes.
func pduWithReader(data []byte) *pduService {
	pdu := NewPDUService().(*pduService)
	r := bufio.NewReader(bytes.NewReader(data))
	w := bufio.NewWriter(&bytes.Buffer{})
	pdu.SetConn(bufio.NewReadWriter(r, w))
	return pdu
}

// pduHeader encodes a 10-byte PDU header with the given item type and length.
func pduHeader(itemType byte, pduLength uint32) []byte {
	return []byte{
		itemType, 0x00,
		byte(pduLength >> 24), byte(pduLength >> 16), byte(pduLength >> 8), byte(pduLength),
		0, 0, 0, 0,
	}
}

func TestReadIncomingPDURejectsOversizedLength(t *testing.T) {
	// A hostile peer advertises a PDU length near 4 GiB. readIncomingPDU must
	// reject it before allocating, rather than attempting a multi-GiB make().
	header := pduHeader(0x04, maxIncomingPDULength+1)
	pdu := pduWithReader(header)

	_, _, err := pdu.readIncomingPDU()
	if err == nil {
		t.Fatal("expected error for oversized PDU length, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("expected 'exceeds maximum' error, got %v", err)
	}
}

func TestReadIncomingPDURejectsTruncatedLength(t *testing.T) {
	header := pduHeader(0x04, 3) // below the 4-byte minimum
	pdu := pduWithReader(header)

	_, _, err := pdu.readIncomingPDU()
	if err == nil {
		t.Fatal("expected error for PDU length below minimum, got nil")
	}
	if !strings.Contains(err.Error(), "minimum") {
		t.Fatalf("expected 'minimum' error, got %v", err)
	}
}

func TestReadIncomingPDUAcceptsLengthAtCeiling(t *testing.T) {
	// A PDU whose declared length is exactly the ceiling is allowed; the read
	// then fails on EOF because no payload follows, proving the cap itself did
	// not reject it.
	header := pduHeader(0x04, maxIncomingPDULength)
	pdu := pduWithReader(header)

	_, _, err := pdu.readIncomingPDU()
	if err != nil && strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("length at ceiling must not be rejected by the cap, got %v", err)
	}
}

func TestReadDynamicRejectsUnderflowingPDVLength(t *testing.T) {
	// Craft a P-DATA-TF body whose PDV length is larger than the bytes the PDU
	// claims to carry. Without the guard, the uint32 subtractions in ReadDynamic
	// underflow and spin the loop on a huge wrapped count.
	var body bytes.Buffer
	body.WriteByte(0x00) // Reserved1
	// PDU data length = 10 bytes follow for the PDV section.
	body.Write([]byte{0x00, 0x00, 0x00, 0x0A})
	// pdv.Length = 0xFFFFFFFF (claims ~4 GiB, far beyond the 10 declared).
	body.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	body.WriteByte(0x01) // PresentationContextID
	body.WriteByte(0x02) // MsgHeader (not last fragment)

	buf := media.NewDICOMBufferFromBytes(body.Bytes())
	buf.SetBigEndian(true)

	pd := &PresentationDataTransfer{Buffer: media.NewDICOMBuffer()}
	err := pd.ReadDynamic(buf)
	if err == nil {
		t.Fatal("expected error for malformed PDV length, got nil")
	}
	if !strings.Contains(err.Error(), "malformed PDV length") {
		t.Fatalf("expected 'malformed PDV length' error, got %v", err)
	}
}

func TestReadDynamicRejectsTooSmallPDVLength(t *testing.T) {
	// pdv.Length must be at least 2 (the PCID + MsgHeader bytes it counts).
	var body bytes.Buffer
	body.WriteByte(0x00)                       // Reserved1
	body.Write([]byte{0x00, 0x00, 0x00, 0x06}) // PDU data length
	body.Write([]byte{0x00, 0x00, 0x00, 0x01}) // pdv.Length = 1 (illegal)
	body.WriteByte(0x01)                       // PCID
	body.WriteByte(0x02)                       // MsgHeader

	buf := media.NewDICOMBufferFromBytes(body.Bytes())
	buf.SetBigEndian(true)

	pd := &PresentationDataTransfer{Buffer: media.NewDICOMBuffer()}
	if err := pd.ReadDynamic(buf); err == nil {
		t.Fatal("expected error for PDV length below 2, got nil")
	}
}
