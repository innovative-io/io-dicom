package jpegls

import (
	"testing"
)

// encodeSynth returns a real JPEG-LS stream for a w*h 16-bit grayscale frame,
// plus the raw pixels it encodes.
func encodeSynth(t *testing.T, w, h int) (stream []byte, raw []byte) {
	t.Helper()
	raw = synthFrame(w, h)
	var out []byte
	var size int
	if err := JLSencode(raw, uint16(w), uint16(h), 1, 16, &out, &size, false); err != nil {
		t.Skipf("JLSencode unavailable: %v", err)
	}
	return out[:size], raw
}

// TestDecodeRejectsTruncatedStream pins truncation detection.
//
// The bit reader pads with zeros past the end of the entropy-coded data, and no
// decode function reported that it had done so, so a truncated stream produced a
// completely filled output buffer and a nil error. An audit measured a stream cut
// to 50% yielding 47.9% wrong pixels — reported as success. In diagnostic imaging
// a plausible-looking wrong image is worse than a failed decode.
func TestDecodeRejectsTruncatedStream(t *testing.T) {
	const w, h = 64, 64
	stream, raw := encodeSynth(t, w, h)

	// Sanity: the intact stream still decodes correctly.
	full := make([]byte, len(raw))
	if err := JLSdecode(stream, uint32(len(stream)), full); err != nil {
		t.Fatalf("intact stream must decode: %v", err)
	}

	for _, pct := range []int{10, 50, 90} {
		cut := len(stream) * pct / 100
		out := make([]byte, len(raw))
		err := JLSdecode(stream[:cut], uint32(cut), out)
		if err == nil {
			t.Errorf("stream truncated to %d%% (%d of %d bytes) decoded with no error",
				pct, cut, len(stream))
		}
	}
}

// TestDecodeRejectsHeaderOnlyStream covers the degenerate case: a stream with a
// complete header and no entropy data at all previously filled the whole buffer
// with a constant and returned nil.
func TestDecodeRejectsHeaderOnlyStream(t *testing.T) {
	const w, h = 64, 64
	stream, raw := encodeSynth(t, w, h)

	// Find the SOS marker (0xFFDA) and keep only its header, dropping every
	// entropy byte that follows.
	sos := -1
	for i := 0; i+1 < len(stream); i++ {
		if stream[i] == 0xFF && stream[i+1] == 0xDA {
			sos = i
			break
		}
	}
	if sos < 0 {
		t.Skip("no SOS marker found in the encoded stream")
	}
	if sos+4 > len(stream) {
		t.Skip("stream too short to isolate the SOS header")
	}
	segLen := int(stream[sos+2])<<8 | int(stream[sos+3])
	headerOnly := stream[:sos+2+segLen]

	out := make([]byte, len(raw))
	if err := JLSdecode(headerOnly, uint32(len(headerOnly)), out); err == nil {
		t.Fatalf("a %d-byte header-only stream decoded %d bytes with no error",
			len(headerOnly), len(out))
	}
}
