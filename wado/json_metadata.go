package wado

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/innovative-io/io-dicom/media"
)

// writeJSONMetadata serialises objects to DICOMweb JSON
// (application/dicom+json) by streaming directly to w, without building an
// intermediate map[string]dicomJSONTag per object.
//
// The previous implementation encoded a []map[string]dicomJSONTag through
// encoding/json's reflection path; a profile of a 500-instance response put
// ~77% of allocations in reflect.unsafe_New (boxing every value into
// []interface{}) and the map encoder, plus ~11% in fmt.Sprintf building the
// hex keys. Streaming eliminates the maps, the interface boxing and the
// per-tag Sprintf.
//
// Scalar and binary values are formatted directly; string values are delegated
// to json.Marshal so their escaping (including encoding/json's HTML escaping) is
// byte-for-byte identical to the old output. DICOMweb JSON objects are
// unordered, so emitting attributes in tag order rather than the old map's
// sorted-key order is an equivalent representation.
func writeJSONMetadata(w http.ResponseWriter, objects []media.DICOMObject) {
	w.Header().Set("Content-Type", "application/dicom+json")
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	bw.WriteByte('[')
	for i, obj := range objects {
		if i > 0 {
			bw.WriteByte(',')
		}
		writeObjectJSON(bw, obj)
	}
	bw.WriteByte(']')
}

const hexUpper = "0123456789ABCDEF"

// writeTagKey writes the 8-character uppercase group+element key, e.g. 00080018.
func writeTagKey(bw *bufio.Writer, group, element uint16) {
	bw.WriteByte('"')
	for _, v := range [2]uint16{group, element} {
		bw.WriteByte(hexUpper[v>>12&0xF])
		bw.WriteByte(hexUpper[v>>8&0xF])
		bw.WriteByte(hexUpper[v>>4&0xF])
		bw.WriteByte(hexUpper[v&0xF])
	}
	bw.WriteByte('"')
}

func writeObjectJSON(bw *bufio.Writer, obj media.DICOMObject) {
	bw.WriteByte('{')
	first := true
	for _, tag := range obj.GetTags() {
		// Omit pixel data, matching objectToJSONTags.
		if tag.Group == 0x7FE0 && tag.Element == 0x0010 {
			continue
		}
		if !first {
			bw.WriteByte(',')
		}
		first = false

		writeTagKey(bw, tag.Group, tag.Element)
		bw.WriteString(`:{"vr":`)
		// A VR is normally a 2-letter uppercase ASCII code (PS3.5 §6.2) needing no
		// escaping, so write it directly. A malformed stream can leave arbitrary
		// bytes in VR, so fall back to json.Marshal when any escaping is needed —
		// keeping the output identical to the reflection encoder in every case.
		if jsonSafeASCII(tag.VR) {
			bw.WriteByte('"')
			bw.WriteString(tag.VR)
			bw.WriteByte('"')
		} else {
			writeJSONValue(bw, tag.VR)
		}

		switch tag.VR {
		case "US":
			bw.WriteString(`,"Value":[`)
			bw.WriteString(strconv.FormatUint(uint64(tag.GetUint16()), 10))
			bw.WriteByte(']')
		case "UL":
			bw.WriteString(`,"Value":[`)
			bw.WriteString(strconv.FormatUint(uint64(tag.GetUint32()), 10))
			bw.WriteByte(']')
		case "FL":
			bw.WriteString(`,"Value":[`)
			writeJSONValue(bw, float64(tag.GetFloat()))
			bw.WriteByte(']')
		case "FD":
			bw.WriteString(`,"Value":[`)
			writeJSONValue(bw, tag.GetFloat64())
			bw.WriteByte(']')
		case "OB", "OW", "OD", "OF", "UN":
			if len(tag.Data) > 0 && len(tag.Data) <= 4096 {
				bw.WriteString(`,"Value":["`)
				writeBase64(bw, tag.Data)
				bw.WriteString(`"]`)
			}
		default:
			if s := tag.GetString(); s != "" {
				bw.WriteString(`,"Value":[`)
				// Most string values (UIDs, dates, times, codes, numbers) are
				// escape-free ASCII; write them directly and fall back to
				// json.Marshal only for text VRs (PN, LO, …) that need escaping.
				if jsonSafeASCII(s) {
					bw.WriteByte('"')
					bw.WriteString(s)
					bw.WriteByte('"')
				} else {
					writeJSONValue(bw, s)
				}
				bw.WriteByte(']')
			}
		}
		bw.WriteByte('}')
	}
	bw.WriteByte('}')
}

// writeJSONValue delegates to encoding/json so escaping and number formatting are
// byte-for-byte identical to the previous []interface{} encoding.
func writeJSONValue(bw *bufio.Writer, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		bw.WriteString("null")
		return
	}
	bw.Write(b)
}

// jsonSafeASCII reports whether s can be written between quotes verbatim, i.e.
// every byte is printable ASCII that encoding/json (with HTML escaping) leaves
// untouched: 0x20-0x7E excluding " \ < > &.
func jsonSafeASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c > 0x7E || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&' {
			return false
		}
	}
	return true
}

func writeBase64(bw *bufio.Writer, data []byte) {
	enc := base64.NewEncoder(base64.StdEncoding, bw)
	enc.Write(data)
	enc.Close()
}
