package openjpeg

// #cgo CFLAGS: -I j2klib/include -I j2klib/linux_amd64
// #cgo LDFLAGS: -L j2klib/linux_amd64 -lopenjpeg -lm
// #include "j2klib/decomj2k.c"
// #include "j2klib/comj2k.c"
import "C"
import (
	"errors"
	"unsafe"
)

// J2Kdecode - J2K File to RAW
func J2Kdecode(j2kData []byte, j2kSize uint32, outputData []byte) error {
	if C.J2KDecode((*C.char)(unsafe.Pointer(&j2kData[0])), C.int(j2kSize), (*C.char)(unsafe.Pointer(&outputData[0]))) {
		return nil
	}
	return errors.New("ERROR, J2Kdecode, JPEG failed")
}

// J2Kencode - RAW File to J2K
func J2Kencode(rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, ratio int) error {
	var j2kData *C.char
	var j2kSize C.int
	if C.J2KEncode((*C.char)(unsafe.Pointer(&rawData[0])), C.int(width), C.int(height), C.int(samples), C.int(bitsa), &j2kData, &j2kSize, C.int(ratio)) {
		if j2kSize > 0 {
			*outData = C.GoBytes(unsafe.Pointer(j2kData), j2kSize)
			*outSize = int(j2kSize)
			C.free(unsafe.Pointer(j2kData))
			return nil
		}
	}
	return errors.New("ERROR, J2KEncode, JPEG failed")
}
