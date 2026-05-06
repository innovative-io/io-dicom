package media

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
	"strings"
	"sync"
)

// tagPool recycles DICOMTag allocations in the read hot path.
// Get returns a zeroed tag; Put resets and returns it to the pool.
var tagPool = sync.Pool{
	New: func() any { return &DICOMTag{} },
}

// newTag retrieves a zeroed DICOMTag from the pool.
func newTag() *DICOMTag {
	t := tagPool.Get().(*DICOMTag)
	*t = DICOMTag{} // zero all fields
	return t
}

// releaseTag returns a tag to the pool. Must not be called if the tag
// has been added to a DICOMObject (the object owns its lifetime).
func releaseTag(t *DICOMTag) {
	tagPool.Put(t)
}

// DICOMTag DICOM tag structure
type DICOMTag struct {
	Name        string
	Description string
	Group       uint16
	Element     uint16
	Length      uint32
	VR          string
	VM          string
	Data        []byte
	BigEndian   bool
}

// GetUint16 convert tag.Data to uint16
func (tag *DICOMTag) GetUint16() uint16 {
	if tag.Length == 2 && len(tag.Data) >= 2 {
		if tag.BigEndian {
			return binary.BigEndian.Uint16(tag.Data)
		}
		return binary.LittleEndian.Uint16(tag.Data)
	}
	return 0
}

// GetUint32 convert tag.Data to uint32
func (tag *DICOMTag) GetUint32() uint32 {
	var val uint32
	if tag.Length == 4 && len(tag.Data) >= 4 {
		if tag.BigEndian {
			val = binary.BigEndian.Uint32(tag.Data)
		} else {
			val = binary.LittleEndian.Uint32(tag.Data)
		}
	}
	return val
}

// GetString convert tag.Data to string
func (tag *DICOMTag) GetString() string {
	if len(tag.Data) == 0 || tag.Length == 0 {
		return ""
	}

	limit := len(tag.Data)
	if int(tag.Length) < limit {
		limit = int(tag.Length)
	}
	data := tag.Data[:limit]

	n := bytes.IndexByte(data, 0)
	if n == -1 {
		n = len(data)
	}
	return strings.TrimSpace(string(data[:n]))
}

// GetFloat decodes a DICOM FL (IEEE 754 single-precision, PS3.5).
func (tag *DICOMTag) GetFloat() float32 {
	if tag.Length == 4 && len(tag.Data) >= 4 && tag.VR == "FL" {
		var bits uint32
		if tag.BigEndian {
			bits = binary.BigEndian.Uint32(tag.Data)
		} else {
			bits = binary.LittleEndian.Uint32(tag.Data)
		}
		return math.Float32frombits(bits)
	}
	val := tag.GetString()
	if s, err := strconv.ParseFloat(val, 32); err == nil {
		return float32(s)
	}
	return 0.0
}

// GetFloat64 decodes a DICOM FD (IEEE 754 double-precision, PS3.5).
func (tag *DICOMTag) GetFloat64() float64 {
	if tag.Length == 8 && len(tag.Data) >= 8 && tag.VR == "FD" {
		var bits uint64
		if tag.BigEndian {
			bits = binary.BigEndian.Uint64(tag.Data)
		} else {
			bits = binary.LittleEndian.Uint64(tag.Data)
		}
		return math.Float64frombits(bits)
	}
	val := tag.GetString()
	if s, err := strconv.ParseFloat(val, 64); err == nil {
		return s
	}
	return 0.0
}

// WriteSeq - Create an SQ tag from a DICOM Object
func (tag *DICOMTag) WriteSeq(group uint16, element uint16, seq DICOMObject) {
	buf := NewDICOMBuffer()

	buf.bigEndian = seq.IsBigEndian()
	tag.BigEndian = seq.IsBigEndian()
	tag.Group = group
	tag.Element = element
	if tag.Group == 0xFFFE {
		tag.VR = ""
	} else {
		tag.VR = "SQ"
	}
	for i := 0; i < seq.TagCount(); i++ {
		temptag := seq.GetTagAt(i)
		buf.WriteTag(temptag, seq.IsExplicitVR())
	}
	tag.Length = uint32(buf.GetSize())
	if tag.Length%2 == 1 {
		tag.Length++
		buf.Write([]byte{0x00}, 1)
	}
	if tag.Length > 0 {
		buf.SetPosition(0)
		data, _ := buf.ReadSlice(int(tag.Length))
		tag.Data = data
	}
}

// ReadSeq - reads a dicom sequence
func (tag *DICOMTag) ReadSeq(ExplicitVR bool) DICOMObject {
	seq := NewEmptyDCMObj()
	// Wrap tag.Data directly instead of copying it into a fresh stream.
	// len(tag.Data) == tag.Length for all tags produced by ReadTag.
	// The backing array stays alive as long as any sub-tag holds a Data
	// sub-slice, which keeps the whole chain alive transitively.
	buf := &DICOMBuffer{data: tag.Data, size: len(tag.Data)}

	for buf.position < buf.size {
		temptag, err := buf.ReadTag(ExplicitVR)
		if err != nil {
			break // position does not advance on error; continuing would loop forever
		}

		if !ExplicitVR {
			temptag.VR = GetDictionaryVR(tag.Group, tag.Element)
		}
		seq.Add(temptag)
	}
	return seq
}
