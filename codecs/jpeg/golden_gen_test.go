//go:build libjpeg && cgo

package jpeg

// Golden-vector generator. Run once (with libjpeg available) to (re)create the
// committed snapshot that replaces the live libjpeg oracle:
//
//	GENERATE_GOLDENS=1 go test -tags "libjpeg cgo" -run TestGenerateJPEGGoldens ./codecs/jpeg/
//
// It records the SHA-256 of the pure-Go decoded output for the lossless (SOF3)
// and 12-bit DCT (SOF1) fixtures. Because the cgo golden tests pass at the
// generating commit (lossless bit-exact, DCT within tolerance of libjpeg), these
// hashes are the libjpeg-validated answers; the portable TestGoJPEGGoldenVectors
// then guards them without needing cgo.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateJPEGGoldens(t *testing.T) {
	if os.Getenv("GENERATE_GOLDENS") == "" {
		t.Skip("set GENERATE_GOLDENS=1 to regenerate committed golden vectors")
	}
	t.Cleanup(func() { SetBackend(nil) })
	if err := os.MkdirAll(jpegGoldenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := UseBackend("gojpeg"); err != nil {
		t.Fatalf("gojpeg unavailable: %v", err)
	}

	var man jpegGoldenManifest

	// Lossless SOF3 fixtures (pure-Go decode is bit-exact with libjpeg).
	for _, fx := range []struct{ name, path string }{
		{"fixture-lossless-process14", "../../testdata/cornerstone-CTImage-jpeg-process14.dcm"},
		{"fixture-lossless-process14sv1", "../../testdata/cornerstone-CTImage-jpeg-process14sv1.dcm"},
	} {
		dcm, err := os.ReadFile(fx.path)
		if err != nil {
			t.Logf("skip %s: %v", fx.name, err)
			continue
		}
		frame := extractFirstDICOMEncapsulatedFrame(t, dcm)
		f, _, err := decodeLossless(frame)
		if err != nil {
			t.Fatalf("%s decodeLossless: %v", fx.name, err)
		}
		bps := 1
		if f.precision > 8 {
			bps = 2
		}
		size := f.width * f.height * len(f.comps) * bps
		man.add(t, fx.name, fx.path, f.precision, size, frame)
	}

	// 12-bit DCT SOF1 fixtures (pure-Go IDCT was within tolerance of libjpeg at
	// snapshot; the hash freezes that validated output against regressions).
	for _, fx := range []struct{ name, path string }{
		{"fixture-dct12-process2-4", "../../testdata/cornerstone-CTImage-jpeg-process2-4.dcm"},
		{"fixture-dct12-jpgextended", "../../testdata/pydicom-JPGExtended.dcm"},
	} {
		dcm, err := os.ReadFile(fx.path)
		if err != nil {
			t.Logf("skip %s: %v", fx.name, err)
			continue
		}
		frame := extractFirstDICOMEncapsulatedFrame(t, dcm)
		f, err := decodeDCT(frame)
		if err != nil {
			t.Fatalf("%s decodeDCT: %v", fx.name, err)
		}
		size := f.width * f.height * len(f.comps) * 2
		man.add(t, fx.name, fx.path, 12, size, frame)
	}

	out, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jpegGoldenDir, "manifest.json"), append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d golden cases to %s", len(man.Cases), jpegGoldenDir)
}

func (m *jpegGoldenManifest) add(t *testing.T, name, fixture string, bits, size int, frame []byte) {
	t.Helper()
	out, err := goDecodeFrame(frame, bits, size)
	if err != nil {
		t.Fatalf("%s pure-Go decode: %v", name, err)
	}
	sum := sha256.Sum256(out)
	m.Cases = append(m.Cases, jpegGoldenCase{
		Name:    name,
		Fixture: fixture,
		Bits:    bits,
		Size:    size,
		SHA256:  hex.EncodeToString(sum[:]),
	})
}
