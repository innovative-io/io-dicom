package network

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/innovative-io/io-dicom/media"
)

// countingWriter records how many times Write is invoked on it. Wrapped behind a
// bufio.Writer, that count is the number of times buffered data was actually
// drained toward the connection — i.e. the syscall count on a real socket.
type countingWriter struct {
	writes int
	bytes  int
	sink   bytes.Buffer
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.writes++
	c.bytes += len(p)
	return c.sink.Write(p)
}

// TestPDataWriteBatchesFragments pins that serialising a dataset does not force
// one underlying write per PDV fragment.
//
// Pdata.Write used to call rw.Flush() after every fragment, draining the bufio
// buffer each time, so a dataset split into N fragments cost N writes to the
// connection. The sole caller (writeEncodedPDU) already flushes once after
// Write returns, so those per-fragment flushes were redundant as well as slow.
func TestPDataWriteBatchesFragments(t *testing.T) {
	const (
		blockSize   = 4096
		fragments   = 32
		payloadSize = blockSize * fragments // 128 KiB, well under the bufio buffer
	)

	buf := media.NewDICOMBuffer()
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i)
	}
	if _, err := buf.Write(payload, len(payload)); err != nil {
		t.Fatalf("seed buffer: %v", err)
	}

	pd := &PresentationDataTransfer{
		Buffer:                buf,
		BlockSize:             blockSize,
		PresentationContextID: 1,
	}

	cw := &countingWriter{}
	// A bufio buffer larger than the whole payload: with batching, everything
	// fits and drains in a single underlying write at the final flush.
	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriterSize(cw, 256<<10))

	if err := pd.Write(rw); err != nil {
		t.Fatalf("Pdata.Write: %v", err)
	}
	// The caller (writeEncodedPDU) owns the flush.
	if err := rw.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// 32 fragments + headers, all under 256 KiB → a couple of underlying writes,
	// not one per fragment. Pin generously below the fragment count so the test
	// tracks intent, not bufio's exact chunking.
	if cw.writes > 4 {
		t.Fatalf("dataset of %d fragments caused %d underlying writes — fragments "+
			"are not being batched (per-fragment flush regressed)", fragments, cw.writes)
	}

	// Every byte must still reach the wire: this implementation emits one PDU per
	// PDV, so each fragment carries a full 12-byte P-DATA-TF + PDV header.
	wantBytes := fragments * (12 + blockSize)
	if cw.bytes != wantBytes {
		t.Fatalf("wrote %d bytes to the wire, want %d", cw.bytes, wantBytes)
	}
}
