package jpeg2000

import (
	"testing"
)

// Decode benchmarks for the pure-Go JPEG 2000 path. The package had none, so
// the EBCOT/DWT hot loops were previously unguarded against regressions.
//
// ratio 0 selects reversible 5/3 (lossless), any positive ratio selects
// irreversible 9/7 (lossy) — the two exercise different inverse DWT kernels.

// synthPixels builds a deterministic gradient with enough local variation that
// the entropy coder does real work rather than run-length collapsing.
func synthPixels(w, h, samples, bitsa int) []byte {
	bytesPerSample := 1
	if bitsa > 8 {
		bytesPerSample = 2
	}
	buf := make([]byte, w*h*samples*bytesPerSample)
	i := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < samples; c++ {
				v := (x*3 + y*5 + c*17 + ((x ^ y) & 0x1f)) & 0xFFFF
				if bytesPerSample == 1 {
					buf[i] = byte(v)
					i++
				} else {
					buf[i] = byte(v)
					buf[i+1] = byte(v >> 8)
					i += 2
				}
			}
		}
	}
	return buf
}

func benchDecode(b *testing.B, w, h, samples, bitsa, ratio int) {
	b.Helper()
	raw := synthPixels(w, h, samples, bitsa)
	var encoded []byte
	var encSize int
	if err := J2Kencode(raw, uint16(w), uint16(h), uint16(samples), uint16(bitsa), &encoded, &encSize, ratio); err != nil {
		b.Skipf("J2Kencode unavailable: %v", err)
	}
	out := make([]byte, len(raw))

	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := J2Kdecode(encoded[:encSize], uint32(encSize), out); err != nil {
			b.Fatalf("J2Kdecode: %v", err)
		}
	}
}

// Reversible 5/3 (lossless) — exercises idwt53_1d.
func BenchmarkJ2KDecodeLossless_Gray8_512(b *testing.B)  { benchDecode(b, 512, 512, 1, 8, 0) }
func BenchmarkJ2KDecodeLossless_Gray16_512(b *testing.B) { benchDecode(b, 512, 512, 1, 16, 0) }
func BenchmarkJ2KDecodeLossless_RGB8_512(b *testing.B)   { benchDecode(b, 512, 512, 3, 8, 0) }

// Irreversible 9/7 (lossy) — exercises idwt97_1d and its four lifting steps.
func BenchmarkJ2KDecodeLossy_Gray16_512(b *testing.B) { benchDecode(b, 512, 512, 1, 16, 10) }
func BenchmarkJ2KDecodeLossy_RGB8_512(b *testing.B)   { benchDecode(b, 512, 512, 3, 8, 10) }

// A larger frame, closer to real modality output, to show scaling.
func BenchmarkJ2KDecodeLossless_Gray16_1024(b *testing.B) { benchDecode(b, 1024, 1024, 1, 16, 0) }

// Encode side, for completeness on the forward DWT path.
func BenchmarkJ2KEncodeLossless_Gray16_512(b *testing.B) {
	raw := synthPixels(512, 512, 1, 16)
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var encoded []byte
		var encSize int
		if err := J2Kencode(raw, 512, 512, 1, 16, &encoded, &encSize, 0); err != nil {
			b.Fatalf("J2Kencode: %v", err)
		}
	}
}
