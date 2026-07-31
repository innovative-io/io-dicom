package network

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// pdataPDU builds one P-DATA-TF PDU carrying a single PDV of payload bytes.
// last controls the PDV's last-fragment bit; a peer that never sets it makes
// NextPDU keep accumulating.
func pdataPDU(pcid byte, payload []byte, last bool) []byte {
	msgHeader := byte(0x00) // data, not last
	if last {
		msgHeader = PDVLastFragment
	}
	pdvLen := uint32(len(payload)) + 2 // PCID + msgHeader
	pduLen := pdvLen + 4               // + the PDV length field

	out := make([]byte, 0, int(pduLen)+6)
	out = append(out, byte(0x04), 0x00) // P-DATA-TF, reserved
	var b4 [4]byte
	binary.BigEndian.PutUint32(b4[:], pduLen)
	out = append(out, b4[:]...)
	binary.BigEndian.PutUint32(b4[:], pdvLen)
	out = append(out, b4[:]...)
	out = append(out, pcid, msgHeader)
	return append(out, payload...)
}

// TestAccumulatedMessageIsBounded pins the ceiling on one DIMSE message.
//
// maxIncomingPDULength caps an individual PDU at 16 MiB, but nothing bounded the
// sum across fragments: NextPDU loops while the last-fragment bit is clear,
// appending every PDV into Pdata.Buffer, so a peer that simply never sets that
// bit grows the buffer without limit.
func TestAccumulatedMessageIsBounded(t *testing.T) {
	const (
		limit     = 64 << 10 // small ceiling so the test stays fast
		fragment  = 4 << 10
		fragments = 64 // 256 KiB total, four times the ceiling
	)

	var wire bytes.Buffer
	payload := make([]byte, fragment)
	for i := 0; i < fragments; i++ {
		// Never set the last-fragment bit: the accumulation must stop anyway.
		wire.Write(pdataPDU(1, payload, false))
	}

	pdu := NewPDUService(WithMaxMessageSize(limit)).(*pduService)
	r := bufio.NewReader(bytes.NewReader(wire.Bytes()))
	w := bufio.NewWriter(&bytes.Buffer{})
	pdu.SetConn(bufio.NewReadWriter(r, w))
	pdu.negotiated = true

	obj, err := pdu.NextPDU()
	if err == nil {
		t.Fatalf("a peer that never ends the message was allowed to accumulate "+
			"%d bytes (obj=%v)", pdu.Pdata.Buffer.GetSize(), obj)
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected a size-limit error, got %v", err)
	}
	// The buffer must not have grown far past the ceiling before erroring: one
	// fragment of overshoot is expected, not the whole stream.
	if got := pdu.Pdata.Buffer.GetSize(); got > limit+fragment {
		t.Fatalf("accumulated %d bytes against a %d-byte ceiling", got, limit)
	}
}

// TestMessageUnderLimitStillAssembles guards the ceiling from breaking ordinary
// multi-fragment messages, which is the normal case for any real instance.
func TestMessageUnderLimitStillAssembles(t *testing.T) {
	const fragment = 1 << 10

	var wire bytes.Buffer
	payload := make([]byte, fragment)
	for i := range payload {
		payload[i] = byte(i)
	}
	// Three continuation fragments then a terminating one.
	for i := 0; i < 3; i++ {
		wire.Write(pdataPDU(1, payload, false))
	}
	wire.Write(pdataPDU(1, payload, true))

	pdu := NewPDUService(WithMaxMessageSize(1 << 20)).(*pduService)
	r := bufio.NewReader(bytes.NewReader(wire.Bytes()))
	w := bufio.NewWriter(&bytes.Buffer{})
	pdu.SetConn(bufio.NewReadWriter(r, w))
	pdu.negotiated = true

	// The assembled payload is not a valid DIMSE command, so NextPDU reports a
	// parse failure — but it must not be a size-limit rejection, and all four
	// fragments must have been accumulated first.
	_, err := pdu.NextPDU()
	if err != nil && strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("a 4 KiB message was rejected against a 1 MiB ceiling: %v", err)
	}
	if got := pdu.Pdata.Buffer.GetSize(); got != 4*fragment {
		t.Fatalf("accumulated %d bytes, want %d — fragments were dropped", got, 4*fragment)
	}
}

// TestDefaultMessageLimitApplies confirms a service built without the option
// still carries a ceiling.
func TestDefaultMessageLimitApplies(t *testing.T) {
	pdu := NewPDUService().(*pduService)
	if pdu.maxMessageBytes != defaultMaxMessageBytes {
		t.Fatalf("default ceiling is %d, want %d", pdu.maxMessageBytes, defaultMaxMessageBytes)
	}
	// A non-positive override restores the default rather than disabling it.
	pdu2 := NewPDUService(WithMaxMessageSize(0)).(*pduService)
	if pdu2.maxMessageBytes != defaultMaxMessageBytes {
		t.Fatalf("WithMaxMessageSize(0) left %d, want the default %d",
			pdu2.maxMessageBytes, defaultMaxMessageBytes)
	}
}
