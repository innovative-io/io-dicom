package network

import (
	"bufio"

	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

// pdvChunkWriter is an io.Writer that splits the bytes written to it into
// P-DATA-TF PDVs of at most blockSize payload bytes and writes them to rw. It
// lets the DIMSE serializer (media.WriteObjTo) stream straight to the wire
// instead of building the entire message in a buffer first.
//
// The last-fragment bit may only be set on the final PDV, but the writer does
// not know which PDV is final until Close. It therefore emits a full block only
// once it holds strictly more than one block's worth of bytes (so that block is
// provably not the last); Close emits whatever remains — possibly empty — as the
// terminating last-fragment PDV. For a given byte stream and blockSize this
// produces exactly the PDV boundaries the old buffer-then-chunk path produced.
type pdvChunkWriter struct {
	rw        *bufio.ReadWriter
	blockSize int
	pcid      byte
	baseMsg   byte // command/dataset bit: PDVCommand or PDVDataset
	pending   []byte
	hdr       [12]byte
	err       error
}

func newPDVChunkWriter(rw *bufio.ReadWriter, blockSize int, pcid, baseMsg byte) *pdvChunkWriter {
	if blockSize <= 0 {
		blockSize = 4096
	}
	return &pdvChunkWriter{
		rw:        rw,
		blockSize: blockSize,
		pcid:      pcid,
		baseMsg:   baseMsg,
		pending:   make([]byte, 0, blockSize),
	}
}

func (w *pdvChunkWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	total := len(p)
	for len(p) > 0 {
		// Zero-copy fast path: staging empty and strictly more than a block
		// available, so this block is provably not the last — emit it straight
		// from p without copying (matters for a multi-MB pixel-data value).
		if len(w.pending) == 0 && len(p) > w.blockSize {
			if err := w.emit(p[:w.blockSize], false); err != nil {
				return total - len(p), err
			}
			p = p[w.blockSize:]
			continue
		}
		// Staging full and more bytes remain → not the last block.
		if len(w.pending) == w.blockSize {
			if err := w.emit(w.pending, false); err != nil {
				return total - len(p), err
			}
			w.pending = w.pending[:0]
			continue
		}
		take := w.blockSize - len(w.pending)
		if take > len(p) {
			take = len(p)
		}
		w.pending = append(w.pending, p[:take]...)
		p = p[take:]
	}
	return total, nil
}

// Close emits the final PDV with the last-fragment bit set. It must be called
// exactly once after the last Write. A zero-length message still emits one
// terminating PDV so the peer unblocks from its read.
func (w *pdvChunkWriter) Close() error {
	if w.err != nil {
		return w.err
	}
	return w.emit(w.pending, true)
}

func (w *pdvChunkWriter) emit(payload []byte, last bool) error {
	msgHeader := w.baseMsg & PDVTypeMask
	if last {
		msgHeader |= PDVLastFragment
	}
	pdvLength := uint32(len(payload)) + 2 // + PCID + msgHeader
	pduLength := pdvLength + 4            // + the 4-byte PDV length field
	if err := writePDVHeader(w.rw, &w.hdr, pdutype.PDUDataTransfer, 0, pduLength, pdvLength, w.pcid, msgHeader); err != nil {
		w.err = err
		return err
	}
	if len(payload) > 0 {
		if _, err := w.rw.Write(payload); err != nil {
			w.err = err
			return err
		}
	}
	return nil
}
