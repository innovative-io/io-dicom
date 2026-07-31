package brotli

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

// withinTimeout runs fn and fails if it has not returned within d. Used because
// the failure mode under test is a non-terminating decode: without a bound the
// test would hang the whole package rather than report a failure.
func withinTimeout(t *testing.T, d time.Duration, name string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not terminate within %s", name, d)
	}
}

// TestDecompressTerminatesOnCraftedHangs pins the fixed-point guard. Both inputs
// previously spun at 100% CPU forever: a command inserting zero literals whose
// copy resolved to a zero-length dictionary transform advanced neither the
// output nor the bit reader, and with single-symbol prefix codes consuming no
// bits and the reader returning implicit zeros past EOF, the state repeated.
//
// Reachable from any received JPEG XL instance carrying a jbrd box.
func TestDecompressTerminatesOnCraftedHangs(t *testing.T) {
	inputs := [][]byte{
		{0x30, 0x06, 0x00, 0x00, 0x04, 0x40, 0x08, 0x12, 0x2c},
		{0x24, 0x30, 0x30, 0x30, 0x00, 0x0a, 0x22, 0x58, 0x0a, 0x42, 0x0a, 0x41, 0x0a, 0xa2, 0x30, 0x0a, 0xa2, 0x30},
	}
	for i, in := range inputs {
		in := in
		withinTimeout(t, 10*time.Second, "Decompress", func() {
			// The result is irrelevant; not hanging is the contract.
			_, _ = Decompress(in, 0)
		})
		t.Logf("input %d (%d bytes) terminated", i, len(in))
	}
}

// TestDecompressEnforcesOutputCap pins the output ceiling on both output paths.
//
// A real bomb cannot be built with this package's own Compress (it emits
// uncompressed meta-blocks, so it never expands), so the cap is exercised by
// decoding a legitimate stream under a ceiling smaller than its output. That is
// the same code path a bomb takes: an audit measured 809 crafted bytes expanding
// to 976 MiB, over 1,200,000:1.
func TestDecompressEnforcesOutputCap(t *testing.T) {
	payload := make([]byte, 1<<20) // 1 MiB
	for i := range payload {
		payload[i] = byte(i)
	}
	stream := Compress(payload)

	// Below the output size: must be rejected rather than materialised.
	if _, err := DecompressBounded(stream, 0, 64<<10); !errors.Is(err, errTooLarge) {
		t.Fatalf("expected errTooLarge decoding 1 MiB under a 64 KiB cap, got %v", err)
	}

	// With an adequate ceiling the same stream decodes correctly.
	out, err := DecompressBounded(stream, 0, 4<<20)
	if err != nil {
		t.Fatalf("decoding under an adequate cap: %v", err)
	}
	if !bytes.Equal(out, payload) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(out), len(payload))
	}
}

// TestDecompressRoundTripUnaffected guards against the bounds work breaking
// ordinary decoding.
func TestDecompressRoundTripUnaffected(t *testing.T) {
	payload := bytes.Repeat([]byte("brotli round trip payload "), 4096)
	out, err := Decompress(Compress(payload), len(payload))
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(out, payload) {
		t.Fatal("round-trip mismatch")
	}
}

// TestReadAlignedBytesChecksBeforeAllocating pins the ordering fix: n comes from
// the codestream (MLEN, up to 2^24), so allocating before the bounds check let a
// 4-byte input reserve 16 MiB before failing.
func TestReadAlignedBytesChecksBeforeAllocating(t *testing.T) {
	b := newBitReader([]byte{0x01, 0x02, 0x03, 0x04})
	if _, err := b.readAlignedBytes(1 << 24); err == nil {
		t.Fatal("expected a truncation error for a length far beyond the input")
	}
}
