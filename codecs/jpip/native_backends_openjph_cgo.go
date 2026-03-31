//go:build openjph && cgo

package jpip

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"unicode"

	"github.com/innovative-io/io-dicom/codecs/internal/nativeenv"
)

const nativeBackendEnabled = true

func init() {
	nativeenv.Init()
}

var (
	errOpenJPHToolingUnavailable = errors.New("openjph/openjpeg tooling is unavailable in PATH")
	errOpenJPHInvalidRawSize     = errors.New("invalid jpip raw payload size for dimensions")
	errOpenJPHUnsupportedPNM     = errors.New("unsupported pnm payload from openjph/openjpeg")
)

type openjphBackend struct{}

func (openjphBackend) Name() string {
	return "openjph"
}

func (openjphBackend) Ready() error {
	_, _, err := resolveCodecTools()
	return err
}

func (openjphBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (openjphBackend) Decode(encoded []byte, output []byte, _ string) error {
	return openjphBackend{}.DecodeContext(context.Background(), encoded, output, "")
}

func (openjphBackend) DecodeContext(ctx context.Context, encoded []byte, output []byte, _ string) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errors.New("invalid JPIP payload size")
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errors.New("invalid JPIP payload size")
	}
	compressCmd, decompressCmd, err := resolveCodecTools()
	_ = compressCmd
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "io-dicom-openjph-decode-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.j2k")
	outPath := filepath.Join(tmpDir, "output.pnm")
	if err := os.WriteFile(inPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write jpip payload: %w", err)
	}

	cmd := nativeenv.CommandContext(ctx, decompressCmd, "-i", inPath, "-o", outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s decode failed: %w: %s", decompressCmd, err, stringsTrim(string(out)))
	}

	pnm, err := os.ReadFile(outPath)
	if err != nil {
		return fmt.Errorf("read decoded payload: %w", err)
	}

	_, _, _, raw, err := parsePNM(pnm)
	if err != nil {
		return err
	}
	if len(raw) > len(output) {
		return errors.New("invalid JPIP payload size")
	}
	copy(output, raw)
	return nil
}

func (openjphBackend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error) {
	return openjphBackend{}.EncodeContext(context.Background(), raw, width, height, samples, bitsa, transferSyntaxUID)
}

func (openjphBackend) EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, _ string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("invalid JPIP payload size")
	}
	if len(raw) > maxCodecPayloadBytes {
		return nil, errors.New("invalid JPIP payload size")
	}
	compressCmd, _, err := resolveCodecTools()
	if err != nil {
		return nil, err
	}

	pnm, err := encodePNM(raw, int(width), int(height), int(samples), int(bitsa))
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "io-dicom-openjph-encode-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.pnm")
	outPath := filepath.Join(tmpDir, "output.j2k")
	if err := os.WriteFile(inPath, pnm, 0o600); err != nil {
		return nil, fmt.Errorf("write pnm payload: %w", err)
	}

	args := []string{"-i", inPath, "-o", outPath}
	if filepath.Base(compressCmd) == "opj_compress" {
		args = append(args, "-n", strconv.Itoa(openjpegResolutionCount(width, height)))
	}
	cmd := nativeenv.CommandContext(ctx, compressCmd, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s encode failed: %w: %s", compressCmd, err, stringsTrim(string(out)))
	}

	encoded, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read encoded payload: %w", err)
	}
	if len(encoded) == 0 {
		return nil, errors.New("openjph/openjpeg encode produced empty payload")
	}
	return encoded, nil
}

var (
	resolvedOpenJPHCompress   string
	resolvedOpenJPHDecompress string
	resolveToolsOnce          sync.Once
	resolveToolsErr           error
)

func resolveCodecTools() (compressCmd string, decompressCmd string, err error) {
	resolveToolsOnce.Do(func() {
		if p, cErr := nativeenv.LookPath("ojph_compress"); cErr == nil {
			if q, dErr := nativeenv.LookPath("ojph_decompress"); dErr == nil {
				resolvedOpenJPHCompress = p
				resolvedOpenJPHDecompress = q
				return
			}
		}
		if p, cErr := nativeenv.LookPath("opj_compress"); cErr == nil {
			if q, dErr := nativeenv.LookPath("opj_decompress"); dErr == nil {
				resolvedOpenJPHCompress = p
				resolvedOpenJPHDecompress = q
				return
			}
		}
		resolveToolsErr = errOpenJPHToolingUnavailable
	})
	if resolveToolsErr != nil {
		return "", "", resolveToolsErr
	}
	return resolvedOpenJPHCompress, resolvedOpenJPHDecompress, nil
}

func openjpegResolutionCount(width uint16, height uint16) int {
	minDim := int(width)
	if int(height) < minDim {
		minDim = int(height)
	}
	if minDim <= 1 {
		return 1
	}
	resolutions := bits.Len(uint(minDim))
	if resolutions > 6 {
		return 6
	}
	return resolutions
}

func encodePNM(raw []byte, width int, height int, samples int, bits int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, errOpenJPHUnsupportedPNM
	}
	if samples != 1 && samples != 3 {
		return nil, errOpenJPHUnsupportedPNM
	}
	if bits != 8 && bits != 12 && bits != 16 {
		return nil, errOpenJPHUnsupportedPNM
	}

	bytesPerSample := 1
	maxVal := 255
	if bits > 8 {
		bytesPerSample = 2
		maxVal = (1 << bits) - 1
	}
	expected := width * height * samples * bytesPerSample
	if len(raw) != expected {
		return nil, errOpenJPHInvalidRawSize
	}

	magic := "P5"
	if samples == 3 {
		magic = "P6"
	}

	header := fmt.Sprintf("%s\n%d %d\n%d\n", magic, width, height, maxVal)
	out := make([]byte, 0, len(header)+len(raw))
	out = append(out, []byte(header)...)
	out = append(out, raw...)
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
			return "", errOpenJPHUnsupportedPNM
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
		return 0, 0, 0, nil, errOpenJPHUnsupportedPNM
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
		return 0, 0, 0, nil, errOpenJPHUnsupportedPNM
	}
	height, err = strconv.Atoi(hToken)
	if err != nil {
		return 0, 0, 0, nil, errOpenJPHUnsupportedPNM
	}
	if width <= 0 || height <= 0 {
		return 0, 0, 0, nil, errOpenJPHUnsupportedPNM
	}
	maxVal, err := strconv.Atoi(mToken)
	if err != nil {
		return 0, 0, 0, nil, errOpenJPHUnsupportedPNM
	}
	if maxVal != 255 && maxVal != 4095 && maxVal != 65535 {
		return 0, 0, 0, nil, errOpenJPHUnsupportedPNM
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
		return 0, 0, 0, nil, errOpenJPHUnsupportedPNM
	}
	raw = make([]byte, expected)
	copy(raw, payload[i:i+expected])
	return width, height, samples, raw, nil
}

func stringsTrim(s string) string {
	return string(bytes.TrimSpace([]byte(s)))
}

func registerNativeBackends() {
	_ = RegisterBackend("openjph", func() Backend { return openjphBackend{} })
}
