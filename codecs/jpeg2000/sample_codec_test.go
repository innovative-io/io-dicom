package jpeg2000

import (
	"os"
	"testing"
)

func loadBytesFromFile(fileName string, t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("failed to read %s: %v", fileName, err)
	}
	return data
}

func TestJ2KdecodeSample(t *testing.T) {
	jpegData := loadBytesFromFile("../../samples/test.j2k", t)
	outSize := 1576 * 1134 * 3
	outData := make([]byte, outSize)

	if err := J2Kdecode(jpegData, uint32(len(jpegData)), outData); err != nil {
		t.Fatalf("J2Kdecode() error = %v", err)
	}
}

func TestJ2KencodeSample(t *testing.T) {
	rawData := loadBytesFromFile("../../samples/test.raw", t)
	var jpegData []byte
	var jpegSize int

	if err := J2Kencode(rawData, 1576, 1134, 3, 8, &jpegData, &jpegSize, 10); err != nil {
		t.Fatalf("J2Kencode() error = %v", err)
	}
}
