package rle

import (
	"bytes"
	"testing"
)

// TestRoundTripAcrossSegmentLayouts covers every segment count DICOM RLE can
// produce: samplesPerPixel * bytesPerSample.
//
// The decoder previously hard-coded (photometric, segmentCount) cases for 1, 2
// and 3 segments only. 16-bit colour legitimately produces 6, so RLEencode
// emitted streams its own decoder rejected with "format not supported", and any
// conforming 6-segment stream from another vendor was refused on ingest.
func TestRoundTripAcrossSegmentLayouts(t *testing.T) {
	const rows, cols = 8, 8
	cases := []struct {
		name     string
		bits     uint16
		samples  uint16
		photoInt string
		wantSegs int
	}{
		{"mono 8-bit", 8, 1, "MONOCHROME2", 1},
		{"mono 16-bit", 16, 1, "MONOCHROME2", 2},
		{"RGB 8-bit", 8, 3, "RGB", 3},
		{"RGB 16-bit", 16, 3, "RGB", 6},
		{"palette 8-bit", 8, 1, "PALETTE COLOR", 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := rows * cols * int(tc.samples) * int(tc.bits/8)
			raw := make([]byte, n)
			for i := range raw {
				raw[i] = byte(i*13 + 7)
			}

			enc, err := RLEencode(raw, rows, cols, tc.bits, tc.samples)
			if err != nil {
				t.Fatalf("RLEencode: %v", err)
			}
			if got := int(enc[0]); got != tc.wantSegs {
				t.Fatalf("header declares %d segments, want %d", got, tc.wantSegs)
			}

			out := make([]byte, n)
			if err := RLEdecode(enc, out, uint32(len(enc)), uint32(n), tc.photoInt); err != nil {
				t.Fatalf("RLEdecode of this package's own %d-segment output: %v", tc.wantSegs, err)
			}
			if !bytes.Equal(out, raw) {
				t.Fatalf("round-trip mismatch for %s", tc.name)
			}
		})
	}
}

// TestEncodeActuallyCompresses pins the replicate-run fix. The encoder emitted
// only PackBits literal runs, so output was always the input plus one byte per
// 128 — meaning even a completely uniform frame expanded, and "RLE Lossless"
// transcoding always grew the object.
func TestEncodeActuallyCompresses(t *testing.T) {
	const rows, cols = 64, 64
	n := rows * cols

	cases := []struct {
		name string
		fill func(i int) byte
		// Highly compressible input must come out substantially smaller.
		maxRatio float64
	}{
		{"all zero", func(int) byte { return 0 }, 0.10},
		{"large uniform regions", func(i int) byte { return byte(i / 256) }, 0.20},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := make([]byte, n)
			for i := range raw {
				raw[i] = tc.fill(i)
			}
			enc, err := RLEencode(raw, rows, cols, 8, 1)
			if err != nil {
				t.Fatalf("RLEencode: %v", err)
			}
			// Exclude the fixed 64-byte header from the ratio.
			ratio := float64(len(enc)-64) / float64(n)
			t.Logf("%d raw -> %d encoded (%.3f of raw, excluding header)", n, len(enc), ratio)
			if ratio > tc.maxRatio {
				t.Fatalf("compressible input did not compress: ratio %.3f exceeds %.3f", ratio, tc.maxRatio)
			}

			// Compression must still be lossless.
			out := make([]byte, n)
			if err := RLEdecode(enc, out, uint32(len(enc)), uint32(n), "MONOCHROME2"); err != nil {
				t.Fatalf("RLEdecode: %v", err)
			}
			if !bytes.Equal(out, raw) {
				t.Fatal("round-trip mismatch after compression")
			}
		})
	}
}

// TestEncodeIncompressibleStaysBounded guards the replicate-run logic from
// pathologically expanding data that has no runs.
func TestEncodeIncompressibleStaysBounded(t *testing.T) {
	const rows, cols = 64, 64
	n := rows * cols
	raw := make([]byte, n)
	for i := range raw {
		raw[i] = byte(i*7 + i/3) // no runs of three
	}
	enc, err := RLEencode(raw, rows, cols, 8, 1)
	if err != nil {
		t.Fatalf("RLEencode: %v", err)
	}
	payload := len(enc) - 64
	// PackBits overhead on incompressible data is one byte per 128.
	if maxExpected := n + n/128 + 1; payload > maxExpected {
		t.Fatalf("incompressible input expanded to %d, more than the %d PackBits bound",
			payload, maxExpected)
	}
	out := make([]byte, n)
	if err := RLEdecode(enc, out, uint32(len(enc)), uint32(n), "MONOCHROME2"); err != nil {
		t.Fatalf("RLEdecode: %v", err)
	}
	if !bytes.Equal(out, raw) {
		t.Fatal("round-trip mismatch on incompressible data")
	}
}
