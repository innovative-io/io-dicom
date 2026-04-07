//go:build st2110 && cgo

package smpte2110

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
	errST2110InvalidRawSize    = errors.New("invalid SMPTE ST 2110 raw payload size for dimensions")
	errST2110UnsupportedFormat = errors.New("unsupported SMPTE ST 2110 raw format")
)

type st2110Backend struct{}

func (st2110Backend) Name() string {
	return "st2110"
}

func (st2110Backend) Ready() error {
	return nil
}

func (st2110Backend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (st2110Backend) Decode(encoded []byte, output []byte, _ string) error {
	return st2110Backend{}.DecodeContext(context.Background(), encoded, output, "")
}

func (st2110Backend) DecodeContext(ctx context.Context, encoded []byte, output []byte, _ string) error {
	_ = ctx
	if len(encoded) == 0 || len(output) == 0 {
		return errors.New("invalid SMPTE ST 2110 payload size")
	}
	if len(encoded) > maxCodecPayloadBytes || len(output) > maxCodecPayloadBytes {
		return errors.New("invalid SMPTE ST 2110 payload size")
	}
	err := ffmpegbridge.Decode(encoded, output)
	if errors.Is(err, ffmpegbridge.ErrInvalidRawSize) {
		return errST2110InvalidRawSize
	}
	if errors.Is(err, ffmpegbridge.ErrUnsupportedLayout) {
		return errST2110UnsupportedFormat
	}
	return err
}

func (st2110Backend) Encode(raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, transferSyntaxUID string) ([]byte, error) {
	return st2110Backend{}.EncodeContext(context.Background(), raw, width, height, samples, bitsa, transferSyntaxUID)
}

func (st2110Backend) EncodeContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, bitsa uint16, _ string) ([]byte, error) {
	_ = ctx
	if len(raw) == 0 {
		return nil, errors.New("invalid SMPTE ST 2110 payload size")
	}
	if len(raw) > maxCodecPayloadBytes {
		return nil, errors.New("invalid SMPTE ST 2110 payload size")
	}
	_, bytesPerSample, err := pixelFormatForLayout(int(samples), int(bitsa))
	if err != nil {
		return nil, err
	}
	expected := int(width) * int(height) * int(samples) * bytesPerSample
	if len(raw) != expected {
		return nil, errST2110InvalidRawSize
	}
	encoded, err := ffmpegbridge.Encode(raw, width, height, samples, bitsa, ffmpegbridge.CodecFFV1, "matroska")
	if errors.Is(err, ffmpegbridge.ErrInvalidRawSize) {
		return nil, errST2110InvalidRawSize
	}
	if errors.Is(err, ffmpegbridge.ErrUnsupportedLayout) {
		return nil, errST2110UnsupportedFormat
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
	return "", 0, errST2110UnsupportedFormat
}

func registerNativeBackends() {
	_ = RegisterBackend("st2110", func() Backend { return st2110Backend{} })
}
