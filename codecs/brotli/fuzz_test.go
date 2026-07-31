package brotli

import (
	"testing"
	"time"
)

// FuzzDecompress exercises the Brotli decoder against arbitrary bytes. The
// contract is that decoding untrusted input must terminate and must not panic
// or allocate without bound — an error is an acceptable outcome.
//
// This package had no fuzz target, which is why a hang reachable from any JPEG
// XL instance carrying a jbrd box went unnoticed: a command inserting zero
// literals whose copy resolved to a zero-length dictionary transform advanced
// neither the output nor the bit reader, and the decoder spun at 100% CPU.
// Fuzzing finds that state in seconds.
func FuzzDecompress(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0x1b, 0x00, 0x00}) // minimal-ish header shapes
	f.Add([]byte{0x21, 0x00, 0x00})
	// Reduced forms of the two hangs found by an audit of this package.
	f.Add([]byte{0x30, 0x06, 0x00, 0x00, 0x04, 0x40, 0x08, 0x12, 0x2c})
	f.Add([]byte{0x24, 0x30, 0x30, 0x30, 0x00, 0x0a, 0x22, 0x58, 0x0a, 0x42, 0x0a, 0x41, 0x0a, 0xa2, 0x30, 0x0a, 0xa2, 0x30})
	// A real round-tripped stream, so the fuzzer starts from valid structure.
	f.Add(Compress([]byte("DICOM DICOM DICOM DICOM brotli round trip seed")))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Guard against a regression reintroducing a non-terminating decode:
		// without a bound a hang would stall the whole fuzz run rather than
		// being reported as a failure.
		done := make(chan struct{})
		go func() {
			defer close(done)
			// A small explicit cap keeps fuzz iterations cheap while still
			// exercising the bomb guard.
			_, _ = DecompressBounded(data, 0, 1<<20)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("Decompress did not terminate on %d-byte input: %x", len(data), data)
		}
	})
}
