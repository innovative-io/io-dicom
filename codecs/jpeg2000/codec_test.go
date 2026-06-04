package jpeg2000

import "testing"

var mockSupportedTransferSyntaxUIDs = []string{"1.2.840.10008.1.2.4.90", "1.2.840.10008.1.2.4.91"}

type mockBackend struct {
	name         string
	decodedBytes int
	encodedBytes []byte
}

func (m *mockBackend) Name() string {
	return m.name
}

func (m *mockBackend) SupportedTransferSyntaxUIDs() []string {
	out := make([]string, len(mockSupportedTransferSyntaxUIDs))
	copy(out, mockSupportedTransferSyntaxUIDs)
	return out
}

func (m *mockBackend) Decode(encoded []byte, output []byte) error {
	if len(output) < m.decodedBytes {
		return errInvalidJ2KPayload
	}
	for i := 0; i < m.decodedBytes; i++ {
		output[i] = byte(200 + i)
	}
	_ = encoded
	return nil
}

func (m *mockBackend) Encode(raw []byte, _ uint16, _ uint16, _ uint16, _ uint16, _ int) ([]byte, error) {
	_ = raw
	out := make([]byte, len(m.encodedBytes))
	copy(out, m.encodedBytes)
	return out, nil
}

func TestJ2KPassthroughDecodeFails(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if CGOEnabled != nativeBackendEnabled {
		t.Fatalf("unexpected CGOEnabled value: got %v want %v", CGOEnabled, nativeBackendEnabled)
	}

	raw := []byte{1, 2, 3, 4}
	var out []byte
	var outSize int
	// There is no pure-Go JPEG 2000 encoder, so without the native backend
	// encode must fail rather than emit the raw bytes as a fake stream.
	if err := J2Kencode(raw, 2, 2, 1, 8, &out, &outSize, 10); err == nil {
		t.Fatal("expected J2Kencode to fail without native backend")
	}

	decoded := make([]byte, len(raw))
	if err := J2Kdecode([]byte{1, 2, 3, 4}, 4, decoded); err == nil {
		t.Fatal("expected J2Kdecode to fail without native backend")
	}
}

func TestJ2KDecodeSizeValidation(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := J2Kdecode([]byte{1, 2, 3}, 4, make([]byte, 4)); err == nil {
		t.Fatal("expected invalid payload size error when source is too small")
	}
	if err := J2Kdecode([]byte{1, 2, 3}, 3, make([]byte, 2)); err == nil {
		t.Fatal("expected invalid payload size error when output is too small")
	}
}

func TestJ2KEncodeNilOutput(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	var out []byte
	var outSize int
	if err := J2Kencode([]byte{1, 2, 3}, 1, 1, 1, 8, nil, &outSize, 10); err == nil {
		t.Fatal("expected nil output pointer error")
	}
	if err := J2Kencode([]byte{1, 2, 3}, 1, 1, 1, 8, &out, nil, 10); err == nil {
		t.Fatal("expected nil output size pointer error")
	}
}

func TestJ2KCustomBackend(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	mock := &mockBackend{name: "mock-j2k", decodedBytes: 3, encodedBytes: []byte{7, 8, 9}}
	SetBackend(mock)

	if BackendName() != "mock-j2k" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	var out []byte
	var outSize int
	if err := J2Kencode([]byte{1, 2, 3, 4}, 2, 2, 1, 8, &out, &outSize, 10); err != nil {
		t.Fatalf("unexpected J2Kencode error with custom backend: %v", err)
	}
	if outSize != 3 {
		t.Fatalf("unexpected encoded size from custom backend: got %d, want 3", outSize)
	}
	if len(out) != 3 || out[0] != 7 || out[1] != 8 || out[2] != 9 {
		t.Fatalf("unexpected encoded payload from custom backend: %v", out)
	}

	decoded := make([]byte, 3)
	if err := J2Kdecode(out, uint32(len(out)), decoded); err != nil {
		t.Fatalf("unexpected J2Kdecode error with custom backend: %v", err)
	}
	if decoded[0] != 200 || decoded[1] != 201 || decoded[2] != 202 {
		t.Fatalf("unexpected decoded payload from custom backend: %v", decoded)
	}
}

func TestJ2KBackendRegistry(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	err := RegisterBackend("unit-mock-j2k", func() Backend {
		return &mockBackend{name: "unit-mock-j2k", decodedBytes: 2, encodedBytes: []byte{6, 5}}
	})
	if err != nil {
		t.Fatalf("RegisterBackend failed: %v", err)
	}

	if err := UseBackend("unit-mock-j2k"); err != nil {
		t.Fatalf("UseBackend failed: %v", err)
	}
	if BackendName() != "unit-mock-j2k" {
		t.Fatalf("unexpected backend name after UseBackend: %s", BackendName())
	}

	found := false
	for _, name := range AvailableBackends() {
		if name == "unit-mock-j2k" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected registered backend in AvailableBackends")
	}
}

func TestJ2KNativeBackendRegistrationMatchesFlag(t *testing.T) {
	foundOpenJPEG := false
	for _, name := range AvailableBackends() {
		if name == "openjpeg" {
			foundOpenJPEG = true
			break
		}
	}

	if CGOEnabled && !foundOpenJPEG {
		t.Fatal("expected openjpeg backend when CGOEnabled is true")
	}
	if !CGOEnabled && foundOpenJPEG {
		t.Fatal("did not expect openjpeg backend when CGOEnabled is false")
	}
}
