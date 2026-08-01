package media

import (
	"bytes"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

// buildStreamTestObj constructs an object exercising the serialization branches
// that matter on the wire: short and long VRs, an empty-value tag, and (when
// encapsulated) an undefined-length pixel-data tag followed by fragment items.
func buildStreamTestObj(explicitVR bool, ts *transfersyntax.TransferSyntax, encapsulated bool) DICOMObject {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(ts)
	obj.SetExplicitVR(explicitVR)

	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.7")
	obj.Write(tags.PatientName, "STREAM^EQUIV") // even length
	obj.Write(tags.Rows, uint16(4))
	obj.Write(tags.Columns, uint16(4))
	obj.Write(tags.BitsAllocated, uint16(16)) // long-ish header path
	// An empty-value tag: header only, no data bytes.
	obj.Add(&DICOMTag{Group: 0x0008, Element: 0x0008, VR: "CS", Length: 0})

	if encapsulated {
		// Undefined-length pixel data + BOT + two fragments, the multi-fragment
		// layout WriteObj emits as a flat run of 0xFFFE items.
		obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, VR: "OB", Length: 0xFFFFFFFF})
		obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, VR: "DL", Length: 0})
		obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, VR: "DL", Length: 6, Data: []byte{1, 2, 3, 4, 5, 6}})
		obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, VR: "DL", Length: 4, Data: []byte{7, 8, 9, 10}})
		obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE0DD, VR: "DL", Length: 0})
	} else {
		pix := make([]byte, 4*4*2)
		for i := range pix {
			pix[i] = byte(i)
		}
		obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, VR: "OW", Length: uint32(len(pix)), Data: pix})
	}
	return obj
}

// bufferWriteObj reproduces exactly what the send path did before streaming:
// WriteObj into a fresh little-endian DICOMBuffer, then read the bytes back.
func bufferWriteObj(obj DICOMObject) []byte {
	buf := NewDICOMBuffer()
	buf.WriteObj(obj)
	out := make([]byte, buf.GetSize())
	copy(out, buf.GetData())
	return out
}

// TestWriteObjToMatchesWriteObj is the correctness contract for the streaming
// send: WriteObjTo must be byte-identical to the buffered WriteObj it replaces,
// or the wire format silently changes.
func TestWriteObjToMatchesWriteObj(t *testing.T) {
	cases := []struct {
		name         string
		explicitVR   bool
		ts           *transfersyntax.TransferSyntax
		encapsulated bool
	}{
		{"implicit LE", false, transfersyntax.ImplicitVRLittleEndian, false},
		{"explicit LE", true, transfersyntax.ExplicitVRLittleEndian, false},
		{"explicit LE encapsulated", true, transfersyntax.JPEGLosslessSV1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := buildStreamTestObj(tc.explicitVR, tc.ts, tc.encapsulated)
			want := bufferWriteObj(obj)

			var got bytes.Buffer
			n, err := WriteObjTo(&got, obj)
			if err != nil {
				t.Fatalf("WriteObjTo: %v", err)
			}
			if int(n) != got.Len() {
				t.Fatalf("returned count %d != bytes written %d", n, got.Len())
			}
			if !bytes.Equal(got.Bytes(), want) {
				t.Fatalf("streamed bytes differ from WriteObj\n stream=%d bytes\n buffer=%d bytes\n first diff at %d",
					got.Len(), len(want), firstDiffAt(got.Bytes(), want))
			}
		})
	}
}

func firstDiffAt(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return n
	}
	return -1
}
