package jpegls

import (
	"encoding/binary"
	"os"
	"testing"
)

// synthFrame builds a 16-bit grayscale frame with smooth gradients plus noise,
// a workload representative of medical pixel data (mix of run and regular mode).
func synthFrame(w, h int) []byte {
	out := make([]byte, w*h*2)
	seed := uint32(12345)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			seed = seed*1664525 + 1013904223
			v := uint16((x*7+y*13)&0x0FFF) ^ uint16(seed>>20&0x3F)
			i := (y*w + x) * 2
			binary.LittleEndian.PutUint16(out[i:], v)
		}
	}
	return out
}

func benchDecode(b *testing.B, stream []byte, outLen int) {
	out := make([]byte, outLen)
	b.SetBytes(int64(outLen))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := decodeJLSInto(stream, out); err != nil {
			b.Fatal(err)
		}
	}
}

func benchSynth(b *testing.B, w, h int) {
	raw := synthFrame(w, h)
	stream, err := encodeJLS(raw, w, h, 1, 16, 0)
	if err != nil {
		b.Fatal(err)
	}
	benchDecode(b, stream, len(raw))
}

func BenchmarkDecodeSynth512(b *testing.B)  { benchSynth(b, 512, 512) }
func BenchmarkDecodeSynth2048(b *testing.B) { benchSynth(b, 2048, 2048) }

// BenchmarkDecodeFixture decodes a real lossless JPEG-LS CT frame (512x512, 16-bit).
func BenchmarkDecodeFixture(b *testing.B) {
	dcm, err := os.ReadFile("../../testdata/cornerstone-CTImage-jpegls-lossless.dcm")
	if err != nil {
		b.Skip(err)
	}
	frame := firstEncapsulatedFrame(b, dcm)
	if frame == nil {
		b.Skip("no frame")
	}
	benchDecode(b, frame, 512*512*2)
}

// firstEncapsulatedFrame extracts the first pixel-data fragment from an
// encapsulated DICOM object (skipping the basic offset table item).
func firstEncapsulatedFrame(b *testing.B, dcm []byte) []byte {
	b.Helper()
	pix := []byte{0xE0, 0x7F, 0x10, 0x00}
	idx := -1
	for i := 0; i < len(dcm)-12; i++ {
		if dcm[i] == pix[0] && dcm[i+1] == pix[1] && dcm[i+2] == pix[2] && dcm[i+3] == pix[3] {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
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
		f := dcm[pos : pos+int(l)]
		pos += int(l)
		return f
	}
	read() // basic offset table
	return read()
}
