package jpeg2000

import (
	"errors"
	"sort"
	"sync"
)

var CGOEnabled = nativeBackendEnabled

var errInvalidJ2KPayload = errors.New("invalid JPEG 2000 payload size")

// Backend defines a JPEG 2000 implementation that can be plugged at runtime.
// The default backend keeps pure-Go passthrough behavior.
type Backend interface {
	Name() string
	Decode(encoded []byte, output []byte) error
	Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, ratio int) ([]byte, error)
}

type passthroughBackend struct{}

func (passthroughBackend) Name() string {
	return "passthrough"
}

func (passthroughBackend) Decode(encoded []byte, output []byte) error {
	if len(encoded) > len(output) {
		return errInvalidJ2KPayload
	}
	copy(output[:len(encoded)], encoded)
	return nil
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

func activeBackend() Backend {
	backendMu.RLock()
	defer backendMu.RUnlock()
	return currentBackend
}

func J2Kdecode(j2kData []byte, j2kSize uint32, outputData []byte) error {
	if j2kSize > uint32(len(j2kData)) {
		return errInvalidJ2KPayload
	}
	return activeBackend().Decode(j2kData[:j2kSize], outputData)
}

func J2Kencode(rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, ratio int) error {
	if outData == nil || outSize == nil {
		return errors.New("nil output pointers")
	}

	encoded, err := activeBackend().Encode(rawData, width, height, samples, bitsa, ratio)
	if err != nil {
		return err
	}
	*outData = append((*outData)[:0], encoded...)
	*outSize = len(encoded)
	return nil
}
