package jpeg

import "testing"

var mockSupportedTransferSyntaxUIDs = []string{"1.2.840.10008.1.2.4.51", "1.2.840.10008.1.2.4.70"}

type mockBackend struct {
	name           string
	decode12Bytes  int
	decode16Bytes  int
	encode12Result []byte
	encode16Result []byte
}

func (m *mockBackend) Name() string {
	return m.name
}

func (m *mockBackend) SupportedTransferSyntaxUIDs() []string {
	out := make([]string, len(mockSupportedTransferSyntaxUIDs))
	copy(out, mockSupportedTransferSyntaxUIDs)
	return out
}

func (m *mockBackend) Decode12(encoded []byte, output []byte) error {
	if len(output) < m.decode12Bytes {
		return errDecode12Failed
	}
	for i := 0; i < m.decode12Bytes; i++ {
		output[i] = byte(40 + i)
	}
	_ = encoded
	return nil
}

func (m *mockBackend) Encode12(raw []byte, _ uint16, _ uint16, _ uint16, _ int) ([]byte, error) {
	_ = raw
	out := make([]byte, len(m.encode12Result))
	copy(out, m.encode12Result)
	return out, nil
}

func (m *mockBackend) Decode16(encoded []byte, output []byte) error {
	if len(output) < m.decode16Bytes {
		return errDecode16Failed
	}
	for i := 0; i < m.decode16Bytes; i++ {
		output[i] = byte(70 + i)
	}
	_ = encoded
	return nil
}

func (m *mockBackend) Encode16(raw []byte, _ uint16, _ uint16, _ uint16, _ int) ([]byte, error) {
	_ = raw
	out := make([]byte, len(m.encode16Result))
	copy(out, m.encode16Result)
	return out, nil
}

var (
	errDecode12Failed = errorString("decode12 failed")
	errDecode16Failed = errorString("decode16 failed")
)

type errorString string

func (e errorString) Error() string {
	return string(e)
}

func TestBaseline8RoundTripGray(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	raw := []byte{
		0, 32, 64, 96,
		128, 160, 192, 224,
		10, 20, 30, 40,
		50, 60, 70, 80,
	}

	var encoded []byte
	var encodedSize int
	if err := EIJG8encode(raw, 4, 4, 1, &encoded, &encodedSize, 0); err != nil {
		t.Fatalf("EIJG8encode failed: %v", err)
	}
	if encodedSize == 0 || len(encoded) == 0 {
		t.Fatal("EIJG8encode returned empty output")
	}

	decoded := make([]byte, len(raw))
	if err := DIJG8decode(encoded, uint32(encodedSize), decoded, uint32(len(decoded))); err != nil {
		t.Fatalf("DIJG8decode failed: %v", err)
	}
}

func TestJPEG12And16Validation(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := DIJG12decode([]byte{1, 2, 3}, 4, make([]byte, 4), 4); err == nil {
		t.Fatal("expected Decode12 size validation error")
	}
	if err := DIJG16decode([]byte{1, 2, 3}, 3, make([]byte, 2), 3); err == nil {
		t.Fatal("expected Decode16 size validation error")
	}
	if err := DIJG8decode([]byte{1, 2, 3}, 3, make([]byte, 2), 3); err == nil {
		t.Fatal("expected Decode8 size validation error")
	}
	var out []byte
	var outSize int
	if err := EIJG12encode([]byte{1}, 1, 1, 1, nil, &outSize, 0); err == nil {
		t.Fatal("expected Encode12 nil output pointer error")
	}
	if err := EIJG16encode([]byte{1}, 1, 1, 1, &out, nil, 0); err == nil {
		t.Fatal("expected Encode16 nil size pointer error")
	}
}

func TestJPEGCustomBackend(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	mock := &mockBackend{
		name:           "mock-jpeg",
		decode12Bytes:  3,
		decode16Bytes:  2,
		encode12Result: []byte{9, 9, 9},
		encode16Result: []byte{8, 8},
	}
	SetBackend(mock)

	if BackendName() != "mock-jpeg" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	var out12 []byte
	var out12Size int
	if err := EIJG12encode([]byte{1, 2, 3, 4}, 2, 2, 1, &out12, &out12Size, 0); err != nil {
		t.Fatalf("unexpected EIJG12encode error with custom backend: %v", err)
	}
	if out12Size != 3 || len(out12) != 3 {
		t.Fatalf("unexpected EIJG12encode output: size=%d data=%v", out12Size, out12)
	}

	decoded12 := make([]byte, 3)
	if err := DIJG12decode(out12, uint32(len(out12)), decoded12, uint32(len(decoded12))); err != nil {
		t.Fatalf("unexpected DIJG12decode error with custom backend: %v", err)
	}
	if decoded12[0] != 40 || decoded12[1] != 41 || decoded12[2] != 42 {
		t.Fatalf("unexpected DIJG12decode payload from custom backend: %v", decoded12)
	}

	var out16 []byte
	var out16Size int
	if err := EIJG16encode([]byte{1, 2, 3, 4}, 2, 2, 1, &out16, &out16Size, 0); err != nil {
		t.Fatalf("unexpected EIJG16encode error with custom backend: %v", err)
	}
	if out16Size != 2 || len(out16) != 2 {
		t.Fatalf("unexpected EIJG16encode output: size=%d data=%v", out16Size, out16)
	}

	decoded16 := make([]byte, 2)
	if err := DIJG16decode(out16, uint32(len(out16)), decoded16, uint32(len(decoded16))); err != nil {
		t.Fatalf("unexpected DIJG16decode error with custom backend: %v", err)
	}
	if decoded16[0] != 70 || decoded16[1] != 71 {
		t.Fatalf("unexpected DIJG16decode payload from custom backend: %v", decoded16)
	}
}

func TestJPEGBackendRegistry(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	err := RegisterBackend("unit-mock-jpeg", func() Backend {
		return &mockBackend{
			name:           "unit-mock-jpeg",
			decode12Bytes:  2,
			decode16Bytes:  2,
			encode12Result: []byte{1, 1},
			encode16Result: []byte{2, 2},
		}
	})
	if err != nil {
		t.Fatalf("RegisterBackend failed: %v", err)
	}

	if err := UseBackend("unit-mock-jpeg"); err != nil {
		t.Fatalf("UseBackend failed: %v", err)
	}
	if BackendName() != "unit-mock-jpeg" {
		t.Fatalf("unexpected backend name after UseBackend: %s", BackendName())
	}

	found := false
	for _, name := range AvailableBackends() {
		if name == "unit-mock-jpeg" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected registered backend in AvailableBackends")
	}
}

func TestJPEGNoLibJPEGBackend(t *testing.T) {
	if CGOEnabled {
		t.Fatal("CGOEnabled should be false after the libjpeg backend was retired")
	}
	for _, name := range AvailableBackends() {
		if name == "libjpeg" {
			t.Fatal("libjpeg backend should not be registered after retirement")
		}
	}
}
