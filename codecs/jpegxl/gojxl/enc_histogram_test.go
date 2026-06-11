package gojxl

import (
	"math/rand"
	"testing"
)

// TestANSHistogramRoundTrip checks that writeANSHistogram (the real, frequency-
// derived ANS histogram encoder) is the exact inverse of readHistogram across a
// wide range of distributions: dense, sparse, single-dominant-symbol, degenerate
// (one symbol) and empty (unused context). The normalized counts the encoder
// commits to must be reproduced bit-for-bit by the decoder, otherwise the rANS
// alias tables on each side diverge and decoding corrupts.
func TestANSHistogramRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 20000; trial++ {
		alpha := 1 + rng.Intn(255)
		freq := make([]int64, alpha)
		dist := rng.Intn(5)
		for i := range freq {
			r := rng.Intn(4)
			switch dist {
			case 0: // dense, wide dynamic range
				if r > 0 {
					freq[i] = int64(1 + rng.Intn(100000))
				}
			case 1: // sparse, small counts
				if r > 2 {
					freq[i] = int64(1 + rng.Intn(10))
				}
			case 2: // one dominant symbol + small tail
				if i == 0 {
					freq[i] = 10000000
				} else if r > 1 {
					freq[i] = int64(1 + rng.Intn(50))
				}
			case 3: // degenerate: a single non-zero symbol
				if i == 0 {
					freq[i] = int64(1 + rng.Intn(1000))
				}
			case 4: // empty: an unused context (all zero)
			}
		}

		counts := normalizeHistogram(freq)
		sum := int32(0)
		for _, c := range counts {
			sum += c
		}
		if sum != ansTabSize {
			t.Fatalf("trial %d (dist %d): normalized sum = %d, want %d", trial, dist, sum, ansTabSize)
		}
		for i := range freq {
			if freq[i] > 0 && counts[i] == 0 {
				t.Fatalf("trial %d (dist %d): non-zero frequency at %d dropped to count 0", trial, dist, i)
			}
		}

		w := newBitWriter()
		writeANSHistogram(w, counts)
		w.ZeroPadToByte()
		got, err := readHistogram(ansLogTabSize, newBitReader(w.Bytes()))
		if err != nil {
			t.Fatalf("trial %d (dist %d, alpha %d): readHistogram: %v", trial, dist, alpha, err)
		}
		for i := range counts {
			g := int32(0)
			if i < len(got) {
				g = got[i]
			}
			if g != counts[i] {
				t.Fatalf("trial %d (dist %d): count[%d] decoded %d, want %d", trial, dist, i, g, counts[i])
			}
		}
	}
}
