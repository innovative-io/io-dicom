package network

import (
	"bufio"
	"bytes"
	"io"
	"testing"

	"github.com/innovative-io/io-dicom/media"
)

// BenchmarkPDataTFWriteManyBlocks fragments a 1 MiB payload at the default 4 KiB
// block size (~256 PDV fragments). Each fragment previously allocated a fresh
// 4 KiB DICOMBuffer just to emit a 12-byte header; the benchmark's allocs/op
// makes that churn (and its removal) visible.
func BenchmarkPDataTFWriteManyBlocks(b *testing.B) {
	payload := make([]byte, 1<<20)
	buf := media.NewDICOMBufferFromBytes(payload)
	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(io.Discard))
	pd := &PresentationDataTransfer{Buffer: buf, PresentationContextID: 1}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pd.BlockSize = 4096 // Write shrinks BlockSize on the final fragment; reset each run.
		if err := pd.Write(rw); err != nil {
			b.Fatal(err)
		}
	}
}
