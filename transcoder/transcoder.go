// Package transcoder provides pixel data transcoding between DICOM transfer
// syntaxes. The primary entry points are ChangeTransferSyntax and
// ChangeTransferSyntaxContext, which accept any media.DICOMObject and
// re-encode its pixel data to the requested output transfer syntax.
//
// Usage:
//
//	err := transcoder.ChangeTransferSyntax(obj, transfersyntax.ExplicitVRLittleEndian)
//	err := transcoder.ChangeTransferSyntaxContext(ctx, obj, transfersyntax.JPEGBaseline8Bit)
package transcoder

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/innovative-io/io-dicom/codecs"
	"github.com/innovative-io/io-dicom/codecs/deflate"
	"github.com/innovative-io/io-dicom/codecs/jpeg"
	"github.com/innovative-io/io-dicom/codecs/jpeg2000"
	"github.com/innovative-io/io-dicom/codecs/jpegls"
	"github.com/innovative-io/io-dicom/codecs/jpegxl"
	"github.com/innovative-io/io-dicom/codecs/jpip"
	"github.com/innovative-io/io-dicom/codecs/mpeg"
	"github.com/innovative-io/io-dicom/codecs/rle"
	"github.com/innovative-io/io-dicom/codecs/smpte2110"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
)

// maxPixelDataBytes is the upper bound for a single pixel data allocation (512 MiB).
const maxPixelDataBytes = 512 * 1024 * 1024

// ChangeTransferSyntax re-encodes obj's pixel data to outTS using a
// background context. It is a convenience wrapper around
// ChangeTransferSyntaxContext.
func ChangeTransferSyntax(obj media.DICOMObject, outTS *transfersyntax.TransferSyntax) error {
	return ChangeTransferSyntaxContext(context.Background(), obj, outTS)
}

// ChangeTransferSyntaxContext re-encodes obj's pixel data to outTS, honouring
// ctx for cancellation during codec operations. When source and destination
// transfer syntaxes share the same tag-level wire encoding and no pixel data
// tag is present, only the transfer syntax header is updated.
func ChangeTransferSyntaxContext(ctx context.Context, obj media.DICOMObject, outTS *transfersyntax.TransferSyntax) error {
	if ctx == nil {
		ctx = context.Background()
	}

	flag := false

	var i int
	var rows, cols, bitss, bitsa, planar, pixelrep uint16
	var PhotoInt string
	sq := 0
	frames := uint32(0)
	RGB := false
	icon := false

	if obj.GetTransferSyntax().UID == outTS.UID {
		return nil
	}

	if !transfersyntax.SupportedTransferSyntax(outTS.UID) {
		return fmt.Errorf("unsupported transfer syntax %s", outTS.Name)
	}

	for i = 0; i < obj.TagCount(); i++ {
		tag := obj.GetTagAt(i)
		if ((tag.VR == "SQ") && (tag.Length == 0xFFFFFFFF)) || ((tag.Group == 0xFFFE) && (tag.Element == 0xE000) && (tag.Length == 0xFFFFFFFF)) {
			sq++
		}
		if sq == 0 {
			if (tag.Group == 0x0028) && (!icon) {
				switch tag.Element {
				case 0x04:
					PhotoInt = tag.GetString()
					if !strings.Contains(PhotoInt, "MONO") {
						RGB = true
					}
				case 0x06:
					planar = tag.GetUint16()
				case 0x08:
					uframes, err := strconv.Atoi(tag.GetString())
					if err != nil {
						frames = 0
					} else {
						frames = uint32(uframes)
					}
				case 0x10:
					rows = tag.GetUint16()
				case 0x11:
					cols = tag.GetUint16()
				case 0x0100:
					bitsa = tag.GetUint16()
				case 0x0101:
					bitss = tag.GetUint16()
				case 0x0103:
					pixelrep = tag.GetUint16()
				}
			}
			if (tag.Group == 0x0088) && (tag.Element == 0x0200) && (tag.Length == 0xFFFFFFFF) {
				icon = true
			}
			if (tag.Group == 0x6003) && (tag.Element == 0x1010) && (tag.Length == 0xFFFFFFFF) {
				icon = true
			}
			if (tag.Group == 0x7FE0) && (tag.Element == 0x0010) && (!icon) {
				// Lossless JPEG Baseline -> JPEG XL JPEG-Recompression transcodes
				// the original encapsulated JPEG bytes directly (no pixel decode),
				// so the result reconstructs each source JPEG byte-for-byte.
				if obj.GetTransferSyntax().UID == transfersyntax.JPEGBaseline8Bit.UID &&
					outTS.UID == transfersyntax.JPEGXLJPEGRecompression.UID &&
					tag.Length == 0xFFFFFFFF {
					if frames == 0 {
						frames = 1
					}
					if err := recompressJPEGToJXL(obj, &i, frames); err != nil {
						return err
					}
					flag = true
					continue
				}
				sizePx := uint64(cols) * uint64(rows) * uint64(bitsa) / 8
				if RGB {
					sizePx = 3 * sizePx
				}
				var size uint32
				if frames > 0 {
					sizePx *= uint64(frames)
				} else {
					frames = 1
				}
				if sizePx == 0 || sizePx > uint64(maxPixelDataBytes) {
					return fmt.Errorf("DICOMObject::ConvertTransferSyntax, invalid pixel data size %d", sizePx)
				}
				size = uint32(sizePx)
				var img []byte
				switch {
				case tag.Length == 0xFFFFFFFF:
					img = make([]byte, size)
					if err := uncompress(ctx, obj, i, img, size, frames, bitsa, PhotoInt); err != nil {
						return fmt.Errorf("DICOMObject::ConvertTransferSyntax, decompress failed: %w", err)
					}
				case RGB && planar == 1: // change from planar=1 to planar=0
					img = make([]byte, size)
					deplanarizeRGBFrames(img, tag.Data, size/frames, frames)
					planar = 0
				case len(tag.Data) >= int(size):
					// Already uncompressed and interleaved. compress only reads img,
					// and the encapsulation helpers reassign tag.Data rather than
					// writing through it, so the tag's own buffer can be used
					// directly — saving a full pixel-data allocation and copy
					// (hundreds of MB on a large multi-frame object).
					img = tag.Data[:size]
				default:
					// Source is shorter than the declared pixel size; allocate so the
					// tail stays zero-padded, matching the previous behavior.
					img = make([]byte, size)
					copy(img, tag.Data)
				}
				if err := compress(ctx, obj, &i, img, RGB, cols, rows, bitss, bitsa, pixelrep, planar, frames, outTS.UID); err != nil {
					return err
				}
				flag = true
			}
		}
		if ((tag.Group == 0xFFFE) && (tag.Element == 0xE00D)) || ((tag.Group == 0xFFFE) && (tag.Element == 0xE0DD)) {
			sq--
		}
	}
	if flag {
		obj.SetTransferSyntax(outTS)
		return nil
	}
	// No pixel data element found. For non-image SOP classes and any object
	// without pixel data, the tag encoding (explicit vs implicit VR, endianness)
	// is the only thing that differs between transfer syntaxes. When both the
	// source and target TS use the same tag encoding, we can just update the TS
	// header without re-encoding any bytes — the resulting PDV will be identical.
	srcExplicit, srcBig := tsTagEncoding(obj.GetTransferSyntax().UID)
	dstExplicit, dstBig := tsTagEncoding(outTS.UID)
	if srcExplicit == dstExplicit && srcBig == dstBig {
		obj.SetTransferSyntax(outTS)
		return nil
	}
	return fmt.Errorf("pixel data (7FE0,0010) not found: cannot convert from %s to %s", obj.GetTransferSyntax().UID, outTS.UID)
}

// tsTagEncoding returns the tag-level wire encoding properties for the given
// transfer syntax UID. All DICOM transfer syntaxes except the two below use
// explicit VR little-endian encoding for non-pixel-data elements.
func tsTagEncoding(uid string) (explicitVR, bigEndian bool) {
	switch uid {
	case "1.2.840.10008.1.2": // Implicit VR Little Endian
		return false, false
	case "1.2.840.10008.1.2.2": // Explicit VR Big Endian (Retired)
		return true, true
	default:
		return true, false
	}
}

func beginEncapsulatedPixelData(obj media.DICOMObject, index int) int {
	tag := obj.GetTagAt(index)
	tag.VR = "OB"
	tag.Length = 0xFFFFFFFF
	tag.Data = nil
	obj.SetTag(index, tag)

	index++
	obj.InsertTag(index, &media.DICOMTag{
		Group:     0xFFFE,
		Element:   0xE000,
		Length:    0,
		VR:        "DL",
		Data:      nil,
		BigEndian: obj.IsBigEndian(),
	})
	return index
}

func appendEncapsulatedFrame(obj media.DICOMObject, index int, payload []byte) int {
	index++
	obj.InsertTag(index, &media.DICOMTag{
		Group:     0xFFFE,
		Element:   0xE000,
		Length:    uint32(len(payload)),
		VR:        "DL",
		Data:      payload,
		BigEndian: obj.IsBigEndian(),
	})
	return index
}

func endEncapsulatedPixelData(obj media.DICOMObject, index int) int {
	index++
	obj.InsertTag(index, &media.DICOMTag{
		Group:     0xFFFE,
		Element:   0xE0DD,
		Length:    0,
		VR:        "DL",
		Data:      nil,
		BigEndian: obj.IsBigEndian(),
	})
	return index
}

// runFrameJobs runs work for each frame index in [0, frames) across a bounded
// pool of goroutines, returning the first error encountered (annotated with the
// frame number). Frames in a multi-frame image are independent, so decoding or
// encoding them concurrently fills the CPU far better than the old one-at-a-time
// loop.
//
// The context handed to work carries a per-frame intra-frame thread count
// (codecs.WithCodecThreads) chosen so workers*threadsPerFrame stays near
// GOMAXPROCS — this gives inter-frame parallelism without oversubscribing the
// CPU, and lets the native codecs skip per-call thread-pool setup when a single
// thread per frame is enough. A single-frame image runs inline with the
// original context so it still uses all cores for that one frame.
//
// work must be safe to call concurrently for distinct j and must not touch
// shared transcoder state (e.g. the DICOMObject); callers apply ordered results
// after this returns.
func runFrameJobs(ctx context.Context, frames uint32, work func(ctx context.Context, j uint32) error) error {
	switch frames {
	case 0:
		return nil
	case 1:
		return work(ctx, 0)
	}

	cores := runtime.GOMAXPROCS(0)
	workers := int(frames)
	if workers > cores {
		workers = cores
	}
	if workers < 1 {
		workers = 1
	}
	threadsPerFrame := cores / workers
	if threadsPerFrame < 1 {
		threadsPerFrame = 1
	}
	fctx := codecs.WithCodecThreads(ctx, threadsPerFrame)

	jobs := make(chan uint32)
	var (
		mu         sync.Mutex
		firstErr   error
		firstFrame uint32
		wg         sync.WaitGroup
	)
	failed := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return firstErr != nil
	}
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				if failed() {
					continue // drain remaining jobs cheaply once one has failed
				}
				if err := work(fctx, j); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr, firstFrame = err, j
					}
					mu.Unlock()
				}
			}
		}()
	}
	for j := uint32(0); j < frames; j++ {
		jobs <- j
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return fmt.Errorf("transcoder: frame %d: %w", firstFrame, firstErr)
	}
	return nil
}

// encodeFramesConcurrent encodes every frame in parallel via runFrameJobs and
// returns the encoded payloads in frame order. encodeOne must return a fresh
// slice for frame j and must not touch shared transcoder state.
func encodeFramesConcurrent(ctx context.Context, frames uint32, encodeOne func(ctx context.Context, j uint32) ([]byte, error)) ([][]byte, error) {
	results := make([][]byte, frames)
	err := runFrameJobs(ctx, frames, func(fctx context.Context, j uint32) error {
		data, e := encodeOne(fctx, j)
		if e != nil {
			return e
		}
		results[j] = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// appendEncapsulatedFrames writes a complete encapsulated pixel-data element
// (offset table, one fragment per frame in order, sequence delimiter) and
// returns the new tag index.
func appendEncapsulatedFrames(obj media.DICOMObject, index int, frames [][]byte) int {
	index = beginEncapsulatedPixelData(obj, index)
	for _, payload := range frames {
		index = appendEncapsulatedFrame(obj, index, payload)
	}
	return endEncapsulatedPixelData(obj, index)
}

// frameBounds returns the sample count and the [start,end) byte range of frame
// j within a packed pixel buffer of the given geometry.
func frameBounds(j uint32, cols uint16, rows uint16, bitsa uint16, RGB bool) (samples uint16, start uint32, end uint32) {
	frameLen := uint32(cols) * uint32(rows) * uint32(bitsa) / 8
	samples = 1
	if RGB {
		samples = 3
		frameLen *= 3
	}
	start = j * frameLen
	end = start + frameLen
	return samples, start, end
}

// encodeJ2KFrame encodes frame j of img as JPEG 2000 / HTJ2K. ratio is the
// OpenJPEG compression ratio (0 for lossless).
func encodeJ2KFrame(ctx context.Context, img []byte, j uint32, cols uint16, rows uint16, bitsa uint16, RGB bool, ratio int) ([]byte, error) {
	samples, start, end := frameBounds(j, cols, rows, bitsa, RGB)
	var data []byte
	var n int
	if err := jpeg2000.J2KencodeContext(ctx, img[start:end], cols, rows, samples, bitsa, &data, &n, ratio); err != nil {
		return nil, err
	}
	return data, nil
}

// encodeHTJ2KFrame encodes frame j of img as a High-Throughput JPEG 2000
// (HTJ2K) codestream (reversible 5/3, HT code-blocks), as required by the HTJ2K
// transfer syntaxes — the plain JPEG 2000 encoder would emit non-HT code-blocks
// under an HTJ2K UID. The pure-Go HTJ2K encoder is lossless only.
func encodeHTJ2KFrame(ctx context.Context, img []byte, j uint32, cols uint16, rows uint16, bitsa uint16, RGB bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	samples, start, end := frameBounds(j, cols, rows, bitsa, RGB)
	return jpeg2000.HTJ2Kencode(img[start:end], int(cols), int(rows), int(samples), int(bitsa))
}

// encodeJXLFrame encodes frame j of img as JPEG XL.
func encodeJXLFrame(ctx context.Context, img []byte, j uint32, cols uint16, rows uint16, bitsa uint16, RGB bool, lossless bool) ([]byte, error) {
	samples, start, end := frameBounds(j, cols, rows, bitsa, RGB)
	var data []byte
	var n int
	if err := jpegxl.JXLencodeContext(ctx, img[start:end], cols, rows, samples, bitsa, &data, &n, lossless); err != nil {
		return nil, err
	}
	return data, nil
}

func compress(ctx context.Context, obj media.DICOMObject, i *int, img []byte, RGB bool, cols uint16, rows uint16, bitss uint16, bitsa uint16, pixelrep uint16, planar uint16, frames uint32, outTS string) error {
	var offset, size, j uint32
	var JPEGData []byte
	var JPEGBytes int

	single := uint32(cols) * uint32(rows) * uint32(bitsa) / 8
	frameSize := single
	size = frameSize * frames
	if RGB {
		frameSize = 3 * frameSize
		size = frameSize * frames
	}

	index := *i
	tag := obj.GetTagAt(index)

	switch outTS {
	case transfersyntax.DeflatedImageFrameCompression.UID:
		index = beginEncapsulatedPixelData(obj, index)

		for j = 0; j < frames; j++ {
			offset = j * frameSize
			deflated, err := deflate.DeflateFrame(img[offset : offset+frameSize])
			if err != nil {
				return err
			}
			index = appendEncapsulatedFrame(obj, index, deflated)
		}
		index = endEncapsulatedPixelData(obj, index)
		*i = index
	case transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian.UID:
		index = beginEncapsulatedPixelData(obj, index)

		for j = 0; j < frames; j++ {
			offset = j * frameSize
			frameData := make([]byte, frameSize)
			copy(frameData, img[offset:offset+frameSize])
			index = appendEncapsulatedFrame(obj, index, frameData)
		}
		index = endEncapsulatedPixelData(obj, index)
		*i = index
	case transfersyntax.RLELossless.UID:
		index = beginEncapsulatedPixelData(obj, index)

		samplesPerPixel := uint16(1)
		if RGB {
			samplesPerPixel = 3
		}

		for j = 0; j < frames; j++ {
			offset = j * frameSize
			rleData, err := rle.RLEencode(img[offset:offset+frameSize], rows, cols, bitsa, samplesPerPixel)
			if err != nil {
				return err
			}
			index = appendEncapsulatedFrame(obj, index, rleData)
		}
		index = endEncapsulatedPixelData(obj, index)
		*i = index
	case transfersyntax.JPEGLosslessSV1.UID:
		fallthrough
	case transfersyntax.JPEGLossless.UID:
		index = beginEncapsulatedPixelData(obj, index)
		for j = 0; j < frames; j++ {
			// frameBounds gives this frame's [start,end). The previous code
			// built a byte offset and then handed the 16-bit encoder
			// img[offset/2:] — a sample index, and open-ended besides.
			// encodeLosslessJPEG checks width*height*samples*bps == len(raw)
			// exactly, so every multi-frame 16-bit object was rejected outright.
			samples, start, end := frameBounds(j, cols, rows, bitsa, RGB)
			if bitsa == 8 {
				if err := jpeg.EIJG8encode(img[start:end], cols, rows, samples, &JPEGData, &JPEGBytes, 4); err != nil {
					return err
				}
			} else {
				if err := jpeg.EIJG16encodeContext(ctx, img[start:end], cols, rows, samples, &JPEGData, &JPEGBytes, 0); err != nil {
					return err
				}
			}
			index = appendEncapsulatedFrame(obj, index, JPEGData)
			JPEGData = nil
		}
		index = endEncapsulatedPixelData(obj, index)
		*i = index
	case transfersyntax.JPEGBaseline8Bit.UID:
		index = beginEncapsulatedPixelData(obj, index)
		for j = 0; j < frames; j++ {
			samples, start, end := frameBounds(j, cols, rows, bitsa, RGB)
			if bitsa == 8 {
				if err := jpeg.EIJG8encode(img[start:end], cols, rows, samples, &JPEGData, &JPEGBytes, 0); err != nil {
					return err
				}
			} else {
				if err := jpeg.EIJG12encodeContext(ctx, img[start:end], cols, rows, samples, &JPEGData, &JPEGBytes, 0); err != nil {
					return err
				}
			}
			index = appendEncapsulatedFrame(obj, index, JPEGData)
			JPEGData = nil
		}
		index = endEncapsulatedPixelData(obj, index)
		*i = index
	case transfersyntax.JPEGExtended12Bit.UID:
		index = beginEncapsulatedPixelData(obj, index)
		for j = 0; j < frames; j++ {
			samples, start, end := frameBounds(j, cols, rows, bitsa, RGB)
			if err := jpeg.EIJG12encodeContext(ctx, img[start:end], cols, rows, samples, &JPEGData, &JPEGBytes, 0); err != nil {
				return err
			}
			index = appendEncapsulatedFrame(obj, index, JPEGData)
			JPEGData = nil
		}
		index = endEncapsulatedPixelData(obj, index)
		*i = index
	case transfersyntax.JPEGLSLossless.UID:
		fallthrough
	case transfersyntax.JPEGLSNearLossless.UID:
		// Encode every frame before mutating the object, and bound each frame to
		// its own [start,end). Passing img[start:] handed the encoder a slice
		// running to the end of the whole multi-frame buffer, which its
		// exact-size check rejected — so JPEG-LS encoding failed for every
		// object with more than one frame.
		jlsFrames := make([][]byte, 0, frames)
		nearLossless := outTS == transfersyntax.JPEGLSNearLossless.UID
		for j = 0; j < frames; j++ {
			samples, start, end := frameBounds(j, cols, rows, bitsa, RGB)
			if err := jpegls.JLSencodeContext(ctx, img[start:end], cols, rows, samples, bitsa,
				&JPEGData, &JPEGBytes, nearLossless); err != nil {
				return err
			}
			jlsFrames = append(jlsFrames, JPEGData)
			JPEGData = nil
		}
		index = appendEncapsulatedFrames(obj, index, jlsFrames)
		*i = index
	case transfersyntax.JPEG2000Lossless.UID:
		fallthrough
	case transfersyntax.JPEG2000MCLossless.UID:
		frameData, err := encodeFramesConcurrent(ctx, frames, func(fctx context.Context, j uint32) ([]byte, error) {
			return encodeJ2KFrame(fctx, img, j, cols, rows, bitsa, RGB, 0)
		})
		if err != nil {
			return err
		}
		index = appendEncapsulatedFrames(obj, index, frameData)
		*i = index
	case transfersyntax.JPEG2000.UID:
		fallthrough
	case transfersyntax.JPEG2000MC.UID:
		// The .91 syntaxes accept a lossy or reversible codestream. ratio 10 lets a
		// native backend produce a lossy codestream; the pure-Go encoder ignores
		// the ratio and emits a reversible 5/3 codestream, which is also conformant.
		frameData, err := encodeFramesConcurrent(ctx, frames, func(fctx context.Context, j uint32) ([]byte, error) {
			return encodeJ2KFrame(fctx, img, j, cols, rows, bitsa, RGB, 10)
		})
		if err != nil {
			return err
		}
		index = appendEncapsulatedFrames(obj, index, frameData)
		*i = index
	case transfersyntax.HTJ2KLossless.UID:
		fallthrough
	case transfersyntax.HTJ2KLosslessRPCL.UID:
		fallthrough
	case transfersyntax.HTJ2K.UID:
		// HTJ2K syntaxes require HT code-blocks; use the dedicated HTJ2K encoder
		// (reversible 5/3) rather than the plain JPEG 2000 codestream.
		frameData, err := encodeFramesConcurrent(ctx, frames, func(fctx context.Context, j uint32) ([]byte, error) {
			return encodeHTJ2KFrame(fctx, img, j, cols, rows, bitsa, RGB)
		})
		if err != nil {
			return err
		}
		index = appendEncapsulatedFrames(obj, index, frameData)
		*i = index
	case transfersyntax.JPEGXLLossless.UID:
		fallthrough
	case transfersyntax.JPEGXLJPEGRecompression.UID:
		fallthrough
	case transfersyntax.JPEGXL.UID:
		lossless := outTS == transfersyntax.JPEGXLLossless.UID
		frameData, err := encodeFramesConcurrent(ctx, frames, func(fctx context.Context, j uint32) ([]byte, error) {
			return encodeJXLFrame(fctx, img, j, cols, rows, bitsa, RGB, lossless)
		})
		if err != nil {
			return err
		}
		index = appendEncapsulatedFrames(obj, index, frameData)
		*i = index
	case transfersyntax.JPIPHTJ2KReferenced.UID:
		fallthrough
	case transfersyntax.JPIPHTJ2KReferencedDeflate.UID:
		// Encode every frame before mutating the object. The encapsulation
		// helpers rewrite the pixel-data tag in place, so returning an error
		// partway through the loop used to leave the object converted to
		// encapsulated form with no sequence delimiter — neither the original
		// nor a valid result.
		jpipFrames := make([][]byte, 0, frames)
		for j = 0; j < frames; j++ {
			// frameBounds gives a proper [start,end) for this frame. Passing
			// img[start:] instead handed the encoder a slice running to the end
			// of the whole multi-frame buffer; the exact-size check in
			// jpip.purego_backend then rejected it, so JPIP encoding failed for
			// every object with more than one frame.
			samples, start, end := frameBounds(j, cols, rows, bitsa, RGB)
			if err := jpip.JPIPencodeContext(ctx, img[start:end], cols, rows, samples, bitsa,
				&JPEGData, &JPEGBytes, outTS); err != nil {
				return err
			}
			jpipFrames = append(jpipFrames, JPEGData)
			JPEGData = nil
		}
		// appendEncapsulatedFrames wraps the begin/end delimiters itself.
		index = appendEncapsulatedFrames(obj, index, jpipFrames)
		*i = index
	case transfersyntax.MPEG2MPML.UID:
		fallthrough
	case transfersyntax.MPEG2MPMLF.UID:
		fallthrough
	case transfersyntax.MPEG2MPHL.UID:
		fallthrough
	case transfersyntax.MPEG2MPHLF.UID:
		fallthrough
	case transfersyntax.MPEG4HP41.UID:
		fallthrough
	case transfersyntax.MPEG4HP41F.UID:
		fallthrough
	case transfersyntax.MPEG4HP41BD.UID:
		fallthrough
	case transfersyntax.MPEG4HP41BDF.UID:
		fallthrough
	case transfersyntax.MPEG4HP422D.UID:
		fallthrough
	case transfersyntax.MPEG4HP422DF.UID:
		fallthrough
	case transfersyntax.MPEG4HP423D.UID:
		fallthrough
	case transfersyntax.MPEG4HP423DF.UID:
		fallthrough
	case transfersyntax.MPEG4HP42STEREO.UID:
		fallthrough
	case transfersyntax.MPEG4HP42STEREOF.UID:
		fallthrough
	case transfersyntax.HEVCMP51.UID:
		fallthrough
	case transfersyntax.HEVCM10P51.UID:
		index = beginEncapsulatedPixelData(obj, index)
		for j = 0; j < frames; j++ {
			// frameBounds gives this frame's [start,end). The previous code
			// sliced img[offset:] open-ended to the end of the whole multi-frame
			// buffer, so every frame but the last was handed too many bytes. The
			// ffmpeg backend rejects that against an exact-size check; the
			// passthrough backend silently copies the surplus frames in.
			samples, start, end := frameBounds(j, cols, rows, bitsa, RGB)
			if err := mpeg.MPEGencodeContext(ctx, img[start:end], cols, rows, samples, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
				return err
			}
			index = appendEncapsulatedFrame(obj, index, JPEGData)
			JPEGData = nil
		}
		index = endEncapsulatedPixelData(obj, index)
		*i = index
	case transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo.UID:
		fallthrough
	case transfersyntax.SMPTEST211020UncompressedInterlacedActiveVideo.UID:
		fallthrough
	case transfersyntax.SMPTEST211030PCMDigitalAudio.UID:
		index = beginEncapsulatedPixelData(obj, index)
		for j = 0; j < frames; j++ {
			samples, start, end := frameBounds(j, cols, rows, bitsa, RGB)
			if err := smpte2110.SMPTE2110encodeContext(ctx, img[start:end], cols, rows, samples, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
				return err
			}
			index = appendEncapsulatedFrame(obj, index, JPEGData)
			JPEGData = nil
		}
		index = endEncapsulatedPixelData(obj, index)
		*i = index
	default:
		if bitss == 8 {
			tag.VR = "OB"
		} else {
			tag.VR = "OW"
		}
		tag.Length = size
		if tag.Data != nil {
			tag.Data = nil
		}
		tag.Data = make([]byte, tag.Length)
		copy(tag.Data, img)
		obj.SetTag(index, tag)
	}
	return nil
}

func uncompress(ctx context.Context, obj media.DICOMObject, i int, img []byte, size uint32, frames uint32, bitsa uint16, PhotoInt string) error {
	single := size / frames
	tsUID := obj.GetTransferSyntax().UID

	// Collect the compressed payload for every frame first (cheap, and the tag
	// references stay valid after deletion), then decode them concurrently into
	// disjoint regions of img. Decoding is the expensive part and each frame is
	// independent, so this is where the parallelism pays off; the object is only
	// mutated here on the single goroutine.
	obj.DelTag(i + 1) // Delete offset table.
	frameData := make([][]byte, frames)
	for j := uint32(0); j < frames; j++ {
		tag := obj.GetTagAt(i + 1)
		if tag == nil {
			return fmt.Errorf("transcoder: missing frame %d for transfer syntax %s", j, tsUID)
		}
		frameData[j] = tag.Data
		obj.DelTag(i + 1)
	}
	obj.DelTag(i + 1) // Delete sequence delimiter.

	return runFrameJobs(ctx, frames, func(fctx context.Context, j uint32) error {
		offset := j * single
		return codecs.DecompressFrame(fctx, tsUID, frameData[j], bitsa, PhotoInt, img[offset:offset+single])
	})
}

// recompressJPEGToJXL losslessly transcodes the encapsulated baseline-JPEG
// frames at the pixel-data tag (*i) into JPEG XL JPEG-Recompression frames, in
// place. It transcodes the original JPEG bytes directly — no pixel decode — so
// each reconstructed JPEG is byte-identical to the source. All frames are
// transcoded before the object is mutated, so a failure (e.g. a progressive or
// 12-bit JPEG the encoder does not support) leaves the object untouched.
func recompressJPEGToJXL(obj media.DICOMObject, i *int, frames uint32) error {
	index := *i

	// Validate the full encapsulation layout — basic offset table item at
	// index+1, one fragment per frame, then the sequence delimiter — before
	// mutating anything, so a malformed object errors out untouched rather than
	// having the wrong tags deleted.
	isItem := func(at int, group, element uint16) bool {
		tag := obj.GetTagAt(at)
		return tag != nil && tag.Group == group && tag.Element == element
	}
	if !isItem(index+1, 0xFFFE, 0xE000) {
		return fmt.Errorf("transcoder: missing basic offset table for JPEG XL recompression")
	}
	if !isItem(index+2+int(frames), 0xFFFE, 0xE0DD) {
		return fmt.Errorf("transcoder: missing sequence delimiter for JPEG XL recompression")
	}

	// Read each frame's JPEG payload (one fragment per frame after the offset
	// table) without mutating the object yet.
	src := make([][]byte, frames)
	for j := uint32(0); j < frames; j++ {
		tag := obj.GetTagAt(index + 2 + int(j))
		if tag == nil || tag.Group != 0xFFFE || tag.Element != 0xE000 {
			return fmt.Errorf("transcoder: missing JPEG frame %d for JPEG XL recompression", j)
		}
		src[j] = tag.Data
	}

	out := make([][]byte, frames)
	for j := uint32(0); j < frames; j++ {
		jxl, err := jpegxl.EncodeJPEGRecompression(src[j])
		if err != nil {
			return fmt.Errorf("transcoder: JPEG->JPEG XL recompression of frame %d: %w", j, err)
		}
		out[j] = jxl
	}

	// Replace the encapsulated data: drop the offset table, the source frames and
	// the sequence delimiter, then write the new frames.
	obj.DelTag(index + 1) // offset table
	for j := uint32(0); j < frames; j++ {
		obj.DelTag(index + 1)
	}
	obj.DelTag(index + 1) // sequence delimiter
	*i = appendEncapsulatedFrames(obj, index, out)
	return nil
}

// deplanarizeRGBFrames converts RGB pixel data from planar format (all R
// values followed by all G followed by all B, per frame) to interleaved RGB
// in dst. src and dst may overlap or be the same slice. frameSize is the byte
// count of one frame (rows * cols * 3).
func deplanarizeRGBFrames(dst, src []byte, frameSize, frames uint32) {
	for f := uint32(0); f < frames; f++ {
		off := frameSize * f
		pixels := frameSize / 3
		for j := uint32(0); j < pixels; j++ {
			dst[3*j+off] = src[j+off]
			dst[3*j+1+off] = src[j+pixels+off]
			dst[3*j+2+off] = src[j+2*pixels+off]
		}
	}
}
