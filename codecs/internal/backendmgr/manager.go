// Package backendmgr provides a generic, thread-safe backend lifecycle manager
// for codec families. Each codec package creates one Manager instance and
// delegates its SetBackend / UseBackend / RegisterBackend / etc. calls to it,
// eliminating ~100 lines of identical boilerplate per package.
package backendmgr

import (
	"errors"
	"sort"
	"sync"
)

// Namer is the minimum constraint for a managed backend.
type Namer interface {
	Name() string
}

// ReadinessChecker is an optional interface a backend may implement to report
// whether its native dependencies are available.
type ReadinessChecker interface {
	Ready() error
}

// Manager provides thread-safe lifecycle management for a pluggable codec
// backend. B must be an interface type whose zero value is nil (as is true for
// all codec Backend interfaces in this project).
type Manager[B Namer] struct {
	mu              sync.RWMutex
	current         B
	currentName     string
	passthroughName string
	passthrough     func() B
	factories       map[string]func() B
}

// New creates a Manager pre-seeded with the passthrough backend.
func New[B Namer](passthroughFactory func() B) *Manager[B] {
	pt := passthroughFactory()
	name := pt.Name()
	return &Manager[B]{
		current:         pt,
		currentName:     name,
		passthroughName: name,
		passthrough:     passthroughFactory,
		factories:       map[string]func() B{name: passthroughFactory},
	}
}

// Register adds a named factory. Returns an error if the name is already taken.
func (m *Manager[B]) Register(name string, factory func() B) error {
	if name == "" {
		return errors.New("backend name is required")
	}
	if factory == nil {
		return errors.New("backend factory is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.factories[name]; exists {
		return errors.New("backend already registered")
	}
	m.factories[name] = factory
	return nil
}

// Use switches to a previously registered backend.
func (m *Manager[B]) Use(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	factory, exists := m.factories[name]
	if !exists {
		return errors.New("backend not registered")
	}
	backend := factory()
	if any(backend) == nil {
		return errors.New("backend factory returned nil")
	}
	m.current = backend
	m.currentName = name
	return nil
}

// Set overrides the active backend directly.
// Callers that want to reset to passthrough should call Reset instead.
func (m *Manager[B]) Set(backend B) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = backend
	m.currentName = backend.Name()
}

// Reset reverts to the passthrough backend.
func (m *Manager[B]) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = m.passthrough()
	m.currentName = m.passthroughName
}

// Active returns the currently active backend.
func (m *Manager[B]) Active() B {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// BackendName returns the name of the currently active backend.
func (m *Manager[B]) BackendName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentName
}

// Available returns sorted names of all registered backends.
func (m *Manager[B]) Available() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.factories))
	for name := range m.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Validate checks whether a named backend is ready for use.
func (m *Manager[B]) Validate(name string) error {
	m.mu.RLock()
	if name == m.currentName {
		backend := m.current
		m.mu.RUnlock()
		if ready, ok := any(backend).(ReadinessChecker); ok {
			return ready.Ready()
		}
		return nil
	}
	factory, exists := m.factories[name]
	m.mu.RUnlock()
	if !exists {
		return errors.New("backend not registered")
	}
	backend := factory()
	if any(backend) == nil {
		return errors.New("backend factory returned nil")
	}
	if ready, ok := any(backend).(ReadinessChecker); ok {
		return ready.Ready()
	}
	return nil
}

// SelectDefault picks the single non-passthrough registered backend, if exactly
// one exists; otherwise leaves the current selection unchanged.
func (m *Manager[B]) SelectDefault() {
	m.mu.Lock()
	defer m.mu.Unlock()
	preferred := m.preferredLocked()
	if preferred == m.passthroughName {
		return
	}
	factory := m.factories[preferred]
	if factory == nil {
		return
	}
	backend := factory()
	if any(backend) == nil {
		return
	}
	m.current = backend
	m.currentName = preferred
}

func (m *Manager[B]) preferredLocked() string {
	var nativeNames []string
	for name := range m.factories {
		if name == m.passthroughName {
			continue
		}
		nativeNames = append(nativeNames, name)
	}
	if len(nativeNames) != 1 {
		return m.passthroughName
	}
	return nativeNames[0]
}
