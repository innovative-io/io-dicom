package jpeg2000

import (
	"context"
	"errors"
	"log/slog"

	"github.com/innovative-io/io-dicom/codecs/internal/backendmgr"
)

// CGOEnabled reports whether a native (cgo) JPEG 2000 backend is compiled in.
// There is none anymore — decode and lossless encode are pure Go — so it is
// always false. Retained for API compatibility with callers/tests.
var CGOEnabled = nativeBackendEnabled

const maxCodecPayloadBytes = 512 << 20

var errInvalidJ2KPayload = errors.New("invalid JPEG 2000 payload size")
var errBackendUnavailable = errors.New("jpeg 2000: no decode backend available")

var supportedTransferSyntaxUIDs = []string{
	"1.2.840.10008.1.2.4.90",
	"1.2.840.10008.1.2.4.91",
	"1.2.840.10008.1.2.4.92",
	"1.2.840.10008.1.2.4.93",
	"1.2.840.10008.1.2.4.201",
	"1.2.840.10008.1.2.4.202",
	"1.2.840.10008.1.2.4.203",
}

// Backend defines a JPEG 2000 implementation that can be plugged at runtime.
// The default backend keeps pure-Go passthrough behavior.
type Backend interface {
	Name() string
	SupportedTransferSyntaxUIDs() []string
	Decode(encoded []byte, output []byte) error
	Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, ratio int) ([]byte, error)
}

type contextBackend interface {
	DecodeContext(ctx context.Context, encoded []byte, output []byte) error
	EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, ratio int) ([]byte, error)
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

func (passthroughBackend) Encode(_ []byte, _ uint16, _ uint16, _ uint16, _ uint16, _ int) ([]byte, error) {
	// No pure-Go JPEG 2000 encoder exists; erroring is correct rather than
	// returning the raw bytes as if they were a valid compressed stream.
	return nil, errBackendUnavailable
}

var mgr = backendmgr.New(func() Backend { return passthroughBackend{} })

func init() {
	registerNativeBackends() // no-op (no native backend); kept for symmetry
	registerPureGoBackend()  // pure-Go gojpeg2000 (decode + lossless 5/3 encode)
	mgr.SelectDefault()
	if mgr.BackendName() == "passthrough" {
		slog.Warn("jpeg2000: no decode backend available")
	}
}

// SetBackend overrides the active JPEG 2000 backend. Passing nil resets to the
// default (the pure-Go gojpeg2000 backend).
func SetBackend(backend Backend) {
	if backend == nil {
		mgr.Reset()
		mgr.SelectDefault()
		return
	}
	mgr.Set(backend)
}

// BackendName returns the current JPEG 2000 backend name.
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

func encodeWithContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, ratio int) ([]byte, error) {
	backend := activeBackend()
	if withContext, ok := backend.(contextBackend); ok {
		return withContext.EncodeContext(ctx, raw, width, height, samples, bitsa, ratio)
	}
	return backend.Encode(raw, width, height, samples, bitsa, ratio)
}

func J2Kdecode(j2kData []byte, j2kSize uint32, outputData []byte) error {
	return J2KdecodeContext(context.Background(), j2kData, j2kSize, outputData)
}

func J2KdecodeContext(ctx context.Context, j2kData []byte, j2kSize uint32, outputData []byte) error {
	if j2kSize > uint32(len(j2kData)) {
		return errInvalidJ2KPayload
	}
	if j2kSize > maxCodecPayloadBytes || len(outputData) > maxCodecPayloadBytes {
		return errInvalidJ2KPayload
	}
	return decodeWithContext(ctx, j2kData[:j2kSize], outputData)
}

func J2Kencode(rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, ratio int) error {
	return J2KencodeContext(context.Background(), rawData, width, height, samples, bitsa, outData, outSize, ratio)
}

func J2KencodeContext(ctx context.Context, rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, ratio int) error {
	if outData == nil || outSize == nil {
		return errors.New("nil output pointers")
	}
	if len(rawData) > maxCodecPayloadBytes {
		return errInvalidJ2KPayload
	}

	encoded, err := encodeWithContext(ctx, rawData, width, height, samples, bitsa, ratio)
	if err != nil {
		return err
	}
	// encoded is already a freshly allocated slice (C.GoBytes or the passthrough
	// backend's copy), so take ownership directly instead of copying it again.
	*outData = encoded
	*outSize = len(encoded)
	return nil
}
