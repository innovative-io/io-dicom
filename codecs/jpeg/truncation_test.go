package jpeg

import (
	"testing"
)

// TestLosslessRejectsMissingEntropyData pins truncation detection on the
// lossless decoder.
//
// The bit reader returns zeros once the entropy-coded data runs out, and
// nothing reported that it had done so, so a stream whose scan carried no
// entropy bytes at all decoded to a completely filled buffer of the predictor's
// seed value and returned nil. In diagnostic imaging a plausible-looking wrong
// image is worse than a failed decode.
func TestLosslessRejectsMissingEntropyData(t *testing.T) {
	// A conforming 4x2 8-bit lossless stream, but with every entropy byte after
	// SOS removed (SOI, SOF3, DHT, SOS header, then straight to EOI).
	stream := buildLosslessWithRestarts()

	sos := -1
	for i := 0; i+1 < len(stream); i++ {
		if stream[i] == 0xFF && stream[i+1] == 0xDA {
			sos = i
			break
		}
	}
	if sos < 0 {
		t.Fatal("no SOS marker in the generated stream")
	}
	segLen := int(stream[sos+2])<<8 | int(stream[sos+3])
	headerOnly := append([]byte{}, stream[:sos+2+segLen]...)
	headerOnly = append(headerOnly, 0xFF, 0xD9) // EOI

	out := make([]byte, 4*2)
	if err := decodeLosslessInto(headerOnly, out); err == nil {
		t.Fatalf("a scan with no entropy data decoded to %v with no error", out)
	}
}

// TestLosslessIntactStreamStillDecodes guards the truncation check from
// rejecting valid input — including the restart-interval case, where each
// entropy segment is legitimately bit-padded and so contributes some padding.
func TestLosslessIntactStreamStillDecodes(t *testing.T) {
	out := make([]byte, 4*2)
	if err := decodeLosslessInto(buildLosslessWithRestarts(), out); err != nil {
		t.Fatalf("intact stream with restart intervals must decode: %v", err)
	}
	want := []byte{128, 128, 128, 128, 129, 129, 129, 129}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("decoded %v, want %v", out, want)
		}
	}
}
