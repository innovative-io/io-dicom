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
// recompression, animation, ICC, non-identity orientation) return an error so a
// higher-priority native backend (libjxl, when built with -tags libjxl) can
// take over. Encoding supports the lossless Modular subset via gojxl.Encode.
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

// Encode losslessly compresses raw interleaved samples to a JPEG XL codestream
// (pure-Go Modular encoder). Lossy (VarDCT) encoding is not implemented, so a
// non-lossless request errors so a native backend can take over.
func (gojpegxlBackend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, lossless bool) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, errors.New("jpegxl: encode failed")
		}
	}()
	if !lossless {
		return nil, errors.New("jpegxl: pure-Go encoder is lossless-only")
	}
	return gojxl.Encode(raw, int(width), int(height), int(samples), int(bitsa))
}

func registerPureGoBackend() {
	// Priority -1 so a native libjxl backend (priority 0) wins when present.
	_ = mgr.RegisterWithPriority("gojpegxl", func() Backend { return gojpegxlBackend{} }, -1)
}
