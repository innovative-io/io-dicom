//go:build charls && cgo

package jpegls

import "testing"

func TestCharLSBackendSelection(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("charls"); err != nil {
		t.Fatalf("expected charls backend to be registered: %v", err)
	}
	if BackendName() != "charls" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	raw := []byte{1, 2, 3, 4}
	var out []byte
	var outSize int
	err := JLSencode(raw, 2, 2, 1, 8, &out, &outSize, false)
	if err != nil {
		t.Fatalf("unexpected charls encode error: %v", err)
	}

	decoded := make([]byte, len(raw))
	if err := JLSdecode(out, uint32(outSize), decoded); err != nil {
		t.Fatalf("unexpected charls decode error: %v", err)
	}
	for i := range raw {
		if decoded[i] != raw[i] {
			t.Fatalf("decoded[%d]=%d, want %d", i, decoded[i], raw[i])
		}
	}
}
