package jpeg

import (
	"bytes"
	"testing"
)

// buildLosslessWithRestarts assembles a conforming 4x2, 8-bit, single-component
// lossless (SOF3) JPEG that uses a restart interval of 4 — i.e. one RST marker
// between the two rows.
//
// The encoder in this package cannot emit DRI, and no fixture in testdata/ uses
// restart intervals, which is why this whole code path had no coverage.
//
// Huffman table: symbol 0 (difference category 0) = "0", symbol 1 (category 1)
// = "10". Row 0 codes four zero differences; with the lossless predictor seeded
// at 2^(P-1) that yields 128,128,128,128. Row 1 codes +1 then three zeros,
// yielding 129,129,129,129 — chosen so a decoder that fails to consume the RST
// marker (and therefore reads zero bits forever) produces 128s and is caught.
func buildLosslessWithRestarts() []byte {
	var b bytes.Buffer
	b.Write([]byte{0xFF, 0xD8}) // SOI

	// DRI: restart interval = 4 MCUs (one row)
	b.Write([]byte{0xFF, 0xDD, 0x00, 0x04, 0x00, 0x04})

	// SOF3: precision 8, height 2, width 4, 1 component (id 1, sampling 0x11, Tq 0)
	b.Write([]byte{0xFF, 0xC3, 0x00, 0x0B, 0x08, 0x00, 0x02, 0x00, 0x04, 0x01, 0x01, 0x11, 0x00})

	// DHT (class 0, table 0): one 1-bit code and one 2-bit code; symbols 0 and 1.
	b.Write([]byte{0xFF, 0xC4, 0x00, 0x15, 0x00})
	b.Write([]byte{0x01, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}) // counts, lengths 1..16
	b.Write([]byte{0x00, 0x01})                                          // symbols

	// SOS: 1 component (Cs 1, Td/Ta 0), Ss = predictor 1, Se 0, Ah/Al 0
	b.Write([]byte{0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x01, 0x00, 0x00})

	// Row 0: four zero differences -> "0000", padded to a byte with 1s.
	b.WriteByte(0x0F)
	// RST0 terminates the first restart interval.
	b.Write([]byte{0xFF, 0xD0})
	// Row 1: +1 then three zeros -> "10" "1" "000" = "101000", padded with 1s.
	b.WriteByte(0xA3)

	b.Write([]byte{0xFF, 0xD9}) // EOI
	return b.Bytes()
}

// TestLosslessRestartMarkerIsConsumed is a regression test for silent pixel
// corruption on conforming files. restart() previously consumed the RSTn marker
// only when a prior fill() had already latched it; at a real interval boundary
// the accumulator still held bits, so the marker was never consumed. The next
// fill() then latched it and returned false forever, and every interval after
// the first decoded as padding zeros — with no error returned.
func TestLosslessRestartMarkerIsConsumed(t *testing.T) {
	encoded := buildLosslessWithRestarts()
	out := make([]byte, 4*2) // 4x2, 8-bit, 1 component

	if err := decodeLosslessInto(encoded, out); err != nil {
		t.Fatalf("decodeLosslessInto: %v", err)
	}

	want := []byte{128, 128, 128, 128, 129, 129, 129, 129}
	if !bytes.Equal(out, want) {
		t.Fatalf("restart interval decoded incorrectly:\n got %v\nwant %v\n"+
			"(all-128 second row means the RST marker was not consumed)", out, want)
	}
}
