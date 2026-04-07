//go:build libjpeg && cgo

package jpeg

/*
#cgo pkg-config: libjpeg

#include <setjmp.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <jerror.h>
#include <jpeglib.h>

typedef struct {
	struct jpeg_error_mgr pub;
	jmp_buf setjmp_buffer;
	char* err;
	size_t err_len;
} io_libjpeg_error_mgr;

static void io_libjpeg_set_error(char* err, size_t err_len, const char* msg) {
	if (!err || err_len == 0) {
		return;
	}
	if (!msg) {
		err[0] = '\0';
		return;
	}
	snprintf(err, err_len, "%s", msg);
}

METHODDEF(void) io_libjpeg_error_exit(j_common_ptr cinfo) {
	io_libjpeg_error_mgr* err = (io_libjpeg_error_mgr*)cinfo->err;
	char buffer[JMSG_LENGTH_MAX];
	(*cinfo->err->format_message)(cinfo, buffer);
	io_libjpeg_set_error(err->err, err->err_len, buffer);
	longjmp(err->setjmp_buffer, 1);
}

static int io_libjpeg_predictor_value(int predictor) {
	if (predictor < 1 || predictor > 7) {
		return 1;
	}
	return predictor;
}

static int io_libjpeg_encode12(const uint8_t* src, size_t src_size,
		uint16_t width, uint16_t height, uint16_t samples, int predictor,
		uint8_t** out_data, size_t* out_size, char* err, size_t err_len) {
	struct jpeg_compress_struct cinfo;
	io_libjpeg_error_mgr jerr;
	unsigned char* encoded = NULL;
	unsigned long encoded_size = 0;
	J12SAMPLE* row = NULL;
	J12SAMPROW row_ptr[1] = { NULL };
	size_t pixels_per_row = (size_t)width * (size_t)samples;
	size_t expected = pixels_per_row * (size_t)height * 2;

	if (!src || !out_data || !out_size || width == 0 || height == 0 || (samples != 1 && samples != 3)) {
		io_libjpeg_set_error(err, err_len, "invalid libjpeg 12-bit encode parameters");
		return -1;
	}
	if (src_size != expected) {
		io_libjpeg_set_error(err, err_len, "raw payload size does not match 12-bit image dimensions");
		return -1;
	}

	memset(&cinfo, 0, sizeof(cinfo));
	memset(&jerr, 0, sizeof(jerr));
	cinfo.err = jpeg_std_error(&jerr.pub);
	jerr.pub.error_exit = io_libjpeg_error_exit;
	jerr.err = err;
	jerr.err_len = err_len;

	if (setjmp(jerr.setjmp_buffer)) {
		if (row) {
			free(row);
		}
		if (encoded) {
			free(encoded);
		}
		jpeg_destroy_compress(&cinfo);
		return -1;
	}

	jpeg_create_compress(&cinfo);
	jpeg_mem_dest(&cinfo, &encoded, &encoded_size);
	cinfo.image_width = width;
	cinfo.image_height = height;
	cinfo.input_components = samples;
	cinfo.in_color_space = samples == 1 ? JCS_GRAYSCALE : JCS_RGB;
	jpeg_set_defaults(&cinfo);
	cinfo.data_precision = 12;
	jpeg_enable_lossless(&cinfo, io_libjpeg_predictor_value(predictor), 0);
	jpeg_start_compress(&cinfo, TRUE);

	row = (J12SAMPLE*)malloc(pixels_per_row * sizeof(J12SAMPLE));
	if (!row) {
		io_libjpeg_set_error(err, err_len, "failed to allocate libjpeg 12-bit row buffer");
		free(encoded);
		jpeg_destroy_compress(&cinfo);
		return -1;
	}
	row_ptr[0] = row;

	for (uint16_t y = 0; y < height; ++y) {
		const uint8_t* src_row = src + ((size_t)y * pixels_per_row * 2);
		for (size_t idx = 0; idx < pixels_per_row; ++idx) {
			uint16_t sample = (uint16_t)src_row[idx * 2] | ((uint16_t)src_row[idx * 2 + 1] << 8);
			if (sample > MAXJ12SAMPLE) {
				io_libjpeg_set_error(err, err_len, "12-bit sample out of range for libjpeg encode");
				free(row);
				free(encoded);
				jpeg_destroy_compress(&cinfo);
				return -1;
			}
			row[idx] = (J12SAMPLE)sample;
		}
		if (jpeg12_write_scanlines(&cinfo, row_ptr, 1) != 1) {
			io_libjpeg_set_error(err, err_len, "jpeg12_write_scanlines failed");
			free(row);
			free(encoded);
			jpeg_destroy_compress(&cinfo);
			return -1;
		}
	}

	jpeg_finish_compress(&cinfo);
	jpeg_destroy_compress(&cinfo);
	free(row);

	*out_data = encoded;
	*out_size = (size_t)encoded_size;
	return 0;
}

static int io_libjpeg_encode16(const uint8_t* src, size_t src_size,
		uint16_t width, uint16_t height, uint16_t samples, int predictor,
		uint8_t** out_data, size_t* out_size, char* err, size_t err_len) {
	struct jpeg_compress_struct cinfo;
	io_libjpeg_error_mgr jerr;
	unsigned char* encoded = NULL;
	unsigned long encoded_size = 0;
	J16SAMPLE* row = NULL;
	J16SAMPROW row_ptr[1] = { NULL };
	size_t pixels_per_row = (size_t)width * (size_t)samples;
	size_t expected = pixels_per_row * (size_t)height * 2;

	if (!src || !out_data || !out_size || width == 0 || height == 0 || (samples != 1 && samples != 3)) {
		io_libjpeg_set_error(err, err_len, "invalid libjpeg 16-bit encode parameters");
		return -1;
	}
	if (src_size != expected) {
		io_libjpeg_set_error(err, err_len, "raw payload size does not match 16-bit image dimensions");
		return -1;
	}

	memset(&cinfo, 0, sizeof(cinfo));
	memset(&jerr, 0, sizeof(jerr));
	cinfo.err = jpeg_std_error(&jerr.pub);
	jerr.pub.error_exit = io_libjpeg_error_exit;
	jerr.err = err;
	jerr.err_len = err_len;

	if (setjmp(jerr.setjmp_buffer)) {
		if (row) {
			free(row);
		}
		if (encoded) {
			free(encoded);
		}
		jpeg_destroy_compress(&cinfo);
		return -1;
	}

	jpeg_create_compress(&cinfo);
	jpeg_mem_dest(&cinfo, &encoded, &encoded_size);
	cinfo.image_width = width;
	cinfo.image_height = height;
	cinfo.input_components = samples;
	cinfo.in_color_space = samples == 1 ? JCS_GRAYSCALE : JCS_RGB;
	jpeg_set_defaults(&cinfo);
	cinfo.data_precision = 16;
	jpeg_enable_lossless(&cinfo, io_libjpeg_predictor_value(predictor), 0);
	jpeg_start_compress(&cinfo, TRUE);

	row = (J16SAMPLE*)malloc(pixels_per_row * sizeof(J16SAMPLE));
	if (!row) {
		io_libjpeg_set_error(err, err_len, "failed to allocate libjpeg 16-bit row buffer");
		free(encoded);
		jpeg_destroy_compress(&cinfo);
		return -1;
	}
	row_ptr[0] = row;

	for (uint16_t y = 0; y < height; ++y) {
		const uint8_t* src_row = src + ((size_t)y * pixels_per_row * 2);
		for (size_t idx = 0; idx < pixels_per_row; ++idx) {
			uint16_t sample = (uint16_t)src_row[idx * 2] | ((uint16_t)src_row[idx * 2 + 1] << 8);
			row[idx] = (J16SAMPLE)sample;
		}
		if (jpeg16_write_scanlines(&cinfo, row_ptr, 1) != 1) {
			io_libjpeg_set_error(err, err_len, "jpeg16_write_scanlines failed");
			free(row);
			free(encoded);
			jpeg_destroy_compress(&cinfo);
			return -1;
		}
	}

	jpeg_finish_compress(&cinfo);
	jpeg_destroy_compress(&cinfo);
	free(row);

	*out_data = encoded;
	*out_size = (size_t)encoded_size;
	return 0;
}

static int io_libjpeg_decode12(const uint8_t* src, size_t src_size,
		uint8_t* dst, size_t dst_size, char* err, size_t err_len) {
	struct jpeg_decompress_struct cinfo;
	io_libjpeg_error_mgr jerr;
	J12SAMPLE* row = NULL;
	J12SAMPROW row_ptr[1] = { NULL };
	size_t pixels_per_row;
	size_t expected;

	if (!src || !dst || src_size == 0 || dst_size == 0) {
		io_libjpeg_set_error(err, err_len, "invalid libjpeg 12-bit decode parameters");
		return -1;
	}

	memset(&cinfo, 0, sizeof(cinfo));
	memset(&jerr, 0, sizeof(jerr));
	cinfo.err = jpeg_std_error(&jerr.pub);
	jerr.pub.error_exit = io_libjpeg_error_exit;
	jerr.err = err;
	jerr.err_len = err_len;

	if (setjmp(jerr.setjmp_buffer)) {
		if (row) {
			free(row);
		}
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}

	jpeg_create_decompress(&cinfo);
	jpeg_mem_src(&cinfo, src, (unsigned long)src_size);
	if (jpeg_read_header(&cinfo, TRUE) != JPEG_HEADER_OK) {
		io_libjpeg_set_error(err, err_len, "jpeg_read_header failed for 12-bit payload");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}
	if (cinfo.data_precision != 12) {
		io_libjpeg_set_error(err, err_len, "libjpeg payload is not 12-bit precision");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}
	if (!jpeg_start_decompress(&cinfo)) {
		io_libjpeg_set_error(err, err_len, "jpeg_start_decompress failed for 12-bit payload");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}
	if (cinfo.output_components != 1 && cinfo.output_components != 3) {
		io_libjpeg_set_error(err, err_len, "unsupported 12-bit component count from libjpeg decode");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}

	pixels_per_row = (size_t)cinfo.output_width * (size_t)cinfo.output_components;
	expected = pixels_per_row * (size_t)cinfo.output_height * 2;
	if (expected > dst_size) {
		io_libjpeg_set_error(err, err_len, "decoded 12-bit payload does not fit output buffer");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}

	row = (J12SAMPLE*)malloc(pixels_per_row * sizeof(J12SAMPLE));
	if (!row) {
		io_libjpeg_set_error(err, err_len, "failed to allocate libjpeg 12-bit decode row buffer");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}
	row_ptr[0] = row;

	while (cinfo.output_scanline < cinfo.output_height) {
		if (jpeg12_read_scanlines(&cinfo, row_ptr, 1) != 1) {
			io_libjpeg_set_error(err, err_len, "jpeg12_read_scanlines failed");
			free(row);
			jpeg_destroy_decompress(&cinfo);
			return -1;
		}
		uint8_t* dst_row = dst + ((size_t)(cinfo.output_scanline - 1) * pixels_per_row * 2);
		for (size_t idx = 0; idx < pixels_per_row; ++idx) {
			uint16_t sample = (uint16_t)row[idx];
			dst_row[idx * 2] = (uint8_t)(sample & 0xFF);
			dst_row[idx * 2 + 1] = (uint8_t)((sample >> 8) & 0xFF);
		}
	}

	jpeg_finish_decompress(&cinfo);
	jpeg_destroy_decompress(&cinfo);
	free(row);
	return 0;
}

static int io_libjpeg_decode16(const uint8_t* src, size_t src_size,
		uint8_t* dst, size_t dst_size, char* err, size_t err_len) {
	struct jpeg_decompress_struct cinfo;
	io_libjpeg_error_mgr jerr;
	J16SAMPLE* row = NULL;
	J16SAMPROW row_ptr[1] = { NULL };
	size_t pixels_per_row;
	size_t expected;

	if (!src || !dst || src_size == 0 || dst_size == 0) {
		io_libjpeg_set_error(err, err_len, "invalid libjpeg 16-bit decode parameters");
		return -1;
	}

	memset(&cinfo, 0, sizeof(cinfo));
	memset(&jerr, 0, sizeof(jerr));
	cinfo.err = jpeg_std_error(&jerr.pub);
	jerr.pub.error_exit = io_libjpeg_error_exit;
	jerr.err = err;
	jerr.err_len = err_len;

	if (setjmp(jerr.setjmp_buffer)) {
		if (row) {
			free(row);
		}
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}

	jpeg_create_decompress(&cinfo);
	jpeg_mem_src(&cinfo, src, (unsigned long)src_size);
	if (jpeg_read_header(&cinfo, TRUE) != JPEG_HEADER_OK) {
		io_libjpeg_set_error(err, err_len, "jpeg_read_header failed for 16-bit payload");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}
	if (cinfo.data_precision != 16) {
		io_libjpeg_set_error(err, err_len, "libjpeg payload is not 16-bit precision");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}
	if (!jpeg_start_decompress(&cinfo)) {
		io_libjpeg_set_error(err, err_len, "jpeg_start_decompress failed for 16-bit payload");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}
	if (cinfo.output_components != 1 && cinfo.output_components != 3) {
		io_libjpeg_set_error(err, err_len, "unsupported 16-bit component count from libjpeg decode");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}

	pixels_per_row = (size_t)cinfo.output_width * (size_t)cinfo.output_components;
	expected = pixels_per_row * (size_t)cinfo.output_height * 2;
	if (expected > dst_size) {
		io_libjpeg_set_error(err, err_len, "decoded 16-bit payload does not fit output buffer");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}

	row = (J16SAMPLE*)malloc(pixels_per_row * sizeof(J16SAMPLE));
	if (!row) {
		io_libjpeg_set_error(err, err_len, "failed to allocate libjpeg 16-bit decode row buffer");
		jpeg_destroy_decompress(&cinfo);
		return -1;
	}
	row_ptr[0] = row;

	while (cinfo.output_scanline < cinfo.output_height) {
		if (jpeg16_read_scanlines(&cinfo, row_ptr, 1) != 1) {
			io_libjpeg_set_error(err, err_len, "jpeg16_read_scanlines failed");
			free(row);
			jpeg_destroy_decompress(&cinfo);
			return -1;
		}
		uint8_t* dst_row = dst + ((size_t)(cinfo.output_scanline - 1) * pixels_per_row * 2);
		for (size_t idx = 0; idx < pixels_per_row; ++idx) {
			uint16_t sample = (uint16_t)row[idx];
			dst_row[idx * 2] = (uint8_t)(sample & 0xFF);
			dst_row[idx * 2 + 1] = (uint8_t)((sample >> 8) & 0xFF);
		}
	}

	jpeg_finish_decompress(&cinfo);
	jpeg_destroy_decompress(&cinfo);
	free(row);
	return 0;
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

var errLibJPEGInvalidPayload = errors.New("ERROR, Decode12, JPEG failed")

type libjpegBackend struct{}

func (libjpegBackend) Name() string {
	return "libjpeg"
}

func (libjpegBackend) Ready() error {
	return nil
}

func (libjpegBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (libjpegBackend) Decode12(encoded []byte, output []byte) error {
	return libjpegBackend{}.Decode12Context(context.Background(), encoded, output)
}

func (libjpegBackend) Decode12Context(_ context.Context, encoded []byte, output []byte) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errLibJPEGInvalidPayload
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errLibJPEGInvalidPayload
	}

	errBuf := make([]C.char, 256)
	rc := C.io_libjpeg_decode12(
		(*C.uint8_t)(unsafe.Pointer(&encoded[0])),
		C.size_t(len(encoded)),
		(*C.uint8_t)(unsafe.Pointer(&output[0])),
		C.size_t(len(output)),
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
	)
	if rc != 0 {
		if msg := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))); msg != "" {
			if msg == "decoded 12-bit payload does not fit output buffer" {
				return errLibJPEGInvalidPayload
			}
			return fmt.Errorf("libjpeg decode12: %s", msg)
		}
		return errors.New("libjpeg decode12 failed")
	}
	return nil
}

func (libjpegBackend) Encode12(raw []byte, width uint16, height uint16, samples uint16, mode int) ([]byte, error) {
	return libjpegBackend{}.Encode12Context(context.Background(), raw, width, height, samples, mode)
}

func (libjpegBackend) Encode12Context(_ context.Context, raw []byte, width uint16, height uint16, samples uint16, _ int) ([]byte, error) {
	if len(raw) == 0 || len(raw) > maxCodecPayloadBytes {
		return nil, errLibJPEGInvalidPayload
	}
	if width == 0 || height == 0 {
		return nil, errLibJPEGInvalidPayload
	}

	errBuf := make([]C.char, 256)
	var outPtr *C.uint8_t
	var outSize C.size_t
	rc := C.io_libjpeg_encode12(
		(*C.uint8_t)(unsafe.Pointer(&raw[0])),
		C.size_t(len(raw)),
		C.uint16_t(width),
		C.uint16_t(height),
		C.uint16_t(samples),
		C.int(1),
		&outPtr,
		&outSize,
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
	)
	if rc != 0 {
		if msg := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))); msg != "" {
			if msg == "raw payload size does not match 12-bit image dimensions" {
				return nil, errLibJPEGInvalidPayload
			}
			return nil, fmt.Errorf("libjpeg encode12: %s", msg)
		}
		return nil, errors.New("libjpeg encode12 failed")
	}
	defer C.free(unsafe.Pointer(outPtr))

	encoded := C.GoBytes(unsafe.Pointer(outPtr), C.int(outSize))
	if len(encoded) == 0 {
		return nil, errors.New("libjpeg encode12 produced empty payload")
	}
	return encoded, nil
}

func (libjpegBackend) Decode16(encoded []byte, output []byte) error {
	return libjpegBackend{}.Decode16Context(context.Background(), encoded, output)
}

func (libjpegBackend) Decode16Context(_ context.Context, encoded []byte, output []byte) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errors.New("ERROR, Decode16, JPEG failed")
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errors.New("ERROR, Decode16, JPEG failed")
	}

	errBuf := make([]C.char, 256)
	rc := C.io_libjpeg_decode16(
		(*C.uint8_t)(unsafe.Pointer(&encoded[0])),
		C.size_t(len(encoded)),
		(*C.uint8_t)(unsafe.Pointer(&output[0])),
		C.size_t(len(output)),
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
	)
	if rc != 0 {
		if msg := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))); msg != "" {
			if msg == "decoded 16-bit payload does not fit output buffer" {
				return errors.New("ERROR, Decode16, JPEG failed")
			}
			return fmt.Errorf("libjpeg decode16: %s", msg)
		}
		return errors.New("libjpeg decode16 failed")
	}
	return nil
}

func (libjpegBackend) Encode16(raw []byte, width uint16, height uint16, samples uint16, mode int) ([]byte, error) {
	return libjpegBackend{}.Encode16Context(context.Background(), raw, width, height, samples, mode)
}

func (libjpegBackend) Encode16Context(_ context.Context, raw []byte, width uint16, height uint16, samples uint16, _ int) ([]byte, error) {
	if len(raw) == 0 || len(raw) > maxCodecPayloadBytes {
		return nil, errors.New("ERROR, Encode16, JPEG failed")
	}
	if width == 0 || height == 0 {
		return nil, errors.New("ERROR, Encode16, JPEG failed")
	}

	errBuf := make([]C.char, 256)
	var outPtr *C.uint8_t
	var outSize C.size_t
	rc := C.io_libjpeg_encode16(
		(*C.uint8_t)(unsafe.Pointer(&raw[0])),
		C.size_t(len(raw)),
		C.uint16_t(width),
		C.uint16_t(height),
		C.uint16_t(samples),
		C.int(1),
		&outPtr,
		&outSize,
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
	)
	if rc != 0 {
		if msg := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))); msg != "" {
			if msg == "raw payload size does not match 16-bit image dimensions" {
				return nil, errors.New("ERROR, Encode16, JPEG failed")
			}
			return nil, fmt.Errorf("libjpeg encode16: %s", msg)
		}
		return nil, errors.New("libjpeg encode16 failed")
	}
	defer C.free(unsafe.Pointer(outPtr))

	encoded := C.GoBytes(unsafe.Pointer(outPtr), C.int(outSize))
	if len(encoded) == 0 {
		return nil, errors.New("libjpeg encode16 produced empty payload")
	}
	return encoded, nil
}

func registerNativeBackends() {
	_ = RegisterBackend("libjpeg", func() Backend { return libjpegBackend{} })
}
