package codecs

import (
	"context"
	"errors"
	"fmt"

	"github.com/innovative-io/io-dicom/codecs/deflate"
	"github.com/innovative-io/io-dicom/codecs/internal/codecctx"
	jpegcodec "github.com/innovative-io/io-dicom/codecs/jpeg"
	jpeg2000codec "github.com/innovative-io/io-dicom/codecs/jpeg2000"
	jpeglscodec "github.com/innovative-io/io-dicom/codecs/jpegls"
	jpegxlcodec "github.com/innovative-io/io-dicom/codecs/jpegxl"
	jpipcodec "github.com/innovative-io/io-dicom/codecs/jpip"
	mpegcodec "github.com/innovative-io/io-dicom/codecs/mpeg"
	rlecodec "github.com/innovative-io/io-dicom/codecs/rle"
	smptecodec "github.com/innovative-io/io-dicom/codecs/smpte2110"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

// WithCodecThreads returns a context requesting that native codec backends use
// n intra-frame worker threads for decode/encode. n <= 0 means auto (all
// cores), the default. Pass n == 1 when decoding or encoding many independent
// frames concurrently so each call stays single-threaded and the frames
// themselves provide the parallelism, avoiding thread oversubscription.
//
// Native codec backends that support intra-frame threading honor the hint; the
// pure-Go codecs ignore it.
func WithCodecThreads(ctx context.Context, n int) context.Context {
	return codecctx.WithThreads(ctx, n)
}

// DecompressFrame decodes a single compressed frame payload into out. The
// caller must pre-allocate out with the exact uncompressed frame byte size.
//
// tsUID identifies the declared transfer syntax of the frame. For uncompressed
// transfer syntaxes the frame bytes are copied directly; for encapsulated
// syntaxes the appropriate codec is invoked.
//
// For nominally-uncompressed transfer syntaxes (Explicit/Implicit VR LE,
// EncapsulatedUncompressed) the payload is first checked for known compressed
// byte signatures so that non-conformant files that store compressed data
// under an uncompressed TS are handled gracefully.
func DecompressFrame(ctx context.Context, tsUID string, compressed []byte, bitsa uint16, photoInt string, out []byte) error {
	compLen := uint32(len(compressed))
	outLen := uint32(len(out))

	switch tsUID {
	case transfersyntax.DeflatedImageFrameCompression.UID:
		inflated, err := deflate.InflateFrame(compressed, int(outLen))
		if err != nil {
			return err
		}
		copy(out, inflated)

	case transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian.UID,
		// Non-conformant files may declare an uncompressed TS but store
		// compressed fragments. Check byte signatures first.
		transfersyntax.ExplicitVRLittleEndian.UID,
		transfersyntax.ImplicitVRLittleEndian.UID:
		if len(compressed) >= 2 {
			b0, b1 := compressed[0], compressed[1]
			switch {
			case b0 == 0xFF && b1 == 0xD8:
				// JPEG family (SOI marker). Check for JPEG-LS (SOF55 = 0xFF 0xF7).
				if len(compressed) >= 4 && compressed[2] == 0xFF && compressed[3] == 0xF7 {
					return jpeglscodec.JLSdecodeContext(ctx, compressed, compLen, out)
				}
				// JPEG lossless / baseline / extended: dispatch by SOF precision.
				prec := jpegcodec.SOFPrecision(compressed)
				if prec == 0 {
					if bitsa == 8 {
						return jpegcodec.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
					}
					if bitsa <= 12 {
						return jpegcodec.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)
					}
					return jpegcodec.DIJG16decodeContext(ctx, compressed, compLen, out, outLen)
				}
				switch {
				case prec == 8:
					return jpegcodec.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
				case prec <= 12:
					return jpegcodec.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)
				default:
					return jpegcodec.DIJG16decodeContext(ctx, compressed, compLen, out, outLen)
				}
			case b0 == 0xFF && b1 == 0x4F:
				// JPEG 2000 / HTJ2K codestream (SOC marker).
				return jpeg2000codec.J2KdecodeContext(ctx, compressed, compLen, out)
			case b0 == 0xFF && b1 == 0x0A:
				// JPEG XL bare codestream.
				return jpegxlcodec.JXLdecodeContext(ctx, compressed, compLen, out)
			}
		}
		if int(outLen) > len(compressed) {
			return errors.New("encapsulated uncompressed frame too small")
		}
		copy(out, compressed[:outLen])

	case transfersyntax.RLELossless.UID:
		return rlecodec.RLEdecode(compressed, out, compLen, outLen, photoInt)

	case transfersyntax.JPEGLosslessSV1.UID,
		transfersyntax.JPEGLossless.UID:
		prec := jpegcodec.SOFPrecision(compressed)
		if prec == 0 {
			if bitsa == 8 {
				return jpegcodec.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
			}
			if bitsa <= 12 {
				return jpegcodec.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)
			}
			return jpegcodec.DIJG16decodeContext(ctx, compressed, compLen, out, outLen)
		}
		switch {
		case prec == 8:
			return jpegcodec.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
		case prec <= 12:
			return jpegcodec.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)
		default:
			return jpegcodec.DIJG16decodeContext(ctx, compressed, compLen, out, outLen)
		}

	case transfersyntax.JPEGBaseline8Bit.UID:
		if bitsa == 8 {
			return jpegcodec.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
		}
		return jpegcodec.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)

	case transfersyntax.JPEGExtended12Bit.UID:
		if jpegcodec.SOFPrecision(compressed) == 8 {
			return jpegcodec.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
		}
		return jpegcodec.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)

	case transfersyntax.JPEGLSLossless.UID,
		transfersyntax.JPEGLSNearLossless.UID:
		return jpeglscodec.JLSdecodeContext(ctx, compressed, compLen, out)

	case transfersyntax.JPEG2000Lossless.UID,
		transfersyntax.JPEG2000MCLossless.UID,
		transfersyntax.HTJ2KLossless.UID,
		transfersyntax.HTJ2KLosslessRPCL.UID,
		transfersyntax.JPEG2000.UID,
		transfersyntax.JPEG2000MC.UID,
		transfersyntax.HTJ2K.UID:
		return jpeg2000codec.J2KdecodeContext(ctx, compressed, compLen, out)

	case transfersyntax.JPEGXLJPEGRecompression.UID:
		// A JPEG XL JPEG-recompression frame is a losslessly transcoded JPEG.
		// Reconstruct the original JPEG (pure-Go, byte-exact) and decode it with
		// the JPEG codec, which already handles photometric/precision. Fall back
		// to the JXL backend for inputs the reconstructor does not yet handle
		// (e.g. progressive scans).
		if jpegxlcodec.IsJPEGRecompression(compressed) {
			if jpegBytes, rerr := jpegxlcodec.ReconstructJPEG(compressed); rerr == nil {
				jb := uint32(len(jpegBytes))
				prec := jpegcodec.SOFPrecision(jpegBytes)
				switch {
				case prec == 8 || (prec == 0 && bitsa == 8):
					return jpegcodec.DIJG8decodeContext(ctx, jpegBytes, jb, out, outLen)
				case prec <= 12 || (prec == 0 && bitsa <= 12):
					return jpegcodec.DIJG12decodeContext(ctx, jpegBytes, jb, out, outLen)
				default:
					return jpegcodec.DIJG16decodeContext(ctx, jpegBytes, jb, out, outLen)
				}
			}
		}
		return jpegxlcodec.JXLdecodeContext(ctx, compressed, compLen, out)

	case transfersyntax.JPEGXLLossless.UID,
		transfersyntax.JPEGXL.UID:
		return jpegxlcodec.JXLdecodeContext(ctx, compressed, compLen, out)

	case transfersyntax.JPIPHTJ2KReferenced.UID,
		transfersyntax.JPIPHTJ2KReferencedDeflate.UID:
		return jpipcodec.JPIPdecodeContext(ctx, compressed, compLen, out, tsUID)

	case transfersyntax.MPEG2MPML.UID,
		transfersyntax.MPEG2MPMLF.UID,
		transfersyntax.MPEG2MPHL.UID,
		transfersyntax.MPEG2MPHLF.UID,
		transfersyntax.MPEG4HP41.UID,
		transfersyntax.MPEG4HP41F.UID,
		transfersyntax.MPEG4HP41BD.UID,
		transfersyntax.MPEG4HP41BDF.UID,
		transfersyntax.MPEG4HP422D.UID,
		transfersyntax.MPEG4HP422DF.UID,
		transfersyntax.MPEG4HP423D.UID,
		transfersyntax.MPEG4HP423DF.UID,
		transfersyntax.MPEG4HP42STEREO.UID,
		transfersyntax.MPEG4HP42STEREOF.UID,
		transfersyntax.HEVCMP51.UID,
		transfersyntax.HEVCM10P51.UID:
		return mpegcodec.MPEGdecodeContext(ctx, compressed, compLen, out, tsUID)

	case transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo.UID,
		transfersyntax.SMPTEST211020UncompressedInterlacedActiveVideo.UID,
		transfersyntax.SMPTEST211030PCMDigitalAudio.UID:
		return smptecodec.SMPTE2110decodeContext(ctx, compressed, compLen, out, tsUID)

	default:
		return fmt.Errorf("codecs: unsupported transfer syntax for frame decompression: %s", tsUID)
	}
	return nil
}
