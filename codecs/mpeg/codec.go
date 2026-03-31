package mpeg

import (
	"context"
	"errors"

	"github.com/innovative-io/io-dicom/codecs/internal/backendmgr"
)

var CGOEnabled = nativeBackendEnabled

const maxCodecPayloadBytes = 512 << 20

var errUnsupportedTransferSyntax = errors.New("unsupported MPEG/HEVC transfer syntax")
var errBackendUnavailable = errors.New("mpeg/hevc decode requires the ffmpeg native backend (build with -tags ffmpeg)")

var supportedTransferSyntaxUIDs = []string{
	"1.2.840.10008.1.2.4.100",
	"1.2.840.10008.1.2.4.100.1",
	"1.2.840.10008.1.2.4.101",
	"1.2.840.10008.1.2.4.101.1",
	"1.2.840.10008.1.2.4.102",
	"1.2.840.10008.1.2.4.102.1",
	"1.2.840.10008.1.2.4.103",
	"1.2.840.10008.1.2.4.103.1",
	"1.2.840.10008.1.2.4.104",
	"1.2.840.10008.1.2.4.104.1",
	"1.2.840.10008.1.2.4.105",
	"1.2.840.10008.1.2.4.105.1",
	"1.2.840.10008.1.2.4.106",
	"1.2.840.10008.1.2.4.106.1",
	"1.2.840.10008.1.2.4.107",
	"1.2.840.10008.1.2.4.108",
}

// Backend defines an MPEG/HEVC implementation that can be plugged at runtime.
// The default backend keeps pure-Go passthrough behavior.
type Backend interface {
	Name() string
	SupportedTransferSyntaxUIDs() []string
	Decode(encoded []byte, output []byte, transferSyntaxUID string) error
	Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error)
}

type contextBackend interface {
	DecodeContext(ctx context.Context, encoded []byte, output []byte, transferSyntaxUID string) error
	EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error)
}

type passthroughBackend struct{}

func (passthroughBackend) Name() string {
	return "passthrough"
}

func (passthroughBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (passthroughBackend) Decode(_ []byte, _ []byte, _ string) error {
	return errBackendUnavailable
}

func (passthroughBackend) Encode(raw []byte, _ uint16, _ uint16, _ uint16, _ uint16, _ string) ([]byte, error) {
	encoded := make([]byte, len(raw))
	copy(encoded, raw)
	return encoded, nil
}

var mgr = backendmgr.New(func() Backend { return passthroughBackend{} })

func init() {
	registerNativeBackends()
	mgr.SelectDefault()
}

// SetBackend overrides the active MPEG backend. Passing nil resets to passthrough.
func SetBackend(backend Backend) {
	if backend == nil {
		mgr.Reset()
		return
	}
	mgr.Set(backend)
}

// BackendName returns the current MPEG backend name.
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

func decodeWithContext(ctx context.Context, encoded []byte, output []byte, transferSyntaxUID string) error {
	backend := activeBackend()
	if withContext, ok := backend.(contextBackend); ok {
		return withContext.DecodeContext(ctx, encoded, output, transferSyntaxUID)
	}
	return backend.Decode(encoded, output, transferSyntaxUID)
}

func encodeWithContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error) {
	backend := activeBackend()
	if withContext, ok := backend.(contextBackend); ok {
		return withContext.EncodeContext(ctx, raw, width, height, samples, bitsa, transferSyntaxUID)
	}
	return backend.Encode(raw, width, height, samples, bitsa, transferSyntaxUID)
}

func isSupportedMPEGUID(uid string) bool {
	switch uid {
	case "1.2.840.10008.1.2.4.100",
		"1.2.840.10008.1.2.4.100.1",
		"1.2.840.10008.1.2.4.101",
		"1.2.840.10008.1.2.4.101.1",
		"1.2.840.10008.1.2.4.102",
		"1.2.840.10008.1.2.4.102.1",
		"1.2.840.10008.1.2.4.103",
		"1.2.840.10008.1.2.4.103.1",
		"1.2.840.10008.1.2.4.104",
		"1.2.840.10008.1.2.4.104.1",
		"1.2.840.10008.1.2.4.105",
		"1.2.840.10008.1.2.4.105.1",
		"1.2.840.10008.1.2.4.106",
		"1.2.840.10008.1.2.4.106.1",
		"1.2.840.10008.1.2.4.107",
		"1.2.840.10008.1.2.4.108":
		return true
	default:
		return false
	}
}

func MPEGdecode(videoData []byte, videoSize uint32, outputData []byte, transferSyntaxUID string) error {
	return MPEGdecodeContext(context.Background(), videoData, videoSize, outputData, transferSyntaxUID)
}

func MPEGdecodeContext(ctx context.Context, videoData []byte, videoSize uint32, outputData []byte, transferSyntaxUID string) error {
	if !isSupportedMPEGUID(transferSyntaxUID) {
		return errUnsupportedTransferSyntax
	}
	if videoSize > uint32(len(videoData)) {
		return errors.New("invalid MPEG payload size")
	}
	if videoSize > maxCodecPayloadBytes || len(outputData) > maxCodecPayloadBytes {
		return errors.New("invalid MPEG payload size")
	}
	return decodeWithContext(ctx, videoData[:videoSize], outputData, transferSyntaxUID)
}

func MPEGencode(rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, transferSyntaxUID string) error {
	return MPEGencodeContext(context.Background(), rawData, width, height, samples, bitsa, outData, outSize, transferSyntaxUID)
}

func MPEGencodeContext(ctx context.Context, rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, transferSyntaxUID string) error {
	if !isSupportedMPEGUID(transferSyntaxUID) {
		return errUnsupportedTransferSyntax
	}
	if outData == nil || outSize == nil {
		return errors.New("nil output pointers")
	}
	if len(rawData) > maxCodecPayloadBytes {
		return errors.New("invalid MPEG payload size")
	}

	encoded, err := encodeWithContext(ctx, rawData, width, height, samples, bitsa, transferSyntaxUID)
	if err != nil {
		return err
	}
	*outData = append((*outData)[:0], encoded...)
	*outSize = len(encoded)
	return nil
}
