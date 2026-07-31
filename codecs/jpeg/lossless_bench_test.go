package jpeg

import "testing"

// Decode benchmarks for the pure-Go lossless JPEG path. The package had none,
// so neither the decoder's speed nor its allocation behaviour was measurable.
// Sizes span the range where the full-image intermediate stops fitting in cache,
// which is where the streaming and buffering strategies diverge.

func benchLLDecode(b *testing.B, w, h, precision int) {
	bps := 1
	if precision > 8 {
		bps = 2
	}
	raw := make([]byte, w*h*bps)
	for i := range raw {
		raw[i] = byte(i*13 + i/7)
	}
	enc, err := encodeLosslessJPEG(raw, w, h, 1, precision)
	if err != nil {
		b.Skip(err)
	}
	out := make([]byte, len(raw))
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := decodeLosslessInto(enc, out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLLDecode512(b *testing.B)  { benchLLDecode(b, 512, 512, 16) }
func BenchmarkLLDecode2048(b *testing.B) { benchLLDecode(b, 2048, 2048, 16) }

func BenchmarkLLDecode1024(b *testing.B) { benchLLDecode(b, 1024, 1024, 16) }
func BenchmarkLLDecode1536(b *testing.B) { benchLLDecode(b, 1536, 1536, 16) }
