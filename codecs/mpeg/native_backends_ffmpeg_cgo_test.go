//go:build ffmpeg && cgo

package mpeg

import (
	"testing"
)

func TestFFmpegBackendSelection(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if err := UseBackend("ffmpeg"); err != nil {
		t.Fatalf("expected ffmpeg backend to be registered: %v", err)
	}
	if BackendName() != "ffmpeg" {
		t.Fatalf("unexpected backend name: %s", BackendName())
	}

	raw := []byte{1, 2, 3, 4}
	var out []byte
	var outSize int
	err := MPEGencode(raw, 2, 2, 1, 8, &out, &outSize, "1.2.840.10008.1.2.4.102")
	if err != nil {
		t.Fatalf("unexpected MPEGencode error: %v", err)
	}
	if outSize == 0 || len(out) == 0 {
		t.Fatal("expected non-empty encoded output")
	}

	decoded := make([]byte, len(raw))
	if err := MPEGdecode(out, uint32(outSize), decoded, "1.2.840.10008.1.2.4.102"); err != nil {
		t.Fatalf("unexpected MPEGdecode error: %v", err)
	}
	hasSignal := false
	for _, value := range decoded {
		if value != 0 {
			hasSignal = true
			break
		}
	}
	if !hasSignal {
		t.Fatalf("expected decoded payload to contain signal, got %v", decoded)
	}
}
