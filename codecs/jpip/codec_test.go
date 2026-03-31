package jpip

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
		output[i] = byte(30 + i)
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

func TestJPIPRoundTrip(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	raw := []byte{10, 20, 30, 40}
	var out []byte
	var outSize int
	if err := JPIPencode(raw, 2, 2, 1, 8, &out, &outSize, "1.2.840.10008.1.2.4.204"); err != nil {
		t.Fatalf("JPIPencode failed: %v", err)
	}
	if outSize != len(raw) {
		t.Fatalf("unexpected outSize: got %d want %d", outSize, len(raw))
	}

	decoded := make([]byte, len(raw))
	if err := JPIPdecode(out, uint32(outSize), decoded, "1.2.840.10008.1.2.4.204"); err != nil {
		t.Fatalf("JPIPdecode failed: %v", err)
	}
	for i := range raw {
		if decoded[i] != raw[i] {
			t.Fatalf("decoded[%d]=%d want=%d", i, decoded[i], raw[i])
		}
	}
}

func TestJPIPInvalidUID(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	var out []byte
	var outSize int
	if err := JPIPencode([]byte{1, 2}, 1, 1, 1, 8, &out, &outSize, "1.2.840.invalid"); err == nil {
		t.Fatal("expected JPIPencode to fail with invalid transfer syntax")
	}
	if err := JPIPdecode([]byte{1, 2}, 2, make([]byte, 2), "1.2.840.invalid"); err == nil {
		t.Fatal("expected JPIPdecode to fail with invalid transfer syntax")
	}
}

func TestJPIPSizeValidation(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := JPIPdecode([]byte{1, 2}, 3, make([]byte, 3), "1.2.840.10008.1.2.4.205"); err == nil {
		t.Fatal("expected decode to fail when streamSize exceeds input length")
	}
	if err := JPIPdecode([]byte{1, 2, 3}, 3, make([]byte, 2), "1.2.840.10008.1.2.4.205"); err == nil {
		t.Fatal("expected decode to fail when output is too small")
	}
}

func TestJPIPCustomBackend(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	mock := &mockBackend{name: "mock-jpip", decodedBytes: 3, encodedBytes: []byte{4, 5, 6}}
	SetBackend(mock)

	if BackendName() != "mock-jpip" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	var out []byte
	var outSize int
	if err := JPIPencode([]byte{1, 2, 3, 4}, 2, 2, 1, 8, &out, &outSize, "1.2.840.10008.1.2.4.204"); err != nil {
		t.Fatalf("unexpected JPIPencode error with custom backend: %v", err)
	}
	if outSize != 3 {
		t.Fatalf("unexpected encoded size from custom backend: got %d, want 3", outSize)
	}
	if len(out) != 3 || out[0] != 4 || out[1] != 5 || out[2] != 6 {
		t.Fatalf("unexpected encoded payload from custom backend: %v", out)
	}

	decoded := make([]byte, 3)
	if err := JPIPdecode(out, uint32(len(out)), decoded, "1.2.840.10008.1.2.4.204"); err != nil {
		t.Fatalf("unexpected JPIPdecode error with custom backend: %v", err)
	}
	if decoded[0] != 30 || decoded[1] != 31 || decoded[2] != 32 {
		t.Fatalf("unexpected decoded payload from custom backend: %v", decoded)
	}
}

func TestJPIPBackendRegistry(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	err := RegisterBackend("unit-mock-jpip", func() Backend {
		return &mockBackend{name: "unit-mock-jpip", decodedBytes: 2, encodedBytes: []byte{6, 7}}
	})
	if err != nil {
		t.Fatalf("RegisterBackend failed: %v", err)
	}

	if err := UseBackend("unit-mock-jpip"); err != nil {
		t.Fatalf("UseBackend failed: %v", err)
	}
	if BackendName() != "unit-mock-jpip" {
		t.Fatalf("unexpected backend name after UseBackend: %s", BackendName())
	}

	found := false
	for _, name := range AvailableBackends() {
		if name == "unit-mock-jpip" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected registered backend in AvailableBackends")
	}
}

func TestJPIPNativeBackendRegistrationMatchesFlag(t *testing.T) {
	foundOpenJPH := false
	for _, name := range AvailableBackends() {
		if name == "openjph" {
			foundOpenJPH = true
			break
		}
	}

	if CGOEnabled && !foundOpenJPH {
		t.Fatal("expected openjph backend when CGOEnabled is true")
	}
	if !CGOEnabled && foundOpenJPH {
		t.Fatal("did not expect openjph backend when CGOEnabled is false")
	}
}
