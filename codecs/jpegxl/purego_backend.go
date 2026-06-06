package jpegxl

import (
	"errors"

	"github.com/innovative-io/io-dicom/codecs/jpegxl/gojxl"
)

// gojpegxlBackend is the built-in pure-Go JPEG XL decoder. It handles the
// lossless Modular subset (single frame, RCT/Palette/Squeeze, single- and
// multi-group); inputs outside that subset (VarDCT lossy, JPEG recompression,
// animation, ICC, non-identity orientation) return an error so a higher-
// priority native backend (libjxl, when built with -tags libjxl) can take over.
// There is no pure-Go encoder yet.
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

// Encode is not implemented in pure Go.
func (gojpegxlBackend) Encode(_ []byte, _ uint16, _ uint16, _ uint16, _ uint16, _ bool) ([]byte, error) {
	return nil, errors.New("jpegxl: pure-Go encoder not implemented")
}

func registerPureGoBackend() {
	// Priority -1 so a native libjxl backend (priority 0) wins when present.
	_ = mgr.RegisterWithPriority("gojpegxl", func() Backend { return gojpegxlBackend{} }, -1)
}
