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

	"github.com/innovative-io/io-dicom/codecs/jpeg"
	"github.com/innovative-io/io-dicom/codecs/jpeg2000"
	"github.com/innovative-io/io-dicom/codecs/jpegls"
	"github.com/innovative-io/io-dicom/codecs/jpegxl"
	"github.com/innovative-io/io-dicom/codecs/jpip"
	"github.com/innovative-io/io-dicom/codecs/mpeg"
	"github.com/innovative-io/io-dicom/codecs/smpte2110"
	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/transcoder"
)

// maxPixelDataBytes is the upper bound for a single pixel data allocation (512 MiB).
// It prevents unbounded allocations from malformed or malicious DICOM input.
const maxPixelDataBytes = 512 * 1024 * 1024

// DICOMObject - DICOM Object structure
type DICOMObject interface {
	Add(tag *DICOMTag)
	AddConceptNameSeq(group uint16, element uint16, CodeValue string, CodeMeaning string)
	AddSRText(text string)
	DumpTags(w io.Writer)
	IsExplicitVR() bool
	SetExplicitVR(explicit bool)
	IsBigEndian() bool
	SetBigEndian(bigEndian bool)
	GetDate(tag *tags.Tag) time.Time
	GetPixelData(frame int) ([]byte, error)
	GetTagAt(i int) *DICOMTag
	GetTag(tag *tags.Tag) *DICOMTag
	GetTagGE(group uint16, element uint16) *DICOMTag
	SetTag(i int, tag *DICOMTag)
	InsertTag(i int, tag *DICOMTag)
	DelTag(i int)
	GetTags() []*DICOMTag
	GetUShort(tag *tags.Tag) uint16
	GetUInt(tag *tags.Tag) uint32
	GetString(tag *tags.Tag) string
	GetUShortGE(group uint16, element uint16) uint16
	GetUIntGE(group uint16, element uint16) uint32
	GetStringGE(group uint16, element uint16) string
	WriteDate(tag *tags.Tag, date time.Time)
	WriteDateRange(tag *tags.Tag, startDate time.Time, endDate time.Time)
	WriteTime(tag *tags.Tag, date time.Time)
	WriteUint16(tag *tags.Tag, val uint16)
	WriteUint32(tag *tags.Tag, val uint32)
	WriteString(tag *tags.Tag, content string)
	WriteUint16GE(group uint16, element uint16, vr string, val uint16)
	WriteUint32GE(group uint16, element uint16, vr string, val uint32)
	WriteStringGE(group uint16, element uint16, vr string, content string)
	GetTransferSyntax() *transfersyntax.TransferSyntax
	SetTransferSyntax(ts *transfersyntax.TransferSyntax)
	ChangeTransferSyntax(ts *transfersyntax.TransferSyntax) error
	ChangeTransferSyntaxContext(ctx context.Context, ts *transfersyntax.TransferSyntax) error
	TagCount() int
	CreateSR(study DICOMStudy, SeriesInstanceUID string, SOPInstanceUID string)
	CreatePDF(study DICOMStudy, SeriesInstanceUID string, SOPInstanceUID string, fileName string)
	WriteToBytes() []byte
	WriteToFile(fileName string) error
}

type dicomObject struct {
	Tags           []*DICOMTag
	tagIndex       map[uint32]*DICOMTag
	TransferSyntax *transfersyntax.TransferSyntax
	ExplicitVR     bool
	BigEndian      bool
	SQtag          *DICOMTag
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
		tagIndex:       make(map[uint32]*DICOMTag),
		TransferSyntax: nil,
		ExplicitVR:     false,
		BigEndian:      false,
		SQtag:          &DICOMTag{},
	}
}

// NewDCMObjFromFile - Read from a DICOM file into a DICOM Object
func NewDCMObjFromFile(fileName string) (DICOMObject, error) {
	bufdata, err := NewBufDataFromFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("media: NewDCMObjFromFile %q: %w", fileName, err)
	}

	return parseBufData(bufdata)
}

// NewDCMObjFromBytes - Read from a DICOM bytes into a DICOM Object
func NewDCMObjFromBytes(data []byte) (DICOMObject, error) {
	return parseBufData(NewBufDataFromBytes(data))
}

func parseBufData(bufdata DICOMBuffer) (DICOMObject, error) {
	isBigEndian := false

	transferSyntax, err := bufdata.ReadMeta()
	if err != nil {
		return nil, err
	}

	dicomObj := &dicomObject{
		Tags:           make([]*DICOMTag, 0),
		tagIndex:       make(map[uint32]*DICOMTag),
		TransferSyntax: transferSyntax,
		ExplicitVR:     false,
		BigEndian:      false,
		SQtag:          &DICOMTag{},
	}

	if dicomObj.TransferSyntax == nil {
		return nil, fmt.Errorf("unable to read transfer syntax from data")
	}

	if dicomObj.TransferSyntax == transfersyntax.ImplicitVRLittleEndian {
		dicomObj.ExplicitVR = false
	} else {
		dicomObj.ExplicitVR = true
	}
	if dicomObj.TransferSyntax == transfersyntax.ExplicitVRBigEndian {
		isBigEndian = true
	}
	bufdata.SetBigEndian(isBigEndian)

	if dicomObj.TransferSyntax == transfersyntax.DeflatedExplicitVRLittleEndian {
		remaining := bufdata.GetSize() - bufdata.GetPosition()
		if remaining <= 0 {
			return dicomObj, nil
		}

		deflatedData, err := bufdata.Read(remaining)
		if err != nil {
			return nil, err
		}

		inflatedData, err := transcoder.InflateFrame(deflatedData, -1)
		if err != nil {
			return nil, err
		}

		inflatedBuf := NewBufDataFromBytes(inflatedData)
		inflatedBuf.SetBigEndian(false)
		if err := inflatedBuf.ReadObj(dicomObj); err != nil {
			return nil, err
		}
		return dicomObj, nil
	}

	if err := bufdata.ReadObj(dicomObj); err != nil {
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

func (obj *dicomObject) GetTagGE(group uint16, element uint16) *DICOMTag {
	obj.ensureTagIndex()
	return obj.tagIndex[tagKey(group, element)]
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
			return fmt.Sprintf("%d", tag.GetUShort())
		}
		return "(invalid)"
	case "UL":
		if len(tag.Data) >= 4 {
			return fmt.Sprintf("%d", tag.GetUInt())
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
	case "OB", "OW", "UN":
		return fmt.Sprintf("(%d bytes)", tag.Length)
	default:
		return tag.GetString()
	}
}

func (obj *dicomObject) GetDate(tag *tags.Tag) time.Time {
	date := obj.GetString(tag)
	data, _ := time.Parse("20060102", date)
	return data
}

func (obj *dicomObject) GetUShort(tag *tags.Tag) uint16 {
	return obj.GetUShortGE(tag.Group, tag.Element)
}

// findTagGE performs a top-level-only linear scan that skips tags nested
// inside sequences and returns the first matching tag with a non-empty,
// non-undefined-length value, or nil if none is found.
func (obj *dicomObject) findTagGE(group uint16, element uint16) *DICOMTag {
	sequenceDepth := 0
	for i := 0; i < obj.TagCount(); i++ {
		t := obj.GetTagAt(i)
		if ((t.VR == "SQ") && (t.Length == 0xFFFFFFFF)) || ((t.Group == 0xFFFE) && (t.Element == 0xE000) && (t.Length == 0xFFFFFFFF)) {
			sequenceDepth++
		}
		if (sequenceDepth == 0) && (t.Length > 0) && (t.Length != 0xFFFFFFFF) {
			if (t.Group == group) && (t.Element == element) {
				return t
			}
		}
		if ((t.Group == 0xFFFE) && (t.Element == 0xE00D)) || ((t.Group == 0xFFFE) && (t.Element == 0xE0DD)) {
			sequenceDepth--
		}
	}
	return nil
}

// GetUShortGE - return the Uint16 for this group & element
func (obj *dicomObject) GetUShortGE(group uint16, element uint16) uint16 {
	if t := obj.findTagGE(group, element); t != nil {
		return t.GetUShort()
	}
	return 0
}

func (obj *dicomObject) GetUInt(tag *tags.Tag) uint32 {
	return obj.GetUIntGE(tag.Group, tag.Element)
}

// GetUIntGE - return the Uint32 for this group & element
func (obj *dicomObject) GetUIntGE(group uint16, element uint16) uint32 {
	if t := obj.findTagGE(group, element); t != nil {
		return t.GetUInt()
	}
	return 0
}

func (obj *dicomObject) GetString(tag *tags.Tag) string {
	return obj.GetStringGE(tag.Group, tag.Element)
}

// GetStringGE - return the String for this group & element
func (obj *dicomObject) GetStringGE(group uint16, element uint16) string {
	if t := obj.findTagGE(group, element); t != nil {
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
	bufdata := NewEmptyBufData()

	if obj.TransferSyntax.UID == transfersyntax.ExplicitVRBigEndian.UID {
		bufdata.SetBigEndian(true)
	}
	SOPClassUID := obj.GetStringGE(0x08, 0x16)
	SOPInstanceUID := obj.GetStringGE(0x08, 0x18)
	bufdata.WriteMeta(SOPClassUID, SOPInstanceUID, obj.TransferSyntax.UID)
	if obj.TransferSyntax.UID == transfersyntax.DeflatedExplicitVRLittleEndian.UID {
		rawDataset := NewEmptyBufData()
		rawDataset.WriteObj(obj)
		compressed, err := transcoder.DeflateFrame(rawDataset.GetAllBytes())
		if err == nil {
			bufdata.Write(compressed, len(compressed))
		} else {
			bufdata.WriteObj(obj)
		}
	} else {
		bufdata.WriteObj(obj)
	}
	bufdata.SetPosition(0)
	return bufdata.GetAllBytes()
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
	if err := ValidateFileWrite(obj); err != nil {
		return err
	}
	data := obj.WriteToBytes()
	return os.WriteFile(fileName, data, 0o600)
}

func (obj *dicomObject) WriteDate(tag *tags.Tag, date time.Time) {
	obj.WriteString(tag, date.Format("20060102"))
}

func (obj *dicomObject) WriteDateRange(tag *tags.Tag, startDate time.Time, endDate time.Time) {
	obj.WriteString(tag, fmt.Sprintf("%s-%s", startDate.Format("20060102"), endDate.Format("20060102")))
}

func (obj *dicomObject) WriteTime(tag *tags.Tag, date time.Time) {
	obj.WriteString(tag, date.Format("150405"))
}

func (obj *dicomObject) WriteUint16(tag *tags.Tag, val uint16) {
	obj.WriteUint16GE(tag.Group, tag.Element, tag.VR, val)
}

func (obj *dicomObject) WriteUint32(tag *tags.Tag, val uint32) {
	obj.WriteUint32GE(tag.Group, tag.Element, tag.VR, val)
}

func (obj *dicomObject) WriteString(tag *tags.Tag, content string) {
	obj.WriteStringGE(tag.Group, tag.Element, tag.VR, content)
}

// WriteUint16GE - Writes a Uint16 to a DICOM tag
func (obj *dicomObject) WriteUint16GE(group uint16, element uint16, vr string, val uint16) {
	c := make([]byte, 2)
	if obj.BigEndian {
		binary.BigEndian.PutUint16(c, val)
	} else {
		binary.LittleEndian.PutUint16(c, val)
	}

	tag := &DICOMTag{
		Group:     group,
		Element:   element,
		Length:    2,
		VR:        vr,
		Data:      c,
		BigEndian: obj.BigEndian,
	}
	FillTag(tag)
	obj.Tags = append(obj.Tags, tag)
	if obj.tagIndex != nil {
		k := tagKey(tag.Group, tag.Element)
		if _, exists := obj.tagIndex[k]; !exists {
			obj.tagIndex[k] = tag
		}
	}
}

// WriteUint32GE - Writes a Uint32 to a DICOM tag
func (obj *dicomObject) WriteUint32GE(group uint16, element uint16, vr string, val uint32) {
	c := make([]byte, 4)
	if obj.BigEndian {
		binary.BigEndian.PutUint32(c, val)
	} else {
		binary.LittleEndian.PutUint32(c, val)
	}

	tag := &DICOMTag{
		Group:     group,
		Element:   element,
		Length:    4,
		VR:        vr,
		Data:      c,
		BigEndian: obj.BigEndian,
	}
	FillTag(tag)
	obj.Tags = append(obj.Tags, tag)
	if obj.tagIndex != nil {
		k := tagKey(tag.Group, tag.Element)
		if _, exists := obj.tagIndex[k]; !exists {
			obj.tagIndex[k] = tag
		}
	}
}

// WriteStringGE - Writes a String to a DICOM tag
func (obj *dicomObject) WriteStringGE(group uint16, element uint16, vr string, content string) {
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
	tag := &DICOMTag{
		Group:     group,
		Element:   element,
		Length:    uint32(length),
		VR:        vr,
		Data:      data,
		BigEndian: false,
	}
	FillTag(tag)
	obj.Tags = append(obj.Tags, tag)
	if obj.tagIndex != nil {
		k := tagKey(tag.Group, tag.Element)
		if _, exists := obj.tagIndex[k]; !exists {
			obj.tagIndex[k] = tag
		}
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

func (obj *dicomObject) GetPixelData(frame int) ([]byte, error) {
	var i int
	var rows, cols, bitsa, planar uint16
	var PhotoInt string
	sq := 0
	frames := uint32(0)
	RGB := false
	icon := false

	if !transfersyntax.SupportedTransferSyntax(obj.TransferSyntax.UID) {
		return nil, fmt.Errorf("unsupported transfer syntax %s", obj.TransferSyntax.Name)
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
					planar = tag.GetUShort()
				case 0x08:
					uframes, err := strconv.Atoi(tag.GetString())
					if err != nil {
						frames = 0
					} else {
						frames = uint32(uframes)
					}
				case 0x10:
					rows = tag.GetUShort()
				case 0x11:
					cols = tag.GetUShort()
				case 0x0100:
					bitsa = tag.GetUShort()
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
				if frames > 0 {
					sizePx *= uint64(frames)
				} else {
					frames = 1
				}
				if sizePx == 0 || sizePx > uint64(maxPixelDataBytes) {
					return nil, fmt.Errorf("DICOMObject::GetPixelData, invalid pixel data size %d", sizePx)
				}
				size := uint32(sizePx)

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
				} else {
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
					} else {
						imgSize := size / frames
						offset := uint32(frame) * imgSize
						if offset+imgSize > uint32(len(tag.Data)) {
							return nil, fmt.Errorf("frame %d out of range", frame)
						}
						out := make([]byte, imgSize)
						copy(out, tag.Data[offset:offset+imgSize])
						return out, nil
					}
				}
			}
		}
		if ((tag.Group == 0xFFFE) && (tag.Element == 0xE00D)) || ((tag.Group == 0xFFFE) && (tag.Element == 0xE0DD)) {
			sq--
		}
	}
	return nil, fmt.Errorf("there was an error getting pixel data")
}

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
					planar = tag.GetUShort()
				case 0x08:
					uframes, err := strconv.Atoi(tag.GetString())
					if err != nil {
						frames = 0
					} else {
						frames = uint32(uframes)
					}
				case 0x10:
					rows = tag.GetUShort()
				case 0x11:
					cols = tag.GetUShort()
				case 0x0100:
					bitsa = tag.GetUShort()
				case 0x0101:
					bitss = tag.GetUShort()
				case 0x0103:
					pixelrep = tag.GetUShort()
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
	return fmt.Errorf("there was an error changing the transfer syntax")
}

// newChildObject allocates a dicomObject that inherits the byte-order and VR
// encoding of its parent. Used for building nested SQ item/sequence pairs.
func newChildObject(parent *dicomObject) *dicomObject {
	return &dicomObject{
		Tags:       make([]*DICOMTag, 0),
		ExplicitVR: parent.ExplicitVR,
		BigEndian:  parent.BigEndian,
		SQtag:      new(DICOMTag),
	}
}

// AddConceptNameSeq - Concept Name Sequence for DICOM SR
func (obj *dicomObject) AddConceptNameSeq(group uint16, element uint16, CodeValue string, CodeMeaning string) {
	item := newChildObject(obj)
	seq := newChildObject(obj)
	tag := new(DICOMTag)

	item.WriteString(tags.CodeValue, CodeValue)
	item.WriteString(tags.CodingSchemeDesignator, "odb")
	item.WriteString(tags.CodeMeaning, CodeMeaning)
	tag.WriteSeq(0xFFFE, 0xE000, item)
	seq.Add(tag)
	tag.WriteSeq(group, element, seq)
	obj.Add(tag)
}

// AddSRText - add Text to SR
func (obj *dicomObject) AddSRText(text string) {
	item := newChildObject(obj)
	seq := newChildObject(obj)
	tag := new(DICOMTag)

	item.WriteString(tags.RelationshipType, "CONTAINS")
	item.WriteString(tags.ValueType, "TEXT")
	item.AddConceptNameSeq(0x40, 0xA043, "2222", "Report Text")
	item.WriteString(tags.TextValue, text)
	tag.WriteSeq(0xFFFE, 0xE000, item)
	seq.Add(tag)
	tag.WriteSeq(0x40, 0xA730, seq)
	obj.Add(tag)
}

// CreateSR - Create a DICOM SR object
func (obj *dicomObject) CreateSR(study DICOMStudy, SeriesInstanceUID string, SOPInstanceUID string) {
	obj.WriteString(tags.InstanceCreationDate, time.Now().Format("20060102"))
	obj.WriteString(tags.InstanceCreationTime, time.Now().Format("150405"))
	obj.WriteString(tags.SOPClassUID, sopclass.BasicTextSRStorage.UID)
	obj.WriteString(tags.SOPInstanceUID, SOPInstanceUID)
	obj.WriteString(tags.AccessionNumber, study.AccessionNumber)
	obj.WriteString(tags.Modality, "SR")
	obj.WriteString(tags.InstitutionName, study.InstitutionName)
	obj.WriteString(tags.ReferringPhysicianName, study.ReferringPhysician)
	obj.WriteString(tags.StudyDescription, study.Description)
	obj.WriteString(tags.SeriesDescription, "REPORT")
	obj.WriteString(tags.PatientName, study.PatientName)
	obj.WriteString(tags.PatientID, study.PatientID)
	obj.WriteString(tags.PatientBirthDate, study.PatientBirthDate)
	obj.WriteString(tags.PatientSex, study.PatientSex)
	obj.WriteString(tags.StudyInstanceUID, study.StudyInstanceUID)
	obj.WriteString(tags.SeriesInstanceUID, SeriesInstanceUID)
	obj.WriteString(tags.SeriesNumber, "200")
	obj.WriteString(tags.InstanceNumber, "1")
	obj.WriteString(tags.ValueType, "CONTAINER")
	obj.AddConceptNameSeq(0x0040, 0xA043, "1111", "Radiology Report")
	obj.WriteString(tags.ContinuityOfContent, "SEPARATE")
	obj.WriteString(tags.VerifyingObserverName, study.ObserverName)
	obj.WriteString(tags.CompletionFlag, "COMPLETE")
	obj.WriteString(tags.VerificationFlag, "VERIFIED")
	obj.AddSRText(study.ReportText)
}

// CreatePDF - Create a DICOM SR object
func (obj *dicomObject) CreatePDF(study DICOMStudy, SeriesInstanceUID string, SOPInstanceUID string, fileName string) {
	obj.WriteString(tags.InstanceCreationDate, time.Now().Format("20060102"))
	obj.WriteString(tags.InstanceCreationTime, time.Now().Format("150405"))
	obj.WriteString(tags.SOPClassUID, sopclass.EncapsulatedPDFStorage.UID)
	obj.WriteString(tags.SOPInstanceUID, SOPInstanceUID)
	obj.WriteString(tags.AccessionNumber, study.AccessionNumber)
	obj.WriteString(tags.Modality, "OT")
	obj.WriteString(tags.InstitutionName, study.InstitutionName)
	obj.WriteString(tags.ReferringPhysicianName, study.ReferringPhysician)
	obj.WriteString(tags.StudyDescription, study.Description)
	obj.WriteString(tags.PatientName, study.PatientName)
	obj.WriteString(tags.PatientID, study.PatientID)
	obj.WriteString(tags.PatientBirthDate, study.PatientBirthDate)
	obj.WriteString(tags.PatientSex, study.PatientSex)
	obj.WriteString(tags.StudyInstanceUID, study.StudyInstanceUID)
	obj.WriteString(tags.SeriesInstanceUID, SeriesInstanceUID)
	obj.WriteString(tags.SeriesNumber, "300")
	obj.WriteString(tags.InstanceNumber, "1")

	mstream, _ := NewMemoryStreamFromFile(fileName)

	mstream.SetPosition(0)
	size := uint32(mstream.GetSize())
	if size%2 == 1 {
		size++
		mstream.Append([]byte{0x00})
	}
	obj.WriteString(tags.DocumentTitle, fileName)
	obj.Add(&DICOMTag{
		Group:     0x42,
		Element:   0x11,
		Length:    size,
		VR:        "OB",
		Data:      mstream.GetData(),
		BigEndian: obj.BigEndian,
	})
	obj.WriteString(tags.MIMETypeOfEncapsulatedDocument, "application/pdf")
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
			deflated, err := transcoder.DeflateFrame(img[offset : offset+frameSize])
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
			rleData, err := transcoder.RLEencode(img[offset:offset+frameSize], rows, cols, bitsa, samplesPerPixel)
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
			inflated, err := transcoder.InflateFrame(tag.Data, int(single))
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
			if err := transcoder.RLEdecode(tag.Data, img[offset:], tag.Length, single, PhotoInt); err != nil {
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
