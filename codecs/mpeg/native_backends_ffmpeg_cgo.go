//go:build ffmpeg && cgo

package mpeg

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

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

func (ffmpegBackend) Decode(encoded []byte, output []byte, transferSyntaxUID string) error {
	if len(encoded) == 0 || len(output) == 0 {
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

	width, height, err := probeDimensions(inPath)
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
	cmd := exec.Command("ffmpeg", args...)
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
	if len(raw) == 0 {
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
	cmd := exec.Command("ffmpeg", args...)
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

func ensureFFmpegTools() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return errFFmpegToolingUnavailable
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return errFFmpegToolingUnavailable
	}
	return nil
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
	if !isSupportedMPEGUID(uid) {
		return "", errUnsupportedTransferSyntax
	}
	// mpeg2video is widely available across ffmpeg builds and keeps tagged tests portable.
	return "mpeg2video", nil
}

func probeDimensions(path string) (int, int, error) {
	cmd := exec.Command(
		"ffprobe",
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
	parts := strings.Split(strings.TrimSpace(string(out)), "x")
	if len(parts) != 2 {
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
