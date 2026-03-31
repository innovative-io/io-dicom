package jpeg2000

import (
	"context"
	"errors"
	"sort"
	"sync"
)

var CGOEnabled = nativeBackendEnabled

const maxCodecPayloadBytes = 512 << 20

var errInvalidJ2KPayload = errors.New("invalid JPEG 2000 payload size")
var errBackendUnavailable = errors.New("jpeg 2000 decode requires the openjpeg native backend (build with -tags openjpeg)")

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

func (passthroughBackend) Decode(encoded []byte, output []byte) error {
	_ = encoded
	_ = output
	return errBackendUnavailable
}

func (passthroughBackend) Encode(raw []byte, _ uint16, _ uint16, _ uint16, _ uint16, _ int) ([]byte, error) {
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

// SetBackend overrides the active JPEG 2000 backend. Passing nil resets to default passthrough behavior.
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

// BackendName returns the current JPEG 2000 backend name.
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
	*outData = append((*outData)[:0], encoded...)
	*outSize = len(encoded)
	return nil
}
