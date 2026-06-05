package jpeg

// Portable golden-vector oracle. These committed vectors are the frozen,
// libjpeg-validated replacement for the live libjpeg comparison: each entry
// pairs an independent JPEG stream (a real DICOM fixture) with the SHA-256 of
// the pure-Go decoded output captured when the cgo golden tests last passed.
// Pure-Go decode is deterministic; lossless decode is bit-exact with libjpeg and
// the 12-bit DCT decode was within tolerance of libjpeg at the snapshot commit,
// so any mismatch here is a real regression. These vectors were generated against
// the libjpeg reference at the retirement commit (see git history of codecs/jpeg
// for the libjpeg-tagged generator); regenerating requires restoring that backend.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type jpegGoldenCase struct {
	Name    string `json:"name"`
	Fixture string `json:"fixture"`
	Bits    int    `json:"bits"`
	Size    int    `json:"size"`
	SHA256  string `json:"sha256"`
}

type jpegGoldenManifest struct {
	Cases []jpegGoldenCase `json:"cases"`
}

const jpegGoldenDir = "golden"

// goDecodeFrame decodes a JPEG frame with the active (pure-Go) backend, picking
// the precision-appropriate entry point.
func goDecodeFrame(frame []byte, bits, size int) ([]byte, error) {
	out := make([]byte, size)
	var err error
	switch {
	case bits <= 8:
		err = DIJG8decode(frame, uint32(len(frame)), out, uint32(size))
	case bits <= 12:
		err = DIJG12decode(frame, uint32(len(frame)), out, uint32(size))
	default:
		err = DIJG16decode(frame, uint32(len(frame)), out, uint32(size))
	}
	return out, err
}

func TestGoJPEGGoldenVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(jpegGoldenDir, "manifest.json"))
	if err != nil {
		t.Skipf("golden manifest unavailable: %v", err)
	}
	var man jpegGoldenManifest
	if err := json.Unmarshal(raw, &man); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(man.Cases) == 0 {
		t.Fatal("empty golden manifest")
	}
	t.Cleanup(func() { SetBackend(nil) })
	for _, c := range man.Cases {
		t.Run(c.Name, func(t *testing.T) {
			dcm := loadBytesFromFile(c.Fixture, t)
			frame := extractFirstDICOMEncapsulatedFrame(t, dcm)
			out, err := goDecodeFrame(frame, c.Bits, c.Size)
			if err != nil {
				t.Fatalf("pure-Go decode: %v", err)
			}
			sum := sha256.Sum256(out)
			if got := hex.EncodeToString(sum[:]); got != c.SHA256 {
				t.Fatalf("decoded output hash mismatch:\n got  %s\n want %s", got, c.SHA256)
			}
		})
	}
}
