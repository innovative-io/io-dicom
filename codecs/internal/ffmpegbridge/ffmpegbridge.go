//go:build cgo && (ffmpeg || st2110)

package ffmpegbridge

/*
#cgo pkg-config: libavcodec libavformat libavutil libswscale

#include <libavcodec/avcodec.h>
#include <stdint.h>
#include <stdlib.h>

enum {
	io_codec_mpeg2video = AV_CODEC_ID_MPEG2VIDEO,
	io_codec_mpeg4 = AV_CODEC_ID_MPEG4,
	io_codec_hevc = AV_CODEC_ID_HEVC,
	io_codec_ffv1 = AV_CODEC_ID_FFV1,
};

int io_ffmpeg_encode(const uint8_t* src, size_t src_size,
		int width, int height, int samples, int bits,
		int codec_id, const char* format_name,
		uint8_t** out_data, size_t* out_size, char* err, size_t err_len);
int io_ffmpeg_decode(const uint8_t* src, size_t src_size,
		uint8_t* dst, size_t dst_size, char* err, size_t err_len);
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type CodecID int

const (
	CodecMPEG2Video CodecID = CodecID(C.io_codec_mpeg2video)
	CodecMPEG4      CodecID = CodecID(C.io_codec_mpeg4)
	CodecHEVC       CodecID = CodecID(C.io_codec_hevc)
	CodecFFV1       CodecID = CodecID(C.io_codec_ffv1)
)

var ErrInvalidRawSize = errors.New("invalid raw payload size for dimensions")
var ErrUnsupportedLayout = errors.New("unsupported raw pixel layout")

func Decode(encoded []byte, output []byte) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errors.New("invalid encoded payload size")
	}
	errBuf := make([]C.char, 256)
	rc := C.io_ffmpeg_decode(
		(*C.uint8_t)(unsafe.Pointer(&encoded[0])),
		C.size_t(len(encoded)),
		(*C.uint8_t)(unsafe.Pointer(&output[0])),
		C.size_t(len(output)),
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
	)
	if rc != 0 {
		return mapError("ffmpeg decode", errBuf)
	}
	return nil
}

func Encode(raw []byte, width uint16, height uint16, samples uint16, bits uint16, codecID CodecID, formatName string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("invalid raw payload size")
	}
	if width == 0 || height == 0 {
		return nil, errors.New("invalid raw payload size")
	}
	cFormat := C.CString(formatName)
	defer C.free(unsafe.Pointer(cFormat))

	errBuf := make([]C.char, 256)
	var outPtr *C.uint8_t
	var outSize C.size_t
	rc := C.io_ffmpeg_encode(
		(*C.uint8_t)(unsafe.Pointer(&raw[0])),
		C.size_t(len(raw)),
		C.int(width),
		C.int(height),
		C.int(samples),
		C.int(bits),
		C.int(codecID),
		cFormat,
		&outPtr,
		&outSize,
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
	)
	if rc != 0 {
		return nil, mapError("ffmpeg encode", errBuf)
	}
	defer C.free(unsafe.Pointer(outPtr))

	encoded := C.GoBytes(unsafe.Pointer(outPtr), C.int(outSize))
	if len(encoded) == 0 {
		return nil, errors.New("ffmpeg encode produced empty payload")
	}
	return encoded, nil
}

func mapError(prefix string, errBuf []C.char) error {
	msg := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0])))
	switch msg {
	case "raw payload size does not match image dimensions", "decoded payload does not match frame dimensions":
		return ErrInvalidRawSize
	case "unsupported raw pixel layout", "unsupported decoded frame layout":
		return ErrUnsupportedLayout
	case "":
		return fmt.Errorf("%s failed", prefix)
	default:
		return fmt.Errorf("%s: %s", prefix, msg)
	}
}
