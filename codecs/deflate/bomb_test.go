package deflate

import (
	"bytes"
	"compress/zlib"
	"strings"
	"testing"
)

// makeBomb returns a zlib stream that inflates to n bytes of zeros. Highly
// compressible input yields a very small payload for a very large output.
func makeBomb(t *testing.T, n int) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		t.Fatalf("NewWriterLevel: %v", err)
	}
	zeros := make([]byte, 1<<20)
	for written := 0; written < n; written += len(zeros) {
		chunk := zeros
		if n-written < len(chunk) {
			chunk = zeros[:n-written]
		}
		if _, err := zw.Write(chunk); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return buf.Bytes()
}

// TestInflateFrameRejectsDecompressionBomb pins the bound on the unknown-size
// path. media's dataset parser calls InflateFrame(data, -1) on the remainder of
// the file, before any validation, so an unbounded read there is reachable
// directly from the wire: a sub-megabyte instance could inflate to gigabytes and
// exhaust memory on a C-STORE receiver.
func TestInflateFrameRejectsDecompressionBomb(t *testing.T) {
	// One byte over the cap is enough to prove the bound without allocating
	// anything close to it in the success path.
	bomb := makeBomb(t, MaxInflatedBytes+1)
	t.Logf("compressed %d bytes -> declared %d bytes (%.0f:1)",
		len(bomb), MaxInflatedBytes+1, float64(MaxInflatedBytes+1)/float64(len(bomb)))

	out, err := InflateFrame(bomb, -1)
	if err == nil {
		t.Fatalf("expected the oversized stream to be rejected, got %d bytes", len(out))
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected a size-cap error, got %v", err)
	}
}

// TestInflateFrameUnknownSizeUnderCap confirms the bound does not reject
// legitimate streams.
func TestInflateFrameUnknownSizeUnderCap(t *testing.T) {
	payload := bytes.Repeat([]byte("DICOM"), 200000) // ~1 MB, well under the cap
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	zw.Close()

	out, err := InflateFrame(buf.Bytes(), -1)
	if err != nil {
		t.Fatalf("InflateFrame(-1) on a legitimate stream: %v", err)
	}
	if !bytes.Equal(out, payload) {
		t.Fatal("round-trip mismatch on the unknown-size path")
	}
}

// TestInflateFrameExactSizeRejectsOverlong ensures the known-size path still
// rejects a stream longer than declared, now that it reads exactly expectedSize
// rather than everything.
func TestInflateFrameExactSizeRejectsOverlong(t *testing.T) {
	payload := []byte("0123456789ABCDEF")
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	zw.Close()

	if _, err := InflateFrame(buf.Bytes(), len(payload)-1); err == nil {
		t.Fatal("expected an error when the stream is longer than expectedSize")
	}
	if _, err := InflateFrame(buf.Bytes(), len(payload)+1); err == nil {
		t.Fatal("expected an error when the stream is shorter than expectedSize")
	}
	out, err := InflateFrame(buf.Bytes(), len(payload))
	if err != nil {
		t.Fatalf("exact size should succeed: %v", err)
	}
	if !bytes.Equal(out, payload) {
		t.Fatal("round-trip mismatch on the exact-size path")
	}
}
