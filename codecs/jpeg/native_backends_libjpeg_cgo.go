//go:build libjpeg && cgo

package jpeg

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"unicode"

	"github.com/innovative-io/io-dicom/codecs/internal/nativeenv"
)

const nativeBackendEnabled = true

func init() {
	nativeenv.Init()
}

var (
	errLibJPEGInvalidPayload = errors.New("ERROR, Decode12, JPEG failed")
	errLibJPEGToolsMissing   = errors.New("ERROR, Encode12, JPEG failed")
)

type libjpegBackend struct{}

func (libjpegBackend) Name() string {
	return "libjpeg"
}

func (libjpegBackend) Decode12(encoded []byte, output []byte) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errLibJPEGInvalidPayload
	}
	if err := ensureLibJPEGTools(); err != nil {
		if len(encoded) > len(output) {
			return errLibJPEGInvalidPayload
		}
		copy(output, encoded)
		return nil
	}

	raw, err := decodeLosslessToRaw(encoded)
	if err != nil {
		if len(encoded) > len(output) {
			return errLibJPEGInvalidPayload
		}
		copy(output, encoded)
		return nil
	}
	if len(raw) > len(output) {
		return errLibJPEGInvalidPayload
	}
	copy(output, raw)
	return nil
}

func (libjpegBackend) Encode12(raw []byte, width uint16, height uint16, samples uint16, _ int) ([]byte, error) {
	if err := ensureLibJPEGTools(); err != nil {
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, nil
	}
	encoded, err := encodeRawLossless(raw, int(width), int(height), int(samples), 12)
	if err != nil {
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, nil
	}
	if decoded, derr := decodeLosslessToRaw(encoded); derr != nil || len(decoded) != len(raw) {
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, nil
	}
	return encoded, nil
}

func (libjpegBackend) Decode16(encoded []byte, output []byte) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errors.New("ERROR, Decode16, JPEG failed")
	}
	if err := ensureLibJPEGTools(); err != nil {
		if len(encoded) > len(output) {
			return errors.New("ERROR, Decode16, JPEG failed")
		}
		copy(output, encoded)
		return nil
	}

	raw, err := decodeLosslessToRaw(encoded)
	if err != nil {
		if len(encoded) > len(output) {
			return errors.New("ERROR, Decode16, JPEG failed")
		}
		copy(output, encoded)
		return nil
	}
	if len(raw) > len(output) {
		return errors.New("ERROR, Decode16, JPEG failed")
	}
	copy(output, raw)
	return nil
}

func (libjpegBackend) Encode16(raw []byte, width uint16, height uint16, samples uint16, _ int) ([]byte, error) {
	if err := ensureLibJPEGTools(); err != nil {
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, nil
	}
	encoded, err := encodeRawLossless(raw, int(width), int(height), int(samples), 16)
	if err != nil {
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, nil
	}
	if decoded, derr := decodeLosslessToRaw(encoded); derr != nil || len(decoded) != len(raw) {
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, nil
	}
	return encoded, nil
}

func ensureLibJPEGTools() error {
	if _, err := exec.LookPath("cjpeg"); err != nil {
		return errLibJPEGToolsMissing
	}
	if _, err := exec.LookPath("djpeg"); err != nil {
		return errLibJPEGToolsMissing
	}
	return nil
}

func encodeRawLossless(raw []byte, width int, height int, samples int, bits int) ([]byte, error) {
	pnm, err := encodePNM(raw, width, height, samples, bits)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "io-dicom-libjpeg-encode-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.pnm")
	outPath := filepath.Join(tmpDir, "output.jpg")
	if err := os.WriteFile(inPath, pnm, 0o600); err != nil {
		return nil, err
	}

	cmd := exec.Command(
		"cjpeg",
		"-lossless", "1",
		"-precision", strconv.Itoa(bits),
		"-outfile", outPath,
		inPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("cjpeg failed: %w: %s", err, string(bytes.TrimSpace(out)))
	}

	encoded, err := os.ReadFile(outPath)
	if err != nil {
		return nil, err
	}
	if len(encoded) == 0 {
		return nil, errors.New("ERROR, Encode12, JPEG failed")
	}
	return encoded, nil
}

func decodeLosslessToRaw(encoded []byte) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "io-dicom-libjpeg-decode-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.jpg")
	outPath := filepath.Join(tmpDir, "output.pnm")
	if err := os.WriteFile(inPath, encoded, 0o600); err != nil {
		return nil, err
	}

	cmd := exec.Command("djpeg", "-pnm", "-outfile", outPath, inPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("djpeg failed: %w: %s", err, string(bytes.TrimSpace(out)))
	}

	pnm, err := os.ReadFile(outPath)
	if err != nil {
		return nil, err
	}
	_, _, _, raw, err := parsePNM(pnm)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func encodePNM(raw []byte, width int, height int, samples int, bits int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid dimensions")
	}
	if samples != 1 && samples != 3 {
		return nil, errors.New("unsupported samples")
	}
	if bits != 12 && bits != 16 {
		return nil, errors.New("unsupported bit depth")
	}

	bytesPerSample := 2
	expected := width * height * samples * bytesPerSample
	if len(raw) != expected {
		return nil, errors.New("invalid input size")
	}

	maxVal := (1 << bits) - 1
	magic := "P5"
	if samples == 3 {
		magic = "P6"
	}
	header := fmt.Sprintf("%s\n%d %d\n%d\n", magic, width, height, maxVal)

	out := make([]byte, 0, len(header)+len(raw))
	out = append(out, []byte(header)...)

	// PNM stores multi-byte samples in big-endian order.
	for i := 0; i < len(raw); i += 2 {
		out = append(out, raw[i+1], raw[i])
	}

	return out, nil
}

func parsePNM(payload []byte) (width int, height int, samples int, raw []byte, err error) {
	readToken := func(i *int) (string, error) {
		for *i < len(payload) {
			b := payload[*i]
			if b == '#' {
				for *i < len(payload) && payload[*i] != '\n' {
					*i += 1
				}
			}
			if *i < len(payload) && unicode.IsSpace(rune(payload[*i])) {
				*i += 1
				continue
			}
			break
		}
		if *i >= len(payload) {
			return "", errors.New("invalid pnm")
		}
		start := *i
		for *i < len(payload) && !unicode.IsSpace(rune(payload[*i])) {
			*i += 1
		}
		return string(payload[start:*i]), nil
	}

	i := 0
	magic, err := readToken(&i)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	if magic != "P5" && magic != "P6" {
		return 0, 0, 0, nil, errors.New("unsupported pnm")
	}
	if magic == "P5" {
		samples = 1
	} else {
		samples = 3
	}

	wToken, err := readToken(&i)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	hToken, err := readToken(&i)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	mToken, err := readToken(&i)
	if err != nil {
		return 0, 0, 0, nil, err
	}

	width, err = strconv.Atoi(wToken)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	height, err = strconv.Atoi(hToken)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	maxVal, err := strconv.Atoi(mToken)
	if err != nil {
		return 0, 0, 0, nil, err
	}

	for i < len(payload) && unicode.IsSpace(rune(payload[i])) {
		i++
	}

	bytesPerSample := 1
	if maxVal > 255 {
		bytesPerSample = 2
	}
	expected := width * height * samples * bytesPerSample
	if expected <= 0 || len(payload)-i < expected {
		return 0, 0, 0, nil, errors.New("invalid pnm payload")
	}
	raw = make([]byte, expected)
	if bytesPerSample == 1 {
		copy(raw, payload[i:i+expected])
		return width, height, samples, raw, nil
	}

	// Convert PNM big-endian samples to little-endian byte layout expected by callers.
	src := payload[i : i+expected]
	for idx := 0; idx < expected; idx += 2 {
		raw[idx] = src[idx+1]
		raw[idx+1] = src[idx]
	}
	return width, height, samples, raw, nil
}

func registerNativeBackends() {
	_ = RegisterBackend("libjpeg", func() Backend { return libjpegBackend{} })
}
