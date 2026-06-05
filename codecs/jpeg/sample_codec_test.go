package jpeg

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

// extractFirstDICOMEncapsulatedFrame returns the raw payload of the first frame
// item in a DICOM encapsulated pixel-data sequence (skipping the Basic Offset
// Table item). Sufficient for the single-frame test fixtures.
func extractFirstDICOMEncapsulatedFrame(t *testing.T, dcmData []byte) []byte {
	t.Helper()
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
	pos := idx + 12 // tag(4) + VR "OB"(2) + reserved(2) + length 0xFFFFFFFF(4)

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

	if bot := readItem(); bot == nil {
		t.Fatal("could not read BOT item from pixel data sequence")
	}
	frame := readItem()
	if len(frame) == 0 {
		t.Fatal("could not read frame item from pixel data sequence")
	}
	return frame
}

func TestDIJG8decodeSample(t *testing.T) {
	jpegData := loadBytesFromFile("../../testdata/test8.jpg", t)
	outSize := 1576 * 1134 * 3
	outData := make([]byte, outSize)

	if err := DIJG8decode(jpegData, uint32(len(jpegData)), outData, uint32(outSize)); err != nil {
		t.Fatalf("DIJG8decode() error = %v", err)
	}
}

func TestEIJG8encodeSample(t *testing.T) {
	rawData := loadBytesFromFile("../../testdata/test.raw", t)
	var jpegData []byte
	var jpegSize int

	if err := EIJG8encode(rawData, 1576, 1134, 3, &jpegData, &jpegSize, 4); err != nil {
		t.Fatalf("EIJG8encode() error = %v", err)
	}
}
