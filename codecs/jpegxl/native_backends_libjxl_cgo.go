//go:build libjxl && cgo

package jpegxl

/*
#cgo pkg-config: libjxl libjxl_threads

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <jxl/codestream_header.h>
#include <jxl/color_encoding.h>
#include <jxl/decode.h>
#include <jxl/encode.h>
#include <jxl/thread_parallel_runner.h>
#include <jxl/types.h>

static void io_libjxl_set_error(char* err, size_t err_len, const char* msg) {
	if (!err || err_len == 0) {
		return;
	}
	if (!msg) {
		err[0] = '\0';
		return;
	}
	snprintf(err, err_len, "%s", msg);
}

static void io_libjxl_set_color_encoding(JxlColorEncoding* color, uint16_t samples) {
	memset(color, 0, sizeof(*color));
	color->color_space = samples == 1 ? JXL_COLOR_SPACE_GRAY : JXL_COLOR_SPACE_RGB;
	color->white_point = JXL_WHITE_POINT_D65;
	color->white_point_xy[0] = 0.3127;
	color->white_point_xy[1] = 0.3290;
	color->primaries = JXL_PRIMARIES_SRGB;
	color->primaries_red_xy[0] = 0.639998686;
	color->primaries_red_xy[1] = 0.330010138;
	color->primaries_green_xy[0] = 0.300003784;
	color->primaries_green_xy[1] = 0.600003357;
	color->primaries_blue_xy[0] = 0.150002046;
	color->primaries_blue_xy[1] = 0.059997204;
	color->transfer_function = JXL_TRANSFER_FUNCTION_LINEAR;
	color->gamma = 1.0;
	color->rendering_intent = JXL_RENDERING_INTENT_RELATIVE;
}

static int io_libjxl_configure_basic_info(JxlBasicInfo* info, uint16_t width, uint16_t height, uint16_t samples, uint16_t bitsa) {
	if (!info || width == 0 || height == 0 || (samples != 1 && samples != 3)) {
		return 0;
	}
	JxlEncoderInitBasicInfo(info);
	info->xsize = width;
	info->ysize = height;
	info->bits_per_sample = bitsa;
	info->exponent_bits_per_sample = 0;
	info->num_color_channels = samples;
	info->num_extra_channels = 0;
	info->alpha_bits = 0;
	info->alpha_exponent_bits = 0;
	info->alpha_premultiplied = JXL_FALSE;
	info->orientation = JXL_ORIENT_IDENTITY;
	info->uses_original_profile = JXL_TRUE;
	return 1;
}

static int io_libjxl_encode_impl(const uint8_t* src, size_t src_size,
		uint16_t width, uint16_t height, uint16_t samples, uint16_t bitsa, int lossless,
		uint8_t** out_data, size_t* out_size, char* err, size_t err_len, void* runner, int effort) {
	JxlEncoder* enc = NULL;
	JxlEncoderFrameSettings* frame_settings = NULL;
	uint8_t* encoded = NULL;
	size_t encoded_capacity = 0;
	JxlBasicInfo info;
	JxlColorEncoding color;
	JxlPixelFormat format;
	JxlBitDepth bit_depth;
	size_t expected;

	if (!src || !out_data || !out_size || !(bitsa == 8 || bitsa == 12 || bitsa == 16)) {
		io_libjxl_set_error(err, err_len, "invalid libjxl encode parameters");
		return -1;
	}
	expected = (size_t)width * (size_t)height * (size_t)samples * (bitsa > 8 ? 2u : 1u);
	if (expected == 0 || expected != src_size) {
		io_libjxl_set_error(err, err_len, "raw payload size does not match image dimensions");
		return -1;
	}

	enc = JxlEncoderCreate(NULL);
	if (!enc) {
		io_libjxl_set_error(err, err_len, "JxlEncoderCreate failed");
		return -1;
	}
	// A NULL runner means pool creation failed; fall back to single-threaded.
	if (runner != NULL &&
		JxlEncoderSetParallelRunner(enc, JxlThreadParallelRunner, runner) != JXL_ENC_SUCCESS) {
		io_libjxl_set_error(err, err_len, "JxlEncoderSetParallelRunner failed");
		JxlEncoderDestroy(enc);
		return -1;
	}
	if (!io_libjxl_configure_basic_info(&info, width, height, samples, bitsa)) {
		io_libjxl_set_error(err, err_len, "unsupported libjxl image layout");
		JxlEncoderDestroy(enc);
		return -1;
	}
	if (JxlEncoderSetBasicInfo(enc, &info) != JXL_ENC_SUCCESS) {
		io_libjxl_set_error(err, err_len, "JxlEncoderSetBasicInfo failed");
		JxlEncoderDestroy(enc);
		return -1;
	}

	io_libjxl_set_color_encoding(&color, samples);
	if (JxlEncoderSetColorEncoding(enc, &color) != JXL_ENC_SUCCESS) {
		io_libjxl_set_error(err, err_len, "JxlEncoderSetColorEncoding failed");
		JxlEncoderDestroy(enc);
		return -1;
	}

	frame_settings = JxlEncoderFrameSettingsCreate(enc, NULL);
	if (!frame_settings) {
		io_libjxl_set_error(err, err_len, "JxlEncoderFrameSettingsCreate failed");
		JxlEncoderDestroy(enc);
		return -1;
	}
	if (JxlEncoderFrameSettingsSetOption(frame_settings, JXL_ENC_FRAME_SETTING_EFFORT, (int64_t)effort) != JXL_ENC_SUCCESS) {
		io_libjxl_set_error(err, err_len, "JxlEncoderFrameSettingsSetOption(effort) failed");
		JxlEncoderDestroy(enc);
		return -1;
	}
	if (lossless) {
		if (JxlEncoderSetFrameLossless(frame_settings, JXL_TRUE) != JXL_ENC_SUCCESS) {
			io_libjxl_set_error(err, err_len, "JxlEncoderSetFrameLossless failed");
			JxlEncoderDestroy(enc);
			return -1;
		}
	} else if (JxlEncoderSetFrameDistance(frame_settings, 1.0f) != JXL_ENC_SUCCESS) {
		io_libjxl_set_error(err, err_len, "JxlEncoderSetFrameDistance failed");
		JxlEncoderDestroy(enc);
		return -1;
	}

	memset(&format, 0, sizeof(format));
	format.num_channels = samples;
	format.data_type = bitsa > 8 ? JXL_TYPE_UINT16 : JXL_TYPE_UINT8;
	// Little-endian matches the DICOM uncompressed convention and the other
	// 16-bit codecs (libjpeg, charls, ffmpeg, openjpeg).
	format.endianness = bitsa > 8 ? JXL_LITTLE_ENDIAN : JXL_NATIVE_ENDIAN;
	format.align = 0;

	if (bitsa != 8) {
		memset(&bit_depth, 0, sizeof(bit_depth));
		bit_depth.type = JXL_BIT_DEPTH_FROM_CODESTREAM;
		bit_depth.bits_per_sample = bitsa;
		bit_depth.exponent_bits_per_sample = 0;
		if (JxlEncoderSetFrameBitDepth(frame_settings, &bit_depth) != JXL_ENC_SUCCESS) {
			io_libjxl_set_error(err, err_len, "JxlEncoderSetFrameBitDepth failed");
			JxlEncoderDestroy(enc);
			return -1;
		}
	}

	if (JxlEncoderAddImageFrame(frame_settings, &format, src, src_size) != JXL_ENC_SUCCESS) {
		JxlEncoderError encoder_error = JxlEncoderGetError(enc);
		char buffer[128];
		snprintf(buffer, sizeof(buffer), "JxlEncoderAddImageFrame failed (%d)", (int)encoder_error);
		io_libjxl_set_error(err, err_len, buffer);
		JxlEncoderDestroy(enc);
		return -1;
	}

	JxlEncoderCloseInput(enc);
	// The output loop below uses *out_size as the running write offset into
	// `encoded`; reset it so we never index past the buffer if a caller passed
	// a non-zero or uninitialized value.
	*out_size = 0;
	encoded_capacity = 4096;
	encoded = (uint8_t*)malloc(encoded_capacity);
	if (!encoded) {
		io_libjxl_set_error(err, err_len, "failed to allocate libjxl output buffer");
		JxlEncoderDestroy(enc);
		return -1;
	}

	for (;;) {
		uint8_t* next_out = encoded + *out_size;
		size_t avail_out = encoded_capacity - *out_size;
		JxlEncoderStatus status;
		if (avail_out < 32) {
			size_t next_capacity = encoded_capacity * 2;
			uint8_t* next_buffer = (uint8_t*)realloc(encoded, next_capacity);
			if (!next_buffer) {
				io_libjxl_set_error(err, err_len, "failed to grow libjxl output buffer");
				free(encoded);
				JxlEncoderDestroy(enc);
				return -1;
			}
			encoded = next_buffer;
			encoded_capacity = next_capacity;
			next_out = encoded + *out_size;
			avail_out = encoded_capacity - *out_size;
		}

		status = JxlEncoderProcessOutput(enc, &next_out, &avail_out);
		if (status == JXL_ENC_SUCCESS) {
			*out_size = (size_t)(next_out - encoded);
			break;
		}
		if (status == JXL_ENC_NEED_MORE_OUTPUT) {
			*out_size = (size_t)(next_out - encoded);
			size_t next_capacity = encoded_capacity * 2;
			uint8_t* next_buffer = (uint8_t*)realloc(encoded, next_capacity);
			if (!next_buffer) {
				io_libjxl_set_error(err, err_len, "failed to grow libjxl output buffer");
				free(encoded);
				JxlEncoderDestroy(enc);
				return -1;
			}
			encoded = next_buffer;
			encoded_capacity = next_capacity;
			continue;
		}
		{
			JxlEncoderError encoder_error = JxlEncoderGetError(enc);
			char buffer[128];
			snprintf(buffer, sizeof(buffer), "JxlEncoderProcessOutput failed (%d)", (int)encoder_error);
			io_libjxl_set_error(err, err_len, buffer);
			free(encoded);
			JxlEncoderDestroy(enc);
			return -1;
		}
	}

	JxlEncoderDestroy(enc);
	*out_data = encoded;
	return 0;
}

static int io_libjxl_decode_impl(const uint8_t* src, size_t src_size,
		uint8_t* dst, size_t dst_size, char* err, size_t err_len, void* runner) {
	JxlDecoder* dec = NULL;
	JxlBasicInfo info;
	JxlPixelFormat format;
	JxlBitDepth bit_depth;
	int have_output = 0;

	if (!src || !dst || src_size == 0 || dst_size == 0) {
		io_libjxl_set_error(err, err_len, "invalid libjxl decode parameters");
		return -1;
	}

	dec = JxlDecoderCreate(NULL);
	if (!dec) {
		io_libjxl_set_error(err, err_len, "JxlDecoderCreate failed");
		return -1;
	}
	// A NULL runner means pool creation failed; fall back to single-threaded.
	if (runner != NULL &&
		JxlDecoderSetParallelRunner(dec, JxlThreadParallelRunner, runner) != JXL_DEC_SUCCESS) {
		io_libjxl_set_error(err, err_len, "JxlDecoderSetParallelRunner failed");
		JxlDecoderDestroy(dec);
		return -1;
	}
	if (JxlDecoderSubscribeEvents(dec, JXL_DEC_BASIC_INFO | JXL_DEC_FULL_IMAGE) != JXL_DEC_SUCCESS) {
		io_libjxl_set_error(err, err_len, "JxlDecoderSubscribeEvents failed");
		JxlDecoderDestroy(dec);
		return -1;
	}
	if (JxlDecoderSetInput(dec, src, src_size) != JXL_DEC_SUCCESS) {
		io_libjxl_set_error(err, err_len, "JxlDecoderSetInput failed");
		JxlDecoderDestroy(dec);
		return -1;
	}
	JxlDecoderCloseInput(dec);

	for (;;) {
		JxlDecoderStatus status = JxlDecoderProcessInput(dec);
		if (status == JXL_DEC_BASIC_INFO) {
			size_t required_size = 0;
			if (JxlDecoderGetBasicInfo(dec, &info) != JXL_DEC_SUCCESS) {
				io_libjxl_set_error(err, err_len, "JxlDecoderGetBasicInfo failed");
				JxlDecoderDestroy(dec);
				return -1;
			}
			if (info.exponent_bits_per_sample != 0 || info.num_extra_channels != 0 ||
				!(info.bits_per_sample == 8 || info.bits_per_sample == 12 || info.bits_per_sample == 16) ||
				!(info.num_color_channels == 1 || info.num_color_channels == 3)) {
				io_libjxl_set_error(err, err_len, "unsupported libjxl decoded image layout");
				JxlDecoderDestroy(dec);
				return -1;
			}

			memset(&format, 0, sizeof(format));
			format.num_channels = info.num_color_channels;
			format.data_type = info.bits_per_sample > 8 ? JXL_TYPE_UINT16 : JXL_TYPE_UINT8;
			// Little-endian matches the DICOM uncompressed convention and the other
			// 16-bit codecs (libjpeg, charls, ffmpeg, openjpeg).
			format.endianness = info.bits_per_sample > 8 ? JXL_LITTLE_ENDIAN : JXL_NATIVE_ENDIAN;
			format.align = 0;

			if (JxlDecoderImageOutBufferSize(dec, &format, &required_size) != JXL_DEC_SUCCESS) {
				io_libjxl_set_error(err, err_len, "JxlDecoderImageOutBufferSize failed");
				JxlDecoderDestroy(dec);
				return -1;
			}
			if (required_size > dst_size) {
				io_libjxl_set_error(err, err_len, "decoded payload does not fit output buffer");
				JxlDecoderDestroy(dec);
				return -1;
			}
			if (JxlDecoderSetImageOutBuffer(dec, &format, dst, dst_size) != JXL_DEC_SUCCESS) {
				io_libjxl_set_error(err, err_len, "JxlDecoderSetImageOutBuffer failed");
				JxlDecoderDestroy(dec);
				return -1;
			}
			if (info.bits_per_sample != 8) {
				memset(&bit_depth, 0, sizeof(bit_depth));
				bit_depth.type = JXL_BIT_DEPTH_FROM_CODESTREAM;
				bit_depth.bits_per_sample = info.bits_per_sample;
				bit_depth.exponent_bits_per_sample = 0;
				if (JxlDecoderSetImageOutBitDepth(dec, &bit_depth) != JXL_DEC_SUCCESS) {
					io_libjxl_set_error(err, err_len, "JxlDecoderSetImageOutBitDepth failed");
					JxlDecoderDestroy(dec);
					return -1;
				}
			}
			have_output = 1;
			continue;
		}
		if (status == JXL_DEC_FULL_IMAGE) {
			continue;
		}
		if (status == JXL_DEC_SUCCESS) {
			JxlDecoderDestroy(dec);
			return have_output ? 0 : -1;
		}
		if (status == JXL_DEC_ERROR) {
			io_libjxl_set_error(err, err_len, "JxlDecoderProcessInput failed");
			JxlDecoderDestroy(dec);
			return -1;
		}
		if (status == JXL_DEC_NEED_IMAGE_OUT_BUFFER) {
			if (!have_output) {
				io_libjxl_set_error(err, err_len, "decoder requested output buffer before basic info");
				JxlDecoderDestroy(dec);
				return -1;
			}
			continue;
		}
		if (status == JXL_DEC_NEED_MORE_INPUT) {
			io_libjxl_set_error(err, err_len, "libjxl decoder requires more input");
			JxlDecoderDestroy(dec);
			return -1;
		}
	}
}

// Thin wrappers that own a multithreaded runner for the lifetime of a single
// encode/decode. The runner is created here and destroyed only after the impl
// returns (which has already torn down its encoder/decoder), so it always
// outlives the libjxl object that uses it. A NULL runner degrades gracefully
// to single-threaded operation inside the impl.
static int io_libjxl_encode(const uint8_t* src, size_t src_size,
		uint16_t width, uint16_t height, uint16_t samples, uint16_t bitsa, int lossless,
		uint8_t** out_data, size_t* out_size, char* err, size_t err_len, int effort) {
	void* runner = JxlThreadParallelRunnerCreate(NULL, JxlThreadParallelRunnerDefaultNumWorkerThreads());
	int rc = io_libjxl_encode_impl(src, src_size, width, height, samples, bitsa, lossless,
		out_data, out_size, err, err_len, runner, effort);
	if (runner != NULL) {
		JxlThreadParallelRunnerDestroy(runner);
	}
	return rc;
}

static int io_libjxl_decode(const uint8_t* src, size_t src_size,
		uint8_t* dst, size_t dst_size, char* err, size_t err_len) {
	void* runner = JxlThreadParallelRunnerCreate(NULL, JxlThreadParallelRunnerDefaultNumWorkerThreads());
	int rc = io_libjxl_decode_impl(src, src_size, dst, dst_size, err, err_len, runner);
	if (runner != NULL) {
		JxlThreadParallelRunnerDestroy(runner);
	}
	return rc;
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"github.com/innovative-io/io-dicom/codecs/internal/nativeenv"
)

const nativeBackendEnabled = true

func init() {
	nativeenv.Init()
}

var (
	errLibJXLInvalidRawSize = errors.New("invalid raw payload size for dimensions")
	errLibJXLUnsupportedPNM = errors.New("unsupported pixel payload from libjxl")
)

type libjxlBackend struct{}

func (libjxlBackend) Name() string {
	return "libjxl"
}

func (libjxlBackend) Ready() error {
	return nil
}

func (libjxlBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (libjxlBackend) Decode(encoded []byte, output []byte) error {
	return libjxlBackend{}.DecodeContext(context.Background(), encoded, output)
}

func (libjxlBackend) DecodeContext(_ context.Context, encoded []byte, output []byte) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errInvalidJXLPayload
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errInvalidJXLPayload
	}
	errBuf := make([]C.char, 256)
	rc := C.io_libjxl_decode(
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
				return errInvalidJXLPayload
			}
			return fmt.Errorf("libjxl decode: %s", msg)
		}
		return errors.New("libjxl decode failed")
	}
	return nil
}

func (libjxlBackend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, lossless bool) ([]byte, error) {
	return libjxlBackend{}.EncodeContext(context.Background(), raw, width, height, samples, bitsa, lossless)
}

func (libjxlBackend) EncodeContext(_ context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, lossless bool) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errInvalidJXLPayload
	}
	if len(raw) > maxCodecPayloadBytes {
		return nil, errInvalidJXLPayload
	}
	if width == 0 || height == 0 {
		return nil, errInvalidJXLPayload
	}
	if samples != 1 && samples != 3 {
		return nil, errLibJXLUnsupportedPNM
	}
	if bitsa != 8 && bitsa != 12 && bitsa != 16 {
		return nil, errLibJXLUnsupportedPNM
	}
	expected := int(width) * int(height) * int(samples)
	if bitsa > 8 {
		expected *= 2
	}
	if len(raw) != expected {
		return nil, errLibJXLInvalidRawSize
	}

	errBuf := make([]C.char, 256)
	var outPtr *C.uint8_t
	var outSize C.size_t
	rc := C.io_libjxl_encode(
		(*C.uint8_t)(unsafe.Pointer(&raw[0])),
		C.size_t(len(raw)),
		C.uint16_t(width),
		C.uint16_t(height),
		C.uint16_t(samples),
		C.uint16_t(bitsa),
		C.int(boolToInt(lossless)),
		&outPtr,
		&outSize,
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
		C.int(EncodeEffort()),
	)
	if rc != 0 {
		if msg := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))); msg != "" {
			if msg == "raw payload size does not match image dimensions" {
				return nil, errLibJXLInvalidRawSize
			}
			return nil, fmt.Errorf("libjxl encode: %s", msg)
		}
		return nil, errors.New("libjxl encode failed")
	}
	defer C.free(unsafe.Pointer(outPtr))

	encoded := C.GoBytes(unsafe.Pointer(outPtr), C.int(outSize))
	if len(encoded) == 0 {
		return nil, errors.New("libjxl encode produced empty payload")
	}
	return encoded, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func registerNativeBackends() {
	_ = RegisterBackend("libjxl", func() Backend { return libjxlBackend{} })
}
