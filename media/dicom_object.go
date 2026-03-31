package media

import (
	"encoding/binary"
	"errors"
	"fmt"
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
	DumpTags()
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
	TagCount() int
	CreateSR(study DICOMStudy, SeriesInstanceUID string, SOPInstanceUID string)
	CreatePDF(study DICOMStudy, SeriesInstanceUID string, SOPInstanceUID string, fileName string)
	WriteToBytes() []byte
	WriteToFile(fileName string) error
	dumpSeq(indent int)
	compress(i *int, img []byte, RGB bool, cols uint16, rows uint16, bitss uint16, bitsa uint16, pixelrep uint16, planar uint16, frames uint32, outTS string) error
	uncompress(i int, img []byte, size uint32, frames uint32, bitsa uint16, PhotoInt string) error
}

type dicomObject struct {
	Tags           []*DICOMTag
	TransferSyntax *transfersyntax.TransferSyntax
	ExplicitVR     bool
	BigEndian      bool
	SQtag          *DICOMTag
}

// NewEmptyDCMObj - Create as an interface to a new empty dicomObject
func NewEmptyDCMObj() DICOMObject {
	return &dicomObject{
		Tags:           make([]*DICOMTag, 0),
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
		if os.IsNotExist(err) {
			return nil, errors.New("DICOMObject::Read, file does not exist")
		}
		return nil, err
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
	for _, currentTag := range obj.Tags {
		if currentTag.Group == dictTag.Group && currentTag.Element == dictTag.Element {
			return currentTag
		}
	}
	return nil
}

func (obj *dicomObject) GetTagGE(group uint16, element uint16) *DICOMTag {
	for _, currentTag := range obj.Tags {
		if currentTag.Group == group && currentTag.Element == element {
			return currentTag
		}
	}
	return nil
}

func (obj *dicomObject) SetTag(index int, tag *DICOMTag) {
	FillTag(tag)
	if index >= 0 && index < obj.TagCount() {
		obj.Tags[index] = tag
	}
}

func (obj *dicomObject) InsertTag(index int, tag *DICOMTag) {
	FillTag(tag)
	if index < 0 || index > len(obj.Tags) {
		return
	}
	obj.Tags = append(obj.Tags[:index+1], obj.Tags[index:]...)
	obj.Tags[index] = tag
}

func (obj *dicomObject) GetTags() []*DICOMTag {
	return obj.Tags
}

func (obj *dicomObject) DelTag(index int) {
	obj.Tags = append(obj.Tags[:index], obj.Tags[index+1:]...)
}

func (obj *dicomObject) DumpTags() {
	for _, tag := range obj.Tags {
		if tag.VR == "SQ" {
			fmt.Printf("\t(%04X,%04X) %s - %s\n", tag.Group, tag.Element, tag.VR, tag.Description)
			seq := tag.ReadSeq(obj.IsExplicitVR())
			seq.dumpSeq(1)
			continue
		}
		if tag.Length > 128 {
			fmt.Printf("\t(%04X,%04X) %s - %s : (Not displayed)\n", tag.Group, tag.Element, tag.VR, tag.Description)
			continue
		}
		switch tag.VR {
		case "US":
			if len(tag.Data) >= 2 {
				fmt.Printf("\t(%04X,%04X) %s - %s : %d\n", tag.Group, tag.Element, tag.VR, tag.Description, binary.LittleEndian.Uint16(tag.Data))
			} else {
				fmt.Printf("\t(%04X,%04X) %s - %s : (invalid)\n", tag.Group, tag.Element, tag.VR, tag.Description)
			}
		default:
			fmt.Printf("\t(%04X,%04X) %s - %s : %s\n", tag.Group, tag.Element, tag.VR, tag.Description, tag.Data)
		}
	}
	fmt.Println()
}

func (obj *dicomObject) dumpSeq(indent int) {
	indentTabs := "\t"
	for level := 0; level < indent; level++ {
		indentTabs += "\t"
	}

	for _, tag := range obj.Tags {
		if tag.VR == "SQ" {
			fmt.Printf("%s(%04X,%04X) %s - %s\n", indentTabs, tag.Group, tag.Element, tag.VR, tag.Description)
			seq := tag.ReadSeq(obj.IsExplicitVR())
			seq.dumpSeq(indent + 1)
			continue
		}
		if tag.Length > 128 {
			fmt.Printf("%s(%04X,%04X) %s - %s : (Not displayed)\n", indentTabs, tag.Group, tag.Element, tag.VR, tag.Description)
			continue
		}
		switch tag.VR {
		case "US":
			if len(tag.Data) >= 2 {
				fmt.Printf("%s(%04X,%04X) %s - %s : %d\n", indentTabs, tag.Group, tag.Element, tag.VR, tag.Description, binary.LittleEndian.Uint16(tag.Data))
			} else {
				fmt.Printf("%s(%04X,%04X) %s - %s : (invalid)\n", indentTabs, tag.Group, tag.Element, tag.VR, tag.Description)
			}
		default:
			fmt.Printf("%s(%04X,%04X) %s - %s : %s\n", indentTabs, tag.Group, tag.Element, tag.VR, tag.Description, tag.Data)
		}
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

// GetUShortGE - return the Uint16 for this group & element
func (obj *dicomObject) GetUShortGE(group uint16, element uint16) uint16 {
	var index int
	var currentTag *DICOMTag
	sequenceDepth := 0
	for index = 0; index < obj.TagCount(); index++ {
		currentTag = obj.GetTagAt(index)
		if ((currentTag.VR == "SQ") && (currentTag.Length == 0xFFFFFFFF)) || ((currentTag.Group == 0xFFFE) && (currentTag.Element == 0xE000) && (currentTag.Length == 0xFFFFFFFF)) {
			sequenceDepth++
		}
		if (sequenceDepth == 0) && (currentTag.Length > 0) && (currentTag.Length != 0xFFFFFFFF) {
			if (currentTag.Group == group) && (currentTag.Element == element) {
				break
			}
		}
		if ((currentTag.Group == 0xFFFE) && (currentTag.Element == 0xE00D)) || ((currentTag.Group == 0xFFFE) && (currentTag.Element == 0xE0DD)) {
			sequenceDepth--
		}
	}
	if index < obj.TagCount() {
		return currentTag.GetUShort()
	}
	return 0
}

func (obj *dicomObject) GetUInt(tag *tags.Tag) uint32 {
	return obj.GetUIntGE(tag.Group, tag.Element)
}

// GetUIntGE - return the Uint32 for this group & element
func (obj *dicomObject) GetUIntGE(group uint16, element uint16) uint32 {
	var index int
	var currentTag *DICOMTag
	sequenceDepth := 0
	for index = 0; index < obj.TagCount(); index++ {
		currentTag = obj.GetTagAt(index)
		if ((currentTag.VR == "SQ") && (currentTag.Length == 0xFFFFFFFF)) || ((currentTag.Group == 0xFFFE) && (currentTag.Element == 0xE000) && (currentTag.Length == 0xFFFFFFFF)) {
			sequenceDepth++
		}
		if (sequenceDepth == 0) && (currentTag.Length > 0) && (currentTag.Length != 0xFFFFFFFF) {
			if (currentTag.Group == group) && (currentTag.Element == element) {
				break
			}
		}
		if ((currentTag.Group == 0xFFFE) && (currentTag.Element == 0xE00D)) || ((currentTag.Group == 0xFFFE) && (currentTag.Element == 0xE0DD)) {
			sequenceDepth--
		}
	}
	if index < obj.TagCount() {
		return currentTag.GetUInt()
	}
	return 0
}

func (obj *dicomObject) GetString(tag *tags.Tag) string {
	return obj.GetStringGE(tag.Group, tag.Element)
}

// GetStringGE - return the String for this group & element
func (obj *dicomObject) GetStringGE(group uint16, element uint16) string {
	var index int
	var currentTag *DICOMTag
	sequenceDepth := 0
	for index = 0; index < obj.TagCount(); index++ {
		currentTag = obj.GetTagAt(index)
		if ((currentTag.VR == "SQ") && (currentTag.Length == 0xFFFFFFFF)) || ((currentTag.Group == 0xFFFE) && (currentTag.Element == 0xE000) && (currentTag.Length == 0xFFFFFFFF)) {
			sequenceDepth++
		}
		if (sequenceDepth == 0) && (currentTag.Length > 0) && (currentTag.Length != 0xFFFFFFFF) {
			if (currentTag.Group == group) && (currentTag.Element == element) {
				break
			}
		}
		if ((currentTag.Group == 0xFFFE) && (currentTag.Element == 0xE00D)) || ((currentTag.Group == 0xFFFE) && (currentTag.Element == 0xE0DD)) {
			sequenceDepth--
		}
	}
	if index < obj.TagCount() {
		return currentTag.GetString()
	}
	return ""
}

// Add - add a new DICOM Tag to a DICOM Object
func (obj *dicomObject) Add(tag *DICOMTag) {
	obj.Tags = append(obj.Tags, tag)
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

// Wrote - Write a DICOM Object to a DICOM File
func (obj *dicomObject) WriteToFile(fileName string) error {
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
}

func (obj *dicomObject) GetTransferSyntax() *transfersyntax.TransferSyntax {
	return obj.TransferSyntax
}

func (obj *dicomObject) SetTransferSyntax(ts *transfersyntax.TransferSyntax) {
	obj.TransferSyntax = ts
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
					tagIdx := i + 2 + frame
					if tagIdx >= len(obj.Tags) {
						return nil, fmt.Errorf("frame %d out of range", frame)
					}
					t := obj.GetTagAt(tagIdx)
					if t == nil {
						return nil, fmt.Errorf("frame %d pixel item is nil", frame)
					}
					out := make([]byte, len(t.Data))
					copy(out, t.Data)
					return out, nil
				} else {
					if RGB && (planar == 1) {
						var img_offset, img_size uint32
						img_size = size / frames
						img := make([]byte, img_size)
						for f := uint32(0); f < frames; f++ {
							img_offset = img_size * f
							for j := uint32(0); j < img_size/3; j++ {
								img[3*j] = tag.Data[j+img_offset]
								img[3*j+1] = tag.Data[j+img_size/3+img_offset]
								img[3*j+2] = tag.Data[j+2*img_size/3+img_offset]
							}
							if f == uint32(frame) {
								return img, nil
							}
						}
						planar = 0
					} else {
						out := make([]byte, len(tag.Data))
						copy(out, tag.Data)
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
					if err := obj.uncompress(i, img, size, frames, bitsa, PhotoInt); err != nil {
						return fmt.Errorf("DICOMObject::ConvertTransferSyntax, decompress failed: %w", err)
					}
				} else { // Uncompressed
					if RGB && (planar == 1) { // change from planar=1 to planar=0
						var img_offset, img_size uint32
						img_size = size / frames
						for f := uint32(0); f < frames; f++ {
							img_offset = img_size * f
							for j := uint32(0); j < img_size/3; j++ {
								img[3*j+img_offset] = tag.Data[j+img_offset]
								img[3*j+1+img_offset] = tag.Data[j+img_size/3+img_offset]
								img[3*j+2+img_offset] = tag.Data[j+2*img_size/3+img_offset]
							}
						}
						planar = 0
					} else {
						copy(img, tag.Data)
					}
				}
				if err := obj.compress(&i, img, RGB, cols, rows, bitss, bitsa, pixelrep, planar, frames, outTS.UID); err != nil {
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

// AddConceptNameSeq - Concept Name Sequence for DICOM SR
func (obj *dicomObject) AddConceptNameSeq(group uint16, element uint16, CodeValue string, CodeMeaning string) {
	item := &dicomObject{
		Tags:           make([]*DICOMTag, 0),
		TransferSyntax: nil,
		ExplicitVR:     false,
		BigEndian:      false,
		SQtag:          new(DICOMTag),
	}
	seq := &dicomObject{
		Tags:           make([]*DICOMTag, 0),
		TransferSyntax: nil,
		ExplicitVR:     false,
		BigEndian:      false,
		SQtag:          new(DICOMTag),
	}
	tag := new(DICOMTag)

	item.BigEndian = obj.BigEndian
	item.ExplicitVR = obj.ExplicitVR
	seq.BigEndian = obj.BigEndian
	seq.ExplicitVR = obj.ExplicitVR

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
	item := &dicomObject{
		Tags:           make([]*DICOMTag, 0),
		TransferSyntax: nil,
		ExplicitVR:     false,
		BigEndian:      false,
		SQtag:          new(DICOMTag),
	}
	seq := &dicomObject{
		Tags:           make([]*DICOMTag, 0),
		TransferSyntax: nil,
		ExplicitVR:     false,
		BigEndian:      false,
		SQtag:          new(DICOMTag),
	}
	tag := new(DICOMTag)

	item.BigEndian = obj.BigEndian
	item.ExplicitVR = obj.ExplicitVR
	seq.BigEndian = obj.BigEndian
	seq.ExplicitVR = obj.ExplicitVR

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

func (obj *dicomObject) compress(i *int, img []byte, RGB bool, cols uint16, rows uint16, bitss uint16, bitsa uint16, pixelrep uint16, planar uint16, frames uint32, outTS string) error {
	var offset, size, jpeg_size, j uint32
	var JPEGData []byte
	var JPEGBytes, index int

	single := uint32(cols) * uint32(rows) * uint32(bitsa) / 8
	frameSize := single
	size = frameSize * frames
	if RGB {
		frameSize = 3 * frameSize
		size = frameSize * frames
	}

	index = *i
	tag := obj.GetTagAt(index)

	switch outTS {
	case transfersyntax.DeflatedImageFrameCompression.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		tag.Data = nil
		obj.SetTag(index, tag)
		index++

		offsetTableTag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, offsetTableTag)

		for j = 0; j < frames; j++ {
			index++
			offset = j * frameSize
			deflated, err := transcoder.DeflateFrame(img[offset : offset+frameSize])
			if err != nil {
				return err
			}

			frameTag := &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(len(deflated)),
				VR:        "DL",
				Data:      deflated,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, frameTag)
		}

		index++
		sequenceEndTag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, sequenceEndTag)
		*i = index
	case transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		tag.Data = nil
		obj.SetTag(index, tag)
		index++

		offsetTableTag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, offsetTableTag)

		for j = 0; j < frames; j++ {
			index++
			offset = j * frameSize
			frameData := make([]byte, frameSize)
			copy(frameData, img[offset:offset+frameSize])

			frameTag := &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(len(frameData)),
				VR:        "DL",
				Data:      frameData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, frameTag)
		}

		index++
		sequenceEndTag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, sequenceEndTag)
		*i = index
	case transfersyntax.RLELossless.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		tag.Data = nil
		obj.SetTag(index, tag)
		index++

		offsetTableTag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, offsetTableTag)

		samplesPerPixel := uint16(1)
		if RGB {
			samplesPerPixel = 3
		}

		for j = 0; j < frames; j++ {
			index++
			offset = j * frameSize
			rleData, err := transcoder.RLEencode(img[offset:offset+frameSize], rows, cols, bitsa, samplesPerPixel)
			if err != nil {
				return err
			}

			frameTag := &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(len(rleData)),
				VR:        "DL",
				Data:      rleData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, frameTag)
		}

		index++

		sequenceEndTag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, sequenceEndTag)
		*i = index
	case transfersyntax.JPEGLosslessSV1.UID:
		fallthrough
	case transfersyntax.JPEGLossless.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		if tag.Data != nil {
			tag.Data = nil
		}
		obj.SetTag(index, tag)
		index++
		newtag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		for j = 0; j < frames; j++ {
			index++
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
				if err := jpeg.EIJG16encode(img[offset/2:], cols, rows, 1, &JPEGData, &JPEGBytes, 0); err != nil {
					return err
				}
			}
			newtag = &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(JPEGBytes),
				VR:        "DL",
				Data:      JPEGData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, newtag)
			JPEGData = nil
		}
		index++
		newtag = &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		*i = index
	case transfersyntax.JPEGBaseline8Bit.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		if tag.Data != nil {
			tag.Data = nil
		}
		obj.SetTag(index, tag)
		index++
		newtag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		jpeg_size = 0
		for j = 0; j < frames; j++ {
			index++
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
					if err := jpeg.EIJG12encode(img[offset:], cols, rows, 1, &JPEGData, &JPEGBytes, 0); err != nil {
						return err
					}
				}
			}
			newtag = &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(JPEGBytes),
				VR:        "DL",
				Data:      JPEGData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, newtag)
			JPEGData = nil
			jpeg_size = jpeg_size + uint32(JPEGBytes)
		}
		index++
		newtag = &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		*i = index
	case transfersyntax.JPEGExtended12Bit.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		if tag.Data != nil {
			tag.Data = nil
		}
		obj.SetTag(index, tag)
		index++
		newtag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		jpeg_size = 0
		for j = 0; j < frames; j++ {
			index++
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if err := jpeg.EIJG12encode(img[offset/2:], cols, rows, 1, &JPEGData, &JPEGBytes, 0); err != nil {
				return err
			}
			newtag = &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(JPEGBytes),
				VR:        "DL",
				Data:      JPEGData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, newtag)
			JPEGData = nil
			jpeg_size = jpeg_size + uint32(JPEGBytes)
		}
		index++
		newtag = &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		*i = index
	case transfersyntax.JPEGLSLossless.UID:
		fallthrough
	case transfersyntax.JPEGLSNearLossless.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		if tag.Data != nil {
			tag.Data = nil
		}
		obj.SetTag(index, tag)
		index++
		newtag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		for j = 0; j < frames; j++ {
			index++
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpegls.JLSencode(img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, outTS == transfersyntax.JPEGLSNearLossless.UID); err != nil {
					return err
				}
			} else {
				if err := jpegls.JLSencode(img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, outTS == transfersyntax.JPEGLSNearLossless.UID); err != nil {
					return err
				}
			}
			newtag = &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(JPEGBytes),
				VR:        "DL",
				Data:      JPEGData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, newtag)
			JPEGData = nil
		}
		index++
		newtag = &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		*i = index
	case transfersyntax.JPEG2000Lossless.UID:
		fallthrough
	case transfersyntax.JPEG2000MCLossless.UID:
		fallthrough
	case transfersyntax.HTJ2KLossless.UID:
		fallthrough
	case transfersyntax.HTJ2KLosslessRPCL.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		if tag.Data != nil {
			tag.Data = nil
		}
		obj.SetTag(index, tag)
		index++
		newtag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		for j = 0; j < frames; j++ {
			index++
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpeg2000.J2Kencode(img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, 0); err != nil {
					return err
				}
			} else {
				if err := jpeg2000.J2Kencode(img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, 0); err != nil {
					return err
				}
			}
			newtag = &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(JPEGBytes),
				VR:        "DL",
				Data:      JPEGData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, newtag)
			JPEGData = nil
		}
		index++
		newtag = &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		*i = index
	case transfersyntax.JPEG2000.UID:
		fallthrough
	case transfersyntax.JPEG2000MC.UID:
		fallthrough
	case transfersyntax.HTJ2K.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		if tag.Data != nil {
			tag.Data = nil
		}
		obj.SetTag(index, tag)
		index++
		newtag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		jpeg_size = 0
		for j = 0; j < frames; j++ {
			index++
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpeg2000.J2Kencode(img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, 10); err != nil {
					return err
				}
			} else {
				if err := jpeg2000.J2Kencode(img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, 10); err != nil {
					return err
				}
			}
			newtag = &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(JPEGBytes),
				VR:        "DL",
				Data:      JPEGData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, newtag)
			JPEGData = nil
			jpeg_size = jpeg_size + uint32(JPEGBytes)
		}
		index++
		newtag = &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		*i = index
	case transfersyntax.JPEGXLLossless.UID:
		fallthrough
	case transfersyntax.JPEGXLJPEGRecompression.UID:
		fallthrough
	case transfersyntax.JPEGXL.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		if tag.Data != nil {
			tag.Data = nil
		}
		obj.SetTag(index, tag)
		index++
		newtag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		for j = 0; j < frames; j++ {
			index++
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpegxl.JXLencode(img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, outTS == transfersyntax.JPEGXLLossless.UID); err != nil {
					return err
				}
			} else {
				if err := jpegxl.JXLencode(img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, outTS == transfersyntax.JPEGXLLossless.UID); err != nil {
					return err
				}
			}
			newtag = &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(JPEGBytes),
				VR:        "DL",
				Data:      JPEGData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, newtag)
			JPEGData = nil
		}
		index++
		newtag = &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		*i = index
	case transfersyntax.JPIPHTJ2KReferenced.UID:
		fallthrough
	case transfersyntax.JPIPHTJ2KReferencedDeflate.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		if tag.Data != nil {
			tag.Data = nil
		}
		obj.SetTag(index, tag)
		index++
		newtag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		for j = 0; j < frames; j++ {
			index++
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := jpip.JPIPencode(img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			} else {
				if err := jpip.JPIPencode(img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			}
			newtag = &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(JPEGBytes),
				VR:        "DL",
				Data:      JPEGData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, newtag)
			JPEGData = nil
		}
		index++
		newtag = &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
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
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		if tag.Data != nil {
			tag.Data = nil
		}
		obj.SetTag(index, tag)
		index++
		newtag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		for j = 0; j < frames; j++ {
			index++
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := mpeg.MPEGencode(img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			} else {
				if err := mpeg.MPEGencode(img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			}
			newtag = &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(JPEGBytes),
				VR:        "DL",
				Data:      JPEGData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, newtag)
			JPEGData = nil
		}
		index++
		newtag = &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		*i = index
	case transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo.UID:
		fallthrough
	case transfersyntax.SMPTEST211020UncompressedInterlacedActiveVideo.UID:
		fallthrough
	case transfersyntax.SMPTEST211030PCMDigitalAudio.UID:
		tag.VR = "OB"
		tag.Length = 0xFFFFFFFF
		if tag.Data != nil {
			tag.Data = nil
		}
		obj.SetTag(index, tag)
		index++
		newtag := &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE000,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
		for j = 0; j < frames; j++ {
			index++
			offset = j * uint32(cols) * uint32(rows) * uint32(bitsa) / 8
			if RGB {
				offset = 3 * offset
				if err := smpte2110.SMPTE2110encode(img[offset:], cols, rows, 3, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			} else {
				if err := smpte2110.SMPTE2110encode(img[offset:], cols, rows, 1, bitsa, &JPEGData, &JPEGBytes, outTS); err != nil {
					return err
				}
			}
			newtag = &DICOMTag{
				Group:     0xFFFE,
				Element:   0xE000,
				Length:    uint32(JPEGBytes),
				VR:        "DL",
				Data:      JPEGData,
				BigEndian: obj.IsBigEndian(),
			}
			obj.InsertTag(index, newtag)
			JPEGData = nil
		}
		index++
		newtag = &DICOMTag{
			Group:     0xFFFE,
			Element:   0xE0DD,
			Length:    0,
			VR:        "DL",
			Data:      nil,
			BigEndian: obj.IsBigEndian(),
		}
		obj.InsertTag(index, newtag)
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

func (obj *dicomObject) uncompress(i int, img []byte, size uint32, frames uint32, bitsa uint16, PhotoInt string) error {
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
				if err := jpeg.DIJG8decode(tag.Data, tag.Length, img[offset:], single); err != nil {
					return err
				}
			} else {
				if err := jpeg.DIJG16decode(tag.Data, tag.Length, img[offset:], single); err != nil {
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
				if err := jpeg.DIJG8decode(tag.Data, tag.Length, img[offset:], single); err != nil {
					return err
				}
			} else {
				if err := jpeg.DIJG12decode(tag.Data, tag.Length, img[offset:], single); err != nil {
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
			if err := jpeg.DIJG12decode(tag.Data, tag.Length, img[offset:], single); err != nil {
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
			if err := jpegls.JLSdecode(tag.Data, tag.Length, img[offset:]); err != nil {
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
			if err := jpeg2000.J2Kdecode(tag.Data, tag.Length, img[offset:]); err != nil {
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
			if err := jpeg2000.J2Kdecode(tag.Data, tag.Length, img[offset:]); err != nil {
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
			if err := jpegxl.JXLdecode(tag.Data, tag.Length, img[offset:]); err != nil {
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
			if err := jpip.JPIPdecode(tag.Data, tag.Length, img[offset:], obj.TransferSyntax.UID); err != nil {
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
			if err := mpeg.MPEGdecode(tag.Data, tag.Length, img[offset:], obj.TransferSyntax.UID); err != nil {
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
			if err := smpte2110.SMPTE2110decode(tag.Data, tag.Length, img[offset:], obj.TransferSyntax.UID); err != nil {
				return err
			}
			obj.DelTag(i + 1)
		}
		obj.DelTag(i + 1)
	}
	return nil
}
