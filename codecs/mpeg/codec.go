package mpeg

import (
	"context"
	"errors"
	"sort"
	"sync"
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

type readinessBackend interface {
	Ready() error
}

type passthroughBackend struct{}

func (passthroughBackend) Name() string {
	return "passthrough"
}

func (passthroughBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (passthroughBackend) Decode(encoded []byte, output []byte, _ string) error {
	_ = encoded
	_ = output
	return errBackendUnavailable
}

func (passthroughBackend) Encode(raw []byte, _ uint16, _ uint16, _ uint16, _ uint16, _ string) ([]byte, error) {
	encoded := make([]byte, len(raw))
	copy(encoded, raw)
	return encoded, nil
}

var (
	backendMu        sync.RWMutex
	currentBackend   Backend = passthroughBackend{}
	currentName              = "passthrough"
	backendFactories         = map[string]func() Backend{
		"passthrough": func() Backend { return passthroughBackend{} },
	}
)

func init() {
	registerNativeBackends()
	selectDefaultBackend()
}

func selectDefaultBackend() {
	backendMu.Lock()
	defer backendMu.Unlock()

	preferred := preferredBackendNameLocked()
	if preferred == "passthrough" {
		return
	}
	factory := backendFactories[preferred]
	if factory == nil {
		return
	}
	backend := factory()
	if backend == nil {
		return
	}
	currentBackend = backend
	currentName = preferred
}

func preferredBackendNameLocked() string {
	nativeNames := make([]string, 0, len(backendFactories))
	for name := range backendFactories {
		if name == "passthrough" {
			continue
		}
		nativeNames = append(nativeNames, name)
	}
	if len(nativeNames) != 1 {
		return "passthrough"
	}
	return nativeNames[0]
}

// SetBackend overrides the active MPEG backend. Passing nil resets to default passthrough behavior.
func SetBackend(backend Backend) {
	backendMu.Lock()
	defer backendMu.Unlock()
	if backend == nil {
		currentBackend = passthroughBackend{}
		currentName = "passthrough"
		return
	}
	currentBackend = backend
	currentName = backend.Name()
}

// BackendName returns the current MPEG backend name.
func BackendName() string {
	backendMu.RLock()
	defer backendMu.RUnlock()
	return currentName
}

// RegisterBackend registers a named backend factory for runtime selection.
func RegisterBackend(name string, factory func() Backend) error {
	if name == "" {
		return errors.New("backend name is required")
	}
	if factory == nil {
		return errors.New("backend factory is required")
	}

	backendMu.Lock()
	defer backendMu.Unlock()
	if _, exists := backendFactories[name]; exists {
		return errors.New("backend already registered")
	}
	backendFactories[name] = factory
	return nil
}

// UseBackend switches to a previously registered backend by name.
func UseBackend(name string) error {
	backendMu.Lock()
	defer backendMu.Unlock()
	factory, exists := backendFactories[name]
	if !exists {
		return errors.New("backend not registered")
	}
	backend := factory()
	if backend == nil {
		return errors.New("backend factory returned nil")
	}
	currentBackend = backend
	currentName = name
	return nil
}

// AvailableBackends returns the list of registered backend names.
func AvailableBackends() []string {
	backendMu.RLock()
	defer backendMu.RUnlock()
	names := make([]string, 0, len(backendFactories))
	for name := range backendFactories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func ValidateBackend(name string) error {
	backendMu.RLock()
	if name == currentName {
		backend := currentBackend
		backendMu.RUnlock()
		if ready, ok := backend.(readinessBackend); ok {
			return ready.Ready()
		}
		return nil
	}
	factory, exists := backendFactories[name]
	backendMu.RUnlock()
	if !exists {
		return errors.New("backend not registered")
	}
	backend := factory()
	if backend == nil {
		return errors.New("backend factory returned nil")
	}
	if ready, ok := backend.(readinessBackend); ok {
		return ready.Ready()
	}
	return nil
}

func activeBackend() Backend {
	backendMu.RLock()
	defer backendMu.RUnlock()
	return currentBackend
}

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
