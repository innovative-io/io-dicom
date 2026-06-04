package media_test

import (
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
)

type roundTripAssertionMode string

const (
	roundTripAssertionExact    roundTripAssertionMode = "exact"
	roundTripAssertionDelta3   roundTripAssertionMode = "delta3"
	roundTripAssertionNonEmpty roundTripAssertionMode = "non-empty"
)

type representativeRoundTripCase struct {
	name              string
	ts                *transfersyntax.TransferSyntax
	newObj            func() media.DICOMObject
	want              []byte
	passthroughMode   roundTripAssertionMode
	nativeFamily      string
	nativeBackend     string
	nativeMode        roundTripAssertionMode
	nativeToolingHint string
	// passthroughCannotEncode marks codecs with no pure-Go encoder (JPEG 2000,
	// JPEG XL): with passthrough backends the forward transcode to this syntax
	// must fail rather than emit raw bytes as a fake compressed stream.
	passthroughCannotEncode bool
}

func representativePixelRoundTripCases() []representativeRoundTripCase {
	return []representativeRoundTripCase{
		{
			name:            "EncapsulatedUncompressed",
			ts:              transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian,
			newObj:          newRGBRoundTripObject,
			want:            []byte{10, 20, 30, 40, 50, 60},
			passthroughMode: roundTripAssertionExact,
		},
		{
			name:            "DeflatedImageFrameCompression",
			ts:              transfersyntax.DeflatedImageFrameCompression,
			newObj:          newRGBRoundTripObject,
			want:            []byte{10, 20, 30, 40, 50, 60},
			passthroughMode: roundTripAssertionExact,
		},
		{
			name:            "RLELossless",
			ts:              transfersyntax.RLELossless,
			newObj:          newRGBRoundTripObject,
			want:            []byte{10, 20, 30, 40, 50, 60},
			passthroughMode: roundTripAssertionExact,
		},
		{
			name:            "JPEGBaseline8Bit",
			ts:              transfersyntax.JPEGBaseline8Bit,
			newObj:          func() media.DICOMObject { return newMonoRoundTripObject(transfersyntax.ExplicitVRLittleEndian) },
			want:            []byte{7, 17, 27, 37},
			passthroughMode: roundTripAssertionDelta3,
		},
		{
			name:            "JPEGLosslessSV1",
			ts:              transfersyntax.JPEGLosslessSV1,
			newObj:          func() media.DICOMObject { return newMonoRoundTripObject(transfersyntax.ExplicitVRLittleEndian) },
			want:            []byte{7, 17, 27, 37},
			passthroughMode: roundTripAssertionDelta3,
		},
		{
			name:              "JPEGExtended12Bit",
			ts:                transfersyntax.JPEGExtended12Bit,
			newObj:            newMono12BitRoundTripObject,
			want:              []byte{0x34, 0x02, 0xAB, 0x03},
			passthroughMode:   roundTripAssertionExact,
			nativeFamily:      "jpeg",
			nativeBackend:     "libjpeg",
			nativeMode:        roundTripAssertionExact,
			nativeToolingHint: "tooling is unavailable",
		},
		{
			name:              "JPEGLSLossless",
			ts:                transfersyntax.JPEGLSLossless,
			newObj:            func() media.DICOMObject { return newMonoRoundTripObject(transfersyntax.ExplicitVRLittleEndian) },
			want:              []byte{7, 17, 27, 37},
			passthroughMode:   roundTripAssertionExact,
			nativeFamily:      "jpegls",
			nativeBackend:     "charls",
			nativeMode:        roundTripAssertionExact,
			nativeToolingHint: "tooling is unavailable",
		},
		{
			name:                    "JPEG2000Lossless",
			ts:                      transfersyntax.JPEG2000Lossless,
			newObj:                  func() media.DICOMObject { return newMonoRoundTripObject(transfersyntax.ExplicitVRLittleEndian) },
			want:                    []byte{7, 17, 27, 37},
			passthroughMode:         roundTripAssertionExact,
			nativeFamily:            "jpeg2000",
			nativeBackend:           "openjpeg",
			nativeMode:              roundTripAssertionExact,
			nativeToolingHint:       "tooling is unavailable",
			passthroughCannotEncode: true,
		},
		{
			name:                    "JPEGXL",
			ts:                      transfersyntax.JPEGXL,
			newObj:                  func() media.DICOMObject { return newMonoRoundTripObject(transfersyntax.ExplicitVRLittleEndian) },
			want:                    []byte{7, 17, 27, 37},
			passthroughMode:         roundTripAssertionExact,
			nativeFamily:            "jpegxl",
			nativeBackend:           "libjxl",
			nativeMode:              roundTripAssertionDelta3,
			nativeToolingHint:       "tooling is unavailable",
			passthroughCannotEncode: true,
		},
		{
			name:                    "JPEGXLLossless",
			ts:                      transfersyntax.JPEGXLLossless,
			newObj:                  func() media.DICOMObject { return newMonoRoundTripObject(transfersyntax.ExplicitVRLittleEndian) },
			want:                    []byte{7, 17, 27, 37},
			passthroughMode:         roundTripAssertionExact,
			nativeFamily:            "jpegxl",
			nativeBackend:           "libjxl",
			nativeMode:              roundTripAssertionExact,
			nativeToolingHint:       "tooling is unavailable",
			passthroughCannotEncode: true,
		},
		{
			name:              "JPIPHTJ2KReferenced",
			ts:                transfersyntax.JPIPHTJ2KReferenced,
			newObj:            func() media.DICOMObject { return newMonoRoundTripObject(transfersyntax.ExplicitVRLittleEndian) },
			want:              []byte{7, 17, 27, 37},
			passthroughMode:   roundTripAssertionExact,
			nativeFamily:      "jpip",
			nativeBackend:     "openjph",
			nativeMode:        roundTripAssertionExact,
			nativeToolingHint: "tooling is unavailable",
		},
		{
			name:              "MPEG2MPML",
			ts:                transfersyntax.MPEG2MPML,
			newObj:            func() media.DICOMObject { return newMonoRoundTripObject(transfersyntax.ExplicitVRLittleEndian) },
			want:              []byte{7, 17, 27, 37},
			passthroughMode:   roundTripAssertionExact,
			nativeFamily:      "mpeg",
			nativeBackend:     "ffmpeg",
			nativeMode:        roundTripAssertionNonEmpty,
			nativeToolingHint: "tooling is unavailable",
		},
		{
			name:              "SMPTEST211020Progressive",
			ts:                transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo,
			newObj:            func() media.DICOMObject { return newMonoRoundTripObject(transfersyntax.ExplicitVRLittleEndian) },
			want:              []byte{7, 17, 27, 37},
			passthroughMode:   roundTripAssertionExact,
			nativeFamily:      "smpte2110",
			nativeBackend:     "st2110",
			nativeMode:        roundTripAssertionNonEmpty,
			nativeToolingHint: "tooling is unavailable",
		},
	}
}

func assertRoundTripByMode(t *testing.T, obj media.DICOMObject, want []byte, mode roundTripAssertionMode) {
	t.Helper()

	switch mode {
	case roundTripAssertionExact:
		assertPixelPrefix(t, obj, want)
	case roundTripAssertionDelta3:
		assertPixelPrefixWithinDelta(t, obj, want, 3)
	case roundTripAssertionNonEmpty:
		out, err := obj.GetPixelData(0)
		if err != nil {
			t.Fatalf("GetPixelData failed: %v", err)
		}
		if len(out) == 0 {
			t.Fatalf("expected non-empty pixel payload")
		}
	default:
		t.Fatalf("unknown assertion mode %q", mode)
	}
}
