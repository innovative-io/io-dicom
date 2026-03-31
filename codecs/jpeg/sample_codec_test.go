package jpeg

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

func TestDIJG8decodeSample(t *testing.T) {
	jpegData := loadBytesFromFile("../../samples/test8.jpg", t)
	outSize := 1576 * 1134 * 3
	outData := make([]byte, outSize)

	if err := DIJG8decode(jpegData, uint32(len(jpegData)), outData, uint32(outSize)); err != nil {
		t.Fatalf("DIJG8decode() error = %v", err)
	}
}

func TestEIJG8encodeSample(t *testing.T) {
	rawData := loadBytesFromFile("../../samples/test.raw", t)
	var jpegData []byte
	var jpegSize int

	if err := EIJG8encode(rawData, 1576, 1134, 3, &jpegData, &jpegSize, 4); err != nil {
		t.Fatalf("EIJG8encode() error = %v", err)
	}
}

func TestEIJG16encodeSample(t *testing.T) {
	rawData := loadBytesFromFile("../../samples/test.raw", t)
	var jpegData []byte
	var jpegSize int

	if err := EIJG16encode(rawData, 1576, 1134, 3, &jpegData, &jpegSize, 4); err != nil {
		t.Fatalf("EIJG16encode() error = %v", err)
	}
}
