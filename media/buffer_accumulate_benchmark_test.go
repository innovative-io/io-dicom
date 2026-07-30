package media

import (
	"fmt"
	"testing"
)

// BenchmarkBufferAccumulate mirrors the DICOM receive path: pdu_service resets
// the P-DATA buffer per message and then appends every inbound PDV fragment
// into it. Reset deliberately drops the backing array (callers may hold
// zero-copy sub-slices of the previous message), so the buffer re-grows from
// nothing on every message and the growth factor decides how much of a C-STORE
// is spent reallocating and copying.
//
// The reported B/op is the true cost of accumulating msgSize bytes; anything
// well above msgSize is regrowth waste.
func benchAccumulate(b *testing.B, msgSize, fragment int) {
	chunk := make([]byte, fragment)
	buf := NewDICOMBuffer()

	b.SetBytes(int64(msgSize))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		for written := 0; written < msgSize; written += fragment {
			n := fragment
			if written+n > msgSize {
				n = msgSize - written
			}
			if _, err := buf.Write(chunk[:n], n); err != nil {
				b.Fatalf("Write: %v", err)
			}
		}
		if buf.GetSize() != msgSize {
			b.Fatalf("got %d bytes, want %d", buf.GetSize(), msgSize)
		}
	}
}

func BenchmarkBufferAccumulate(b *testing.B) {
	// 16 KiB fragments is the negotiated P-DATA-TF maximum.
	for _, mb := range []int{1, 4, 16} {
		b.Run(fmt.Sprintf("%dMiB_16KiBfrags", mb), func(b *testing.B) {
			benchAccumulate(b, mb<<20, 16<<10)
		})
	}
	// 4 KiB is the default send-side block size, so a peer may fragment that small.
	b.Run("4MiB_4KiBfrags", func(b *testing.B) {
		benchAccumulate(b, 4<<20, 4<<10)
	})
}
