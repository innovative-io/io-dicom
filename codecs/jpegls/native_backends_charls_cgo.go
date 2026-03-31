//go:build charls && cgo

package jpegls

/*
#cgo pkg-config: charls

#include <stdint.h>
#include <stddef.h>
#include <charls/charls.h>

static int io_charls_decode(const uint8_t* src, size_t src_size, uint8_t* dst, size_t dst_size) {
	charls_jpegls_decoder* decoder = charls_jpegls_decoder_create();
	if (!decoder)
		return -100;

	charls_jpegls_errc ec = charls_jpegls_decoder_set_source_buffer(decoder, src, src_size);
	if (ec != 0) {
		charls_jpegls_decoder_destroy(decoder);
		return (int)ec;
	}

	ec = charls_jpegls_decoder_read_header(decoder);
	if (ec != 0) {
		charls_jpegls_decoder_destroy(decoder);
		return (int)ec;
	}

	ec = charls_jpegls_decoder_decode_to_buffer(decoder, dst, dst_size, 0);
	if (ec != 0) {
		charls_jpegls_decoder_destroy(decoder);
		return (int)ec;
	}

	charls_jpegls_decoder_destroy(decoder);
	return 0;
}

static int io_charls_encode(const uint8_t* src, size_t src_size,
							uint16_t width, uint16_t height, uint16_t components, uint16_t bits_per_sample,
							int32_t near_lossless,
							uint8_t* dst, size_t dst_capacity,
							size_t* bytes_written) {
	charls_jpegls_encoder* encoder = charls_jpegls_encoder_create();
	if (!encoder)
		return -100;

	charls_frame_info frame_info;
	frame_info.width = width;
	frame_info.height = height;
	frame_info.component_count = components;
	frame_info.bits_per_sample = bits_per_sample;

	charls_jpegls_errc ec = charls_jpegls_encoder_set_frame_info(encoder, &frame_info);
	if (ec != 0) {
		charls_jpegls_encoder_destroy(encoder);
		return (int)ec;
	}

	ec = charls_jpegls_encoder_set_near_lossless(encoder, near_lossless);
	if (ec != 0) {
		charls_jpegls_encoder_destroy(encoder);
		return (int)ec;
	}

	ec = charls_jpegls_encoder_set_interleave_mode(encoder,
		 components > 1 ? CHARLS_INTERLEAVE_MODE_LINE : CHARLS_INTERLEAVE_MODE_NONE);
	if (ec != 0) {
		charls_jpegls_encoder_destroy(encoder);
		return (int)ec;
	}

	ec = charls_jpegls_encoder_set_destination_buffer(encoder, dst, dst_capacity);
	if (ec != 0) {
		charls_jpegls_encoder_destroy(encoder);
		return (int)ec;
	}

	ec = charls_jpegls_encoder_encode_from_buffer(encoder, src, src_size, 0);
	if (ec != 0) {
		charls_jpegls_encoder_destroy(encoder);
		return (int)ec;
	}

	ec = charls_jpegls_encoder_get_bytes_written(encoder, bytes_written);
	if (ec != 0) {
		charls_jpegls_encoder_destroy(encoder);
		return (int)ec;
	}

	charls_jpegls_encoder_destroy(encoder);
	return 0;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/innovative-io/io-dicom/codecs/internal/nativeenv"
)

const nativeBackendEnabled = true

func init() {
	nativeenv.Init()
}

var errCharLSInputSize = errors.New("invalid charls input size")

type charlsBackend struct{}

func (charlsBackend) Name() string {
	return "charls"
}

func (charlsBackend) Ready() error {
	return nil
}

func (charlsBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (charlsBackend) Decode(encoded []byte, output []byte) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errCharLSInputSize
	}
	rc := C.io_charls_decode(
		(*C.uint8_t)(unsafe.Pointer(&encoded[0])),
		C.size_t(len(encoded)),
		(*C.uint8_t)(unsafe.Pointer(&output[0])),
		C.size_t(len(output)),
	)
	if rc != 0 {
		return fmt.Errorf("charls decode failed: %d", int(rc))
	}
	return nil
}

func (charlsBackend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, nearLossless bool) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errCharLSInputSize
	}

	// Keep destination sizing conservative to avoid spurious encoder failures on wide images.
	capEstimate := len(raw)*2 + int(height)*4 + 4096
	out := make([]byte, capEstimate)

	near := C.int32_t(0)
	if nearLossless {
		near = 1
	}

	var written C.size_t
	rc := C.io_charls_encode(
		(*C.uint8_t)(unsafe.Pointer(&raw[0])),
		C.size_t(len(raw)),
		C.uint16_t(width),
		C.uint16_t(height),
		C.uint16_t(samples),
		C.uint16_t(bitsa),
		near,
		(*C.uint8_t)(unsafe.Pointer(&out[0])),
		C.size_t(len(out)),
		&written,
	)
	if rc != 0 {
		return nil, fmt.Errorf("charls encode failed: %d", int(rc))
	}
	return out[:int(written)], nil
}

func registerNativeBackends() {
	_ = RegisterBackend("charls", func() Backend { return charlsBackend{} })
}
