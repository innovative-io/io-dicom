//go:build openjph && cgo

package jpip

/*
#cgo pkg-config: openjph
#cgo CXXFLAGS: -std=c++14

#include <stdint.h>
#include <stdlib.h>

int io_openjph_encode(const uint8_t* src, size_t src_size,
		uint16_t width, uint16_t height, uint16_t samples, uint16_t bitsa,
		uint8_t** out_data, size_t* out_size, char* err, size_t err_len);
int io_openjph_decode(const uint8_t* src, size_t src_size,
		uint8_t* dst, size_t dst_size, char* err, size_t err_len);
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"unsafe"

	"github.com/innovative-io/io-dicom/codecs/internal/nativeenv"
)

const nativeBackendEnabled = true

func init() {
	nativeenv.Init()
}

var (
	errOpenJPHInvalidRawSize    = errors.New("invalid jpip raw payload size for dimensions")
	errOpenJPHUnsupportedLayout = errors.New("unsupported jpip pixel layout for openjph")
)

type openjphBackend struct{}

func (openjphBackend) Name() string {
	return "openjph"
}

func (openjphBackend) Ready() error {
	return nil
}

func (openjphBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (openjphBackend) Decode(encoded []byte, output []byte, _ string) error {
	return openjphBackend{}.DecodeContext(context.Background(), encoded, output, "")
}

func (openjphBackend) DecodeContext(ctx context.Context, encoded []byte, output []byte, _ string) error {
	_ = ctx
	if len(encoded) == 0 || len(output) == 0 {
		return errors.New("invalid JPIP payload size")
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errors.New("invalid JPIP payload size")
	}
	errBuf := make([]C.char, 256)
	rc := C.io_openjph_decode(
		(*C.uint8_t)(unsafe.Pointer(&encoded[0])),
		C.size_t(len(encoded)),
		(*C.uint8_t)(unsafe.Pointer(&output[0])),
		C.size_t(len(output)),
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
	)
	if rc != 0 {
		if msg := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))); msg != "" {
			if msg == "decoded payload does not fit output buffer" {
				return errors.New("invalid JPIP payload size")
			}
			return fmt.Errorf("openjph decode: %s", msg)
		}
		return errors.New("openjph decode failed")
	}
	return nil
}

func (openjphBackend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error) {
	return openjphBackend{}.EncodeContext(context.Background(), raw, width, height, samples, bitsa, transferSyntaxUID)
}

func (openjphBackend) EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, _ string) ([]byte, error) {
	_ = ctx
	if len(raw) == 0 {
		return nil, errors.New("invalid JPIP payload size")
	}
	if len(raw) > maxCodecPayloadBytes {
		return nil, errors.New("invalid JPIP payload size")
	}
	if width == 0 || height == 0 {
		return nil, errors.New("invalid JPIP payload size")
	}
	if samples != 1 && samples != 3 {
		return nil, errOpenJPHUnsupportedLayout
	}
	if bitsa != 8 && bitsa != 12 && bitsa != 16 {
		return nil, errOpenJPHUnsupportedLayout
	}
	expected := int(width) * int(height) * int(samples)
	if bitsa > 8 {
		expected *= 2
	}
	if len(raw) != expected {
		return nil, errOpenJPHInvalidRawSize
	}

	errBuf := make([]C.char, 256)
	var outPtr *C.uint8_t
	var outSize C.size_t
	rc := C.io_openjph_encode(
		(*C.uint8_t)(unsafe.Pointer(&raw[0])),
		C.size_t(len(raw)),
		C.uint16_t(width),
		C.uint16_t(height),
		C.uint16_t(samples),
		C.uint16_t(bitsa),
		&outPtr,
		&outSize,
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
	)
	if rc != 0 {
		if msg := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))); msg != "" {
			switch msg {
			case "raw payload size does not match image dimensions":
				return nil, errOpenJPHInvalidRawSize
			case "unsupported OpenJPH image layout":
				return nil, errOpenJPHUnsupportedLayout
			}
			return nil, fmt.Errorf("openjph encode: %s", msg)
		}
		return nil, errors.New("openjph encode failed")
	}
	defer C.free(unsafe.Pointer(outPtr))

	encoded := C.GoBytes(unsafe.Pointer(outPtr), C.int(outSize))
	if len(encoded) == 0 {
		return nil, errors.New("openjph encode produced empty payload")
	}
	return encoded, nil
}

func openjphDecompositionCount(width uint16, height uint16) int {
	minDim := int(width)
	if int(height) < minDim {
		minDim = int(height)
	}
	if minDim <= 1 {
		return 0
	}
	decompositions := bits.Len(uint(minDim)) - 1
	if decompositions > 5 {
		return 5
	}
	return decompositions
}

func registerNativeBackends() {
	_ = RegisterBackend("openjph", func() Backend { return openjphBackend{} })
}
