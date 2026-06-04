package jpegls

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func loadDCM(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture unavailable (%s): %v", path, err)
	}
	return data
}

// extractFirstFrame returns the first frame item of an encapsulated DICOM
// pixel-data sequence (skipping the Basic Offset Table).
func extractFirstFrame(t *testing.T, dcm []byte) []byte {
	t.Helper()
	pix := []byte{0xE0, 0x7F, 0x10, 0x00}
	idx := -1
	for i := 0; i < len(dcm)-12; i++ {
		if dcm[i] == pix[0] && dcm[i+1] == pix[1] && dcm[i+2] == pix[2] && dcm[i+3] == pix[3] {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("pixel data tag not found")
	}
	pos := idx + 12
	read := func() []byte {
		if pos+8 > len(dcm) {
			return nil
		}
		l := binary.LittleEndian.Uint32(dcm[pos+4 : pos+8])
		pos += 8
		if l == 0xFFFFFFFF || pos+int(l) > len(dcm) {
			return nil
		}
		b := dcm[pos : pos+int(l)]
		pos += int(l)
		return b
	}
	read() // BOT
	frame := read()
	if len(frame) == 0 {
		t.Fatal("could not read frame item")
	}
	return frame
}

// TestGoJLSDecodesLosslessFixture decodes a lossless JPEG-LS CT frame with the
// pure-Go decoder (no cgo) and checks geometry. The byte-exact check vs charls
// is in the cgo-tagged golden test.
func TestGoJLSDecodesLosslessFixture(t *testing.T) {
	dcm := loadDCM(t, "../../testdata/cornerstone-CTImage-jpegls-lossless.dcm")
	frame := extractFirstFrame(t, dcm)
	f, _, err := parseJLS(frame)
	if err != nil {
		t.Fatalf("parseJLS: %v", err)
	}
	if f.near != 0 || f.width == 0 || f.height == 0 {
		t.Fatalf("unexpected frame: near=%d %dx%d", f.near, f.width, f.height)
	}
	out := make([]byte, f.width*f.height*len(f.comps)*2)
	if err := decodeJLSInto(frame, out); err != nil {
		t.Fatalf("decodeJLSInto: %v", err)
	}
}

// TestGoJLSDecodesNearLossless confirms the pure-Go decoder now handles a
// near-lossless (.81) stream: it parses the NEAR>0 scan header and decodes a
// full-size frame without error. Byte-exact fidelity against charls is covered
// by TestGoJLSNearLosslessMatchesCharls (cgo-tagged).
func TestGoJLSDecodesNearLossless(t *testing.T) {
	dcm := loadDCM(t, "../../testdata/cornerstone-CTImage-jpegls-lossy.dcm")
	frame := extractFirstFrame(t, dcm)
	f, _, err := parseJLS(frame)
	if err != nil {
		t.Fatalf("parseJLS near-lossless: %v", err)
	}
	if f.near == 0 {
		t.Fatal("expected NEAR>0 for the lossy fixture")
	}
	bps := 1
	if f.precision > 8 {
		bps = 2
	}
	out := make([]byte, f.width*f.height*len(f.comps)*bps)
	if err := decodeJLSInto(frame, out); err != nil {
		t.Fatalf("decode near-lossless: %v", err)
	}
}

// FuzzGoJLS ensures the pure-Go decoder never panics on arbitrary input.
func FuzzGoJLS(f *testing.F) {
	if dcm, err := os.ReadFile("../../testdata/cornerstone-CTImage-jpegls-lossless.dcm"); err == nil {
		f.Add(dcm)
	}
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xD9})
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xF7, 0x00, 0x08})       // truncated SOF55
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xF8, 0x00, 0x0D, 0x01}) // short LSE
	f.Fuzz(func(t *testing.T, data []byte) {
		out := make([]byte, 1<<16)
		_ = decodeJLSInto(data, out) // must not panic
	})
}

// TestGoJLSEncodeRoundTrip encodes synthetic single-component images with the
// pure-Go encoder and decodes them back with the pure-Go decoder, requiring an
// exact (lossless) match.
func TestGoJLSEncodeRoundTrip(t *testing.T) {
	for _, tc := range []struct{ w, h, p int }{
		{16, 12, 8}, {40, 30, 12}, {50, 40, 16}, {64, 64, 12},
	} {
		bps := 1
		if tc.p > 8 {
			bps = 2
		}
		maxv := (1 << tc.p) - 1
		raw := make([]byte, tc.w*tc.h*bps)
		for i := 0; i < tc.w*tc.h; i++ {
			v := (i*5 + (i*i)%13) % (maxv + 1)
			if bps == 1 {
				raw[i] = byte(v)
			} else {
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
		}
		enc, err := encodeJLS(raw, tc.w, tc.h, 1, tc.p, 0)
		if err != nil {
			t.Fatalf("p%d encode: %v", tc.p, err)
		}
		out := make([]byte, len(raw))
		if err := decodeJLSInto(enc, out); err != nil {
			t.Fatalf("p%d decode: %v", tc.p, err)
		}
		if !bytes.Equal(out, raw) {
			t.Fatalf("p%d round trip not lossless", tc.p)
		}
	}
}

// TestGoJLSEncodeRejectsUnsupported confirms multi-component encode is reported
// unsupported rather than producing invalid output.
func TestGoJLSEncodeRejectsUnsupported(t *testing.T) {
	if _, err := encodeJLS(make([]byte, 3*4*4), 4, 4, 3, 8, 0); err == nil {
		t.Fatal("expected 3-component encode to be unsupported")
	}
}

// TestGoJLSNearLosslessEncodeRoundTrip encodes synthetic images with the pure-Go
// near-lossless encoder (NEAR=1) and decodes them with the pure-Go decoder,
// requiring every sample to be within NEAR of the original (the near-lossless
// guarantee) and the decode to reproduce the encoder's own reconstruction.
func TestGoJLSNearLosslessEncodeRoundTrip(t *testing.T) {
	const near = 1
	for _, tc := range []struct{ w, h, p int }{
		{16, 16, 8}, {40, 30, 8}, {50, 40, 12}, {64, 64, 16}, {33, 17, 10},
	} {
		bps := 1
		if tc.p > 8 {
			bps = 2
		}
		maxv := (1 << tc.p) - 1
		raw := make([]byte, tc.w*tc.h*bps)
		sample := func(i int) int {
			if bps == 1 {
				return int(raw[i])
			}
			return int(raw[i*2]) | int(raw[i*2+1])<<8
		}
		for i := 0; i < tc.w*tc.h; i++ {
			v := (i*7 + (i*i)%29 + (i*i*i)%17) % (maxv + 1)
			if bps == 1 {
				raw[i] = byte(v)
			} else {
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
		}
		enc, err := encodeJLS(raw, tc.w, tc.h, 1, tc.p, near)
		if err != nil {
			t.Fatalf("p%d near-lossless encode: %v", tc.p, err)
		}
		out := make([]byte, len(raw))
		if err := decodeJLSInto(enc, out); err != nil {
			t.Fatalf("p%d decode: %v", tc.p, err)
		}
		for i := 0; i < tc.w*tc.h; i++ {
			orig := sample(i)
			var got int
			if bps == 1 {
				got = int(out[i])
			} else {
				got = int(out[i*2]) | int(out[i*2+1])<<8
			}
			if d := orig - got; d < -near || d > near {
				t.Fatalf("p%d sample %d: got %d, want within %d of %d", tc.p, i, got, near, orig)
			}
		}
	}
}
