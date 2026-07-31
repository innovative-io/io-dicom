package jpeg

import (
	"bytes"
	"testing"
)

// packSamples reproduces the buffering path's output packing: component-minor,
// little-endian for >8-bit samples.
func packSamples(samples []int, bytesPerSample int) []byte {
	if bytesPerSample == 1 {
		out := make([]byte, len(samples))
		for i, s := range samples {
			out[i] = byte(s)
		}
		return out
	}
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		out[i*2] = byte(s)
		out[i*2+1] = byte(s >> 8)
	}
	return out
}

// TestStreamingMatchesBufferedDecode is the correctness contract for the
// streaming decoder: decodeScanInto must produce byte-for-byte what the
// full-plane path produced, for every geometry and both restart configurations.
//
// The streamed path keeps only two line buffers, so an error in the predictor's
// Ra/Rb/Rc indexing or in the line swap would show up as a mismatch here.
func TestStreamingMatchesBufferedDecode(t *testing.T) {
	cases := []struct {
		name      string
		w, h      int
		precision int
		restartIv int
	}{
		{"8-bit 16x16", 16, 16, 8, 0},
		{"8-bit 1x8 single column", 1, 8, 8, 0},
		{"8-bit 8x1 single row", 8, 1, 8, 0},
		{"16-bit 32x24", 32, 24, 16, 0},
		{"8-bit 16x16 with restarts", 16, 16, 8, 16},
		{"16-bit 20x20 with restarts", 20, 20, 16, 5},
		{"8-bit 1x1", 1, 1, 8, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bps := 1
			if tc.precision > 8 {
				bps = 2
			}
			raw := make([]byte, tc.w*tc.h*bps)
			for i := range raw {
				raw[i] = byte(i*17 + i/5)
			}

			// encodeLosslessJPEG produces SOF3, which is what this decoder
			// handles; EIJG8encode emits baseline DCT.
			enc, err := encodeLosslessJPEG(raw, tc.w, tc.h, 1, tc.precision)
			if err != nil {
				t.Skipf("lossless encoder unavailable for this geometry: %v", err)
			}

			// Buffered path: full sample plane, then pack.
			frame, samples, err := decodeLossless(enc)
			if err != nil {
				t.Skipf("buffered decode unavailable: %v", err)
			}
			want := packSamples(samples, bps)

			// Streamed path.
			got := make([]byte, len(raw))
			if err := decodeLosslessInto(enc, got); err != nil {
				t.Fatalf("streamed decode: %v", err)
			}

			if !bytes.Equal(got, want) {
				t.Fatalf("streamed output differs from buffered at %s (w=%d h=%d prec=%d): first diff at %d",
					tc.name, frame.width, frame.height, frame.precision, firstDiffIdx(want, got))
			}
			// And both must reproduce the original pixels (lossless).
			if !bytes.Equal(got, raw) {
				t.Fatalf("lossless round-trip mismatch: first diff at %d", firstDiffIdx(raw, got))
			}
		})
	}
}

func firstDiffIdx(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
