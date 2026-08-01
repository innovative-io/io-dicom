package jpeg

import "testing"

// receiveBitByBit is the reference implementation the batched receive replaced:
// n individual readBit calls. The batched receive must match it exactly,
// including its padBits/realBits bookkeeping and its behaviour past end-of-data.
func receiveBitByBit(br *bitReader, n int) int {
	v := 0
	for i := 0; i < n; i++ {
		v = v<<1 | br.readBit()
	}
	return v
}

// TestReceiveMatchesBitByBit pins that the batched receive is bit-for-bit
// equivalent to n readBit calls, over the same byte stream and read pattern —
// including 0xFF byte-stuffing and reading past the end into zero padding.
func TestReceiveMatchesBitByBit(t *testing.T) {
	// Include a stuffed 0xFF00 (byte-stuffing) and a spread of byte values so
	// the fill path and marker handling are exercised.
	data := []byte{0x9C, 0xFF, 0x00, 0x3A, 0xE1, 0x00, 0x7F, 0xFF, 0x00, 0x55, 0xAA, 0x12}

	// A repeating pattern of receive widths, deliberately crossing byte and
	// accumulator boundaries, and continuing well past the data to force padding.
	widths := []int{1, 3, 8, 5, 16, 2, 7, 11, 4, 9, 13, 6, 16, 15, 10, 8, 8, 8}

	ref := &bitReader{data: data}
	got := &bitReader{data: data}

	for step, n := range widths {
		want := receiveBitByBit(ref, n)
		have := got.receive(n)
		if have != want {
			t.Fatalf("step %d receive(%d) = %d, want %d", step, n, have, want)
		}
		// The accounting must track identically, or truncation detection diverges.
		if got.padBits != ref.padBits {
			t.Fatalf("step %d padBits = %d, want %d", step, got.padBits, ref.padBits)
		}
		if got.realBits != ref.realBits {
			t.Fatalf("step %d realBits = %d, want %d", step, got.realBits, ref.realBits)
		}
		if got.nbits != ref.nbits {
			t.Fatalf("step %d nbits = %d, want %d", step, got.nbits, ref.nbits)
		}
	}
}

// TestReceiveExhaustiveWidths runs the equivalence check for every width 1..16
// starting from a fresh reader, catching any single-width off-by-one.
func TestReceiveExhaustiveWidths(t *testing.T) {
	data := []byte{0xB7, 0x2D, 0xFF, 0x00, 0x8E, 0x41, 0x6A, 0xD3}
	for n := 1; n <= 16; n++ {
		ref := &bitReader{data: data}
		got := &bitReader{data: data}
		// Read several values of this width, past the end, to cover padding too.
		for k := 0; k < 10; k++ {
			want := receiveBitByBit(ref, n)
			have := got.receive(n)
			if have != want || got.padBits != ref.padBits {
				t.Fatalf("n=%d k=%d: got (%d,pad %d), want (%d,pad %d)",
					n, k, have, got.padBits, want, ref.padBits)
			}
		}
	}
}
