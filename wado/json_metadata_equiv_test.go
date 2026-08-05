package wado

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"testing"

	"github.com/innovative-io/io-dicom/media"
)

func strTag(group, element uint16, vr, s string) *media.DICOMTag {
	return &media.DICOMTag{Group: group, Element: element, VR: vr,
		Length: uint32(len(s)), Data: []byte(s)}
}

func binTag(group, element uint16, vr string, data []byte) *media.DICOMTag {
	return &media.DICOMTag{Group: group, Element: element, VR: vr,
		Length: uint32(len(data)), Data: data}
}

func u16Tag(group, element uint16, v uint16) *media.DICOMTag {
	return &media.DICOMTag{Group: group, Element: element, VR: "US",
		Length: 2, Data: []byte{byte(v), byte(v >> 8)}}
}

func u32Tag(group, element uint16, v uint32) *media.DICOMTag {
	return &media.DICOMTag{Group: group, Element: element, VR: "UL", Length: 4,
		Data: []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}}
}

func f32Tag(group, element uint16, v float32) *media.DICOMTag {
	bits := math.Float32bits(v)
	return &media.DICOMTag{Group: group, Element: element, VR: "FL", Length: 4,
		Data: []byte{byte(bits), byte(bits >> 8), byte(bits >> 16), byte(bits >> 24)}}
}

// equivCorpus exercises every VR branch and the string values most likely to
// expose an escaping divergence.
func equivCorpus() []media.DICOMObject {
	tricky := []string{
		"plain", `with "double" quotes`, `back\slash`, "tab\tnl\ncr\r",
		"html <b> & </b> chars", "unicode café ☕ 日本語 🔬", "\x01\x02 control",
		"line sep para", "trailing-nonspace-ok",
	}
	var objs []media.DICOMObject

	// One object per tricky string (as a LO value), plus a scalar/binary object.
	for i, s := range tricky {
		obj := media.NewEmptyDCMObj()
		obj.SetExplicitVR(true)
		obj.Add(strTag(0x0010, 0x0010, "PN", s))
		obj.Add(strTag(0x0008, 0x0018, "UI", "1.2.3."+s[:1]))
		obj.Add(u16Tag(0x0028, 0x0010, uint16(256+i)))
		objs = append(objs, obj)
	}

	// A mixed object hitting US/UL/FL/binary/empty branches.
	mixed := media.NewEmptyDCMObj()
	mixed.SetExplicitVR(true)
	mixed.Add(u16Tag(0x0028, 0x0010, 512))
	mixed.Add(u32Tag(0x0028, 0x0008, 4000000000))
	mixed.Add(f32Tag(0x0018, 0x0050, 1.25))
	mixed.Add(binTag(0x0008, 0x0000, "UL", []byte{0x10, 0x20, 0x30, 0x40}))
	mixed.Add(binTag(0x7FE0, 0x0001, "OB", []byte{0, 1, 2, 3, 255, 254}))           // small binary
	mixed.Add(binTag(0x7FE0, 0x0010, "OW", []byte{9, 9, 9, 9}))                     // pixel data: omitted
	mixed.Add(&media.DICOMTag{Group: 0x0008, Element: 0x0008, VR: "CS", Length: 0}) // empty value
	mixed.Add(binTag(0x0009, 0x0001, "OB", make([]byte, 5000)))                     // > 4096: omitted
	objs = append(objs, mixed)

	// A malformed VR containing characters that require escaping, to exercise the
	// direct-write fallback path.
	weird := media.NewEmptyDCMObj()
	weird.SetExplicitVR(true)
	weird.Add(strTag(0x0011, 0x0011, "a\"b", "value"))
	weird.Add(strTag(0x0011, 0x0012, "<>", "value2"))
	objs = append(objs, weird)

	return objs
}

// TestStreamingJSONSemanticallyMatchesReflection is the correctness contract for
// the streaming encoder: for every object, its output must unmarshal to exactly
// the same structure the old reflection-based writeJSONMetadata produced.
// DICOMweb JSON is unordered, so this compares decoded values, not bytes.
func TestStreamingJSONSemanticallyMatchesReflection(t *testing.T) {
	objs := equivCorpus()

	oldRW := &nopCapture{}
	referenceJSONMetadata(oldRW, objs) // the reflection-based reference

	newRW := &nopCapture{}
	writeJSONMetadata(newRW, objs) // the production streaming encoder

	var oldVal, newVal interface{}
	if err := json.Unmarshal(oldRW.buf, &oldVal); err != nil {
		t.Fatalf("old output is not valid JSON: %v\n%s", err, oldRW.buf)
	}
	if err := json.Unmarshal(newRW.buf, &newVal); err != nil {
		t.Fatalf("streamed output is not valid JSON: %v\n%s", err, newRW.buf)
	}
	if !reflect.DeepEqual(oldVal, newVal) {
		t.Fatalf("streamed JSON differs semantically from reflection output\nOLD: %s\nNEW: %s",
			oldRW.buf, newRW.buf)
	}
}

// nopCapture is an http.ResponseWriter that captures the body for comparison.
type nopCapture struct {
	buf []byte
	hdr http.Header
}

func (n *nopCapture) Header() http.Header {
	if n.hdr == nil {
		n.hdr = http.Header{}
	}
	return n.hdr
}
func (n *nopCapture) Write(p []byte) (int, error) { n.buf = append(n.buf, p...); return len(p), nil }
func (n *nopCapture) WriteHeader(int)             {}

// referenceJSONMetadata is the original reflection-based encoder, retained here
// as the oracle the streaming production encoder is checked against. If the two
// ever diverge, TestStreamingJSONSemanticallyMatchesReflection fails.
type refJSONTag struct {
	VR    string        `json:"vr"`
	Value []interface{} `json:"Value,omitempty"`
}

func referenceJSONMetadata(w http.ResponseWriter, objects []media.DICOMObject) {
	result := make([]map[string]refJSONTag, 0, len(objects))
	for _, obj := range objects {
		m := make(map[string]refJSONTag)
		for _, tag := range obj.GetTags() {
			if tag.Group == 0x7FE0 && tag.Element == 0x0010 {
				continue
			}
			key := fmt.Sprintf("%04X%04X", tag.Group, tag.Element)
			entry := refJSONTag{VR: tag.VR}
			switch tag.VR {
			case "US":
				entry.Value = []interface{}{tag.GetUint16()}
			case "UL":
				entry.Value = []interface{}{tag.GetUint32()}
			case "FL":
				entry.Value = []interface{}{tag.GetFloat()}
			case "FD":
				entry.Value = []interface{}{tag.GetFloat64()}
			case "OB", "OW", "OD", "OF", "UN":
				if len(tag.Data) > 0 && len(tag.Data) <= 4096 {
					entry.Value = []interface{}{tag.Data}
				}
			default:
				if str := tag.GetString(); str != "" {
					entry.Value = []interface{}{str}
				}
			}
			m[key] = entry
		}
		result = append(result, m)
	}
	w.Header().Set("Content-Type", "application/dicom+json")
	_ = json.NewEncoder(w).Encode(result)
}
