package jpegxl

import "testing"

type mockBackend struct {
	name         string
	decodedBytes int
	encodedBytes []byte
}

func (m *mockBackend) Name() string {
	return m.name
}

func (m *mockBackend) Decode(encoded []byte, output []byte) error {
	if len(output) < m.decodedBytes {
		return errInvalidJXLPayload
	}
	for i := 0; i < m.decodedBytes; i++ {
		output[i] = byte(100 + i)
	}
	_ = encoded
	return nil
}

func (m *mockBackend) Encode(raw []byte, _ uint16, _ uint16, _ uint16, _ uint16, _ bool) ([]byte, error) {
	_ = raw
	out := make([]byte, len(m.encodedBytes))
	copy(out, m.encodedBytes)
	return out, nil
}

func TestJXLPassthroughRoundTrip(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if CGOEnabled != nativeBackendEnabled {
		t.Fatalf("unexpected CGOEnabled value: got %v want %v", CGOEnabled, nativeBackendEnabled)
	}

	raw := []byte{1, 2, 3, 4, 5, 6}
	var out []byte
	var outSize int
	if err := JXLencode(raw, 2, 1, 3, 8, &out, &outSize, true); err != nil {
		t.Fatalf("unexpected JXLencode error: %v", err)
	}
	if outSize != len(raw) {
		t.Fatalf("unexpected JXL encoded size: got %d, want %d", outSize, len(raw))
	}

	decoded := make([]byte, len(raw))
	if err := JXLdecode(out, uint32(outSize), decoded); err != nil {
		t.Fatalf("unexpected JXLdecode error: %v", err)
	}
	for i := range raw {
		if decoded[i] != raw[i] {
			t.Fatalf("decoded[%d]=%d, want %d", i, decoded[i], raw[i])
		}
	}
}

func TestJXLDecodeSizeValidation(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := JXLdecode([]byte{1, 2, 3}, 4, make([]byte, 4)); err == nil {
		t.Fatal("expected invalid payload size error when source is too small")
	}
	if err := JXLdecode([]byte{1, 2, 3}, 3, make([]byte, 2)); err == nil {
		t.Fatal("expected invalid payload size error when output is too small")
	}
}

func TestJXLEncodeNilOutput(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	var out []byte
	var outSize int
	if err := JXLencode([]byte{1, 2, 3}, 1, 1, 1, 8, nil, &outSize, true); err == nil {
		t.Fatal("expected nil output pointer error")
	}
	if err := JXLencode([]byte{1, 2, 3}, 1, 1, 1, 8, &out, nil, true); err == nil {
		t.Fatal("expected nil output size pointer error")
	}
}

func TestJXLCustomBackend(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	mock := &mockBackend{name: "mock-jxl", decodedBytes: 3, encodedBytes: []byte{3, 2, 1}}
	SetBackend(mock)

	if BackendName() != "mock-jxl" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	var out []byte
	var outSize int
	if err := JXLencode([]byte{1, 2, 3, 4}, 2, 2, 1, 8, &out, &outSize, true); err != nil {
		t.Fatalf("unexpected JXLencode error with custom backend: %v", err)
	}
	if outSize != 3 {
		t.Fatalf("unexpected encoded size from custom backend: got %d, want 3", outSize)
	}
	if len(out) != 3 || out[0] != 3 || out[1] != 2 || out[2] != 1 {
		t.Fatalf("unexpected encoded payload from custom backend: %v", out)
	}

	decoded := make([]byte, 3)
	if err := JXLdecode(out, uint32(len(out)), decoded); err != nil {
		t.Fatalf("unexpected JXLdecode error with custom backend: %v", err)
	}
	if decoded[0] != 100 || decoded[1] != 101 || decoded[2] != 102 {
		t.Fatalf("unexpected decoded payload from custom backend: %v", decoded)
	}
}

func TestJXLBackendRegistry(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	err := RegisterBackend("unit-mock-jxl", func() Backend {
		return &mockBackend{name: "unit-mock-jxl", decodedBytes: 2, encodedBytes: []byte{2, 3}}
	})
	if err != nil {
		t.Fatalf("RegisterBackend failed: %v", err)
	}

	if err := UseBackend("unit-mock-jxl"); err != nil {
		t.Fatalf("UseBackend failed: %v", err)
	}
	if BackendName() != "unit-mock-jxl" {
		t.Fatalf("unexpected backend name after UseBackend: %s", BackendName())
	}

	found := false
	for _, name := range AvailableBackends() {
		if name == "unit-mock-jxl" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected registered backend in AvailableBackends")
	}
}

func TestJXLNativeBackendRegistrationMatchesFlag(t *testing.T) {
	foundLibJXL := false
	for _, name := range AvailableBackends() {
		if name == "libjxl" {
			foundLibJXL = true
			break
		}
	}

	if CGOEnabled && !foundLibJXL {
		t.Fatal("expected libjxl backend when CGOEnabled is true")
	}
	if !CGOEnabled && foundLibJXL {
		t.Fatal("did not expect libjxl backend when CGOEnabled is false")
	}
}
