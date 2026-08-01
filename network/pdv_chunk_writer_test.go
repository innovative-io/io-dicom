package network

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/innovative-io/io-dicom/media"
)

// oldPathWire reproduces the pre-streaming send: the full byte stream lives in a
// DICOMBuffer that PresentationDataTransfer.Write chunks into PDVs.
func oldPathWire(t *testing.T, stream []byte, blockSize int, pcid, baseMsg byte) []byte {
	t.Helper()
	buf := media.NewDICOMBuffer()
	if len(stream) > 0 {
		if _, err := buf.Write(stream, len(stream)); err != nil {
			t.Fatalf("seed buffer: %v", err)
		}
	}
	pd := &PresentationDataTransfer{
		Buffer:                buf,
		BlockSize:             uint32(blockSize),
		PresentationContextID: pcid,
		MsgHeader:             baseMsg,
	}
	var sink bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&sink))
	if err := pd.Write(rw); err != nil {
		t.Fatalf("old Pdata.Write: %v", err)
	}
	if err := rw.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return sink.Bytes()
}

// newPathWire runs the same stream through pdvChunkWriter, feeding it in
// writeChunk-sized pieces to exercise accumulation across Write calls.
func newPathWire(t *testing.T, stream []byte, blockSize, writeChunk int, pcid, baseMsg byte) []byte {
	t.Helper()
	var sink bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&sink))
	cw := newPDVChunkWriter(rw, blockSize, pcid, baseMsg)
	for off := 0; off < len(stream); off += writeChunk {
		end := off + writeChunk
		if end > len(stream) {
			end = len(stream)
		}
		if _, err := cw.Write(stream[off:end]); err != nil {
			t.Fatalf("chunk writer Write: %v", err)
		}
	}
	if err := cw.Close(); err != nil {
		t.Fatalf("chunk writer Close: %v", err)
	}
	if err := rw.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return sink.Bytes()
}

// TestPDVChunkWriterMatchesBufferedChunking is the wire-format contract for the
// streaming send: for any stream length and block size, the PDVs produced by
// pdvChunkWriter must be byte-identical to those the old buffer-then-chunk path
// produced — and independent of how the serializer split the bytes across
// Write calls.
func TestPDVChunkWriterMatchesBufferedChunking(t *testing.T) {
	const (
		blockSize = 64
		pcid      = 3
	)
	// Lengths chosen around block boundaries: sub-block, exact multiples,
	// multiples ± 1, and an empty message (which must still emit one PDV).
	lengths := []int{0, 1, 63, 64, 65, 127, 128, 129, 200, 256, 257}
	// Feed the new path in several granularities, including ones that never
	// align with blockSize, to prove the boundaries come from the stream, not
	// the Write calls.
	writeChunks := []int{1, 7, 64, 65, 1000}

	for _, baseMsg := range []byte{PDVDataset, PDVCommand} {
		for _, n := range lengths {
			stream := make([]byte, n)
			for i := range stream {
				stream[i] = byte(i*31 + 5)
			}
			want := oldPathWire(t, stream, blockSize, pcid, baseMsg)

			for _, wc := range writeChunks {
				got := newPathWire(t, stream, blockSize, wc, pcid, baseMsg)
				if !bytes.Equal(got, want) {
					t.Fatalf("len=%d baseMsg=%#x writeChunk=%d: streamed wire differs "+
						"from buffered (got %d bytes, want %d, first diff %d)",
						n, baseMsg, wc, len(got), len(want), firstDiffIndex(got, want))
				}
			}
		}
	}
}

// TestPDVChunkWriterDefaultsZeroBlockSize pins the blockSize<=0 guard, matching
// the old path's `if BlockSize == 0 { BlockSize = 4096 }`.
func TestPDVChunkWriterDefaultsZeroBlockSize(t *testing.T) {
	if w := newPDVChunkWriter(nil, 0, 1, PDVDataset); w.blockSize != 4096 {
		t.Fatalf("blockSize 0 -> %d, want 4096", w.blockSize)
	}
	if w := newPDVChunkWriter(nil, -5, 1, PDVDataset); w.blockSize != 4096 {
		t.Fatalf("negative blockSize -> %d, want 4096", w.blockSize)
	}
}

func firstDiffIndex(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return n
	}
	return -1
}
