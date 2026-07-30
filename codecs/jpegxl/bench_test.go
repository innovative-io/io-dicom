package jpegxl

import (
	"testing"
)

// Decode/encode benchmarks for the pure-Go JPEG XL path. This package is the
// largest codec in the tree (~11k LOC) and had no benchmarks at all, so its
// VarDCT reconstruct, loop-filter and XYB hot paths were unguarded against
// regressions and their allocation behaviour was unmeasured.

// synthPixels builds a deterministic gradient with local variation, so the
// entropy coder does real work instead of collapsing runs.
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

func encodeFixture(tb testing.TB, w, h, samples, bitsa int, lossless bool) ([]byte, int, []byte) {
	tb.Helper()
	raw := synthPixels(w, h, samples, bitsa)
	var encoded []byte
	var encSize int
	if err := JXLencode(raw, uint16(w), uint16(h), uint16(samples), uint16(bitsa), &encoded, &encSize, lossless); err != nil {
		tb.Skipf("JXLencode unavailable for %dx%d s=%d b=%d lossless=%v: %v", w, h, samples, bitsa, lossless, err)
	}
	return encoded, encSize, raw
}

func benchDecode(b *testing.B, w, h, samples, bitsa int, lossless bool) {
	b.Helper()
	encoded, encSize, raw := encodeFixture(b, w, h, samples, bitsa, lossless)
	out := make([]byte, len(raw))

	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := JXLdecode(encoded[:encSize], uint32(encSize), out); err != nil {
			b.Fatalf("JXLdecode: %v", err)
		}
	}
}

// Lossless exercises the modular path; lossy exercises VarDCT, which the audit
// flagged as allocating per varblock.
func BenchmarkJXLDecodeLossless_Gray8_256(b *testing.B)  { benchDecode(b, 256, 256, 1, 8, true) }
func BenchmarkJXLDecodeLossless_Gray16_256(b *testing.B) { benchDecode(b, 256, 256, 1, 16, true) }
func BenchmarkJXLDecodeLossy_Gray8_256(b *testing.B)     { benchDecode(b, 256, 256, 1, 8, false) }
func BenchmarkJXLDecodeLossy_RGB8_256(b *testing.B)      { benchDecode(b, 256, 256, 3, 8, false) }

// Larger frames show how cost scales with varblock count.
func BenchmarkJXLDecodeLossy_Gray8_512(b *testing.B)  { benchDecode(b, 512, 512, 1, 8, false) }
func BenchmarkJXLDecodeLossy_Gray16_512(b *testing.B) { benchDecode(b, 512, 512, 1, 16, false) }
func BenchmarkJXLDecodeLossless_Gray16_512(b *testing.B) {
	benchDecode(b, 512, 512, 1, 16, true)
}

func benchEncode(b *testing.B, w, h, samples, bitsa int, lossless bool) {
	b.Helper()
	raw := synthPixels(w, h, samples, bitsa)
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var encoded []byte
		var encSize int
		if err := JXLencode(raw, uint16(w), uint16(h), uint16(samples), uint16(bitsa), &encoded, &encSize, lossless); err != nil {
			b.Skipf("JXLencode unavailable: %v", err)
		}
	}
}

func BenchmarkJXLEncodeLossless_Gray16_256(b *testing.B) { benchEncode(b, 256, 256, 1, 16, true) }
func BenchmarkJXLEncodeLossy_Gray8_256(b *testing.B)     { benchEncode(b, 256, 256, 1, 8, false) }
