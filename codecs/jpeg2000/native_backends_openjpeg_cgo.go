//go:build openjpeg && cgo

package jpeg2000

/*
#cgo pkg-config: libopenjp2

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <openjpeg.h>

typedef struct {
	const uint8_t* read_data;
	size_t read_size;
	size_t read_offset;
	uint8_t* write_data;
	size_t write_size;
	size_t write_capacity;
} io_openjpeg_memstream;

static void io_openjpeg_set_error(char* err, size_t err_len, const char* msg) {
	if (!err || err_len == 0) {
		return;
	}
	if (!msg) {
		err[0] = '\0';
		return;
	}
	snprintf(err, err_len, "%s", msg);
}

static void io_openjpeg_error_callback(const char* msg, void* client_data) {
	io_openjpeg_set_error((char*)client_data, 256, msg);
}

static OPJ_SIZE_T io_openjpeg_read_callback(void* buffer, OPJ_SIZE_T bytes, void* user_data) {
	io_openjpeg_memstream* stream = (io_openjpeg_memstream*)user_data;
	if (!stream || !buffer) {
		return (OPJ_SIZE_T)-1;
	}
	if (stream->read_offset >= stream->read_size) {
		return (OPJ_SIZE_T)-1;
	}
	size_t remaining = stream->read_size - stream->read_offset;
	if ((size_t)bytes > remaining) {
		bytes = (OPJ_SIZE_T)remaining;
	}
	memcpy(buffer, stream->read_data + stream->read_offset, (size_t)bytes);
	stream->read_offset += (size_t)bytes;
	return bytes;
}

static OPJ_OFF_T io_openjpeg_skip_read_callback(OPJ_OFF_T bytes, void* user_data) {
	io_openjpeg_memstream* stream = (io_openjpeg_memstream*)user_data;
	if (!stream || bytes < 0) {
		return (OPJ_OFF_T)-1;
	}
	size_t remaining = stream->read_size - stream->read_offset;
	size_t to_skip = (size_t)bytes;
	if (to_skip > remaining) {
		to_skip = remaining;
	}
	stream->read_offset += to_skip;
	return (OPJ_OFF_T)to_skip;
}

static OPJ_BOOL io_openjpeg_seek_read_callback(OPJ_OFF_T offset, void* user_data) {
	io_openjpeg_memstream* stream = (io_openjpeg_memstream*)user_data;
	if (!stream || offset < 0) {
		return OPJ_FALSE;
	}
	if ((size_t)offset > stream->read_size) {
		return OPJ_FALSE;
	}
	stream->read_offset = (size_t)offset;
	return OPJ_TRUE;
}

static int io_openjpeg_grow_write_buffer(io_openjpeg_memstream* stream, size_t needed) {
	if (needed <= stream->write_capacity) {
		return 1;
	}
	size_t capacity = stream->write_capacity ? stream->write_capacity : 4096;
	while (capacity < needed) {
		if (capacity > ((size_t)-1) / 2) {
			capacity = needed;
			break;
		}
		capacity *= 2;
	}
	uint8_t* next = (uint8_t*)realloc(stream->write_data, capacity);
	if (!next) {
		return 0;
	}
	stream->write_data = next;
	stream->write_capacity = capacity;
	return 1;
}

static OPJ_SIZE_T io_openjpeg_write_callback(void* buffer, OPJ_SIZE_T bytes, void* user_data) {
	io_openjpeg_memstream* stream = (io_openjpeg_memstream*)user_data;
	if (!stream || (!buffer && bytes != 0)) {
		return (OPJ_SIZE_T)-1;
	}
	size_t needed = stream->write_size + (size_t)bytes;
	if (!io_openjpeg_grow_write_buffer(stream, needed)) {
		return (OPJ_SIZE_T)-1;
	}
	if (bytes > 0) {
		memcpy(stream->write_data + stream->write_size, buffer, (size_t)bytes);
		stream->write_size += (size_t)bytes;
	}
	return bytes;
}

static OPJ_OFF_T io_openjpeg_skip_write_callback(OPJ_OFF_T bytes, void* user_data) {
	io_openjpeg_memstream* stream = (io_openjpeg_memstream*)user_data;
	if (!stream || bytes < 0) {
		return (OPJ_OFF_T)-1;
	}
	size_t needed = stream->write_size + (size_t)bytes;
	if (!io_openjpeg_grow_write_buffer(stream, needed)) {
		return (OPJ_OFF_T)-1;
	}
	memset(stream->write_data + stream->write_size, 0, (size_t)bytes);
	stream->write_size = needed;
	return bytes;
}

static OPJ_BOOL io_openjpeg_seek_write_callback(OPJ_OFF_T offset, void* user_data) {
	io_openjpeg_memstream* stream = (io_openjpeg_memstream*)user_data;
	if (!stream || offset < 0) {
		return OPJ_FALSE;
	}
	size_t target = (size_t)offset;
	if (!io_openjpeg_grow_write_buffer(stream, target)) {
		return OPJ_FALSE;
	}
	if (target > stream->write_size) {
		memset(stream->write_data + stream->write_size, 0, target - stream->write_size);
		stream->write_size = target;
	}
	return OPJ_TRUE;
}

static int io_openjpeg_resolution_count(uint16_t width, uint16_t height) {
	int min_dim = (int)width;
	if ((int)height < min_dim) {
		min_dim = (int)height;
	}
	if (min_dim <= 1) {
		return 1;
	}
	int resolutions = 0;
	while (min_dim > 0) {
		resolutions++;
		min_dim >>= 1;
	}
	if (resolutions > 6) {
		return 6;
	}
	return resolutions;
}

static int io_openjpeg_decode(const uint8_t* src, size_t src_size, uint8_t* dst, size_t dst_size, char* err, size_t err_len, int threads) {
	opj_codec_t* codec = NULL;
	opj_stream_t* stream = NULL;
	opj_image_t* image = NULL;
	opj_dparameters_t parameters;
	io_openjpeg_memstream mem;
	memset(&mem, 0, sizeof(mem));
	mem.read_data = src;
	mem.read_size = src_size;

	if (!src || src_size == 0 || !dst || dst_size == 0) {
		io_openjpeg_set_error(err, err_len, "invalid JPEG 2000 buffer");
		return -1;
	}

	codec = opj_create_decompress(OPJ_CODEC_J2K);
	if (!codec) {
		io_openjpeg_set_error(err, err_len, "opj_create_decompress failed");
		return -1;
	}
	opj_set_error_handler(codec, io_openjpeg_error_callback, err);
	opj_set_default_decoder_parameters(&parameters);
	if (!opj_setup_decoder(codec, &parameters)) {
		io_openjpeg_set_error(err, err_len, err[0] ? err : "opj_setup_decoder failed");
		opj_destroy_codec(codec);
		return -1;
	}
	if (opj_has_thread_support()) {
		// threads <= 0 means auto (all cores); otherwise honor the caller's
		// request, e.g. 1 when frames are decoded concurrently elsewhere.
		int num_threads = threads > 0 ? threads : opj_get_num_cpus();
		if (num_threads > 1) {
			opj_codec_set_threads(codec, num_threads);
		}
	}

	stream = opj_stream_create(1024, OPJ_TRUE);
	if (!stream) {
		io_openjpeg_set_error(err, err_len, "opj_stream_create failed");
		opj_destroy_codec(codec);
		return -1;
	}
	opj_stream_set_user_data(stream, &mem, NULL);
	opj_stream_set_user_data_length(stream, (OPJ_UINT64)src_size);
	opj_stream_set_read_function(stream, io_openjpeg_read_callback);
	opj_stream_set_skip_function(stream, io_openjpeg_skip_read_callback);
	opj_stream_set_seek_function(stream, io_openjpeg_seek_read_callback);

	if (!opj_read_header(stream, codec, &image)) {
		io_openjpeg_set_error(err, err_len, err[0] ? err : "opj_read_header failed");
		opj_stream_destroy(stream);
		opj_destroy_codec(codec);
		opj_image_destroy(image);
		return -1;
	}
	if (!image || image->numcomps == 0) {
		io_openjpeg_set_error(err, err_len, "openjpeg returned no image components");
		opj_stream_destroy(stream);
		opj_destroy_codec(codec);
		opj_image_destroy(image);
		return -1;
	}

	if (!opj_set_decode_area(codec, image, image->x0, image->y0, image->x1, image->y1) ||
		!opj_decode(codec, stream, image) ||
		!opj_end_decompress(codec, stream)) {
		io_openjpeg_set_error(err, err_len, err[0] ? err : "openjpeg decode failed");
		opj_stream_destroy(stream);
		opj_destroy_codec(codec);
		opj_image_destroy(image);
		return -1;
	}

	// Use component[0] dimensions: these reflect the actual decoded sample
	// grid and are correct even when the J2K reference-grid origin is non-zero.
	OPJ_UINT32 width = image->comps[0].w;
	OPJ_UINT32 height = image->comps[0].h;
	OPJ_UINT32 numcomps = image->numcomps;
	OPJ_UINT32 prec = image->comps[0].prec;
	OPJ_UINT32 sgnd = image->comps[0].sgnd;
	if (!(prec == 8 || prec == 12 || prec == 16)) {
		io_openjpeg_set_error(err, err_len, "unsupported decoded precision");
		opj_stream_destroy(stream);
		opj_destroy_codec(codec);
		opj_image_destroy(image);
		return -1;
	}
	if (!(numcomps == 1 || numcomps == 3)) {
		io_openjpeg_set_error(err, err_len, "unsupported decoded component count");
		opj_stream_destroy(stream);
		opj_destroy_codec(codec);
		opj_image_destroy(image);
		return -1;
	}
	for (OPJ_UINT32 comp = 0; comp < numcomps; ++comp) {
		if (image->comps[comp].sgnd != sgnd || image->comps[comp].prec != prec ||
			image->comps[comp].w != width || image->comps[comp].h != height || image->comps[comp].data == NULL) {
			io_openjpeg_set_error(err, err_len, "unsupported decoded image layout");
			opj_stream_destroy(stream);
			opj_destroy_codec(codec);
			opj_image_destroy(image);
			return -1;
		}
	}

	size_t bytes_per_sample = prec > 8 ? 2 : 1;
	size_t expected = (size_t)width * (size_t)height * (size_t)numcomps * bytes_per_sample;
	if (expected > dst_size) {
		io_openjpeg_set_error(err, err_len, "decoded payload does not fit output buffer");
		opj_stream_destroy(stream);
		opj_destroy_codec(codec);
		opj_image_destroy(image);
		return -1;
	}

	size_t pixels = (size_t)width * (size_t)height;
	// min_val/max_val depend only on prec/sgnd, which are constant for the whole
	// image; hoist them out of the per-sample loop. Cache the component data
	// pointers too so the inner loop avoids the image->comps[comp].data
	// indirection on every sample (numcomps is validated to be 1 or 3 above).
	OPJ_INT32 min_val = sgnd ? -(OPJ_INT32)(1u << (prec - 1)) : 0;
	OPJ_INT32 max_val = sgnd ? (OPJ_INT32)((1u << (prec - 1)) - 1) : (OPJ_INT32)((1u << prec) - 1);
	OPJ_INT32* comp_data[3];
	for (OPJ_UINT32 comp = 0; comp < numcomps; ++comp) {
		comp_data[comp] = image->comps[comp].data;
	}
	if (bytes_per_sample == 1) {
		for (size_t i = 0; i < pixels; ++i) {
			for (OPJ_UINT32 comp = 0; comp < numcomps; ++comp) {
				OPJ_INT32 sample = comp_data[comp][i];
				if (sample < min_val || sample > max_val) {
					io_openjpeg_set_error(err, err_len, "decoded sample out of range");
					opj_stream_destroy(stream);
					opj_destroy_codec(codec);
					opj_image_destroy(image);
					return -1;
				}
				// Cast through int8_t preserves the two's-complement bit pattern
				// for both signed and unsigned 8-bit values.
				dst[i * numcomps + comp] = (uint8_t)(OPJ_INT8)sample;
			}
		}
	} else {
		for (size_t i = 0; i < pixels; ++i) {
			for (OPJ_UINT32 comp = 0; comp < numcomps; ++comp) {
				OPJ_INT32 sample = comp_data[comp][i];
				if (sample < min_val || sample > max_val) {
					io_openjpeg_set_error(err, err_len, "decoded sample out of range");
					opj_stream_destroy(stream);
					opj_destroy_codec(codec);
					opj_image_destroy(image);
					return -1;
				}
				// Use OPJ_UINT32 cast to get defined little-endian byte extraction
				// for both signed and unsigned samples (preserves bit pattern).
				// Little-endian matches the DICOM uncompressed convention and the
				// other 16-bit codecs (libjpeg, charls, ffmpeg) so decoded frames
				// are interpreted consistently regardless of source codec.
				OPJ_UINT32 uval = (OPJ_UINT32)sample;
				size_t offset = (i * numcomps + comp) * 2;
				dst[offset] = (uint8_t)(uval & 0xFF);
				dst[offset + 1] = (uint8_t)((uval >> 8) & 0xFF);
			}
		}
	}

	opj_stream_destroy(stream);
	opj_destroy_codec(codec);
	opj_image_destroy(image);
	return 0;
}

static int io_openjpeg_encode(const uint8_t* src, size_t src_size,
		uint16_t width, uint16_t height, uint16_t samples, uint16_t bitsa, int ratio,
		uint8_t** out_data, size_t* out_size, char* err, size_t err_len, int threads) {
	opj_codec_t* codec = NULL;
	opj_stream_t* stream = NULL;
	opj_image_t* image = NULL;
	opj_cparameters_t parameters;
	io_openjpeg_memstream mem;
	memset(&mem, 0, sizeof(mem));

	if (!src || src_size == 0 || !out_data || !out_size) {
		io_openjpeg_set_error(err, err_len, "invalid raw JPEG 2000 input");
		return -1;
	}
	if (!(samples == 1 || samples == 3) || !(bitsa == 8 || bitsa == 12 || bitsa == 16) || width == 0 || height == 0) {
		io_openjpeg_set_error(err, err_len, "unsupported OpenJPEG encode parameters");
		return -1;
	}

	size_t bytes_per_sample = bitsa > 8 ? 2 : 1;
	size_t expected = (size_t)width * (size_t)height * (size_t)samples * bytes_per_sample;
	if (expected != src_size) {
		io_openjpeg_set_error(err, err_len, "raw payload size does not match image dimensions");
		return -1;
	}

	opj_set_default_encoder_parameters(&parameters);
	parameters.numresolution = io_openjpeg_resolution_count(width, height);
	if (ratio > 0) {
		parameters.tcp_numlayers = 1;
		parameters.cp_disto_alloc = 1;
		parameters.tcp_rates[0] = (float)ratio;
		parameters.irreversible = 1;
	}

	opj_image_cmptparm_t cmptparm[3];
	memset(cmptparm, 0, sizeof(cmptparm));
	for (uint16_t comp = 0; comp < samples; ++comp) {
		cmptparm[comp].dx = (OPJ_UINT32)parameters.subsampling_dx;
		cmptparm[comp].dy = (OPJ_UINT32)parameters.subsampling_dy;
		cmptparm[comp].w = width;
		cmptparm[comp].h = height;
		cmptparm[comp].prec = bitsa;
		cmptparm[comp].sgnd = 0;
	}

	image = opj_image_create((OPJ_UINT32)samples, cmptparm, samples == 1 ? OPJ_CLRSPC_GRAY : OPJ_CLRSPC_SRGB);
	if (!image) {
		io_openjpeg_set_error(err, err_len, "opj_image_create failed");
		return -1;
	}
	image->x0 = 0;
	image->y0 = 0;
	image->x1 = width;
	image->y1 = height;

	size_t pixels = (size_t)width * (size_t)height;
	// Cache the planar component data pointers so the inner loop avoids the
	// image->comps[comp].data indirection on every sample (samples is validated
	// to be 1 or 3 above).
	OPJ_INT32* comp_data[3];
	for (uint16_t comp = 0; comp < samples; ++comp) {
		comp_data[comp] = image->comps[comp].data;
	}
	if (bytes_per_sample == 1) {
		for (size_t i = 0; i < pixels; ++i) {
			for (uint16_t comp = 0; comp < samples; ++comp) {
				comp_data[comp][i] = src[i * samples + comp];
			}
		}
	} else {
		for (size_t i = 0; i < pixels; ++i) {
			for (uint16_t comp = 0; comp < samples; ++comp) {
				size_t offset = (i * samples + comp) * 2;
				// Little-endian input to match the DICOM uncompressed convention.
				comp_data[comp][i] = (OPJ_INT32)src[offset] | ((OPJ_INT32)src[offset + 1] << 8);
			}
		}
	}

	codec = opj_create_compress(OPJ_CODEC_J2K);
	if (!codec) {
		io_openjpeg_set_error(err, err_len, "opj_create_compress failed");
		opj_image_destroy(image);
		return -1;
	}
	opj_set_error_handler(codec, io_openjpeg_error_callback, err);
	if (!opj_setup_encoder(codec, &parameters, image)) {
		io_openjpeg_set_error(err, err_len, err[0] ? err : "opj_setup_encoder failed");
		opj_destroy_codec(codec);
		opj_image_destroy(image);
		return -1;
	}
	if (opj_has_thread_support()) {
		// threads <= 0 means auto (all cores); otherwise honor the caller's
		// request, e.g. 1 when frames are encoded concurrently elsewhere.
		int num_threads = threads > 0 ? threads : opj_get_num_cpus();
		if (num_threads > 1) {
			opj_codec_set_threads(codec, num_threads);
		}
	}

	stream = opj_stream_create(1024, OPJ_FALSE);
	if (!stream) {
		io_openjpeg_set_error(err, err_len, "opj_stream_create failed");
		opj_destroy_codec(codec);
		opj_image_destroy(image);
		return -1;
	}
	opj_stream_set_user_data(stream, &mem, NULL);
	opj_stream_set_write_function(stream, io_openjpeg_write_callback);
	opj_stream_set_skip_function(stream, io_openjpeg_skip_write_callback);
	opj_stream_set_seek_function(stream, io_openjpeg_seek_write_callback);

	if (!opj_start_compress(codec, image, stream) || !opj_encode(codec, stream) || !opj_end_compress(codec, stream)) {
		io_openjpeg_set_error(err, err_len, err[0] ? err : "openjpeg encode failed");
		free(mem.write_data);
		opj_stream_destroy(stream);
		opj_destroy_codec(codec);
		opj_image_destroy(image);
		return -1;
	}

	*out_data = mem.write_data;
	*out_size = mem.write_size;
	opj_stream_destroy(stream);
	opj_destroy_codec(codec);
	opj_image_destroy(image);
	return 0;
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"unsafe"

	"github.com/innovative-io/io-dicom/codecs/internal/codecctx"
)

const nativeBackendEnabled = true

var (
	errOpenJPEGInvalidRawSize = errors.New("invalid raw payload size for dimensions")
	errOpenJPEGUnsupportedPNM = errors.New("unsupported jpeg 2000 pixel layout for openjpeg")
)

type openjpegBackend struct{}

func (openjpegBackend) Name() string {
	return "openjpeg"
}

func (openjpegBackend) Ready() error {
	return nil
}

func (openjpegBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (openjpegBackend) Decode(encoded []byte, output []byte) error {
	return openjpegBackend{}.DecodeContext(context.Background(), encoded, output)
}

func (openjpegBackend) DecodeContext(ctx context.Context, encoded []byte, output []byte) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errInvalidJ2KPayload
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errInvalidJ2KPayload
	}

	var errBuf [256]C.char
	rc := C.io_openjpeg_decode(
		(*C.uint8_t)(unsafe.Pointer(&encoded[0])),
		C.size_t(len(encoded)),
		(*C.uint8_t)(unsafe.Pointer(&output[0])),
		C.size_t(len(output)),
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
		C.int(codecctx.Threads(ctx)),
	)
	if rc != 0 {
		if msg := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))); msg != "" {
			if msg == "decoded payload does not fit output buffer" {
				return errInvalidJ2KPayload
			}
			return fmt.Errorf("openjpeg decode failed: %s", msg)
		}
		return errors.New("openjpeg decode failed")
	}
	return nil
}

func (openjpegBackend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, ratio int) ([]byte, error) {
	return openjpegBackend{}.EncodeContext(context.Background(), raw, width, height, samples, bitsa, ratio)
}

func (openjpegBackend) EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, ratio int) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errInvalidJ2KPayload
	}
	if len(raw) > maxCodecPayloadBytes {
		return nil, errInvalidJ2KPayload
	}
	if samples != 1 && samples != 3 {
		return nil, errOpenJPEGUnsupportedPNM
	}
	if bitsa != 8 && bitsa != 12 && bitsa != 16 {
		return nil, errOpenJPEGUnsupportedPNM
	}
	bytesPerSample := 1
	if bitsa > 8 {
		bytesPerSample = 2
	}
	if int(width)*int(height)*int(samples)*bytesPerSample != len(raw) {
		return nil, errOpenJPEGInvalidRawSize
	}

	var errBuf [256]C.char
	var outPtr *C.uint8_t
	var outSize C.size_t
	rc := C.io_openjpeg_encode(
		(*C.uint8_t)(unsafe.Pointer(&raw[0])),
		C.size_t(len(raw)),
		C.uint16_t(width),
		C.uint16_t(height),
		C.uint16_t(samples),
		C.uint16_t(bitsa),
		C.int(ratio),
		&outPtr,
		&outSize,
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
		C.int(codecctx.Threads(ctx)),
	)
	if rc != 0 {
		if msg := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))); msg != "" {
			if msg == "raw payload size does not match image dimensions" {
				return nil, errOpenJPEGInvalidRawSize
			}
			if msg == "unsupported OpenJPEG encode parameters" {
				return nil, errOpenJPEGUnsupportedPNM
			}
			return nil, fmt.Errorf("openjpeg encode failed: %s", msg)
		}
		return nil, errors.New("openjpeg encode failed")
	}
	defer C.free(unsafe.Pointer(outPtr))

	if outPtr == nil || outSize == 0 {
		return nil, errors.New("openjpeg encode produced empty payload")
	}
	encoded := C.GoBytes(unsafe.Pointer(outPtr), C.int(outSize))
	if len(encoded) == 0 {
		return nil, errors.New("openjpeg encode produced empty payload")
	}
	return encoded, nil
}

func openjpegResolutionCount(width uint16, height uint16) int {
	minDim := int(width)
	if int(height) < minDim {
		minDim = int(height)
	}
	if minDim <= 1 {
		return 1
	}
	resolutions := bits.Len(uint(minDim))
	if resolutions > 6 {
		return 6
	}
	return resolutions
}

func registerNativeBackends() {
	_ = RegisterBackend("openjpeg", func() Backend { return openjpegBackend{} })
}
