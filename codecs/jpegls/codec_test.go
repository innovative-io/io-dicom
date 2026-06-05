package jpegls

import (
	"bytes"
	"testing"
)

var mockSupportedTransferSyntaxUIDs = []string{"1.2.840.10008.1.2.4.80", "1.2.840.10008.1.2.4.81"}

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
		return errInvalidJLSPayload
	}
	for i := 0; i < m.decodedBytes; i++ {
		output[i] = byte(255 - i)
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

func TestJLSDefaultBackendRoundTrips(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if CGOEnabled {
		t.Fatal("CGOEnabled should be false after the charls backend was retired")
	}
	if BackendName() != "gojpegls" {
		t.Fatalf("expected default backend gojpegls, got %s", BackendName())
	}

	raw := []byte{10, 20, 30, 40}
	var out []byte
	var outSize int
	if err := JLSencode(raw, 2, 2, 1, 8, &out, &outSize, false); err != nil {
		t.Fatalf("unexpected JLSencode error: %v", err)
	}
	decoded := make([]byte, len(raw))
	if err := JLSdecode(out, uint32(outSize), decoded); err != nil {
		t.Fatalf("unexpected JLSdecode error: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatalf("pure-Go JPEG-LS round trip mismatch: got %v want %v", decoded, raw)
	}
}

func TestJLSDecodeSizeValidation(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := JLSdecode([]byte{1, 2, 3}, 4, make([]byte, 4)); err == nil {
		t.Fatal("expected invalid payload size error when source is too small")
	}
	if err := JLSdecode([]byte{1, 2, 3}, 3, make([]byte, 2)); err == nil {
		t.Fatal("expected invalid payload size error when output is too small")
	}
}

func TestJLSEncodeNilOutput(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	var out []byte
	var outSize int
	if err := JLSencode([]byte{1, 2, 3}, 1, 1, 1, 8, nil, &outSize, false); err == nil {
		t.Fatal("expected nil output pointer error")
	}
	if err := JLSencode([]byte{1, 2, 3}, 1, 1, 1, 8, &out, nil, false); err == nil {
		t.Fatal("expected nil output size pointer error")
	}
}

func TestJLSCustomBackend(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	mock := &mockBackend{name: "mock-jls", decodedBytes: 3, encodedBytes: []byte{9, 8, 7}}
	SetBackend(mock)

	if BackendName() != "mock-jls" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	var out []byte
	var outSize int
	if err := JLSencode([]byte{1, 2, 3, 4}, 2, 2, 1, 8, &out, &outSize, false); err != nil {
		t.Fatalf("unexpected JLSencode error with custom backend: %v", err)
	}
	if outSize != 3 {
		t.Fatalf("unexpected encoded size from custom backend: got %d, want 3", outSize)
	}
	if len(out) != 3 || out[0] != 9 || out[1] != 8 || out[2] != 7 {
		t.Fatalf("unexpected encoded payload from custom backend: %v", out)
	}

	decoded := make([]byte, 3)
	if err := JLSdecode(out, uint32(len(out)), decoded); err != nil {
		t.Fatalf("unexpected JLSdecode error with custom backend: %v", err)
	}
	if decoded[0] != 255 || decoded[1] != 254 || decoded[2] != 253 {
		t.Fatalf("unexpected decoded payload from custom backend: %v", decoded)
	}
}

func TestJLSBackendRegistry(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	err := RegisterBackend("unit-mock-jls", func() Backend {
		return &mockBackend{name: "unit-mock-jls", decodedBytes: 2, encodedBytes: []byte{5, 4}}
	})
	if err != nil {
		t.Fatalf("RegisterBackend failed: %v", err)
	}

	if err := UseBackend("unit-mock-jls"); err != nil {
		t.Fatalf("UseBackend failed: %v", err)
	}
	if BackendName() != "unit-mock-jls" {
		t.Fatalf("unexpected backend name after UseBackend: %s", BackendName())
	}

	found := false
	for _, name := range AvailableBackends() {
		if name == "unit-mock-jls" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected registered backend in AvailableBackends")
	}
}

func TestJLSNoCharLSBackend(t *testing.T) {
	for _, name := range AvailableBackends() {
		if name == "charls" {
			t.Fatal("charls backend should not be registered after retirement")
		}
	}
}
