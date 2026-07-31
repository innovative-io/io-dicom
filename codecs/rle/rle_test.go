package rle

import (
	"encoding/binary"
	"testing"
)

func encodeLiteralRun(data []byte) []byte {
	if len(data) == 0 || len(data) > 128 {
		panic("test helper only supports 1..128 bytes")
	}
	out := make([]byte, 1+len(data))
	out[0] = byte(len(data) - 1)
	copy(out[1:], data)
	return out
}

func buildRLEStream(t *testing.T, segments [][]byte) []byte {
	t.Helper()
	if len(segments) == 0 || len(segments) > rleMaxSegments {
		t.Fatalf("invalid segment count: %d", len(segments))
	}

	headerSize := 64
	offsets := make([]uint32, rleMaxSegments)
	cursor := uint32(headerSize)
	payload := make([]byte, 0)

	for i, seg := range segments {
		offsets[i] = cursor
		enc := encodeLiteralRun(seg)
		payload = append(payload, enc...)
		cursor += uint32(len(enc))
	}

	out := make([]byte, headerSize+len(payload))
	binary.LittleEndian.PutUint32(out[0:4], uint32(len(segments)))
	for i := 0; i < rleMaxSegments; i++ {
		binary.LittleEndian.PutUint32(out[4+i*4:8+i*4], offsets[i])
	}
	copy(out[headerSize:], payload)
	return out
}

func TestRLEdecodeMono8(t *testing.T) {
	in := buildRLEStream(t, [][]byte{{1, 2, 3, 4}})
	out := make([]byte, 4)

	err := RLEdecode(in, out, uint32(len(in)), 4, "MONOCHROME2")
	if err != nil {
		t.Fatalf("RLEdecode failed: %v", err)
	}

	want := []byte{1, 2, 3, 4}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d]=%d want=%d", i, out[i], want[i])
		}
	}
}

func TestRLEdecodeMono16TwoSegments(t *testing.T) {
	in := buildRLEStream(t, [][]byte{{0x34, 0x78}, {0x12, 0x56}})
	out := make([]byte, 4)

	err := RLEdecode(in, out, uint32(len(in)), 4, "MONOCHROME2")
	if err != nil {
		t.Fatalf("RLEdecode failed: %v", err)
	}

	want := []byte{0x12, 0x34, 0x56, 0x78}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d]=%d want=%d", i, out[i], want[i])
		}
	}
}

func TestRLEdecodeRGB(t *testing.T) {
	in := buildRLEStream(t, [][]byte{{10, 20}, {30, 40}, {50, 60}})
	out := make([]byte, 6)

	err := RLEdecode(in, out, uint32(len(in)), 6, "RGB")
	if err != nil {
		t.Fatalf("RLEdecode failed: %v", err)
	}

	want := []byte{10, 30, 50, 20, 40, 60}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d]=%d want=%d", i, out[i], want[i])
		}
	}
}

// TestRLEdecodePaletteColor: PALETTE COLOR is single-sample, so a one-segment
// stream decodes as a plain plane. This previously fell to the reassembly
// switch's default and errored with "format not supported", even though PALETTE
// COLOR RLE is legal DICOM and common in ultrasound and secondary capture.
func TestRLEdecodePaletteColor(t *testing.T) {
	in := buildRLEStream(t, [][]byte{{1, 2, 3, 4}})
	out := make([]byte, 4)

	if err := RLEdecode(in, out, uint32(len(in)), 4, "PALETTE COLOR"); err != nil {
		t.Fatalf("PALETTE COLOR is single-sample and should decode: %v", err)
	}
	for i, want := range []byte{1, 2, 3, 4} {
		if out[i] != want {
			t.Fatalf("decoded %v, want [1 2 3 4]", out)
		}
	}
}

// TestRLEdecodeSegmentCountMismatch: a segment count that is not a multiple of
// the photometric interpretation's sample count is malformed.
func TestRLEdecodeSegmentCountMismatch(t *testing.T) {
	// Two segments for a 3-sample photometric interpretation.
	in := buildRLEStream(t, [][]byte{{1, 2}, {3, 4}})
	out := make([]byte, 4)

	if err := RLEdecode(in, out, uint32(len(in)), 4, "RGB"); err == nil {
		t.Fatal("expected an error for 2 segments with 3 samples per pixel")
	}
}

func TestRLEdecodeInvalidSegmentOffset(t *testing.T) {
	in := buildRLEStream(t, [][]byte{{1, 2, 3, 4}})
	binary.LittleEndian.PutUint32(in[4:8], uint32(len(in)+10))
	out := make([]byte, 4)

	err := RLEdecode(in, out, uint32(len(in)), 4, "MONOCHROME2")
	if err == nil {
		t.Fatal("expected overflow error for invalid segment offset")
	}
}

func TestRLEencodeDecodeRoundTripMono8(t *testing.T) {
	in := []byte{1, 2, 3, 4, 5, 6}
	enc, err := RLEencode(in, 2, 3, 8, 1)
	if err != nil {
		t.Fatalf("RLEencode failed: %v", err)
	}

	out := make([]byte, len(in))
	if err := RLEdecode(enc, out, uint32(len(enc)), uint32(len(out)), "MONOCHROME2"); err != nil {
		t.Fatalf("RLEdecode failed: %v", err)
	}

	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("out[%d]=%d want=%d", i, out[i], in[i])
		}
	}
}

func TestRLEencodeDecodeRoundTripRGB8(t *testing.T) {
	// 2 pixels: [R,G,B, R,G,B]
	in := []byte{10, 20, 30, 40, 50, 60}
	enc, err := RLEencode(in, 1, 2, 8, 3)
	if err != nil {
		t.Fatalf("RLEencode failed: %v", err)
	}

	out := make([]byte, len(in))
	if err := RLEdecode(enc, out, uint32(len(enc)), uint32(len(out)), "RGB"); err != nil {
		t.Fatalf("RLEdecode failed: %v", err)
	}

	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("out[%d]=%d want=%d", i, out[i], in[i])
		}
	}
}
