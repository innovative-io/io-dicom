package jpegls

import (
	"context"
	"errors"
	"log/slog"

	"github.com/innovative-io/io-dicom/codecs/internal/backendmgr"
)

// CGOEnabled reports whether a native cgo JPEG-LS backend is compiled in. The
// charls backend was retired in favor of the pure-Go gojpegls codec, so this is
// always false; it is retained for API compatibility.
const CGOEnabled = false

const maxCodecPayloadBytes = 512 << 20

var errInvalidJLSPayload = errors.New("invalid JPEG-LS payload size")
var errBackendUnavailable = errors.New("no JPEG-LS decode backend available")

var supportedTransferSyntaxUIDs = []string{
	"1.2.840.10008.1.2.4.80",
	"1.2.840.10008.1.2.4.81",
}

// Backend defines a JPEG-LS implementation that can be plugged at runtime.
// The default backend keeps pure-Go passthrough behavior.
type Backend interface {
	Name() string
	SupportedTransferSyntaxUIDs() []string
	Decode(encoded []byte, output []byte) error
	Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, nearLossless bool) ([]byte, error)
}

type contextBackend interface {
	DecodeContext(ctx context.Context, encoded []byte, output []byte) error
	EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, nearLossless bool) ([]byte, error)
}

type passthroughBackend struct{}

func (passthroughBackend) Name() string {
	return "passthrough"
}

func (passthroughBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (passthroughBackend) Decode(_ []byte, _ []byte) error {
	return errBackendUnavailable
}

func (passthroughBackend) Encode(raw []byte, _ uint16, _ uint16, _ uint16, _ uint16, _ bool) ([]byte, error) {
	encoded := make([]byte, len(raw))
	copy(encoded, raw)
	return encoded, nil
}

var mgr = backendmgr.New(func() Backend { return passthroughBackend{} })

func init() {
	registerPureGoBackend() // pure-Go gojpegls is the only built-in backend
	mgr.SelectDefault()
	if mgr.BackendName() == "passthrough" {
		slog.Warn("jpegls: no decode backend available")
	}
}

// SetBackend overrides the active JPEG-LS backend. Passing nil resets to the
// default (pure-Go gojpegls) backend.
func SetBackend(backend Backend) {
	if backend == nil {
		// Reset to the default (pure-Go gojpegls) rather than the passthrough
		// fallback, since gojpegls is the only real backend after charls retired.
		mgr.Reset()
		mgr.SelectDefault()
		return
	}
	mgr.Set(backend)
}

// BackendName returns the current JPEG-LS backend name.
func BackendName() string { return mgr.BackendName() }

// RegisterBackend registers a named factory for runtime selection.
func RegisterBackend(name string, factory func() Backend) error { return mgr.Register(name, factory) }

// UseBackend switches to a previously registered backend by name.
func UseBackend(name string) error { return mgr.Use(name) }

// AvailableBackends returns sorted names of all registered backends.
func AvailableBackends() []string { return mgr.Available() }

// ValidateBackend probes whether a named backend is ready.
func ValidateBackend(name string) error { return mgr.Validate(name) }

func activeBackend() Backend { return mgr.Active() }

func SupportedTransferSyntaxUIDs() []string {
	out := make([]string, len(supportedTransferSyntaxUIDs))
	copy(out, supportedTransferSyntaxUIDs)
	return out
}

func decodeWithContext(ctx context.Context, encoded []byte, output []byte) error {
	backend := activeBackend()
	if withContext, ok := backend.(contextBackend); ok {
		return withContext.DecodeContext(ctx, encoded, output)
	}
	return backend.Decode(encoded, output)
}

func encodeWithContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, nearLossless bool) ([]byte, error) {
	backend := activeBackend()
	if withContext, ok := backend.(contextBackend); ok {
		return withContext.EncodeContext(ctx, raw, width, height, samples, bitsa, nearLossless)
	}
	return backend.Encode(raw, width, height, samples, bitsa, nearLossless)
}

func JLSdecode(jlsData []byte, jlsSize uint32, outputData []byte) error {
	return JLSdecodeContext(context.Background(), jlsData, jlsSize, outputData)
}

func JLSdecodeContext(ctx context.Context, jlsData []byte, jlsSize uint32, outputData []byte) error {
	if jlsSize > uint32(len(jlsData)) {
		return errInvalidJLSPayload
	}
	if jlsSize > maxCodecPayloadBytes || len(outputData) > maxCodecPayloadBytes {
		return errInvalidJLSPayload
	}
	return decodeWithContext(ctx, jlsData[:jlsSize], outputData)
}

func JLSencode(rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, nearLossless bool) error {
	return JLSencodeContext(context.Background(), rawData, width, height, samples, bitsa, outData, outSize, nearLossless)
}

func JLSencodeContext(ctx context.Context, rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, nearLossless bool) error {
	if outData == nil || outSize == nil {
		return errors.New("nil output pointers")
	}
	if len(rawData) > maxCodecPayloadBytes {
		return errInvalidJLSPayload
	}

	encoded, err := encodeWithContext(ctx, rawData, width, height, samples, bitsa, nearLossless)
	if err != nil {
		return err
	}
	*outData = append((*outData)[:0], encoded...)
	*outSize = len(encoded)
	return nil
}
