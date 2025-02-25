package jpeglib

// #cgo CFLAGS: -I dcmjpeg/libijg8 -I dcmjpeg/linux_arm64
// #cgo LDFLAGS: -L dcmjpeg/linux_arm64 -lijg8
// #include "dcmjpeg/dijg8.c"
// #include "dcmjpeg/eijg8.c"
import  "C"
import (
	"errors"
	"unsafe"
)

// DIJG8decode - JPEG File to RAW
func DIJG8decode(jpegData []byte, jpegSize uint32, outputData []byte, outputSize uint32) error {
	if C.decode8((*C.uchar)(unsafe.Pointer(&jpegData[0])), C.int(jpegSize), (*C.uchar)(unsafe.Pointer(&outputData[0])), C.int(outputSize)) == 1 {
		return nil
	}
	return errors.New("ERROR, Decode8, JPEG failed")
}

// EIJG8encode - RAW File to JPEG
func EIJG8encode(rawData []byte, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	var jpegData *C.uchar
	var jpegSize C.int
	if C.encode8((*C.uchar)(unsafe.Pointer(&rawData[0])), C.ushort(width), C.ushort(height), C.ushort(samples), &jpegData, &jpegSize, C.int(mode)) == 1 {
		if jpegSize > 0 {
			*outData = C.GoBytes(unsafe.Pointer(jpegData), jpegSize)
			*outSize = int(jpegSize)
			C.free(unsafe.Pointer(jpegData))
			return nil
		}
	}
	return errors.New("ERROR, Encode8, JPEG failed")
}
