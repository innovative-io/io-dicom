package jpeg

import (
	"runtime"
	"testing"
	"time"
)

// buildSOFOnly returns a tiny JPEG carrying only SOI + a frame header declaring
// huge dimensions, with no entropy data. The decoder must reject it on the
// header alone rather than sizing buffers from those dimensions.
func buildSOFOnly(sofMarker byte, w, h uint16, precision byte) []byte {
	out := []byte{0xFF, 0xD8} // SOI
	// SOF: len(11), precision, height, width, 1 component (id 1, sampling 0x11, Tq 0)
	out = append(out, 0xFF, sofMarker, 0x00, 0x0B, precision,
		byte(h>>8), byte(h), byte(w>>8), byte(w), 0x01, 0x01, 0x11, 0x00)
	out = append(out, 0xFF, 0xD9) // EOI
	return out
}

// TestHeaderDeclaringHugeDimensionsIsRejectedCheaply pins the ordering fix.
//
// decodeLossless/decodeDCT sized their sample buffers from the SOF dimensions
// and ran the whole scan; the output-size check lived in the *Into wrappers,
// i.e. after the fact. A 49-byte header declaring 16384x16384 therefore
// allocated 2 GiB and spent ~1.8s before reporting a size mismatch — about
// 44,000,000:1 amplification, multiplied by the worker count under concurrent
// frame decoding.
func TestHeaderDeclaringHugeDimensionsIsRejectedCheaply(t *testing.T) {
	cases := []struct {
		name   string
		stream []byte
		decode func([]byte, []byte) error
	}{
		{"lossless 16384x16384 16-bit", buildSOFOnly(mSOF3, 16384, 16384, 16), decodeLosslessInto},
		{"lossless 8192x8192 8-bit", buildSOFOnly(mSOF3, 8192, 8192, 8), decodeLosslessInto},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := make([]byte, 1) // deliberately far too small

			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			start := time.Now()

			err := tc.decode(tc.stream, out)

			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			allocated := after.TotalAlloc - before.TotalAlloc

			t.Logf("%d-byte header -> %d bytes allocated in %v", len(tc.stream), allocated, elapsed)

			if err == nil {
				t.Fatal("expected an output-size error")
			}
			// The rejection must be on the header alone. A few KiB of parsing
			// overhead is fine; megabytes means buffers were sized from the
			// declared dimensions.
			if allocated > 1<<20 {
				t.Fatalf("rejecting a %d-byte header allocated %d bytes; the size check "+
					"is still running after the buffers are sized", len(tc.stream), allocated)
			}
		})
	}
}

// TestValidStreamStillDecodesUnderBound guards the ordering change from
// rejecting legitimate input whose output fits exactly.
func TestValidStreamStillDecodesUnderBound(t *testing.T) {
	stream := buildLosslessWithRestarts() // 4x2, 8-bit, 1 component
	out := make([]byte, 4*2)
	if err := decodeLosslessInto(stream, out); err != nil {
		t.Fatalf("a stream whose output fits exactly must decode: %v", err)
	}
	// One byte short must still be rejected.
	if err := decodeLosslessInto(stream, make([]byte, 4*2-1)); err == nil {
		t.Fatal("expected an output-size error when the buffer is one byte short")
	}
}
