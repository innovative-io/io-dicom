package smpte2110

import "testing"

type mockBackend struct {
	name         string
	decodedBytes int
	encodedBytes []byte
}

func (m *mockBackend) Name() string {
	return m.name
}

func (m *mockBackend) Decode(encoded []byte, output []byte, _ string) error {
	if len(output) < m.decodedBytes {
		return errUnsupportedTransferSyntax
	}
	for i := 0; i < m.decodedBytes; i++ {
		output[i] = byte(60 + i)
	}
	_ = encoded
	return nil
}

func (m *mockBackend) Encode(raw []byte, _ uint16, _ uint16, _ uint16, _ uint16, _ string) ([]byte, error) {
	_ = raw
	out := make([]byte, len(m.encodedBytes))
	copy(out, m.encodedBytes)
	return out, nil
}

func TestCGOEnabled(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if CGOEnabled != nativeBackendEnabled {
		t.Fatalf("unexpected CGOEnabled value: got %v want %v", CGOEnabled, nativeBackendEnabled)
	}
}

func TestSMPTE2110RoundTrip(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	raw := []byte{1, 2, 3, 4, 5, 6}
	var out []byte
	var outSize int

	err := SMPTE2110encode(raw, 2, 1, 3, 8, &out, &outSize, "1.2.840.10008.1.2.7.1")
	if err != nil {
		t.Fatalf("SMPTE2110encode failed: %v", err)
	}
	if outSize != len(raw) {
		t.Fatalf("unexpected outSize: got %d want %d", outSize, len(raw))
	}

	decoded := make([]byte, len(raw))
	if err := SMPTE2110decode(out, uint32(outSize), decoded, "1.2.840.10008.1.2.7.1"); err != nil {
		t.Fatalf("SMPTE2110decode failed: %v", err)
	}

	for i := range raw {
		if decoded[i] != raw[i] {
			t.Fatalf("decoded[%d]=%d want=%d", i, decoded[i], raw[i])
		}
	}
}

func TestSMPTE2110InvalidUID(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	var out []byte
	var outSize int
	if err := SMPTE2110encode([]byte{1, 2}, 1, 1, 1, 8, &out, &outSize, "1.2.840.invalid"); err == nil {
		t.Fatal("expected SMPTE2110encode to fail with invalid transfer syntax")
	}
	if err := SMPTE2110decode([]byte{1, 2}, 2, make([]byte, 2), "1.2.840.invalid"); err == nil {
		t.Fatal("expected SMPTE2110decode to fail with invalid transfer syntax")
	}
}

func TestSMPTE2110SizeValidation(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := SMPTE2110decode([]byte{1, 2}, 3, make([]byte, 3), "1.2.840.10008.1.2.7.2"); err == nil {
		t.Fatal("expected decode to fail when streamSize exceeds input length")
	}
	if err := SMPTE2110decode([]byte{1, 2, 3}, 3, make([]byte, 2), "1.2.840.10008.1.2.7.3"); err == nil {
		t.Fatal("expected decode to fail when output is too small")
	}
}

func TestSMPTE2110CustomBackend(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	mock := &mockBackend{name: "mock-smpte", decodedBytes: 3, encodedBytes: []byte{6, 5, 4}}
	SetBackend(mock)

	if BackendName() != "mock-smpte" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	var out []byte
	var outSize int
	if err := SMPTE2110encode([]byte{1, 2, 3, 4}, 2, 2, 1, 8, &out, &outSize, "1.2.840.10008.1.2.7.1"); err != nil {
		t.Fatalf("unexpected SMPTE2110encode error with custom backend: %v", err)
	}
	if outSize != 3 {
		t.Fatalf("unexpected encoded size from custom backend: got %d, want 3", outSize)
	}
	if len(out) != 3 || out[0] != 6 || out[1] != 5 || out[2] != 4 {
		t.Fatalf("unexpected encoded payload from custom backend: %v", out)
	}

	decoded := make([]byte, 3)
	if err := SMPTE2110decode(out, uint32(len(out)), decoded, "1.2.840.10008.1.2.7.1"); err != nil {
		t.Fatalf("unexpected SMPTE2110decode error with custom backend: %v", err)
	}
	if decoded[0] != 60 || decoded[1] != 61 || decoded[2] != 62 {
		t.Fatalf("unexpected decoded payload from custom backend: %v", decoded)
	}
}

func TestSMPTE2110BackendRegistry(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	err := RegisterBackend("unit-mock-smpte", func() Backend {
		return &mockBackend{name: "unit-mock-smpte", decodedBytes: 2, encodedBytes: []byte{2, 1}}
	})
	if err != nil {
		t.Fatalf("RegisterBackend failed: %v", err)
	}

	if err := UseBackend("unit-mock-smpte"); err != nil {
		t.Fatalf("UseBackend failed: %v", err)
	}
	if BackendName() != "unit-mock-smpte" {
		t.Fatalf("unexpected backend name after UseBackend: %s", BackendName())
	}

	found := false
	for _, name := range AvailableBackends() {
		if name == "unit-mock-smpte" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected registered backend in AvailableBackends")
	}
}

func TestSMPTE2110NativeBackendRegistrationMatchesFlag(t *testing.T) {
	foundST2110 := false
	for _, name := range AvailableBackends() {
		if name == "st2110" {
			foundST2110 = true
			break
		}
	}

	if CGOEnabled && !foundST2110 {
		t.Fatal("expected st2110 backend when CGOEnabled is true")
	}
	if !CGOEnabled && foundST2110 {
		t.Fatal("did not expect st2110 backend when CGOEnabled is false")
	}
}
