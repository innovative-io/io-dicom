package media

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/innovative-io/io-dicom/codecs"
	"github.com/innovative-io/io-dicom/codecs/deflate"
	"github.com/innovative-io/io-dicom/codecs/jpeg"
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
	// GetFloat32 returns the IEEE 754 single-precision (FL) value of tag, or 0.
	GetFloat32(tag *tags.Tag) float32
	// GetFloat64 returns the IEEE 754 double-precision (FD) value of tag, or 0.
	GetFloat64(tag *tags.Tag) float64
	// GetDate parses a DICOM DA-encoded tag and returns a time.Time.
	// Returns the zero value if absent or not a valid "YYYYMMDD" date.
	GetDate(tag *tags.Tag) time.Time
	// GetTime parses a DICOM TM-encoded tag and returns a time.Time.
	// Returns the zero value if absent or not a valid "HHMMSS" time.
	GetTime(tag *tags.Tag) time.Time
	// Write encodes value and appends it to the object as a tag. The concrete
	// type of value determines encoding:
	//   - string   → DICOM string tag (padded per VR)
	//   - uint16   → 2-byte little- or big-endian field
	//   - uint32   → 4-byte little- or big-endian field
	//   - int      → dispatched to uint16 or uint32 based on tag.VR
	//   - time.Time → formatted string: "20060102" for DA, "150405" for TM
	//   - DateRange → formatted "YYYYMMDD-YYYYMMDD" string for date range queries
	Write(tag *tags.Tag, value any)
	// WriteString is a type-safe variant of Write for string values.
	WriteString(tag *tags.Tag, value string)
	// WriteUint16 is a type-safe variant of Write for uint16 values.
	WriteUint16(tag *tags.Tag, value uint16)
	// WriteUint32 is a type-safe variant of Write for uint32 values.
	WriteUint32(tag *tags.Tag, value uint32)
	// WriteFloat32 is a type-safe variant of Write for float32 (FL) values.
	WriteFloat32(tag *tags.Tag, value float32)
	// WriteFloat64 is a type-safe variant of Write for float64 (FD) values.
	WriteFloat64(tag *tags.Tag, value float64)
	// WriteTime is a type-safe variant of Write for time.Time values.
	WriteTime(tag *tags.Tag, value time.Time)
	// WriteDateRange is a type-safe variant of Write for DateRange values.
	WriteDateRange(tag *tags.Tag, value DateRange)
	// Clone returns a deep copy of this DICOMObject. The returned object has
	// independent tag data; mutations to either object do not affect the other.
	Clone() DICOMObject
	// Merge copies tags from src into this object. Tags already present in the
	// receiver are left unchanged; only tags absent from the receiver are added.
	// To overwrite existing tags call obj.DelTag first or use Clone + Merge on
	// a fresh object.
	Merge(src DICOMObject)
	GetTransferSyntax() *transfersyntax.TransferSyntax
	SetTransferSyntax(ts *transfersyntax.TransferSyntax)
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
		if obj.tagIndex != nil {
			old := obj.Tags[index]
			delete(obj.tagIndex, tagKey(old.Group, old.Element))
			k := tagKey(tag.Group, tag.Element)
			if _, exists := obj.tagIndex[k]; !exists {
				obj.tagIndex[k] = tag
			}
		}
		obj.Tags[index] = tag
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
	if obj.tagIndex != nil {
		// DICOM tags are unique by (group,element); unconditionally update so the
		// index always points to the correct (possibly newly-inserted) tag.
		obj.tagIndex[tagKey(tag.Group, tag.Element)] = tag
	}
}

func (obj *dicomObject) GetTags() []*DICOMTag {
	return obj.Tags
}

func (obj *dicomObject) DelTag(index int) {
	if obj.tagIndex != nil {
		deleted := obj.Tags[index]
		delete(obj.tagIndex, tagKey(deleted.Group, deleted.Element))
	}
	obj.Tags = append(obj.Tags[:index], obj.Tags[index+1:]...)
}

func (obj *dicomObject) DumpTags(w io.Writer) {
	bw := bufio.NewWriter(w)
	ts := "<none>"
	if obj.TransferSyntax != nil {
		ts = obj.TransferSyntax.Name
	}
	_, _ = fmt.Fprintf(bw, "Transfer Syntax : %s\n", ts)
	_, _ = fmt.Fprintf(bw, "Tags            : %d\n", len(obj.Tags))
	obj.dumpSeq(bw, 0)
	_, _ = fmt.Fprintln(bw)
	_ = bw.Flush()
}

// dumpIndents covers the nesting depths seen in practice; deeper levels fall back to strings.Repeat.
var dumpIndents = [8]string{
	"", "  ", "    ", "      ", "        ",
	"          ", "            ", "              ",
}

func dumpIndent(n int) string {
	if n < len(dumpIndents) {
		return dumpIndents[n]
	}
	return strings.Repeat("  ", n)
}

func (obj *dicomObject) dumpSeq(writer io.Writer, indent int) {
	prefix := dumpIndent(indent)

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

func (obj *dicomObject) GetFloat32(tag *tags.Tag) float32 {
	if t := obj.findTagGE(tag.Group, tag.Element); t != nil {
		return t.GetFloat()
	}
	return 0
}

func (obj *dicomObject) GetFloat64(tag *tags.Tag) float64 {
	if t := obj.findTagGE(tag.Group, tag.Element); t != nil {
		return t.GetFloat64()
	}
	return 0
}

func (obj *dicomObject) GetDate(tag *tags.Tag) time.Time {
	d, _ := time.Parse("20060102", obj.GetString(tag))
	return d
}

func (obj *dicomObject) GetTime(tag *tags.Tag) time.Time {
	tm, _ := time.Parse("150405", obj.GetString(tag))
	return tm
}

// GetDate parses a DICOM DA-encoded tag from obj and returns a time.Time.
// Returns the zero value if the tag is absent or the value is not a valid
// "YYYYMMDD" date string.
//
// Deprecated: call obj.GetDate(tag) directly.
func GetDate(obj DICOMObject, tag *tags.Tag) time.Time {
	return obj.GetDate(tag)
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

// upsertTag overwrites the existing tag with the same group+element key in
// place, or calls Add when no such tag exists yet. This ensures that repeated
// Write calls for the same tag do not accumulate duplicate entries.
func (obj *dicomObject) upsertTag(newTag *DICOMTag) {
	obj.ensureTagIndex()
	k := tagKey(newTag.Group, newTag.Element)
	if existing, ok := obj.tagIndex[k]; ok {
		existing.Data = newTag.Data
		existing.Length = newTag.Length
		existing.VR = newTag.VR
		existing.BigEndian = newTag.BigEndian
		return
	}
	obj.Add(newTag)
}

func (obj *dicomObject) encodeToBytes() []byte {
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

	SOPClassUID := obj.GetString(tags.SOPClassUID)
	SOPInstanceUID := obj.GetString(tags.SOPInstanceUID)
	buf.WriteMeta(SOPClassUID, SOPInstanceUID, obj.TransferSyntax.UID)
	// File Meta Information is always little-endian per DICOM PS3.10 §10.1.
	// The big-endian flag must only be set after WriteMeta so the dataset
	// (not the meta header) is written in the correct byte order.
	if obj.TransferSyntax.UID == transfersyntax.ExplicitVRBigEndian.UID {
		buf.SetBigEndian(true)
	}
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
	return buf.GetAllBytes()
}

func (obj *dicomObject) WriteToBytes() []byte {
	if err := ValidateFileWrite(obj); err != nil {
		return nil
	}
	return obj.encodeToBytes()
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
	sopClass := strings.TrimSpace(obj.GetString(tags.SOPClassUID))
	if sopClass == "" {
		return errors.New("media: SOPClassUID is required for DICOM file output")
	}
	sopInstance := strings.TrimSpace(obj.GetString(tags.SOPInstanceUID))
	if sopInstance == "" {
		return errors.New("media: SOPInstanceUID is required for DICOM file output")
	}
	// Validate group-0002 meta consistency when meta tags are present.
	if msClass := strings.TrimSpace(obj.GetString(tags.MediaStorageSOPClassUID)); msClass != "" && msClass != sopClass {
		return fmt.Errorf("media: MediaStorageSOPClassUID (0002,0002) %q does not match SOPClassUID %q", msClass, sopClass)
	}
	if msInstance := strings.TrimSpace(obj.GetString(tags.MediaStorageSOPInstanceUID)); msInstance != "" && msInstance != sopInstance {
		return fmt.Errorf("media: MediaStorageSOPInstanceUID (0002,0003) %q does not match SOPInstanceUID %q", msInstance, sopInstance)
	}
	return nil
}

// Wrote - Write a DICOM Object to a DICOM File
func (obj *dicomObject) WriteToFile(fileName string) error {
	if err := ValidateFileWrite(obj); err != nil {
		return err
	}
	return os.WriteFile(fileName, obj.encodeToBytes(), 0o600)
}

// DateRange represents a DICOM date range query value (e.g. for C-FIND).
type DateRange struct{ Start, End time.Time }

func (obj *dicomObject) Write(tag *tags.Tag, value any) {
	switch v := value.(type) {
	case string:
		obj.upsertTag(NewStringTag(tag.Group, tag.Element, tag.VR, v))
	case uint16:
		c := make([]byte, 2)
		if obj.BigEndian {
			binary.BigEndian.PutUint16(c, v)
		} else {
			binary.LittleEndian.PutUint16(c, v)
		}
		t := &DICOMTag{Group: tag.Group, Element: tag.Element, Length: 2, VR: tag.VR, Data: c, BigEndian: obj.BigEndian}
		FillTag(t)
		obj.upsertTag(t)
	case uint32:
		c := make([]byte, 4)
		if obj.BigEndian {
			binary.BigEndian.PutUint32(c, v)
		} else {
			binary.LittleEndian.PutUint32(c, v)
		}
		t := &DICOMTag{Group: tag.Group, Element: tag.Element, Length: 4, VR: tag.VR, Data: c, BigEndian: obj.BigEndian}
		FillTag(t)
		obj.upsertTag(t)
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
		obj.upsertTag(NewStringTag(tag.Group, tag.Element, tag.VR, s))
	case float32:
		c := make([]byte, 4)
		if obj.BigEndian {
			binary.BigEndian.PutUint32(c, math.Float32bits(v))
		} else {
			binary.LittleEndian.PutUint32(c, math.Float32bits(v))
		}
		t := &DICOMTag{Group: tag.Group, Element: tag.Element, Length: 4, VR: tag.VR, Data: c, BigEndian: obj.BigEndian}
		FillTag(t)
		obj.upsertTag(t)
	case float64:
		c := make([]byte, 8)
		if obj.BigEndian {
			binary.BigEndian.PutUint64(c, math.Float64bits(v))
		} else {
			binary.LittleEndian.PutUint64(c, math.Float64bits(v))
		}
		t := &DICOMTag{Group: tag.Group, Element: tag.Element, Length: 8, VR: tag.VR, Data: c, BigEndian: obj.BigEndian}
		FillTag(t)
		obj.upsertTag(t)
	case DateRange:
		obj.upsertTag(NewStringTag(tag.Group, tag.Element, tag.VR,
			fmt.Sprintf("%s-%s", v.Start.Format("20060102"), v.End.Format("20060102"))))
	}
}

func (obj *dicomObject) WriteString(tag *tags.Tag, value string)       { obj.Write(tag, value) }
func (obj *dicomObject) WriteUint16(tag *tags.Tag, value uint16)       { obj.Write(tag, value) }
func (obj *dicomObject) WriteUint32(tag *tags.Tag, value uint32)       { obj.Write(tag, value) }
func (obj *dicomObject) WriteFloat32(tag *tags.Tag, value float32)     { obj.Write(tag, value) }
func (obj *dicomObject) WriteFloat64(tag *tags.Tag, value float64)     { obj.Write(tag, value) }
func (obj *dicomObject) WriteTime(tag *tags.Tag, value time.Time)      { obj.Write(tag, value) }
func (obj *dicomObject) WriteDateRange(tag *tags.Tag, value DateRange) { obj.Write(tag, value) }

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

// Clone returns a deep copy of this DICOMObject. Each tag's data bytes are
// copied independently, so mutations to either object's tag data do not affect
// the other. The tagIndex is not copied and will be rebuilt lazily on first use.
func (obj *dicomObject) Clone() DICOMObject {
	cloned := &dicomObject{
		Tags:           make([]*DICOMTag, len(obj.Tags)),
		tagIndex:       nil,
		TransferSyntax: obj.TransferSyntax,
		ExplicitVR:     obj.ExplicitVR,
		BigEndian:      obj.BigEndian,
	}
	for i, t := range obj.Tags {
		cp := *t
		if t.Data != nil {
			cp.Data = make([]byte, len(t.Data))
			copy(cp.Data, t.Data)
		}
		cloned.Tags[i] = &cp
	}
	return cloned
}

// Merge copies tags from src that are not already present in obj.
// Tags already present in the receiver (matched by group+element) are left
// unchanged. Each added tag's data is deep-copied so the two objects remain
// independent after the merge.
func (obj *dicomObject) Merge(src DICOMObject) {
	if src == nil {
		return
	}
	obj.ensureTagIndex()
	for _, t := range src.GetTags() {
		k := tagKey(t.Group, t.Element)
		if _, exists := obj.tagIndex[k]; exists {
			continue
		}
		cp := *t
		if t.Data != nil {
			cp.Data = make([]byte, len(t.Data))
			copy(cp.Data, t.Data)
		}
		obj.Add(&cp)
	}
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
	return bytes.Join(fragments, nil)
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

// compressedFrameBytes returns the raw encoded bytes for frameIndex from the
// encapsulated pixel data sequence whose parent tag is at obj.Tags[pixelTagIdx].
// It is the shared implementation for GetPixelData and GetDecompressedFrame.
func (obj *dicomObject) compressedFrameBytes(pixelTagIdx int, frames uint32, frameIndex int) ([]byte, error) {
	if pixelTagIdx+1 >= len(obj.Tags) {
		return nil, errors.New("missing basic offset table")
	}
	botItem := obj.GetTagAt(pixelTagIdx + 1)
	if botItem == nil || botItem.Group != 0xFFFE || botItem.Element != 0xE000 {
		return nil, errors.New("invalid encapsulated pixel data layout")
	}

	var fragments [][]byte
	var payloadSizes []int
	for tagIdx := pixelTagIdx + 2; tagIdx < len(obj.Tags); tagIdx++ {
		t := obj.GetTagAt(tagIdx)
		if t == nil {
			continue
		}
		if t.Group == 0xFFFE && t.Element == 0xE0DD {
			break
		}
		if t.Group == 0xFFFE && t.Element == 0xE000 {
			fragments = append(fragments, t.Data)
			payloadSizes = append(payloadSizes, len(t.Data))
		}
	}

	if len(fragments) == 0 {
		return nil, fmt.Errorf("frame %d out of range", frameIndex)
	}
	if frames <= 1 {
		return joinFragments(fragments), nil
	}
	if len(fragments) == int(frames) {
		out := make([]byte, len(fragments[frameIndex]))
		copy(out, fragments[frameIndex])
		return out, nil
	}
	botOffsets := parseBasicOffsetTable(botItem.Data)
	if len(botOffsets) >= int(frames) {
		start, end, ok := frameFragmentRangeByBOT(botOffsets[:frames], frameIndex, payloadSizes)
		if ok {
			return joinFragments(fragments[start:end]), nil
		}
	}
	if frameIndex == 0 {
		return joinFragments(fragments), nil
	}
	return nil, fmt.Errorf("frame %d out of range", frameIndex)
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
		return obj.compressedFrameBytes(i, frames, frame)
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
	return codecs.DecompressFrame(ctx, tsUID, compressed, bitsa, photoInt, out)
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
		compressedFrame, err := obj.compressedFrameBytes(i, frames, frameIndex)
		if err != nil {
			return nil, err
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
