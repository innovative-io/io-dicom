package jpip

import "testing"

var mockSupportedTransferSyntaxUIDs = []string{"1.2.840.10008.1.2.4.204", "1.2.840.10008.1.2.4.205"}

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
	if CGOEnabled {
		t.Fatalf("CGOEnabled should be false (openjph retired), got %v", CGOEnabled)
	}
}

// TestJPIPPureGoRoundTrip exercises the default pure-Go HTJ2K backend: encode
// raw pixels to an HTJ2K codestream and decode them back losslessly.
func TestJPIPPureGoRoundTrip(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if BackendName() != "gojpip" {
		t.Fatalf("expected default backend gojpip, got %s", BackendName())
	}

	cases := []struct {
		w, h, samples, bitsa int
	}{
		{4, 4, 1, 8}, {16, 16, 1, 8}, {8, 8, 3, 8}, {10, 7, 1, 16},
	}
	for _, c := range cases {
		bps := 1
		if c.bitsa > 8 {
			bps = 2
		}
		raw := make([]byte, c.w*c.h*c.samples*bps)
		for i := range raw {
			raw[i] = byte((i*7 + 11) & 0xFF)
		}
		var out []byte
		var outSize int
		uid := "1.2.840.10008.1.2.4.204"
		if err := JPIPencode(raw, uint16(c.w), uint16(c.h), uint16(c.samples), uint16(c.bitsa), &out, &outSize, uid); err != nil {
			t.Fatalf("%+v JPIPencode: %v", c, err)
		}
		if outSize == 0 || len(out) == 0 {
			t.Fatalf("%+v: empty encoded output", c)
		}
		decoded := make([]byte, len(raw))
		if err := JPIPdecode(out, uint32(outSize), decoded, uid); err != nil {
			t.Fatalf("%+v JPIPdecode: %v", c, err)
		}
		for i := range raw {
			if raw[i] != decoded[i] {
				t.Fatalf("%+v: byte %d differs: got %d want %d", c, i, decoded[i], raw[i])
			}
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
