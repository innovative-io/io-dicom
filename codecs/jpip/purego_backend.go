package jpip

import (
	"context"
	"errors"

	"github.com/innovative-io/io-dicom/codecs/jpeg2000"
)

// gojpipBackend is the built-in pure-Go JPIP/HTJ2K backend. The JPIP transfer
// syntaxes (.204 / .205) carry HTJ2K (ISO 15444-15) codestreams, so decode
// delegates to the pure-Go JPEG 2000 decoder and encode to the pure-Go HTJ2K
// encoder. This replaces the retired openjph cgo backend.
type gojpipBackend struct{}

var (
	errInvalidJPIPPayload  = errors.New("invalid JPIP payload size")
	errUnsupportedLayout   = errors.New("unsupported jpip pixel layout")
	errInvalidRawForDims   = errors.New("invalid jpip raw payload size for dimensions")
	errEmptyEncodedPayload = errors.New("jpip encode produced empty payload")
)

func (gojpipBackend) Name() string { return "gojpip" }

func (gojpipBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (gojpipBackend) Decode(encoded []byte, output []byte, uid string) error {
	return gojpipBackend{}.DecodeContext(context.Background(), encoded, output, uid)
}

func (gojpipBackend) DecodeContext(_ context.Context, encoded []byte, output []byte, _ string) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errInvalidJPIPPayload
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errInvalidJPIPPayload
	}
	// Decode with the pure-Go HTJ2K decoder directly so JPIP is independent of
	// the JPEG 2000 backend selection (callers may swap that backend).
	return jpeg2000.HTJ2Kdecode(encoded, output)
}

func (gojpipBackend) Encode(raw []byte, width, height, samples, bitsa uint16, uid string) ([]byte, error) {
	return gojpipBackend{}.EncodeContext(context.Background(), raw, width, height, samples, bitsa, uid)
}

func (gojpipBackend) EncodeContext(_ context.Context, raw []byte, width, height, samples, bitsa uint16, _ string) ([]byte, error) {
	if len(raw) == 0 || len(raw) > maxCodecPayloadBytes {
		return nil, errInvalidJPIPPayload
	}
	if width == 0 || height == 0 {
		return nil, errInvalidJPIPPayload
	}
	if samples != 1 && samples != 3 {
		return nil, errUnsupportedLayout
	}
	if bitsa != 8 && bitsa != 12 && bitsa != 16 {
		return nil, errUnsupportedLayout
	}
	expected := int(width) * int(height) * int(samples)
	if bitsa > 8 {
		expected *= 2
	}
	if len(raw) != expected {
		return nil, errInvalidRawForDims
	}

	encoded, err := jpeg2000.HTJ2Kencode(raw, int(width), int(height), int(samples), int(bitsa))
	if err != nil {
		return nil, err
	}
	if len(encoded) == 0 {
		return nil, errEmptyEncodedPayload
	}
	return encoded, nil
}

func registerPureGoBackend() {
	_ = RegisterBackend("gojpip", func() Backend { return gojpipBackend{} })
}
