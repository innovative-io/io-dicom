package jpip

import (
	"errors"
	"sort"
	"sync"
)

var CGOEnabled = nativeBackendEnabled

var errUnsupportedTransferSyntax = errors.New("unsupported JPIP transfer syntax")

// Backend defines a JPIP implementation that can be plugged at runtime.
// The default backend keeps pure-Go passthrough behavior.
type Backend interface {
	Name() string
	Decode(encoded []byte, output []byte, transferSyntaxUID string) error
	Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error)
}

type passthroughBackend struct{}

func (passthroughBackend) Name() string {
	return "passthrough"
}

func (passthroughBackend) Decode(encoded []byte, output []byte, _ string) error {
	if len(encoded) > len(output) {
		return errors.New("invalid JPIP payload size")
	}
	copy(output[:len(encoded)], encoded)
	return nil
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
}

// SetBackend overrides the active JPIP backend. Passing nil resets to default passthrough behavior.
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

// BackendName returns the current JPIP backend name.
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

func isSupportedJPIPUID(uid string) bool {
	switch uid {
	case "1.2.840.10008.1.2.4.204", "1.2.840.10008.1.2.4.205":
		return true
	default:
		return false
	}
}

func JPIPdecode(streamData []byte, streamSize uint32, outputData []byte, transferSyntaxUID string) error {
	if !isSupportedJPIPUID(transferSyntaxUID) {
		return errUnsupportedTransferSyntax
	}
	if streamSize > uint32(len(streamData)) {
		return errors.New("invalid JPIP payload size")
	}
	return activeBackend().Decode(streamData[:streamSize], outputData, transferSyntaxUID)
}

func JPIPencode(rawData []byte, width uint16, height uint16, samples uint16, bitsa uint16, outData *[]byte, outSize *int, transferSyntaxUID string) error {
	if !isSupportedJPIPUID(transferSyntaxUID) {
		return errUnsupportedTransferSyntax
	}
	if outData == nil || outSize == nil {
		return errors.New("nil output pointers")
	}

	encoded, err := activeBackend().Encode(rawData, width, height, samples, bitsa, transferSyntaxUID)
	if err != nil {
		return err
	}
	*outData = append((*outData)[:0], encoded...)
	*outSize = len(encoded)
	return nil
}
