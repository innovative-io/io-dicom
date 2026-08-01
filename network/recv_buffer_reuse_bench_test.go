package network

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"testing"
)

// BenchmarkReceiveMultiPDU measures allocations for receiving one DIMSE message
// split across many P-DATA PDUs — the C-STORE receive shape. With per-PDU
// buffer reuse the raw-buffer allocations collapse from one-per-PDU to one.
func BenchmarkReceiveMultiPDU(b *testing.B) {
	const (
		pcid      = 1
		fragments = 256
		fragLen   = 16 << 10 // 16 KiB, a typical negotiated PDU payload
	)
	var wire bytes.Buffer
	payload := make([]byte, fragLen)
	for i := 0; i < fragments; i++ {
		wire.Write(pdataPDU(pcid, payload, i == fragments-1))
	}
	raw := wire.Bytes()

	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	for i := 0; i < b.N; i++ {
		pdu := NewPDUService(WithMaxMessageSize(fragments*fragLen+4096),
			WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))).(*pduService)
		r := bufio.NewReader(bytes.NewReader(raw))
		w := bufio.NewWriter(&bytes.Buffer{})
		pdu.SetConn(bufio.NewReadWriter(r, w))
		pdu.negotiated = true
		_, _ = pdu.NextPDU()
	}
}
