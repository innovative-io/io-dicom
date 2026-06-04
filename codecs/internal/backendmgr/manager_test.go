package backendmgr

import (
	"errors"
	"testing"
)

type testBackend struct {
	name  string
	ready error
}

func (b testBackend) Name() string { return b.name }
func (b testBackend) Ready() error { return b.ready }

func newPassthrough() testBackend { return testBackend{name: "passthrough"} }
func newNative() testBackend      { return testBackend{name: "native"} }

type simpleBackend struct{ name string }

func (b simpleBackend) Name() string { return b.name }

func newSimplePassthrough() simpleBackend { return simpleBackend{name: "passthrough"} }

func TestNew_SeedsPassthrough(t *testing.T) {
	m := New(newPassthrough)
	if m.BackendName() != "passthrough" {
		t.Fatalf("want passthrough, got %s", m.BackendName())
	}
	if m.Active().Name() != "passthrough" {
		t.Fatal("Active() should be passthrough")
	}
	avail := m.Available()
	if len(avail) != 1 || avail[0] != "passthrough" {
		t.Fatalf("Available() want [passthrough], got %v", avail)
	}
}

func TestRegister_AddsBackend(t *testing.T) {
	m := New(newPassthrough)
	if err := m.Register("native", newNative); err != nil {
		t.Fatal(err)
	}
	if len(m.Available()) != 2 {
		t.Fatalf("want 2 backends, got %v", m.Available())
	}
}

func TestRegister_DuplicateReturnsError(t *testing.T) {
	m := New(newPassthrough)
	_ = m.Register("native", newNative)
	if err := m.Register("native", newNative); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegister_EmptyNameReturnsError(t *testing.T) {
	m := New(newPassthrough)
	if err := m.Register("", newNative); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRegister_NilFactoryReturnsError(t *testing.T) {
	m := New(newPassthrough)
	if err := m.Register("native", nil); err == nil {
		t.Fatal("expected error for nil factory")
	}
}

func TestUse_SwitchesBackend(t *testing.T) {
	m := New(newPassthrough)
	_ = m.Register("native", newNative)
	if err := m.Use("native"); err != nil {
		t.Fatal(err)
	}
	if m.BackendName() != "native" {
		t.Fatalf("want native, got %s", m.BackendName())
	}
	if m.Active().Name() != "native" {
		t.Fatal("Active() should be native")
	}
}

func TestUse_UnknownReturnsError(t *testing.T) {
	m := New(newPassthrough)
	if err := m.Use("unknown"); err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestSet_OverridesBackend(t *testing.T) {
	m := New(newPassthrough)
	m.Set(testBackend{name: "custom"})
	if m.BackendName() != "custom" {
		t.Fatalf("want custom, got %s", m.BackendName())
	}
}

func TestReset_ReturnsToPassthrough(t *testing.T) {
	m := New(newPassthrough)
	_ = m.Register("native", newNative)
	_ = m.Use("native")
	m.Reset()
	if m.BackendName() != "passthrough" {
		t.Fatalf("want passthrough after reset, got %s", m.BackendName())
	}
}

func TestAvailable_Sorted(t *testing.T) {
	m := New(newPassthrough)
	_ = m.Register("zebra", newNative)
	_ = m.Register("alpha", newNative)
	avail := m.Available()
	if avail[0] != "alpha" || avail[1] != "passthrough" || avail[2] != "zebra" {
		t.Fatalf("Available() not sorted: %v", avail)
	}
}

func TestValidate_CurrentReady(t *testing.T) {
	m := New(newPassthrough)
	if err := m.Validate("passthrough"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_CurrentNotReady(t *testing.T) {
	readyErr := errors.New("dependency missing")
	m := New(func() testBackend { return testBackend{name: "passthrough", ready: readyErr} })
	if err := m.Validate("passthrough"); !errors.Is(err, readyErr) {
		t.Fatalf("want readyErr, got %v", err)
	}
}

func TestValidate_OtherBackendReady(t *testing.T) {
	m := New(newPassthrough)
	_ = m.Register("native", newNative)
	if err := m.Validate("native"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_UnknownReturnsError(t *testing.T) {
	m := New(newPassthrough)
	if err := m.Validate("absent"); err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestValidate_NoReadinessChecker(t *testing.T) {
	m := New(newSimplePassthrough)
	if err := m.Validate("passthrough"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelectDefault_OneNativeBecomesActive(t *testing.T) {
	m := New(newPassthrough)
	_ = m.Register("native", newNative)
	m.SelectDefault()
	if m.BackendName() != "native" {
		t.Fatalf("want native after SelectDefault, got %s", m.BackendName())
	}
}

func TestSelectDefault_EqualPriorityPicksDeterministically(t *testing.T) {
	m := New(newPassthrough)
	_ = m.Register("native2", newNative)
	_ = m.Register("native1", newNative)
	m.SelectDefault()
	// Equal priority: ties break by name, so the alphabetically-first wins
	// regardless of registration order.
	if m.BackendName() != "native1" {
		t.Fatalf("want native1 on equal-priority tie, got %s", m.BackendName())
	}
}

func TestSelectDefault_HigherPriorityWins(t *testing.T) {
	m := New(newPassthrough)
	_ = m.RegisterWithPriority("purego", newNative, 0)
	_ = m.RegisterWithPriority("cgo", newNative, 10)
	m.SelectDefault()
	if m.BackendName() != "cgo" {
		t.Fatalf("want cgo (higher priority) selected, got %s", m.BackendName())
	}
}

func TestSelectDefault_LowerPriorityWinsWhenAlone(t *testing.T) {
	// Models charls being removed: only the pure-Go backend remains, so it is
	// selected even though it registered at the lower priority.
	m := New(newPassthrough)
	_ = m.RegisterWithPriority("purego", newNative, 0)
	m.SelectDefault()
	if m.BackendName() != "purego" {
		t.Fatalf("want purego selected when alone, got %s", m.BackendName())
	}
}

func TestSelectDefault_NoNativeStaysPassthrough(t *testing.T) {
	m := New(newPassthrough)
	m.SelectDefault()
	if m.BackendName() != "passthrough" {
		t.Fatalf("want passthrough with no natives, got %s", m.BackendName())
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	m := New(newPassthrough)
	_ = m.Register("native", newNative)
	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				_ = m.Active().Name()
				_ = m.BackendName()
				_ = m.Available()
			}
			done <- struct{}{}
		}()
	}
	go func() {
		for j := 0; j < 1000; j++ {
			_ = m.Use("native")
			m.Reset()
		}
		done <- struct{}{}
	}()
	for i := 0; i < 5; i++ {
		<-done
	}
}
