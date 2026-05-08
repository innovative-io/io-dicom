package jpeg2000

import (
	"encoding/binary"
	"os"
	"testing"
)

func loadBytesFromFile(fileName string, t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(fileName)
	if err != nil {
		t.Skipf("sample fixture unavailable (%s): %v", fileName, err)
	}
	return data
}

// extractFirstDICOMEncapsulatedFrame returns the raw payload of the first item
// in a DICOM encapsulated pixel-data sequence (skips the BOT item if present).
// This is sufficient for single-frame test files.
func extractFirstDICOMEncapsulatedFrame(t *testing.T, dcmData []byte) []byte {
	t.Helper()
	// Search for the pixel-data tag (7FE0,0010) with OB/OW, undefined length.
	pixelTag := []byte{0xE0, 0x7F, 0x10, 0x00}
	idx := -1
	for i := 0; i < len(dcmData)-12; i++ {
		if dcmData[i] == pixelTag[0] && dcmData[i+1] == pixelTag[1] &&
			dcmData[i+2] == pixelTag[2] && dcmData[i+3] == pixelTag[3] {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("pixel data tag not found in DICOM file")
	}
	// Skip tag(4) + VR "OB"(2) + reserved(2) + length 0xFFFFFFFF(4) = 12 bytes
	pos := idx + 12

	// Item tag: FFFE,E000
	itemTag := []byte{0xFE, 0xFF, 0x00, 0xE0}
	readItem := func() []byte {
		if pos+8 > len(dcmData) {
			return nil
		}
		tag := dcmData[pos : pos+4]
		length := binary.LittleEndian.Uint32(dcmData[pos+4 : pos+8])
		pos += 8
		if tag[0] != itemTag[0] || tag[1] != itemTag[1] || tag[2] != itemTag[2] || tag[3] != itemTag[3] {
			return nil
		}
		if length == 0xFFFFFFFF || pos+int(length) > len(dcmData) {
			return nil
		}
		item := dcmData[pos : pos+int(length)]
		pos += int(length)
		return item
	}

	// First item is the Basic Offset Table (may be empty)
	bot := readItem()
	if bot == nil {
		t.Fatal("could not read BOT item from pixel data sequence")
	}
	// If BOT is non-empty, next item is the frame; if BOT is empty, next is also the frame.
	frame := readItem()
	if frame == nil || len(frame) == 0 {
		t.Fatal("could not read frame item from pixel data sequence")
	}
	return frame
}

func TestJ2KdecodeSample(t *testing.T) {
	if !CGOEnabled {
		t.Skip("J2Kdecode sample requires the openjpeg native backend")
	}
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })
	if err := UseBackend("openjpeg"); err != nil {
		t.Fatalf("expected openjpeg backend to be registered: %v", err)
	}
	jpegData := loadBytesFromFile("../../samples/test.j2k", t)
	outSize := 1576 * 1134 * 3
	outData := make([]byte, outSize)

	if err := J2Kdecode(jpegData, uint32(len(jpegData)), outData); err != nil {
		t.Fatalf("J2Kdecode() error = %v", err)
	}
}

// TestJ2KdecodeSampleSignedCT exercises decoding of a JPEG 2000 Lossless frame
// that contains signed 16-bit pixel data (CT Hounsfield values). This was
// previously rejected with "unsupported decoded image layout".
func TestJ2KdecodeSampleSignedCT(t *testing.T) {
	if !CGOEnabled {
		t.Skip("J2Kdecode signed CT sample requires the openjpeg native backend")
	}
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })
	if err := UseBackend("openjpeg"); err != nil {
		t.Fatalf("expected openjpeg backend to be registered: %v", err)
	}

	dcmData := loadBytesFromFile("../../samples/cornerstone-CTImage-jpeg2000-lossless.dcm", t)
	j2kFrame := extractFirstDICOMEncapsulatedFrame(t, dcmData)

	// 512x512, 1 sample, 16-bit (2 bytes/sample)
	const width, height, samples, bytesPerSample = 512, 512, 1, 2
	outData := make([]byte, width*height*samples*bytesPerSample)

	if err := J2Kdecode(j2kFrame, uint32(len(j2kFrame)), outData); err != nil {
		t.Fatalf("J2Kdecode signed CT: %v", err)
	}

	// Sanity check: at least one non-zero sample should exist in a CT image.
	allZero := true
	for _, b := range outData {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("decoded CT image is all zeros; likely a decode bug")
	}
}

func TestJ2KencodeSample(t *testing.T) {
	if CGOEnabled {
		SetBackend(nil)
		t.Cleanup(func() { SetBackend(nil) })
		if err := UseBackend("openjpeg"); err != nil {
			t.Fatalf("expected openjpeg backend to be registered: %v", err)
		}
	}
	rawData := loadBytesFromFile("../../samples/test.raw", t)
	var jpegData []byte
	var jpegSize int

	if err := J2Kencode(rawData, 1576, 1134, 3, 8, &jpegData, &jpegSize, 10); err != nil {
		t.Fatalf("J2Kencode() error = %v", err)
	}
}

