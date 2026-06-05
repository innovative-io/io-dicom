//go:build charls && cgo

package jpegls

// Golden-vector generator. Run once (with charls available) to (re)create the
// committed snapshot that replaces the live charls oracle:
//
//	GENERATE_GOLDENS=1 go test -tags "charls cgo" -run TestGenerateJLSGoldens ./codecs/jpegls/
//
// It uses charls to produce *independent* synthetic streams (so the decode
// oracle exercises a non-pure-Go encoder), then records the SHA-256 of the
// pure-Go decoded output for every stream. Because the cgo golden tests pass at
// the generating commit, those hashes are the charls-validated answers; the
// portable TestGoJLSGoldenVectors then guards them without needing cgo.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateJLSGoldens(t *testing.T) {
	if os.Getenv("GENERATE_GOLDENS") == "" {
		t.Skip("set GENERATE_GOLDENS=1 to regenerate committed golden vectors")
	}
	t.Cleanup(func() { SetBackend(nil) })

	const dir = "golden"
	streamsDir := filepath.Join(dir, "streams")
	if err := os.MkdirAll(streamsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var man goldenManifest

	// Real-world fixtures (independent C-encoder streams).
	for _, fx := range []struct{ name, path string }{
		{"fixture-ct-lossless", "../../testdata/cornerstone-CTImage-jpegls-lossless.dcm"},
		{"fixture-ct-near", "../../testdata/cornerstone-CTImage-jpegls-lossy.dcm"},
		{"fixture-sm-rgb-ilv2", "../../testdata/highdicom-sm_image_jpegls.dcm"},
		{"fixture-sm-rgb-ilv2-nobot", "../../testdata/highdicom-sm_image_jpegls_nobot.dcm"},
		{"fixture-near8", "../../testdata/pydicom-JPEGLSNearLossless_08.dcm"},
		{"fixture-near16", "../../testdata/pydicom-JPEGLSNearLossless_16.dcm"},
	} {
		dcm, err := os.ReadFile(fx.path)
		if err != nil {
			t.Logf("skip fixture %s: %v", fx.name, err)
			continue
		}
		frame := extractFirstFrame(t, dcm)
		man.add(t, fx.name, "", fx.path, frame)
	}

	// Synthetic streams encoded by charls (line-interleaved for multi-component),
	// filling coverage the fixtures lack (ILV=1, 12-bit).
	type syn struct {
		name                 string
		w, h, comps, p, near int
	}
	for _, s := range []syn{
		{"syn-gray12-lossless", 40, 30, 1, 12, 0},
		{"syn-ilv1-3c-8-near0", 16, 16, 3, 8, 0},
		{"syn-ilv1-3c-8-near1", 20, 12, 3, 8, 1},
		{"syn-ilv1-3c-12-near0", 17, 9, 3, 12, 0},
		{"syn-ilv1-3c-16-near2", 32, 8, 3, 16, 2},
		{"syn-ilv1-4c-8-near0", 10, 10, 4, 8, 0},
		{"syn-ilv1-4c-12-near1", 13, 11, 4, 12, 1},
	} {
		bps := 1
		if s.p > 8 {
			bps = 2
		}
		maxv := (1 << s.p) - 1
		n := s.w * s.h * s.comps
		raw := make([]byte, n*bps)
		for i := 0; i < n; i++ {
			v := (i*7 + (i*i)%29 + (i%s.comps)*11) % (maxv + 1)
			if bps == 1 {
				raw[i] = byte(v)
			} else {
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
		}
		if err := UseBackend("charls"); err != nil {
			t.Fatalf("charls unavailable: %v", err)
		}
		var enc []byte
		var encSize int
		if err := JLSencode(raw, uint16(s.w), uint16(s.h), uint16(s.comps), uint16(s.p), &enc, &encSize, s.near != 0); err != nil {
			t.Fatalf("%s charls encode: %v", s.name, err)
		}
		enc = enc[:encSize]
		rel := filepath.Join("streams", s.name+".jls")
		if err := os.WriteFile(filepath.Join(dir, rel), enc, 0o644); err != nil {
			t.Fatal(err)
		}
		man.add(t, s.name, rel, "", enc)
	}

	out, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d golden cases to %s", len(man.Cases), dir)
}

// add decodes stream with the pure-Go backend and records its output hash.
func (m *goldenManifest) add(t *testing.T, name, streamRel, fixture string, stream []byte) {
	t.Helper()
	f, _, err := parseJLS(stream)
	if err != nil {
		t.Fatalf("%s parseJLS: %v", name, err)
	}
	bps := 1
	if f.precision > 8 {
		bps = 2
	}
	size := f.width * f.height * len(f.comps) * bps
	if err := UseBackend("gojpegls"); err != nil {
		t.Fatalf("gojpegls unavailable: %v", err)
	}
	out := make([]byte, size)
	if err := JLSdecode(stream, uint32(len(stream)), out); err != nil {
		t.Fatalf("%s pure-Go decode: %v", name, err)
	}
	sum := sha256.Sum256(out)
	m.Cases = append(m.Cases, goldenCase{
		Name:    name,
		Stream:  streamRel,
		Fixture: fixture,
		Size:    size,
		SHA256:  hex.EncodeToString(sum[:]),
	})
}
