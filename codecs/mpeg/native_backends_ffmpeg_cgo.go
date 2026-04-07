//go:build ffmpeg && cgo

package mpeg

import (
	"context"
	"errors"

	"github.com/innovative-io/io-dicom/codecs/internal/ffmpegbridge"
	"github.com/innovative-io/io-dicom/codecs/internal/nativeenv"
)

const nativeBackendEnabled = true

func init() {
	nativeenv.Init()
}

var (
	errFFmpegInvalidRawSize    = errors.New("invalid raw payload size for dimensions")
	errFFmpegUnsupportedFormat = errors.New("unsupported raw format for ffmpeg backend")
)

type ffmpegBackend struct{}

func (ffmpegBackend) Name() string {
	return "ffmpeg"
}

func (ffmpegBackend) Ready() error {
	return nil
}

func (ffmpegBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (ffmpegBackend) Decode(encoded []byte, output []byte, transferSyntaxUID string) error {
	return ffmpegBackend{}.DecodeContext(context.Background(), encoded, output, transferSyntaxUID)
}

func (ffmpegBackend) DecodeContext(ctx context.Context, encoded []byte, output []byte, transferSyntaxUID string) error {
	_ = ctx
	_ = transferSyntaxUID
	if len(encoded) == 0 || len(output) == 0 {
		return errors.New("invalid MPEG payload size")
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errors.New("invalid MPEG payload size")
	}
	err := ffmpegbridge.Decode(encoded, output)
	if errors.Is(err, ffmpegbridge.ErrInvalidRawSize) {
		return errFFmpegInvalidRawSize
	}
	if errors.Is(err, ffmpegbridge.ErrUnsupportedLayout) {
		return errFFmpegUnsupportedFormat
	}
	return err
}

func (ffmpegBackend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error) {
	return ffmpegBackend{}.EncodeContext(context.Background(), raw, width, height, samples, bitsa, transferSyntaxUID)
}

func (ffmpegBackend) EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error) {
	_ = ctx
	if len(raw) == 0 {
		return nil, errors.New("invalid MPEG payload size")
	}
	if len(raw) > maxCodecPayloadBytes {
		return nil, errors.New("invalid MPEG payload size")
	}
	_, bytesPerSample, err := pixelFormatForLayout(int(samples), int(bitsa))
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
	encoded, err := ffmpegbridge.Encode(raw, width, height, samples, bitsa, codec, "mpegts")
	if errors.Is(err, ffmpegbridge.ErrInvalidRawSize) {
		return nil, errFFmpegInvalidRawSize
	}
	if errors.Is(err, ffmpegbridge.ErrUnsupportedLayout) {
		return nil, errFFmpegUnsupportedFormat
	}
	return encoded, err
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

func codecForUID(uid string) (ffmpegbridge.CodecID, error) {
	switch uid {
	case "1.2.840.10008.1.2.4.100", "1.2.840.10008.1.2.4.100.1",
		"1.2.840.10008.1.2.4.101", "1.2.840.10008.1.2.4.101.1":
		return ffmpegbridge.CodecMPEG2Video, nil
	case "1.2.840.10008.1.2.4.102", "1.2.840.10008.1.2.4.102.1",
		"1.2.840.10008.1.2.4.103", "1.2.840.10008.1.2.4.103.1",
		"1.2.840.10008.1.2.4.104", "1.2.840.10008.1.2.4.104.1",
		"1.2.840.10008.1.2.4.105", "1.2.840.10008.1.2.4.105.1",
		"1.2.840.10008.1.2.4.106", "1.2.840.10008.1.2.4.106.1":
		return ffmpegbridge.CodecMPEG4, nil
	case "1.2.840.10008.1.2.4.107", "1.2.840.10008.1.2.4.108":
		return ffmpegbridge.CodecHEVC, nil
	default:
		return 0, errUnsupportedTransferSyntax
	}
}

func registerNativeBackends() {
	_ = RegisterBackend("ffmpeg", func() Backend { return ffmpegBackend{} })
}
