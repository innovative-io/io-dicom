package brotli

import (
	"bytes"
	"encoding/hex"
	"os/exec"
	"testing"
)

// hexToBytes decodes a hex string used to embed compressed test vectors.
func hexToBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex vector: %v", err)
	}
	return b
}

// repeat builds an expected-output helper.
func repeat(s string, n int) []byte { return bytes.Repeat([]byte(s), n) }

func seq256() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

// TestDecompressVectors decodes Brotli streams produced by the reference
// encoder and checks the output byte-for-byte. The vectors exercise stored,
// literal, backward-reference, static-dictionary and word-transform paths.
func TestDecompressVectors(t *testing.T) {
	cases := []struct {
		name string
		comp string // hex of the compressed stream
		want []byte
	}{
		{"empty", "3f", nil},
		{"hello", "1f16000024406a9064f2a83c4f4701", []byte("hello hello hello world")},
		{
			"english",
			"1f5a0000c4dc46a95e486bc95014a722cf64c3d729070eadb5d5120ac80e40070f59695c53cd5c6164fe22a89c81241775f56990cdfa81f168fe3f44c90483881c5688c9802efa8dc926",
			[]byte("The quick brown fox jumps over the lazy dog. The function returns the value of the request."),
		},
		{"inc256", "", seq256()},
		{"repeat", "1f5702f80576c0e62e73216a8b0330e88e01", repeat("abc123", 100)},
		{"zeros500", "1ff30100240082b1405a01", make([]byte, 500)},
	}
	// The inc256 vector is long; set it from the full hex constant to keep the
	// table readable.
	cases[3].comp = incVectorHex

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decompress(hexToBytes(t, tc.comp), len(tc.want))
			if err != nil {
				t.Fatalf("Decompress: %v", err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("mismatch: got %d bytes, want %d bytes", len(got), len(tc.want))
			}
		})
	}
}

// TestTransforms validates a sample of the dictionary word transforms.
func TestTransforms(t *testing.T) {
	word := []byte("function")
	cases := []struct {
		id   int
		want string
	}{
		{0, "function"},     // identity
		{1, "function "},    // identity + " "
		{3, "unction"},      // omit first 1
		{9, "Function"},     // uppercase first
		{44, "FUNCTION"},    // uppercase all
		{12, "functio"},     // omit last 1
		{49, "functioing "}, // omit last 1 ("functio") + "ing "
	}
	for _, tc := range cases {
		got, err := applyTransform(nil, tc.id, word)
		if err != nil {
			t.Fatalf("transform %d: %v", tc.id, err)
		}
		if string(got) != tc.want {
			t.Errorf("transform %d: got %q want %q", tc.id, got, tc.want)
		}
	}
}

// TestDecompressCLIFuzz cross-checks the decoder against the reference `brotli`
// command-line tool across many inputs, qualities and window sizes. It is
// skipped when the tool is not installed.
func TestDecompressCLIFuzz(t *testing.T) {
	if _, err := exec.LookPath("brotli"); err != nil {
		t.Skip("brotli CLI not available")
	}
	inputs := map[string][]byte{
		"empty":   nil,
		"text":    repeat("The quick brown fox. ", 200),
		"inc":     seq256(),
		"zeros":   make([]byte, 5000),
		"pattern": repeat("\x00\x01\x02\x03", 2000),
		"binaryish": func() []byte {
			b := make([]byte, 8000)
			x := uint32(7)
			for i := range b {
				x = x*1103515245 + 12345
				b[i] = byte(x >> 16)
			}
			return b
		}(),
	}
	qualities := []string{"0", "2", "5", "9", "11"}
	windows := []string{"10", "16", "22", "24"}
	for name, data := range inputs {
		for _, q := range qualities {
			for _, w := range windows {
				cmd := exec.Command("brotli", "-c", "-q", q, "-w", w)
				cmd.Stdin = bytes.NewReader(data)
				var comp bytes.Buffer
				cmd.Stdout = &comp
				if err := cmd.Run(); err != nil {
					t.Fatalf("brotli compress %s q%s w%s: %v", name, q, w, err)
				}
				got, err := Decompress(comp.Bytes(), len(data))
				if err != nil {
					t.Fatalf("%s q%s w%s: decode: %v", name, q, w, err)
				}
				if !bytes.Equal(got, data) {
					t.Fatalf("%s q%s w%s: output mismatch (got %d want %d)", name, q, w, len(got), len(data))
				}
			}
		}
	}
}

// TestCompressRoundTrip checks the stored-block encoder produces streams the
// decoder reads back exactly.
func TestCompressRoundTrip(t *testing.T) {
	cases := [][]byte{
		nil,
		[]byte("x"),
		[]byte("JFIF\x00 marker data"),
		bytes.Repeat([]byte{0xFF, 0x00, 0xAB}, 1000),
		seq256(),
		make([]byte, 70000), // multi-nibble length
	}
	for i, src := range cases {
		got, err := Decompress(Compress(src), len(src))
		if err != nil {
			t.Errorf("case %d: decompress: %v", i, err)
			continue
		}
		if !bytes.Equal(got, src) {
			t.Errorf("case %d: round-trip mismatch (%d vs %d bytes)", i, len(got), len(src))
		}
	}
}
