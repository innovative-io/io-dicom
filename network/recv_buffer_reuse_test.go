package network

import (
	"bufio"
	"bytes"
	"testing"
)

// TestRecvScratchReuseAccumulatesCorrectly is the safety contract for reusing
// one raw buffer across inbound P-DATA PDUs. Each fragment carries a distinct
// byte pattern; if reading fragment i+1 into the reused scratch clobbered
// fragment i before it was copied into Pdata.Buffer, the accumulated bytes would
// be wrong or misordered.
func TestRecvScratchReuseAccumulatesCorrectly(t *testing.T) {
	const (
		pcid      = 1
		fragments = 8
		fragLen   = 300 // > header, spans the reused buffer several times
	)

	var wire bytes.Buffer
	var want []byte
	for i := 0; i < fragments; i++ {
		payload := make([]byte, fragLen)
		for j := range payload {
			payload[j] = byte(i*7 + j) // unique per fragment
		}
		want = append(want, payload...)
		wire.Write(pdataPDU(pcid, payload, i == fragments-1))
	}

	pdu := NewPDUService().(*pduService)
	r := bufio.NewReader(bytes.NewReader(wire.Bytes()))
	w := bufio.NewWriter(&bytes.Buffer{})
	pdu.SetConn(bufio.NewReadWriter(r, w))
	pdu.negotiated = true

	// The accumulated payload is not a valid DIMSE command, so NextPDU returns a
	// parse error — but Pdata.Buffer must hold the exact concatenation first.
	_, _ = pdu.NextPDU()

	got := pdu.Pdata.Buffer.GetData()
	if !bytes.Equal(got, want) {
		firstDiff := 0
		for firstDiff < len(got) && firstDiff < len(want) && got[firstDiff] == want[firstDiff] {
			firstDiff++
		}
		t.Fatalf("accumulated %d bytes across %d reused-buffer reads, want %d; first "+
			"difference at %d — buffer reuse corrupted an earlier fragment",
			len(got), fragments, len(want), firstDiff)
	}

	// The reused scratch must actually have been used for these (sub-cap) PDUs.
	if pdu.recvScratch == nil {
		t.Fatal("recvScratch was never used for P-DATA PDUs")
	}
}

// TestRecvScratchNotReusedForLargePDU pins the reuse cap: a P-DATA PDU larger
// than recvScratchReuseMax must get its own buffer, so a single outsized PDU
// cannot pin a large scratch for the association's lifetime.
func TestRecvScratchNotReusedForLargePDU(t *testing.T) {
	// One PDV whose total PDU size exceeds the reuse cap but stays under the
	// hard 16 MiB per-PDU ceiling.
	payload := make([]byte, recvScratchReuseMax+1024)
	wire := pdataPDU(1, payload, true)

	pdu := NewPDUService(WithMaxMessageSize(len(payload) + 4096)).(*pduService)
	r := bufio.NewReader(bytes.NewReader(wire))
	w := bufio.NewWriter(&bytes.Buffer{})
	pdu.SetConn(bufio.NewReadWriter(r, w))
	pdu.negotiated = true

	_, _ = pdu.NextPDU()

	if c := cap(pdu.recvScratch); c > recvScratchReuseMax {
		t.Fatalf("an outsized PDU inflated the reusable scratch to %d bytes "+
			"(cap is %d)", c, recvScratchReuseMax)
	}
}
