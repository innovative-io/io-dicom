package jpegxl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/innovative-io/io-dicom/codecs/internal/backendmgr"
)

// CGOEnabled reports whether a native cgo JPEG XL backend is compiled in. The
// libjxl backend was retired in favor of the pure-Go gojpegxl codec, so this is
// always false; it is retained for API compatibility.
const CGOEnabled = false

const maxCodecPayloadBytes = 512 << 20

// Encoder effort bounds: 1 = fastest/largest, 10 = slowest/smallest. Retained
// for API compatibility; the pure-Go backend does not vary its output by effort,
// so this is currently advisory only.
const (
	minEncodeEffort     = 1
	maxEncodeEffort     = 10
	defaultEncodeEffort = 7
)

var jxlEncodeEffort = func() *atomic.Int32 {
	v := new(atomic.Int32)
	v.Store(defaultEncodeEffort)
	return v
}()

// SetEncodeEffort sets the JPEG XL encoder effort (1 = fastest, largest output;
// 10 = slowest, smallest output). It returns an error for out-of-range values.
// Effort does not affect correctness: lossless encodes stay lossless.
func SetEncodeEffort(effort int) error {
	if effort < minEncodeEffort || effort > maxEncodeEffort {
		return fmt.Errorf("jpegxl: encode effort %d out of range [%d,%d]", effort, minEncodeEffort, maxEncodeEffort)
	}
	jxlEncodeEffort.Store(int32(effort))
	return nil
}

// EncodeEffort returns the current JPEG XL encoder effort.
func EncodeEffort() int { return int(jxlEncodeEffort.Load()) }

var errInvalidJXLPayload = errors.New("invalid JPEG XL payload size")
var errBackendUnavailable = errors.New("no JPEG XL decode backend available")

var supportedTransferSyntaxUIDs = []string{
	"1.2.840.10008.1.2.4.110",
	"1.2.840.10008.1.2.4.111",
	"1.2.840.10008.1.2.4.112",
}

// Backend defines a JPEG XL implementation that can be plugged at runtime.
// The default backend keeps pure-Go passthrough behavior.
type Backend interface {
	Name() string
	SupportedTransferSyntaxUIDs() []string
	Decode(encoded []byte, output []byte) error
	Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, lossless bool) ([]byte, error)
}

type contextBackend interface {
	DecodeContext(ctx context.Context, encoded []byte, output []byte) error
	EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, lossless bool) ([]byte, error)
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

func (passthroughBackend) Encode(_ []byte, _ uint16, _ uint16, _ uint16, _ uint16, _ bool) ([]byte, error) {
	// No pure-Go JPEG XL encoder exists; erroring is correct rather than
	// returning the raw bytes as if they were a valid compressed stream.
	return nil, errBackendUnavailable
}

var mgr = backendmgr.New(func() Backend { return passthroughBackend{} })

func init() {
	registerPureGoBackend() // pure-Go gojpegxl is the only built-in backend
	mgr.SelectDefault()
	if mgr.BackendName() == "passthrough" {
		slog.Warn("jpegxl: no decode backend available")
	}
}

// SetBackend overrides the active JPEG XL backend. Passing nil resets to the
// default (pure-Go gojpegxl).
func SetBackend(backend Backend) {
	if backend == nil {
		// Reset to the default (pure-Go gojpegxl) rather than the passthrough
		// fallback, since gojpegxl is the only built-in backend after libjxl retired.
		mgr.Reset()
		mgr.SelectDefault()
		return
	}
	mgr.Set(backend)
}

// BackendName returns the current JPEG XL backend name.
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

func encodeWithContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, lossless bool) ([]byte, error) {
	backend := activeBackend()
	if withContext, ok := backend.(contextBackend); ok {
		return withContext.EncodeContext(ctx, raw, width, height, samples, bitsa, lossless)
	}
	return backend.Encode(raw, width, height, samples, bitsa, lossless)
}

func JXLdecode(jxlData []byte, jxlSize uint32, outputData []byte) error {
	return JXLdecodeContext(context.Background(), jxlData, jxlSize, outputData)
}

func JXLdecodeContext(ctx context.Context, jxlData []byte, jxlSize uint32, outputData []byte) error {
	if jxlSize > uint32(len(jxlData)) {
		return errInvalidJXLPayload
	}
	if jxlSize > maxCodecPayloadBytes || len(outputData) > maxCodecPayloadBytes {
		return errInvalidJXLPayload
	}
	return decodeWithContext(ctx, jxlData[:jxlSize], outputData)
}

func JXLencode(rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, lossless bool) error {
	return JXLencodeContext(context.Background(), rawData, width, height, samples, bitsa, outData, outSize, lossless)
}

func JXLencodeContext(ctx context.Context, rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, lossless bool) error {
	if outData == nil || outSize == nil {
		return errors.New("nil output pointers")
	}
	if len(rawData) > maxCodecPayloadBytes {
		return errInvalidJXLPayload
	}

	encoded, err := encodeWithContext(ctx, rawData, width, height, samples, bitsa, lossless)
	if err != nil {
		return err
	}
	// encoded is already a freshly allocated slice (C.GoBytes or the passthrough
	// backend's copy), so take ownership directly instead of copying it again.
	*outData = encoded
	*outSize = len(encoded)
	return nil
}
