package mpeg

import (
	"errors"
	"testing"
)

var mockSupportedTransferSyntaxUIDs = []string{"1.2.840.10008.1.2.4.100", "1.2.840.10008.1.2.4.102"}

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

func (m *mockBackend) Decode(encoded []byte, output []byte, _ string) error {
	if len(output) < m.decodedBytes {
		return errors.New("invalid MPEG payload size")
	}
	for i := 0; i < m.decodedBytes; i++ {
		output[i] = byte(10 + i)
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

func TestMPEGPassthroughDecodeFails(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if CGOEnabled != nativeBackendEnabled {
		t.Fatalf("unexpected CGOEnabled value: got %v want %v", CGOEnabled, nativeBackendEnabled)
	}

	raw := []byte{1, 2, 3, 4, 5, 6}
	var out []byte
	var outSize int
	if err := MPEGencode(raw, 2, 1, 3, 8, &out, &outSize, "1.2.840.10008.1.2.4.102"); err != nil {
		t.Fatalf("unexpected MPEGencode error: %v", err)
	}
	if outSize != len(raw) {
		t.Fatalf("unexpected MPEG encoded size: got %d, want %d", outSize, len(raw))
	}

	decoded := make([]byte, len(raw))
	if err := MPEGdecode(out, uint32(outSize), decoded, "1.2.840.10008.1.2.4.102"); err == nil {
		t.Fatal("expected MPEGdecode to fail without native backend")
	}
}

func TestMPEGRejectsUnsupportedUID(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	var out []byte
	var outSize int
	err := MPEGencode([]byte{1, 2}, 1, 1, 1, 8, &out, &outSize, "1.2.3")
	if err == nil {
		t.Fatal("expected unsupported transfer syntax error")
	}
}

func TestMPEGSizeValidation(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := MPEGdecode([]byte{1, 2, 3}, 4, make([]byte, 4), "1.2.840.10008.1.2.4.100"); err == nil {
		t.Fatal("expected invalid payload size error when source is too small")
	}
	if err := MPEGdecode([]byte{1, 2, 3}, 3, make([]byte, 2), "1.2.840.10008.1.2.4.100"); err == nil {
		t.Fatal("expected invalid payload size error when output is too small")
	}
}

func TestMPEGCustomBackend(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	mock := &mockBackend{name: "mock-mpeg", decodedBytes: 3, encodedBytes: []byte{9, 8, 7}}
	SetBackend(mock)

	if BackendName() != "mock-mpeg" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	var out []byte
	var outSize int
	if err := MPEGencode([]byte{1, 2, 3, 4}, 2, 2, 1, 8, &out, &outSize, "1.2.840.10008.1.2.4.102"); err != nil {
		t.Fatalf("unexpected MPEGencode error with custom backend: %v", err)
	}
	if outSize != 3 {
		t.Fatalf("unexpected encoded size from custom backend: got %d, want 3", outSize)
	}
	if len(out) != 3 || out[0] != 9 || out[1] != 8 || out[2] != 7 {
		t.Fatalf("unexpected encoded payload from custom backend: %v", out)
	}

	decoded := make([]byte, 3)
	if err := MPEGdecode(out, uint32(len(out)), decoded, "1.2.840.10008.1.2.4.102"); err != nil {
		t.Fatalf("unexpected MPEGdecode error with custom backend: %v", err)
	}
	if decoded[0] != 10 || decoded[1] != 11 || decoded[2] != 12 {
		t.Fatalf("unexpected decoded payload from custom backend: %v", decoded)
	}
}

func TestMPEGBackendRegistry(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	err := RegisterBackend("unit-mock-mpeg", func() Backend {
		return &mockBackend{name: "unit-mock-mpeg", decodedBytes: 2, encodedBytes: []byte{1, 0}}
	})
	if err != nil {
		t.Fatalf("RegisterBackend failed: %v", err)
	}

	if err := UseBackend("unit-mock-mpeg"); err != nil {
		t.Fatalf("UseBackend failed: %v", err)
	}
	if BackendName() != "unit-mock-mpeg" {
		t.Fatalf("unexpected backend name after UseBackend: %s", BackendName())
	}

	found := false
	for _, name := range AvailableBackends() {
		if name == "unit-mock-mpeg" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected registered backend in AvailableBackends")
	}
}

func TestMPEGNativeBackendRegistrationMatchesFlag(t *testing.T) {
	foundFFmpeg := false
	for _, name := range AvailableBackends() {
		if name == "ffmpeg" {
			foundFFmpeg = true
			break
		}
	}

	if CGOEnabled && !foundFFmpeg {
		t.Fatal("expected ffmpeg backend when CGOEnabled is true")
	}
	if !CGOEnabled && foundFFmpeg {
		t.Fatal("did not expect ffmpeg backend when CGOEnabled is false")
	}
}
