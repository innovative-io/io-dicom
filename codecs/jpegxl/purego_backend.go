package jpegxl

import (
	"errors"

	"github.com/innovative-io/io-dicom/codecs/jpegxl/gojxl"
)

// gojpegxlBackend is the built-in pure-Go JPEG XL decoder. It handles the
// lossless Modular subset (single frame, RCT/Palette/Squeeze, single- and
// multi-group) and the lossy VarDCT subset for XYB-encoded RGB/grayscale: the
// full common transform set (square/rectangular/large DCTs, DCT2x2/4x4/4x8/8x4,
// IDENTITY, AFV0-3), multi-group, multi-DC-group, permuted TOC, local/global
// modular trees, one or more AC histogram sets, multiple passes (progressive),
// CfL, Gaborish and EPF — at any image size. Inputs outside those subsets
// (VarDCT with the rarely-used DCT128/256, non-XYB color, extra channels, JPEG
// recompression, animation, ICC, non-identity orientation) return an error.
// Encoding supports the lossless Modular subset via gojxl.Encode.
type gojpegxlBackend struct{}

func (gojpegxlBackend) Name() string { return "gojpegxl" }

func (gojpegxlBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

// Decode decodes a JPEG XL codestream into output. A panic on malformed input
// is converted to an error so the decoder is safe on hostile/fuzzed data.
func (gojpegxlBackend) Decode(encoded []byte, output []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("jpegxl: malformed codestream")
		}
	}()
	if len(encoded) == 0 || len(output) == 0 {
		return errInvalidJXLPayload
	}
	img, derr := gojxl.Decode(encoded)
	if derr != nil {
		return derr
	}
	if len(img.Pixels) > len(output) {
		return errInvalidJXLPayload
	}
	copy(output, img.Pixels)
	return nil
}

// kLossyGlobalScale is the VarDCT quantizer scale used for lossy encoding — a
// high-quality (visually near-lossless) default suitable for medical imagery.
const kLossyGlobalScale = 32768

// Encode compresses raw interleaved samples to a JPEG XL codestream. Lossless
// uses the pure-Go Modular encoder (any size, 8/16-bit, 1/3 channels); lossy
// uses the pure-Go VarDCT encoder (8-bit, 1/3 channels, up to 16384px). Requests
// outside those subsets error so a native backend can take over.
func (gojpegxlBackend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, lossless bool) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, errors.New("jpegxl: encode failed")
		}
	}()
	if !lossless {
		if bitsa != 8 {
			return nil, errors.New("jpegxl: pure-Go lossy encode supports 8-bit only")
		}
		return gojxl.EncodeVarDCT(raw, int(width), int(height), int(samples), kLossyGlobalScale)
	}
	return gojxl.Encode(raw, int(width), int(height), int(samples), int(bitsa))
}

func registerPureGoBackend() {
	// gojpegxl is the only built-in backend; an external one can still be plugged
	// in at a higher priority via RegisterBackend.
	_ = mgr.RegisterWithPriority("gojpegxl", func() Backend { return gojpegxlBackend{} }, -1)
}
