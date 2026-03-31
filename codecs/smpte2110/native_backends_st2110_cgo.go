//go:build st2110 && cgo

package smpte2110

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/innovative-io/io-dicom/codecs/internal/nativeenv"
)

const nativeBackendEnabled = true

func init() {
	nativeenv.Init()
}

var (
	errST2110ToolingUnavailable = errors.New("st2110 ffmpeg tooling is unavailable in PATH")
	errST2110InvalidRawSize     = errors.New("invalid SMPTE ST 2110 raw payload size for dimensions")
	errST2110UnsupportedFormat  = errors.New("unsupported SMPTE ST 2110 raw format")
)

type st2110Backend struct{}

func (st2110Backend) Name() string {
	return "st2110"
}

func (st2110Backend) Ready() error {
	return ensureST2110Tools()
}

func (st2110Backend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (st2110Backend) Decode(encoded []byte, output []byte, _ string) error {
	return st2110Backend{}.DecodeContext(context.Background(), encoded, output, "")
}

func (st2110Backend) DecodeContext(ctx context.Context, encoded []byte, output []byte, _ string) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errors.New("invalid SMPTE ST 2110 payload size")
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errors.New("invalid SMPTE ST 2110 payload size")
	}
	if err := ensureST2110Tools(); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "io-dicom-st2110-decode-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.mkv")
	outPath := filepath.Join(tmpDir, "frame.raw")
	if err := os.WriteFile(inPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write stream payload: %w", err)
	}

	width, height, err := probeDimensions(ctx, inPath)
	if err != nil {
		return err
	}
	pixFmt, err := inferPixelFormatFromOutputLen(len(output), width, height)
	if err != nil {
		return err
	}

	args := []string{
		"-y",
		"-loglevel", "error",
		"-i", inPath,
		"-frames:v", "1",
		"-f", "rawvideo",
		"-pix_fmt", pixFmt,
		outPath,
	}
	cmd := nativeenv.CommandContext(ctx, resolvedST2110FFmpeg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg decode failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		return fmt.Errorf("read decoded frame: %w", err)
	}
	if len(raw) > len(output) {
		return errors.New("invalid SMPTE ST 2110 payload size")
	}
	copy(output, raw)
	return nil
}

func (st2110Backend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error) {
	return st2110Backend{}.EncodeContext(context.Background(), raw, width, height, samples, bitsa, transferSyntaxUID)
}

func (st2110Backend) EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, _ string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("invalid SMPTE ST 2110 payload size")
	}
	if len(raw) > maxCodecPayloadBytes {
		return nil, errors.New("invalid SMPTE ST 2110 payload size")
	}
	if err := ensureST2110Tools(); err != nil {
		return nil, err
	}

	pixFmt, bytesPerSample, err := pixelFormatForLayout(int(samples), int(bitsa))
	if err != nil {
		return nil, err
	}
	expected := int(width) * int(height) * int(samples) * bytesPerSample
	if len(raw) != expected {
		return nil, errST2110InvalidRawSize
	}

	tmpDir, err := os.MkdirTemp("", "io-dicom-st2110-encode-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "frame.raw")
	outPath := filepath.Join(tmpDir, "output.mkv")
	if err := os.WriteFile(inPath, raw, 0o600); err != nil {
		return nil, fmt.Errorf("write raw frame: %w", err)
	}

	sizeArg := fmt.Sprintf("%dx%d", width, height)
	args := []string{
		"-y",
		"-loglevel", "error",
		"-f", "rawvideo",
		"-pix_fmt", pixFmt,
		"-s", sizeArg,
		"-r", "1",
		"-i", inPath,
		"-frames:v", "1",
		"-an",
		"-c:v", "ffv1",
		"-f", "matroska",
		outPath,
	}
	cmd := nativeenv.CommandContext(ctx, resolvedST2110FFmpeg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg encode failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	encoded, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read encoded stream: %w", err)
	}
	if len(encoded) == 0 {
		return nil, errors.New("st2110 encode produced empty payload")
	}
	return encoded, nil
}

var (
	resolvedST2110FFmpeg  string
	resolvedST2110FFprobe string
	st2110ToolsOnce       sync.Once
	st2110ToolsError      error
)

func ensureST2110Tools() error {
	st2110ToolsOnce.Do(func() {
		p, err := nativeenv.LookPath("ffmpeg")
		if err != nil {
			st2110ToolsError = errST2110ToolingUnavailable
			return
		}
		q, err := nativeenv.LookPath("ffprobe")
		if err != nil {
			st2110ToolsError = errST2110ToolingUnavailable
			return
		}
		resolvedST2110FFmpeg = p
		resolvedST2110FFprobe = q
	})
	return st2110ToolsError
}

func pixelFormatForLayout(samples int, bits int) (string, int, error) {
	if bits == 8 {
		switch samples {
		case 1:
			return "gray", 1, nil
		case 3:
			return "rgb24", 1, nil
		}
	}
	if bits == 16 {
		switch samples {
		case 1:
			return "gray16le", 2, nil
		case 3:
			return "rgb48le", 2, nil
		}
	}
	return "", 0, errST2110UnsupportedFormat
}

func probeDimensions(ctx context.Context, path string) (int, int, error) {
	if err := ensureST2110Tools(); err != nil {
		return 0, 0, err
	}
	cmd := nativeenv.CommandContext(
		ctx,
		resolvedST2110FFprobe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0:s=x",
		path,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return parseProbeDimensionsOutput(string(out))
}

func parseProbeDimensionsOutput(output string) (int, int, error) {
	parts := strings.FieldsFunc(strings.TrimSpace(output), func(r rune) bool {
		return r < '0' || r > '9'
	})
	if len(parts) < 2 {
		return 0, 0, errors.New("unexpected ffprobe dimension output")
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, errors.New("invalid ffprobe width")
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, errors.New("invalid ffprobe height")
	}
	return width, height, nil
}

func inferPixelFormatFromOutputLen(outputLen int, width int, height int) (string, error) {
	if width <= 0 || height <= 0 {
		return "", errST2110UnsupportedFormat
	}
	pixels := width * height
	if outputLen == pixels {
		return "gray", nil
	}
	if outputLen == pixels*3 {
		return "rgb24", nil
	}
	if outputLen == pixels*2 {
		return "gray16le", nil
	}
	if outputLen == pixels*6 {
		return "rgb48le", nil
	}
	return "", errST2110InvalidRawSize
}

func registerNativeBackends() {
	_ = RegisterBackend("st2110", func() Backend { return st2110Backend{} })
}
