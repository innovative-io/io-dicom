//go:build ffmpeg && cgo

package mpeg

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
	errFFmpegToolingUnavailable = errors.New("ffmpeg tooling is unavailable in PATH")
	errFFmpegInvalidRawSize     = errors.New("invalid raw payload size for dimensions")
	errFFmpegUnsupportedFormat  = errors.New("unsupported raw format for ffmpeg backend")
)

type ffmpegBackend struct{}

func (ffmpegBackend) Name() string {
	return "ffmpeg"
}

func (ffmpegBackend) Ready() error {
	return ensureFFmpegTools()
}

func (ffmpegBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (ffmpegBackend) Decode(encoded []byte, output []byte, transferSyntaxUID string) error {
	return ffmpegBackend{}.DecodeContext(context.Background(), encoded, output, transferSyntaxUID)
}

func (ffmpegBackend) DecodeContext(ctx context.Context, encoded []byte, output []byte, transferSyntaxUID string) error {
	if len(encoded) == 0 || len(output) == 0 {
		return errors.New("invalid MPEG payload size")
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errors.New("invalid MPEG payload size")
	}
	if err := ensureFFmpegTools(); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "io-dicom-ffmpeg-decode-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.bin")
	outPath := filepath.Join(tmpDir, "frame.raw")
	if err := os.WriteFile(inPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write video payload: %w", err)
	}

	width, height, err := probeDimensions(ctx, inPath)
	if err != nil {
		return err
	}

	pixFmt, err := inferPixelFormatFromOutputLen(len(output), width, height)
	if err != nil {
		return err
	}

	codec, err := codecForUID(transferSyntaxUID)
	if err != nil {
		return err
	}

	args := []string{
		"-y",
		"-loglevel", "error",
		"-c:v", codec,
		"-i", inPath,
		"-frames:v", "1",
		"-f", "rawvideo",
		"-pix_fmt", pixFmt,
		outPath,
	}
	cmd := nativeenv.CommandContext(ctx, resolvedFFmpeg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg decode failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		return fmt.Errorf("read decoded frame: %w", err)
	}
	if len(raw) > len(output) {
		return errors.New("invalid MPEG payload size")
	}
	copy(output, raw)
	return nil
}

func (ffmpegBackend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error) {
	return ffmpegBackend{}.EncodeContext(context.Background(), raw, width, height, samples, bitsa, transferSyntaxUID)
}

func (ffmpegBackend) EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("invalid MPEG payload size")
	}
	if len(raw) > maxCodecPayloadBytes {
		return nil, errors.New("invalid MPEG payload size")
	}
	if err := ensureFFmpegTools(); err != nil {
		return nil, err
	}

	pixFmt, bytesPerSample, err := pixelFormatForLayout(int(samples), int(bitsa))
	if err != nil {
		return nil, err
	}
	expected := int(width) * int(height) * int(samples) * bytesPerSample
	if len(raw) != expected {
		return nil, errFFmpegInvalidRawSize
	}

	codec, err := codecForUID(transferSyntaxUID)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "io-dicom-ffmpeg-encode-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "frame.raw")
	outPath := filepath.Join(tmpDir, "output.ts")
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
		"-c:v", codec,
		"-f", "mpegts",
		outPath,
	}
	cmd := nativeenv.CommandContext(ctx, resolvedFFmpeg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg encode failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	encoded, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read encoded video: %w", err)
	}
	if len(encoded) == 0 {
		return nil, errors.New("ffmpeg encode produced empty payload")
	}
	return encoded, nil
}

var (
	resolvedFFmpeg   string
	resolvedFFprobe  string
	ffmpegToolsOnce  sync.Once
	ffmpegToolsError error
)

func ensureFFmpegTools() error {
	ffmpegToolsOnce.Do(func() {
		p, err := nativeenv.LookPath("ffmpeg")
		if err != nil {
			ffmpegToolsError = errFFmpegToolingUnavailable
			return
		}
		q, err := nativeenv.LookPath("ffprobe")
		if err != nil {
			ffmpegToolsError = errFFmpegToolingUnavailable
			return
		}
		resolvedFFmpeg = p
		resolvedFFprobe = q
	})
	return ffmpegToolsError
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
	return "", 0, errFFmpegUnsupportedFormat
}

func codecForUID(uid string) (string, error) {
	switch uid {
	case "1.2.840.10008.1.2.4.100", "1.2.840.10008.1.2.4.100.1",
		"1.2.840.10008.1.2.4.101", "1.2.840.10008.1.2.4.101.1":
		return "mpeg2video", nil
	case "1.2.840.10008.1.2.4.102", "1.2.840.10008.1.2.4.102.1",
		"1.2.840.10008.1.2.4.103", "1.2.840.10008.1.2.4.103.1",
		"1.2.840.10008.1.2.4.104", "1.2.840.10008.1.2.4.104.1",
		"1.2.840.10008.1.2.4.105", "1.2.840.10008.1.2.4.105.1",
		"1.2.840.10008.1.2.4.106", "1.2.840.10008.1.2.4.106.1":
		return "mpeg4", nil
	case "1.2.840.10008.1.2.4.107", "1.2.840.10008.1.2.4.108":
		return "hevc", nil
	default:
		return "", errUnsupportedTransferSyntax
	}
}

func probeDimensions(ctx context.Context, path string) (int, int, error) {
	cmd := nativeenv.CommandContext(
		ctx,
		resolvedFFprobe,
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
		return "", errFFmpegUnsupportedFormat
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
	return "", errFFmpegInvalidRawSize
}

func registerNativeBackends() {
	_ = RegisterBackend("ffmpeg", func() Backend { return ffmpegBackend{} })
}
