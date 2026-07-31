package rle

import (
	"bytes"
	"testing"
)

// TestYBRFullRoundTripIsLossless is the regression for the decoder's silent
// colour conversion.
//
// RLEdecode used to convert YBR_FULL to RGB in place, but nothing in the
// library rewrites PhotometricInterpretation, so callers received RGB samples
// still labelled YBR_FULL. Two things followed:
//
//   - a conformant consumer, trusting the tag, converted a second time and got
//     wrong colours;
//   - RLEencode does no conversion of its own, so encode-then-decode of a
//     YBR_FULL image was not the identity — it silently corrupted the pixels.
//
// This test pins the second symptom, which needs no consumer to demonstrate.
// RLE is lossless: what goes in must come out.
func TestYBRFullRoundTripIsLossless(t *testing.T) {
	const (
		rows = 8
		cols = 8
	)
	// Chroma values well away from 128 so any YBR<->RGB matrix visibly moves
	// them; a neutral grey would survive the conversion and hide the bug.
	src := make([]byte, rows*cols*3)
	for i := 0; i < rows*cols; i++ {
		src[3*i] = byte(30 + i%200)  // Y
		src[3*i+1] = byte(200 - i%7) // Cb
		src[3*i+2] = byte(20 + i%11) // Cr
	}

	enc, err := RLEencode(src, rows, cols, 8, 3)
	if err != nil {
		t.Fatalf("RLEencode: %v", err)
	}

	got := make([]byte, len(src))
	if err := RLEdecode(enc, got, uint32(len(enc)), uint32(len(got)), "YBR_FULL"); err != nil {
		t.Fatalf("RLEdecode: %v", err)
	}

	if !bytes.Equal(got, src) {
		var idx int
		for i := range src {
			if src[i] != got[i] {
				idx = i
				break
			}
		}
		t.Fatalf("YBR_FULL round-trip is not lossless: first difference at byte %d "+
			"(sent %d, got %d) — the decoder is colour-converting samples the "+
			"encoder never converted", idx, src[idx], got[idx])
	}
}

// TestRGBAndYBRDecodeIdentically pins the contract that the decoder returns
// samples exactly as stored, letting PhotometricInterpretation describe them.
// The photometric name selects sample count, never a colour transform.
func TestRGBAndYBRDecodeIdentically(t *testing.T) {
	const rows, cols = 4, 4
	src := make([]byte, rows*cols*3)
	for i := range src {
		src[i] = byte(i*13 + 7)
	}
	enc, err := RLEencode(src, rows, cols, 8, 3)
	if err != nil {
		t.Fatalf("RLEencode: %v", err)
	}

	asRGB := make([]byte, len(src))
	if err := RLEdecode(enc, asRGB, uint32(len(enc)), uint32(len(asRGB)), "RGB"); err != nil {
		t.Fatalf("decode as RGB: %v", err)
	}
	asYBR := make([]byte, len(src))
	if err := RLEdecode(enc, asYBR, uint32(len(enc)), uint32(len(asYBR)), "YBR_FULL"); err != nil {
		t.Fatalf("decode as YBR_FULL: %v", err)
	}

	if !bytes.Equal(asRGB, asYBR) {
		t.Fatalf("the same 3-segment stream decoded differently under RGB and "+
			"YBR_FULL; the codec must not apply a colour transform\nRGB: %v\nYBR: %v",
			asRGB[:12], asYBR[:12])
	}
	if !bytes.Equal(asRGB, src) {
		t.Fatalf("decode did not reproduce the encoded samples")
	}
}
