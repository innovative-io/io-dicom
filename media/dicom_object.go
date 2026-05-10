package media

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/innovative-io/io-dicom/codecs/deflate"
	"github.com/innovative-io/io-dicom/codecs/jpeg"
	"github.com/innovative-io/io-dicom/codecs/jpeg2000"
	"github.com/innovative-io/io-dicom/codecs/jpegls"
	"github.com/innovative-io/io-dicom/codecs/jpegxl"
	"github.com/innovative-io/io-dicom/codecs/jpip"
	"github.com/innovative-io/io-dicom/codecs/mpeg"
	"github.com/innovative-io/io-dicom/codecs/rle"
	"github.com/innovative-io/io-dicom/codecs/smpte2110"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

// maxPixelDataBytes is the upper bound for a single pixel data allocation (512 MiB).
// It prevents unbounded allocations from malformed or malicious DICOM input.
const maxPixelDataBytes = 512 * 1024 * 1024

// ErrNoTransferSyntax is returned when no transfer syntax can be determined
// from a DICOM data stream (missing or unrecognised group-0002 metadata).
var ErrNoTransferSyntax = errors.New("media: unable to determine transfer syntax")

// DICOMObject - DICOM Object structure
type DICOMObject interface {
	Add(tag *DICOMTag)
	DumpTags(w io.Writer)
	IsExplicitVR() bool
	SetExplicitVR(explicit bool)
	IsBigEndian() bool
	SetBigEndian(bigEndian bool)
	GetPixelData(frame int) ([]byte, error)
	GetDecompressedFrame(ctx context.Context, frameIndex int) ([]byte, error)
	GetTagAt(i int) *DICOMTag
	GetTag(tag *tags.Tag) *DICOMTag
	SetTag(i int, tag *DICOMTag)
	InsertTag(i int, tag *DICOMTag)
	DelTag(i int)
	GetTags() []*DICOMTag
	GetUint16(tag *tags.Tag) uint16
	GetUint32(tag *tags.Tag) uint32
	GetString(tag *tags.Tag) string
	// Write encodes value and appends it to the object as a tag. The concrete
	// type of value determines encoding:
	//   - string   → DICOM string tag (padded per VR)
	//   - uint16   → 2-byte little- or big-endian field
	//   - uint32   → 4-byte little- or big-endian field
	//   - int      → dispatched to uint16 or uint32 based on tag.VR
	//   - time.Time → formatted string: "20060102" for DA, "150405" for TM
	//   - DateRange → formatted "YYYYMMDD-YYYYMMDD" string for date range queries
	Write(tag *tags.Tag, value any)
	GetTransferSyntax() *transfersyntax.TransferSyntax
	SetTransferSyntax(ts *transfersyntax.TransferSyntax)
	ChangeTransferSyntax(ts *transfersyntax.TransferSyntax) error
	ChangeTransferSyntaxContext(ctx context.Context, ts *transfersyntax.TransferSyntax) error
	TagCount() int
	WriteToBytes() []byte
	WriteToFile(fileName string) error
}

type dicomObject struct {
	Tags           []*DICOMTag
	tagIndex       map[uint32]*DICOMTag
	TransferSyntax *transfersyntax.TransferSyntax
	ExplicitVR     bool
	BigEndian      bool
}

// tagKey encodes group and element into a single uint32 map key.
func tagKey(group, element uint16) uint32 {
	return uint32(group)<<16 | uint32(element)
}

// ensureTagIndex rebuilds the tag lookup map if it has been invalidated.
func (obj *dicomObject) ensureTagIndex() {
	if obj.tagIndex != nil {
		return
	}
	obj.tagIndex = make(map[uint32]*DICOMTag, len(obj.Tags))
	for _, t := range obj.Tags {
		k := tagKey(t.Group, t.Element)
		if _, exists := obj.tagIndex[k]; !exists {
			obj.tagIndex[k] = t
		}
	}
}

// NewEmptyDCMObj - Create as an interface to a new empty dicomObject
func NewEmptyDCMObj() DICOMObject {
	return &dicomObject{
		Tags:           make([]*DICOMTag, 0),
		tagIndex:       nil, // built lazily on first lookup
		TransferSyntax: nil,
		ExplicitVR:     false,
		BigEndian:      false,
	}
}

// NewDCMObjFromFile - Read from a DICOM file into a DICOM Object
func NewDCMObjFromFile(fileName string) (DICOMObject, error) {
	buf, err := NewDICOMBufferFromFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("media: NewDCMObjFromFile %q: %w", fileName, err)
	}

	return parseDICOMBuffer(buf)
}

// NewDCMObjFromBytes - Read from a DICOM bytes into a DICOM Object
func NewDCMObjFromBytes(data []byte) (DICOMObject, error) {
	return parseDICOMBuffer(NewDICOMBufferFromBytes(data))
}

func parseDICOMBuffer(buf *DICOMBuffer) (DICOMObject, error) {
	isBigEndian := false

	transferSyntax, err := buf.ReadMeta()
	if err != nil {
		return nil, err
	}

	dicomObj := &dicomObject{
		// Pre-allocate Tags with capacity for 512 entries — enough to hold typical
		// DICOM files (100–500 tags) without any slice growth. Using a fixed value
		// avoids over-allocating on pixel-data-heavy files where fileSize/256 >> tagCount.
		Tags:           make([]*DICOMTag, 0, 512),
		tagIndex:       nil, // built lazily on first lookup; no need to pre-create
		TransferSyntax: transferSyntax,
		ExplicitVR:     false,
		BigEndian:      false,
	}

	if dicomObj.TransferSyntax == nil {
		return nil, fmt.Errorf("unable to read transfer syntax from data: %w", ErrNoTransferSyntax)
	}

	if dicomObj.TransferSyntax == transfersyntax.ImplicitVRLittleEndian {
		dicomObj.ExplicitVR = false
	} else {
		dicomObj.ExplicitVR = true
	}
	if dicomObj.TransferSyntax == transfersyntax.ExplicitVRBigEndian {
		isBigEndian = true
	}
	buf.SetBigEndian(isBigEndian)

	if dicomObj.TransferSyntax == transfersyntax.DeflatedExplicitVRLittleEndian {
		remaining := buf.GetSize() - buf.GetPosition()
		if remaining <= 0 {
			return dicomObj, nil
		}

		deflatedData, err := buf.Read(remaining)
		if err != nil {
			return nil, err
		}

		inflatedData, err := deflate.InflateFrame(deflatedData, -1)
		if err != nil {
			return nil, err
		}

		inflatedBuf := NewDICOMBufferFromBytes(inflatedData)
		inflatedBuf.SetBigEndian(false)
		if err := inflatedBuf.ReadObj(dicomObj); err != nil {
			return nil, err
		}
		return dicomObj, nil
	}

	if err := buf.ReadObj(dicomObj); err != nil {
		return nil, err
	}

	return dicomObj, nil
}

func (obj *dicomObject) IsExplicitVR() bool {
	return obj.ExplicitVR
}

func (obj *dicomObject) SetExplicitVR(explicit bool) {
	obj.ExplicitVR = explicit
}

func (obj *dicomObject) IsBigEndian() bool {
	return obj.BigEndian
}

func (obj *dicomObject) SetBigEndian(bigEndian bool) {
	obj.BigEndian = bigEndian
}

// TagCount - return the Tags number
func (obj *dicomObject) TagCount() int {
	return len(obj.Tags)
}

// GetTagAt - return the Tag at position index
func (obj *dicomObject) GetTagAt(index int) *DICOMTag {
	if index < 0 || index >= len(obj.Tags) {
		return nil
	}
	return obj.Tags[index]
}

func (obj *dicomObject) GetTag(dictTag *tags.Tag) *DICOMTag {
	obj.ensureTagIndex()
	return obj.tagIndex[tagKey(dictTag.Group, dictTag.Element)]
}

func (obj *dicomObject) SetTag(index int, tag *DICOMTag) {
	FillTag(tag)
	if index >= 0 && index < obj.TagCount() {
		obj.Tags[index] = tag
		obj.tagIndex = nil // invalidate; rebuilt on next lookup
	}
}

func (obj *dicomObject) InsertTag(index int, tag *DICOMTag) {
	FillTag(tag)
	if index < 0 || index > len(obj.Tags) {
		return
	}
	obj.Tags = append(obj.Tags, nil)
	copy(obj.Tags[index+1:], obj.Tags[index:])
	obj.Tags[index] = tag
	obj.tagIndex = nil // invalidate; rebuilt on next lookup
}

func (obj *dicomObject) GetTags() []*DICOMTag {
	return obj.Tags
}

func (obj *dicomObject) DelTag(index int) {
	obj.Tags = append(obj.Tags[:index], obj.Tags[index+1:]...)
	obj.tagIndex = nil // invalidate; rebuilt on next lookup
}

func (obj *dicomObject) DumpTags(w io.Writer) {
	ts := "<none>"
	if obj.TransferSyntax != nil {
		ts = obj.TransferSyntax.Name
	}
	_, _ = fmt.Fprintf(w, "Transfer Syntax : %s\n", ts)
	_, _ = fmt.Fprintf(w, "Tags            : %d\n", len(obj.Tags))
	obj.dumpSeq(w, 0)
	_, _ = fmt.Fprintln(w)
}

func (obj *dicomObject) dumpSeq(writer io.Writer, indent int) {
	prefix := strings.Repeat("  ", indent)

	for _, tag := range obj.Tags {
		// Sequence delimiter / item boundary tags — print a visual separator
		if tag.Group == 0xFFFE {
			switch tag.Element {
			case 0xE000:
				_, _ = fmt.Fprintf(writer, "%s--- item ---\n", prefix)
			case 0xE00D, 0xE0DD:
				// item/sequence end — omit, nesting already expressed by indent
			}
			continue
		}

		if tag.VR == "SQ" {
			_, _ = fmt.Fprintf(writer, "%s(%04X,%04X) SQ  %s\n", prefix, tag.Group, tag.Element, tag.Description)
			if seq, ok := tag.ReadSeq(obj.IsExplicitVR()).(*dicomObject); ok {
				seq.dumpSeq(writer, indent+1)
			}
			continue
		}

		if tag.Length > 128 || tag.Length == 0xFFFFFFFF {
			_, _ = fmt.Fprintf(writer, "%s(%04X,%04X) %-4s %s : (%d bytes)\n", prefix, tag.Group, tag.Element, tag.VR, tag.Description, tag.Length)
			continue
		}

		value := obj.formatTagValue(tag)
		_, _ = fmt.Fprintf(writer, "%s(%04X,%04X) %-4s %s : %s\n", prefix, tag.Group, tag.Element, tag.VR, tag.Description, value)
	}
}

// formatTagValue returns a human-readable string for a tag's value.
func (obj *dicomObject) formatTagValue(tag *DICOMTag) string {
	switch tag.VR {
	case "US":
		if len(tag.Data) >= 2 {
			return fmt.Sprintf("%d", tag.GetUint16())
		}
		return "(invalid)"
	case "UL":
		if len(tag.Data) >= 4 {
			return fmt.Sprintf("%d", tag.GetUint32())
		}
		return "(invalid)"
	case "SS":
		if len(tag.Data) >= 2 {
			v := binary.LittleEndian.Uint16(tag.Data)
			return fmt.Sprintf("%d", int16(v))
		}
		return "(invalid)"
	case "SL":
		if len(tag.Data) >= 4 {
			v := binary.LittleEndian.Uint32(tag.Data)
			return fmt.Sprintf("%d", int32(v))
		}
		return "(invalid)"
	case "FL":
		return fmt.Sprintf("%g", tag.GetFloat())
	case "FD":
		return fmt.Sprintf("%g", tag.GetFloat64())
	case "OB", "OW", "UN":
		return fmt.Sprintf("(%d bytes)", tag.Length)
	default:
		return tag.GetString()
	}
}

// GetDate parses a DICOM DA-encoded tag from obj and returns a time.Time.
// Returns the zero value if the tag is absent or the value is not a valid
// "YYYYMMDD" date string.
func GetDate(obj DICOMObject, tag *tags.Tag) time.Time {
	s := obj.GetString(tag)
	d, _ := time.Parse("20060102", s)
	return d
}

// findTagGE returns the first top-level tag matching group/element whose
// length is defined (non-zero, non-0xFFFFFFFF). The tag index provides an
// O(1) hit for the overwhelmingly common case; the linear scan is the
// fallback for the rare case where the indexed entry is an undefined-length
// container (SQ / encapsulated pixel data) or a sequence-nested occurrence
// that was indexed before the top-level one.
func (obj *dicomObject) findTagGE(group uint16, element uint16) *DICOMTag {
	obj.ensureTagIndex()
	if t := obj.tagIndex[tagKey(group, element)]; t != nil && t.Length > 0 && t.Length != 0xFFFFFFFF {
		return t
	}
	// Slow path: the indexed entry is absent, zero-length, or undefined-length.
	// Walk the flat tag list at depth-0 only.
	sequenceDepth := 0
	for i := 0; i < obj.TagCount(); i++ {
		t := obj.GetTagAt(i)
		if (t.VR == "SQ" && t.Length == 0xFFFFFFFF) || (t.Group == 0xFFFE && t.Element == 0xE000 && t.Length == 0xFFFFFFFF) {
			sequenceDepth++
		}
		if sequenceDepth == 0 && t.Length > 0 && t.Length != 0xFFFFFFFF {
			if t.Group == group && t.Element == element {
				return t
			}
		}
		if (t.Group == 0xFFFE && t.Element == 0xE00D) || (t.Group == 0xFFFE && t.Element == 0xE0DD) {
			sequenceDepth--
		}
	}
	return nil
}

func (obj *dicomObject) GetUint16(tag *tags.Tag) uint16 {
	if t := obj.findTagGE(tag.Group, tag.Element); t != nil {
		return t.GetUint16()
	}
	return 0
}

func (obj *dicomObject) GetUint32(tag *tags.Tag) uint32 {
	if t := obj.findTagGE(tag.Group, tag.Element); t != nil {
		return t.GetUint32()
	}
	return 0
}

func (obj *dicomObject) GetString(tag *tags.Tag) string {
	if t := obj.findTagGE(tag.Group, tag.Element); t != nil {
		return t.GetString()
	}
	return ""
}

// Add - add a new DICOM Tag to a DICOM Object
func (obj *dicomObject) Add(tag *DICOMTag) {
	obj.Tags = append(obj.Tags, tag)
	if obj.tagIndex != nil {
		k := tagKey(tag.Group, tag.Element)
		if _, exists := obj.tagIndex[k]; !exists {
			obj.tagIndex[k] = tag
		}
	}
}

func (obj *dicomObject) WriteToBytes() []byte {
	if err := ValidateFileWrite(obj); err != nil {
		return nil
	}

	// Pre-size the output buffer by summing the actual data length of every tag
	// (plus a worst-case 12-byte explicit VR header) and adding meta overhead.
	// This avoids the repeated doubling reallocations that start from 4096 bytes.
	estimatedSize := 256 // preamble (128) + DICM (4) + meta tags (~124)
	for i := 0; i < obj.TagCount(); i++ {
		t := obj.GetTagAt(i)
		if t.Length != 0xFFFFFFFF {
			estimatedSize += 12 + int(t.Length)
		} else {
			estimatedSize += 12 // undefined-length tag (encapsulated pixel data parent)
		}
	}
	buf := NewDICOMBufferWithCapacity(estimatedSize)

	if obj.TransferSyntax.UID == transfersyntax.ExplicitVRBigEndian.UID {
		buf.SetBigEndian(true)
	}
	SOPClassUID := obj.GetString(tags.SOPClassUID)
	SOPInstanceUID := obj.GetString(tags.SOPInstanceUID)
	buf.WriteMeta(SOPClassUID, SOPInstanceUID, obj.TransferSyntax.UID)
	if obj.TransferSyntax.UID == transfersyntax.DeflatedExplicitVRLittleEndian.UID {
		rawDataset := NewDICOMBuffer()
		rawDataset.WriteObj(obj)
		compressed, err := deflate.DeflateFrame(rawDataset.GetAllBytes())
		if err == nil {
			buf.Write(compressed, len(compressed))
		} else {
			buf.WriteObj(obj)
		}
	} else {
		buf.WriteObj(obj)
	}
	buf.SetPosition(0)
	return buf.GetAllBytes()
}

func ValidateFileWrite(obj DICOMObject) error {
	if obj == nil {
		return errors.New("media: cannot write nil DICOM object")
	}
	if obj.GetTransferSyntax() == nil {
		return errors.New("media: TransferSyntax is required for DICOM file output")
	}
	if transfersyntax.GetTransferSyntaxFromUID(obj.GetTransferSyntax().UID) == nil {
		return fmt.Errorf("media: unsupported TransferSyntaxUID %q for DICOM file output", obj.GetTransferSyntax().UID)
	}
	if strings.TrimSpace(obj.GetString(tags.SOPClassUID)) == "" {
		return errors.New("media: SOPClassUID is required for DICOM file output")
	}
	if strings.TrimSpace(obj.GetString(tags.SOPInstanceUID)) == "" {
		return errors.New("media: SOPInstanceUID is required for DICOM file output")
	}
	return nil
}

// Wrote - Write a DICOM Object to a DICOM File
func (obj *dicomObject) WriteToFile(fileName string) error {
	data := obj.WriteToBytes()
	if data == nil {
		return ValidateFileWrite(obj)
	}
	return os.WriteFile(fileName, data, 0o600)
}

// DateRange represents a DICOM date range query value (e.g. for C-FIND).
type DateRange struct{ Start, End time.Time }

func (obj *dicomObject) Write(tag *tags.Tag, value any) {
	switch v := value.(type) {
	case string:
		obj.Add(NewStringTag(tag.Group, tag.Element, tag.VR, v))
	case uint16:
		c := make([]byte, 2)
		if obj.BigEndian {
			binary.BigEndian.PutUint16(c, v)
		} else {
			binary.LittleEndian.PutUint16(c, v)
		}
		t := &DICOMTag{Group: tag.Group, Element: tag.Element, Length: 2, VR: tag.VR, Data: c, BigEndian: obj.BigEndian}
		FillTag(t)
		obj.Add(t)
	case uint32:
		c := make([]byte, 4)
		if obj.BigEndian {
			binary.BigEndian.PutUint32(c, v)
		} else {
			binary.LittleEndian.PutUint32(c, v)
		}
		t := &DICOMTag{Group: tag.Group, Element: tag.Element, Length: 4, VR: tag.VR, Data: c, BigEndian: obj.BigEndian}
		FillTag(t)
		obj.Add(t)
	case int:
		// Untyped integer constants box as int; dispatch to the correct width via VR.
		switch tag.VR {
		case "UL", "SL":
			obj.Write(tag, uint32(v))
		default: // "US", "SS", "OW", "OB" …
			obj.Write(tag, uint16(v))
		}
	case time.Time:
		var s string
		switch tag.VR {
		case "TM":
			s = v.Format("150405")
		default: // "DA"
			s = v.Format("20060102")
		}
		obj.Add(NewStringTag(tag.Group, tag.Element, tag.VR, s))
	case DateRange:
		obj.Add(NewStringTag(tag.Group, tag.Element, tag.VR,
			fmt.Sprintf("%s-%s", v.Start.Format("20060102"), v.End.Format("20060102"))))
	}
}

// NewStringTag builds a DICOMTag with string content, properly encoded and
// padded. Use obj.Add(media.NewStringTag(...)) when the tag identity is known
// only at runtime (no dictionary constant available).
func NewStringTag(group, element uint16, vr, content string) *DICOMTag {
	data := []byte(content)
	length := len(data)
	if length%2 == 1 {
		length++
		if vr == "UI" {
			data = append(data, 0x00)
		} else {
			data = append(data, 0x20)
		}
	}
	t := &DICOMTag{Group: group, Element: element, Length: uint32(length), VR: vr, Data: data, BigEndian: false}
	FillTag(t)
	return t
}

func (obj *dicomObject) GetTransferSyntax() *transfersyntax.TransferSyntax {
	return obj.TransferSyntax
}

func (obj *dicomObject) SetTransferSyntax(ts *transfersyntax.TransferSyntax) {
	obj.TransferSyntax = ts
}

func parseBasicOffsetTable(data []byte) []uint32 {
	if len(data) == 0 || len(data)%4 != 0 {
		return nil
	}
	offsets := make([]uint32, len(data)/4)
	for i := range offsets {
		offsets[i] = binary.LittleEndian.Uint32(data[i*4 : i*4+4])
	}
	return offsets
}

func joinFragments(fragments [][]byte) []byte {
	total := 0
	for _, frag := range fragments {
		total += len(frag)
	}
	out := make([]byte, total)
	pos := 0
	for _, frag := range fragments {
		copy(out[pos:], frag)
		pos += len(frag)
	}
	return out
}

// deplanarizeRGBFrames converts RGB pixel data from planar format (all R values
// followed by all G values followed by all B values, per frame) to interleaved
// RGB format in dst. src and dst may overlap or be the same slice.
// frameSize is the byte count of one frame (rows * cols * 3).
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

func frameFragmentRangeByBOT(offsets []uint32, frame int, fragmentPayloadSizes []int) (int, int, bool) {
	if frame < 0 || frame >= len(offsets) {
		return 0, 0, false
	}
	if len(fragmentPayloadSizes) == 0 {
		return 0, 0, false
	}

	acc := uint32(0)
	startIdx := -1
	for idx, payloadLen := range fragmentPayloadSizes {
		itemStart := acc
		itemSize := uint32(8 + payloadLen)
		if offsets[frame] == itemStart {
			startIdx = idx
			break
		}
		acc += itemSize
	}
	if startIdx < 0 {
		return 0, 0, false
	}

	if frame == len(offsets)-1 {
		return startIdx, len(fragmentPayloadSizes), true
	}

	nextOffset := offsets[frame+1]
	acc = 0
	for idx, payloadLen := range fragmentPayloadSizes {
		if acc == nextOffset {
			if idx <= startIdx {
				return 0, 0, false
			}
			return startIdx, idx, true
		}
		acc += uint32(8 + payloadLen)
	}

	return 0, 0, false
}

// pixelMeta holds the image geometry and photometric attributes collected
// from the 0x0028 group before PixelData is reached.
type pixelMeta struct {
	rows, cols, bitsa, planar uint16
	frames                    uint32
	photoInt                  string
	RGB                       bool
}

// readPixelMeta scans the flat tag list for 0x0028 pixel-geometry attributes
// (Rows, Columns, BitsAllocated, NumberOfFrames, PhotometricInterpretation,
// PlanarConfiguration) and returns the populated pixelMeta together with the
// index of the 7FE0,0010 PixelData tag. pixelIdx is -1 when no PixelData tag
// is found. Icon-sequence tags are skipped following DICOM convention.
func readPixelMeta(tagList []*DICOMTag) (pm pixelMeta, pixelIdx int) {
	sq := 0
	icon := false
	pixelIdx = -1
	for i, tag := range tagList {
		if (tag.VR == "SQ" && tag.Length == 0xFFFFFFFF) || (tag.Group == 0xFFFE && tag.Element == 0xE000 && tag.Length == 0xFFFFFFFF) {
			sq++
		}
		if sq == 0 {
			if tag.Group == 0x0028 && !icon {
				switch tag.Element {
				case 0x0004:
					pm.photoInt = tag.GetString()
					if !strings.Contains(pm.photoInt, "MONO") {
						pm.RGB = true
					}
				case 0x0006:
					pm.planar = tag.GetUint16()
				case 0x0008:
					if n, err := strconv.Atoi(tag.GetString()); err == nil {
						pm.frames = uint32(n)
					}
				case 0x0010:
					pm.rows = tag.GetUint16()
				case 0x0011:
					pm.cols = tag.GetUint16()
				case 0x0100:
					pm.bitsa = tag.GetUint16()
				}
			}
			if tag.Group == 0x0088 && tag.Element == 0x0200 && tag.Length == 0xFFFFFFFF {
				icon = true
			}
			if tag.Group == 0x6003 && tag.Element == 0x1010 && tag.Length == 0xFFFFFFFF {
				icon = true
			}
			if tag.Group == 0x7FE0 && tag.Element == 0x0010 && !icon {
				pixelIdx = i
				return
			}
		}
		if (tag.Group == 0xFFFE && tag.Element == 0xE00D) || (tag.Group == 0xFFFE && tag.Element == 0xE0DD) {
			sq--
		}
	}
	return
}

func (obj *dicomObject) GetPixelData(frame int) ([]byte, error) {
	if !transfersyntax.SupportedTransferSyntax(obj.TransferSyntax.UID) {
		return nil, fmt.Errorf("unsupported transfer syntax %s", obj.TransferSyntax.Name)
	}

	pm, i := readPixelMeta(obj.Tags)
	if i < 0 {
		return nil, fmt.Errorf("pixel data (7FE0,0010) not found for frame %d", frame)
	}

	rows, cols, bitsa, planar := pm.rows, pm.cols, pm.bitsa, pm.planar
	RGB := pm.RGB
	frames := pm.frames
	tag := obj.GetTagAt(i)

	sizePx := uint64(cols) * uint64(rows) * uint64(bitsa) / 8
	if RGB {
		sizePx = 3 * sizePx
	}
	if frames > 0 {
		sizePx *= uint64(frames)
	} else {
		frames = 1
	}
	if sizePx == 0 {
		return nil, fmt.Errorf("DICOMObject::GetPixelData, invalid pixel data size %d", sizePx)
	}

	if frame >= int(frames) {
		return nil, errors.New("invalid frame")
	}

	if tag.Length == 0xFFFFFFFF {
		if i+1 >= len(obj.Tags) {
			return nil, errors.New("missing basic offset table")
		}

		botItem := obj.GetTagAt(i + 1)
		if botItem == nil || botItem.Group != 0xFFFE || botItem.Element != 0xE000 {
			return nil, errors.New("invalid encapsulated pixel data layout")
		}

		fragments := make([][]byte, 0)
		fragmentPayloadSizes := make([]int, 0)
		for tagIdx := i + 2; tagIdx < len(obj.Tags); tagIdx++ {
			t := obj.GetTagAt(tagIdx)
			if t == nil {
				continue
			}
			if t.Group == 0xFFFE && t.Element == 0xE0DD {
				break
			}
			if t.Group == 0xFFFE && t.Element == 0xE000 {
				fragments = append(fragments, t.Data)
				fragmentPayloadSizes = append(fragmentPayloadSizes, len(t.Data))
			}
		}

		if len(fragments) == 0 {
			return nil, fmt.Errorf("frame %d out of range", frame)
		}

		if frames <= 1 {
			return joinFragments(fragments), nil
		}

		if len(fragments) == int(frames) {
			out := make([]byte, len(fragments[frame]))
			copy(out, fragments[frame])
			return out, nil
		}

		botOffsets := parseBasicOffsetTable(botItem.Data)
		if len(botOffsets) >= int(frames) {
			start, end, ok := frameFragmentRangeByBOT(botOffsets[:frames], frame, fragmentPayloadSizes)
			if ok {
				return joinFragments(fragments[start:end]), nil
			}
		}

		if frame == 0 {
			return joinFragments(fragments), nil
		}

		return nil, fmt.Errorf("frame %d out of range", frame)
	}

	// Uncompressed path: total pixel data must fit within the allocation cap.
	if sizePx > uint64(maxPixelDataBytes) {
		return nil, fmt.Errorf("DICOMObject::GetPixelData, invalid pixel data size %d", sizePx)
	}
	size := uint32(sizePx)
	if RGB && (planar == 1) {
		imgSize := size / frames
		off := imgSize * uint32(frame)
		pixels := imgSize / 3
		img := make([]byte, imgSize)
		for j := uint32(0); j < pixels; j++ {
			img[3*j] = tag.Data[j+off]
			img[3*j+1] = tag.Data[j+pixels+off]
			img[3*j+2] = tag.Data[j+2*pixels+off]
		}
		return img, nil
	}
	imgSize := size / frames
	offset := uint32(frame) * imgSize
	if offset+imgSize > uint32(len(tag.Data)) {
		return nil, fmt.Errorf("frame %d out of range", frame)
	}
	out := make([]byte, imgSize)
	copy(out, tag.Data[offset:offset+imgSize])
	return out, nil
}

// decompressSingleFrame decodes a single compressed frame into out.
// The caller must pre-allocate out with the exact uncompressed frame size.
func decompressSingleFrame(ctx context.Context, tsUID string, compressed []byte, bitsa uint16, photoInt string, out []byte) error {
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
		// Non-conformant files that declare an uncompressed transfer syntax but
		// store pixel data as encapsulated fragments (length 0xFFFFFFFF). Per the
		// DICOM standard these should never be encapsulated, but real-world
		// scanners produce them. Treat the fragment payload as raw pixel bytes.
		transfersyntax.ExplicitVRLittleEndian.UID,
		transfersyntax.ImplicitVRLittleEndian.UID:
		// A non-conformant C-GET SCP may accept an uncompressed transfer syntax
		// during association negotiation but send the image in its native
		// compressed format without transcoding. Always check the byte signature
		// for well-known compressed formats first, regardless of whether the
		// fragment is larger or smaller than the expected raw frame — compressed
		// streams for small images (e.g. JPEG 2000 on 8×8 tiles) are often
		// *larger* than the raw bytes, so the old "fragment smaller than outLen"
		// guard was insufficient.
		if len(compressed) >= 2 {
			b0, b1 := compressed[0], compressed[1]
			switch {
			case b0 == 0xFF && b1 == 0xD8:
				// JPEG family (SOI marker). Check for JPEG-LS (SOF55 = 0xFF 0xF7).
				if len(compressed) >= 4 && compressed[2] == 0xFF && compressed[3] == 0xF7 {
					return jpegls.JLSdecodeContext(ctx, compressed, compLen, out)
				}
				// JPEG lossless / baseline / extended: dispatch by SOF precision.
				prec := jpeg.SOFPrecision(compressed)
				if prec == 0 {
					// Fall back to bitsa heuristic when SOF is unreadable.
					if bitsa == 8 {
						return jpeg.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
					}
					if bitsa <= 12 {
						return jpeg.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)
					}
					return jpeg.DIJG16decodeContext(ctx, compressed, compLen, out, outLen)
				}
				switch {
				case prec == 8:
					return jpeg.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
				case prec <= 12:
					return jpeg.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)
				default:
					return jpeg.DIJG16decodeContext(ctx, compressed, compLen, out, outLen)
				}
			case b0 == 0xFF && b1 == 0x4F:
				// JPEG 2000 / HTJ2K codestream (SOC marker).
				return jpeg2000.J2KdecodeContext(ctx, compressed, compLen, out)
			case b0 == 0xFF && b1 == 0x0A:
				// JPEG XL bare codestream.
				return jpegxl.JXLdecodeContext(ctx, compressed, compLen, out)
			}
		}
		if int(outLen) > len(compressed) {
			return errors.New("encapsulated uncompressed frame too small")
		}
		copy(out, compressed[:outLen])
	case transfersyntax.RLELossless.UID:
		return rle.RLEdecode(compressed, out, compLen, outLen, photoInt)
	case transfersyntax.JPEGLosslessSV1.UID,
		transfersyntax.JPEGLossless.UID:
		// Dispatch on the precision declared in the JPEG SOF header rather than
		// bitsa: DICOM metadata may differ from the actual payload precision
		// (e.g. 10-bit CT stored with bitsa=12, or non-conformant encoders).
		prec := jpeg.SOFPrecision(compressed)
		if prec == 0 {
			// Could not read SOF — fall back to bitsa heuristic.
			if bitsa == 8 {
				return jpeg.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
			}
			if bitsa <= 12 {
				return jpeg.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)
			}
			return jpeg.DIJG16decodeContext(ctx, compressed, compLen, out, outLen)
		}
		switch {
		case prec == 8:
			return jpeg.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
		case prec <= 12:
			return jpeg.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)
		default:
			return jpeg.DIJG16decodeContext(ctx, compressed, compLen, out, outLen)
		}
	case transfersyntax.JPEGBaseline8Bit.UID:
		if bitsa == 8 {
			return jpeg.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
		}
		return jpeg.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)
	case transfersyntax.JPEGExtended12Bit.UID:
		if jpeg.SOFPrecision(compressed) == 8 {
			return jpeg.DIJG8decodeContext(ctx, compressed, compLen, out, outLen)
		}
		return jpeg.DIJG12decodeContext(ctx, compressed, compLen, out, outLen)
	case transfersyntax.JPEGLSLossless.UID,
		transfersyntax.JPEGLSNearLossless.UID:
		return jpegls.JLSdecodeContext(ctx, compressed, compLen, out)
	case transfersyntax.JPEG2000Lossless.UID,
		transfersyntax.JPEG2000MCLossless.UID,
		transfersyntax.HTJ2KLossless.UID,
		transfersyntax.HTJ2KLosslessRPCL.UID,
		transfersyntax.JPEG2000.UID,
		transfersyntax.JPEG2000MC.UID,
		transfersyntax.HTJ2K.UID:
		return jpeg2000.J2KdecodeContext(ctx, compressed, compLen, out)
	case transfersyntax.JPEGXLLossless.UID,
		transfersyntax.JPEGXLJPEGRecompression.UID,
		transfersyntax.JPEGXL.UID:
		return jpegxl.JXLdecodeContext(ctx, compressed, compLen, out)
	case transfersyntax.JPIPHTJ2KReferenced.UID,
		transfersyntax.JPIPHTJ2KReferencedDeflate.UID:
		return jpip.JPIPdecodeContext(ctx, compressed, compLen, out, tsUID)
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
		return mpeg.MPEGdecodeContext(ctx, compressed, compLen, out, tsUID)
	case transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo.UID,
		transfersyntax.SMPTEST211020UncompressedInterlacedActiveVideo.UID,
		transfersyntax.SMPTEST211030PCMDigitalAudio.UID:
		return smpte2110.SMPTE2110decodeContext(ctx, compressed, compLen, out, tsUID)
	default:
		return fmt.Errorf("unsupported transfer syntax for single-frame decompression: %s", tsUID)
	}
	return nil
}

// GetDecompressedFrame returns the raw uncompressed pixel bytes for the requested frame.
// Unlike GetPixelData, this method only allocates memory for a single frame's uncompressed
// data, making it safe for large multi-frame objects whose total pixel data exceeds
// maxPixelDataBytes. For encapsulated (compressed) transfer syntaxes the frame is
// extracted from its fragment and decompressed in-place.
func (obj *dicomObject) GetDecompressedFrame(ctx context.Context, frameIndex int) ([]byte, error) {
	if !transfersyntax.SupportedTransferSyntax(obj.TransferSyntax.UID) {
		return nil, fmt.Errorf("unsupported transfer syntax %s", obj.TransferSyntax.Name)
	}

	pm, i := readPixelMeta(obj.Tags)
	if i < 0 {
		return nil, errors.New("DICOMObject::GetDecompressedFrame, pixel data tag not found")
	}

	rows, cols, bitsa, planar := pm.rows, pm.cols, pm.bitsa, pm.planar
	photoInt, RGB := pm.photoInt, pm.RGB
	frames := pm.frames
	tag := obj.GetTagAt(i)

	// For encapsulated pixel data with missing Rows/Columns tags (e.g. Secondary
	// Capture photographs stored as JPEG), try to recover the dimensions from the
	// JPEG SOF header of the first compressed fragment.
	if (rows == 0 || cols == 0) && tag != nil && tag.Length == 0xFFFFFFFF {
		// Layout: [i] = pixel data tag, [i+1] = BOT item, [i+2] = first fragment.
		if i+2 < len(obj.Tags) {
			if frag := obj.GetTagAt(i + 2); frag != nil &&
				frag.Group == 0xFFFE && frag.Element == 0xE000 && len(frag.Data) > 0 {
				if hdr := jpeg.ReadSOFHeader(frag.Data); hdr.Width > 0 && hdr.Height > 0 {
					cols = hdr.Width
					rows = hdr.Height
					if bitsa == 0 {
						bitsa = uint16(hdr.Precision)
					}
				}
			}
		}
	}

	// Per-frame size only — no frames multiplication.
	frameSz := uint64(cols) * uint64(rows) * uint64(bitsa) / 8
	if RGB {
		frameSz = 3 * frameSz
	}
	if frameSz == 0 || frameSz > uint64(maxPixelDataBytes) {
		return nil, fmt.Errorf("DICOMObject::GetDecompressedFrame, invalid frame size %d", frameSz)
	}
	if frames == 0 {
		frames = 1
	}
	if frameIndex >= int(frames) {
		return nil, errors.New("invalid frame index")
	}
	frameSize := uint32(frameSz)

	if tag.Length == 0xFFFFFFFF {
		// Encapsulated (compressed): extract fragment bytes then decompress.
		if i+1 >= len(obj.Tags) {
			return nil, errors.New("missing basic offset table")
		}
		botItem := obj.GetTagAt(i + 1)
		if botItem == nil || botItem.Group != 0xFFFE || botItem.Element != 0xE000 {
			return nil, errors.New("invalid encapsulated pixel data layout")
		}
		fragments := make([][]byte, 0)
		fragmentPayloadSizes := make([]int, 0)
		for tagIdx := i + 2; tagIdx < len(obj.Tags); tagIdx++ {
			t := obj.GetTagAt(tagIdx)
			if t == nil {
				continue
			}
			if t.Group == 0xFFFE && t.Element == 0xE0DD {
				break
			}
			if t.Group == 0xFFFE && t.Element == 0xE000 {
				fragments = append(fragments, t.Data)
				fragmentPayloadSizes = append(fragmentPayloadSizes, len(t.Data))
			}
		}
		if len(fragments) == 0 {
			return nil, fmt.Errorf("frame %d out of range", frameIndex)
		}
		var compressedFrame []byte
		if frames <= 1 {
			compressedFrame = joinFragments(fragments)
		} else if len(fragments) == int(frames) {
			compressedFrame = fragments[frameIndex]
		} else {
			botOffsets := parseBasicOffsetTable(botItem.Data)
			if len(botOffsets) >= int(frames) {
				start, end, ok := frameFragmentRangeByBOT(botOffsets[:frames], frameIndex, fragmentPayloadSizes)
				if ok {
					compressedFrame = joinFragments(fragments[start:end])
				}
			}
			if compressedFrame == nil {
				if frameIndex == 0 {
					compressedFrame = joinFragments(fragments)
				} else {
					return nil, fmt.Errorf("frame %d out of range", frameIndex)
				}
			}
		}
		out := make([]byte, frameSize)
		if err := decompressSingleFrame(ctx, obj.TransferSyntax.UID, compressedFrame, bitsa, photoInt, out); err != nil {
			return nil, fmt.Errorf("DICOMObject::GetDecompressedFrame, decompress failed: %w", err)
		}
		return out, nil
	}

	// Uncompressed path: slice the requested frame from the flat pixel data.
	// The full pixel data is already in tag.Data (loaded by NewDCMObjFromFile).
	if RGB && (planar == 1) {
		off := frameSize * uint32(frameIndex)
		pixels := frameSize / 3
		img := make([]byte, frameSize)
		for j := uint32(0); j < pixels; j++ {
			img[3*j] = tag.Data[j+off]
			img[3*j+1] = tag.Data[j+pixels+off]
			img[3*j+2] = tag.Data[j+2*pixels+off]
		}
		return img, nil
	}
	offset := uint32(frameIndex) * frameSize
	if offset+frameSize > uint32(len(tag.Data)) {
		return nil, fmt.Errorf("frame %d out of range", frameIndex)
	}
	out := make([]byte, frameSize)
	copy(out, tag.Data[offset:offset+frameSize])
	return out, nil
}
