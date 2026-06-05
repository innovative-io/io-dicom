package jpegls

// Portable golden-vector oracle. These committed vectors are the frozen,
// charls-validated replacement for the live charls comparison: each entry pairs
// an independent JPEG-LS stream (a real DICOM fixture or a charls-encoded
// synthetic stream) with the SHA-256 of the pure-Go decoded output captured when
// the cgo golden tests last passed. Pure-Go decode is deterministic, so any
// mismatch is a real regression. These vectors were generated against the CharLS
// reference at the retirement commit (see git history of codecs/jpegls for the
// charls-tagged generator); regenerating requires restoring that backend.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type goldenCase struct {
	Name    string `json:"name"`
	Stream  string `json:"stream,omitempty"`  // path under the golden dir, when synthetic
	Fixture string `json:"fixture,omitempty"` // DICOM fixture path, when real-world
	Size    int    `json:"size"`
	SHA256  string `json:"sha256"`
}

type goldenManifest struct {
	Cases []goldenCase `json:"cases"`
}

const goldenDir = "golden"

func TestGoJLSGoldenVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(goldenDir, "manifest.json"))
	if err != nil {
		t.Skipf("golden manifest unavailable: %v", err)
	}
	var man goldenManifest
	if err := json.Unmarshal(raw, &man); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(man.Cases) == 0 {
		t.Fatal("empty golden manifest")
	}
	for _, c := range man.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var stream []byte
			switch {
			case c.Stream != "":
				stream, err = os.ReadFile(filepath.Join(goldenDir, c.Stream))
				if err != nil {
					t.Fatalf("read stream: %v", err)
				}
			case c.Fixture != "":
				dcm, err := os.ReadFile(c.Fixture)
				if err != nil {
					t.Skipf("fixture unavailable: %v", err)
				}
				stream = extractFirstFrame(t, dcm)
			default:
				t.Fatal("case has neither stream nor fixture")
			}

			out := make([]byte, c.Size)
			if err := decodeJLSInto(stream, out); err != nil {
				t.Fatalf("pure-Go decode: %v", err)
			}
			sum := sha256.Sum256(out)
			if got := hex.EncodeToString(sum[:]); got != c.SHA256 {
				t.Fatalf("decoded output hash mismatch:\n got  %s\n want %s", got, c.SHA256)
			}
		})
	}
}
