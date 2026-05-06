package media

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

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
)

func (obj *dicomObject) ChangeTransferSyntax(outTS *transfersyntax.TransferSyntax) error {
	return obj.ChangeTransferSyntaxContext(context.Background(), outTS)
}

func (obj *dicomObject) ChangeTransferSyntaxContext(ctx context.Context, outTS *transfersyntax.TransferSyntax) error {
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

	if obj.TransferSyntax.UID == outTS.UID {
		return nil
	}

	if !transfersyntax.SupportedTransferSyntax(outTS.UID) {
		return fmt.Errorf("unsupported transfer syntax %s", outTS.Name)
	}

	for i = 0; i < len(obj.Tags); i++ {
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
				img := make([]byte, size)
				if tag.Length == 0xFFFFFFFF {
					if err := obj.uncompress(ctx, i, img, size, frames, bitsa, PhotoInt); err != nil {
						return fmt.Errorf("DICOMObject::ConvertTransferSyntax, decompress failed: %w", err)
					}
				} else { // Uncompressed
					if RGB && (planar == 1) { // change from planar=1 to planar=0
						deplanarizeRGBFrames(img, tag.Data, size/frames, frames)
						planar = 0
					} else {
						copy(img, tag.Data)
					}
				}
				if err := obj.compress(ctx, &i, img, RGB, cols, rows, bitss, bitsa, pixelrep, planar, frames, outTS.UID); err != nil {
					return err
				} else {
					flag = true
				}
			}
		}
		if ((tag.Group == 0xFFFE) && (tag.Element == 0xE00D)) || ((tag.Group == 0xFFFE) && (tag.Element == 0xE0DD)) {
			sq--
		}
	}
	if flag {
		obj.TransferSyntax = outTS
		return nil
	}
	return fmt.Errorf("pixel data (7FE0,0010) not found: cannot convert from %s to %s", obj.TransferSyntax.UID, outTS.UID)
}

func (obj *dicomObject) beginEncapsulatedPixelData(index int) int {
	tag := obj.GetTagAt(index)
	tag.VR = "OB"
	tag.Length = 0xFFFFFFFF
	tag.Data = nil
	obj.SetTag(index, tag)

	index++
	obj.InsertTag(index, &DICOMTag{
		Group:     0xFFFE,
		Element:   0xE000,
		Length:    0,
		VR:        "DL",
		Data:      nil,
		BigEndian: obj.IsBigEndian(),
	})
	return index
}

func (obj *dicomObject) appendEncapsulatedFrame(index int, payload []byte) int {
	index++
	obj.InsertTag(index, &DICOMTag{
		Group:     0xFFFE,
		Element:   0xE000,
		Length:    uint32(len(payload)),
		VR:        "DL",
		Data:      payload,
		BigEndian: obj.IsBigEndian(),
	})
	return index
}

func (obj *dicomObject) endEncapsulatedPixelData(index int) int {
	index++
	obj.InsertTag(index, &DICOMTag{
		Group:     0xFFFE,
		Element:   0xE0DD,
		Length:    0,
		VR:        "DL",
		Data:      nil,
		BigEndian: obj.IsBigEndian(),
	})
	return index
}

func (obj *dicomObject) compress(ctx context.Context, i *int, img []byte, RGB bool, cols uint16, rows uint16, bitss uint16, bitsa uint16, pixelrep uint16, planar uint16, frames uint32, outTS string) error {
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
		index = obj.beginEncapsulatedPixelData(index)

		for j = 0; j < frames; j++ {
			offset = j * frameSize
			deflated, err := deflate.DeflateFrame(img[offset : offset+frameSize])
			if err != nil {
				return err
			}
			index = obj.appendEncapsulatedFrame(index, deflated)
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian.UID:
		index = obj.beginEncapsulatedPixelData(index)

		for j = 0; j < frames; j++ {
			offset = j * frameSize
			frameData := make([]byte, frameSize)
			copy(frameData, img[offset:offset+frameSize])
			index = obj.appendEncapsulatedFrame(index, frameData)
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.RLELossless.UID:
		index = obj.beginEncapsulatedPixelData(index)

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
			index = obj.appendEncapsulatedFrame(index, rleData)
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.JPEGLosslessSV1.UID:
		fallthrough
	case transfersyntax.JPEGLossless.UID:
		index = obj.beginEncapsulatedPixelData(index)
		for j = 0; j < frames; j++ {
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
			}
			if bitsa == 8 {
				if RGB {
					if err := jpeg.EIJG8encode(img[offset:], cols, rows, 3, &JPEGData, &JPEGBytes, 4); err != nil {
						return err
					}
				} else {
					if err := jpeg.EIJG8encode(img[offset:], cols, rows, 1, &JPEGData, &JPEGBytes, 4); err != nil {
						return err
					}
				}
			} else {
				if err := jpeg.EIJG16encodeContext(ctx, img[offset/2:], cols, rows, 1, &JPEGData, &JPEGBytes, 0); err != nil {
					return err
				}
			}
			index = obj.appendEncapsulatedFrame(index, JPEGData)
			JPEGData = nil
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.JPEGBaseline8Bit.UID:
		index = obj.beginEncapsulatedPixelData(index)
		for j = 0; j < frames; j++ {
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpeg.EIJG8encode(img[offset:], cols, rows, 3, &JPEGData, &JPEGBytes, 0); err != nil {
					return err
				}
			} else {
				if bitsa == 8 {
					if err := jpeg.EIJG8encode(img[offset:], cols, rows, 1, &JPEGData, &JPEGBytes, 0); err != nil {
						return err
					}
				} else {
					if err := jpeg.EIJG12encodeContext(ctx, img[offset:], cols, rows, 1, &JPEGData, &JPEGBytes, 0); err != nil {
						return err
					}
				}
			}
			index = obj.appendEncapsulatedFrame(index, JPEGData)
			JPEGData = nil
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.JPEGExtended12Bit.UID:
		index = obj.beginEncapsulatedPixelData(index)
		for j = 0; j < frames; j++ {
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if err := jpeg.EIJG12encodeContext(ctx, img[offset/2:], cols, rows, 1, &JPEGData, &JPEGBytes, 0); err != nil {
				return err
			}
			index = obj.appendEncapsulatedFrame(index, JPEGData)
			JPEGData = nil
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.JPEGLSLossless.UID:
		fallthrough
	case transfersyntax.JPEGLSNearLossless.UID:
		index = obj.beginEncapsulatedPixelData(index)
		for j = 0; j < frames; j++ {
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpegls.JLSencodeContext(ctx, img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, outTS == transfersyntax.JPEGLSNearLossless.UID); err != nil {
					return err
				}
			} else {
				if err := jpegls.JLSencodeContext(ctx, img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, outTS == transfersyntax.JPEGLSNearLossless.UID); err != nil {
					return err
				}
			}
			index = obj.appendEncapsulatedFrame(index, JPEGData)
			JPEGData = nil
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.JPEG2000Lossless.UID:
		fallthrough
	case transfersyntax.JPEG2000MCLossless.UID:
		fallthrough
	case transfersyntax.HTJ2KLossless.UID:
		fallthrough
	case transfersyntax.HTJ2KLosslessRPCL.UID:
		index = obj.beginEncapsulatedPixelData(index)
		for j = 0; j < frames; j++ {
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpeg2000.J2KencodeContext(ctx, img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, 0); err != nil {
					return err
				}
			} else {
				if err := jpeg2000.J2KencodeContext(ctx, img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, 0); err != nil {
					return err
				}
			}
			index = obj.appendEncapsulatedFrame(index, JPEGData)
			JPEGData = nil
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.JPEG2000.UID:
		fallthrough
	case transfersyntax.JPEG2000MC.UID:
		fallthrough
	case transfersyntax.HTJ2K.UID:
		index = obj.beginEncapsulatedPixelData(index)
		for j = 0; j < frames; j++ {
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpeg2000.J2KencodeContext(ctx, img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, 10); err != nil {
					return err
				}
			} else {
				if err := jpeg2000.J2KencodeContext(ctx, img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, 10); err != nil {
					return err
				}
			}
			index = obj.appendEncapsulatedFrame(index, JPEGData)
			JPEGData = nil
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.JPEGXLLossless.UID:
		fallthrough
	case transfersyntax.JPEGXLJPEGRecompression.UID:
		fallthrough
	case transfersyntax.JPEGXL.UID:
		index = obj.beginEncapsulatedPixelData(index)
		for j = 0; j < frames; j++ {
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpegxl.JXLencodeContext(ctx, img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, outTS == transfersyntax.JPEGXLLossless.UID); err != nil {
					return err
				}
			} else {
				if err := jpegxl.JXLencodeContext(ctx, img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, outTS == transfersyntax.JPEGXLLossless.UID); err != nil {
					return err
				}
			}
			index = obj.appendEncapsulatedFrame(index, JPEGData)
			JPEGData = nil
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.JPIPHTJ2KReferenced.UID:
		fallthrough
	case transfersyntax.JPIPHTJ2KReferencedDeflate.UID:
		index = obj.beginEncapsulatedPixelData(index)
		for j = 0; j < frames; j++ {
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpip.JPIPencodeContext(ctx, img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			} else {
				if err := jpip.JPIPencodeContext(ctx, img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			}
			index = obj.appendEncapsulatedFrame(index, JPEGData)
			JPEGData = nil
		}
		index = obj.endEncapsulatedPixelData(index)
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
		index = obj.beginEncapsulatedPixelData(index)
		for j = 0; j < frames; j++ {
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := mpeg.MPEGencodeContext(ctx, img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			} else {
				if err := mpeg.MPEGencodeContext(ctx, img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			}
			index = obj.appendEncapsulatedFrame(index, JPEGData)
			JPEGData = nil
		}
		index = obj.endEncapsulatedPixelData(index)
		*i = index
	case transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo.UID:
		fallthrough
	case transfersyntax.SMPTEST211020UncompressedInterlacedActiveVideo.UID:
		fallthrough
	case transfersyntax.SMPTEST211030PCMDigitalAudio.UID:
		index = obj.beginEncapsulatedPixelData(index)
		for j = 0; j < frames; j++ {
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := smpte2110.SMPTE2110encodeContext(ctx, img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			} else {
				if err := smpte2110.SMPTE2110encodeContext(ctx, img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			}
			index = obj.appendEncapsulatedFrame(index, JPEGData)
			JPEGData = nil
		}
		index = obj.endEncapsulatedPixelData(index)
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

func (obj *dicomObject) uncompress(ctx context.Context, i int, img []byte, size uint32, frames uint32, bitsa uint16, PhotoInt string) error {
	var j, offset, single uint32
	single = size / frames

	obj.DelTag(i + 1) // Delete offset table.
	switch obj.TransferSyntax.UID {
	case transfersyntax.DeflatedImageFrameCompression.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return errors.New("DICOMObject::ConvertTransferSyntax, invalid deflated frame")
			}
			inflated, err := deflate.InflateFrame(tag.Data, int(single))
			if err != nil {
				return err
			}
			copy(img[offset:offset+single], inflated)
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil || tag.Length < single {
				return errors.New("DICOMObject::ConvertTransferSyntax, invalid encapsulated frame")
			}
			copy(img[offset:offset+single], tag.Data[:single])
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.RLELossless.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing RLE frame %d", j)
			}
			if err := rle.RLEdecode(tag.Data, img[offset:], tag.Length, single, PhotoInt); err != nil {
				return err
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.JPEGLosslessSV1.UID:
		fallthrough
	case transfersyntax.JPEGLossless.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing JPEG lossless frame %d", j)
			}
			if bitsa == 8 {
				if err := jpeg.DIJG8decodeContext(ctx, tag.Data, tag.Length, img[offset:], single); err != nil {
					return err
				}
			} else {
				if err := jpeg.DIJG16decodeContext(ctx, tag.Data, tag.Length, img[offset:], single); err != nil {
					return err
				}
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.JPEGBaseline8Bit.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing JPEG baseline frame %d", j)
			}
			if bitsa == 8 {
				if err := jpeg.DIJG8decodeContext(ctx, tag.Data, tag.Length, img[offset:], single); err != nil {
					return err
				}
			} else {
				if err := jpeg.DIJG12decodeContext(ctx, tag.Data, tag.Length, img[offset:], single); err != nil {
					return err
				}
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.JPEGExtended12Bit.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing JPEG extended frame %d", j)
			}
			if err := jpeg.DIJG12decodeContext(ctx, tag.Data, tag.Length, img[offset:], single); err != nil {
				return err
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.JPEGLSLossless.UID:
		fallthrough
	case transfersyntax.JPEGLSNearLossless.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing JPEG-LS frame %d", j)
			}
			if err := jpegls.JLSdecodeContext(ctx, tag.Data, tag.Length, img[offset:]); err != nil {
				return err
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.JPEG2000Lossless.UID:
		fallthrough
	case transfersyntax.JPEG2000MCLossless.UID:
		fallthrough
	case transfersyntax.HTJ2KLossless.UID:
		fallthrough
	case transfersyntax.HTJ2KLosslessRPCL.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing JPEG 2000 lossless frame %d", j)
			}
			if err := jpeg2000.J2KdecodeContext(ctx, tag.Data, tag.Length, img[offset:]); err != nil {
				return err
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.JPEG2000.UID:
		fallthrough
	case transfersyntax.JPEG2000MC.UID:
		fallthrough
	case transfersyntax.HTJ2K.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing JPEG 2000 frame %d", j)
			}
			if err := jpeg2000.J2KdecodeContext(ctx, tag.Data, tag.Length, img[offset:]); err != nil {
				return err
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.JPEGXLLossless.UID:
		fallthrough
	case transfersyntax.JPEGXLJPEGRecompression.UID:
		fallthrough
	case transfersyntax.JPEGXL.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing JPEG XL frame %d", j)
			}
			if err := jpegxl.JXLdecodeContext(ctx, tag.Data, tag.Length, img[offset:]); err != nil {
				return err
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.JPIPHTJ2KReferenced.UID:
		fallthrough
	case transfersyntax.JPIPHTJ2KReferencedDeflate.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing JPIP frame %d", j)
			}
			if err := jpip.JPIPdecodeContext(ctx, tag.Data, tag.Length, img[offset:], obj.TransferSyntax.UID); err != nil {
				return err
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
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
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing MPEG frame %d", j)
			}
			if err := mpeg.MPEGdecodeContext(ctx, tag.Data, tag.Length, img[offset:], obj.TransferSyntax.UID); err != nil {
				return err
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	case transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo.UID:
		fallthrough
	case transfersyntax.SMPTEST211020UncompressedInterlacedActiveVideo.UID:
		fallthrough
	case transfersyntax.SMPTEST211030PCMDigitalAudio.UID:
		for j = 0; j < frames; j++ {
			offset = j * single
			tag := obj.GetTagAt(i + 1)
			if tag == nil {
				return fmt.Errorf("DICOMObject::ConvertTransferSyntax, missing SMPTE frame %d", j)
			}
			if err := smpte2110.SMPTE2110decodeContext(ctx, tag.Data, tag.Length, img[offset:], obj.TransferSyntax.UID); err != nil {
				return err
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	}
	return nil
}
