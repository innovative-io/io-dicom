package media

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/innovative-io/io-dicom/codecs"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

func TestNewDCMObjFromFile(t *testing.T) {
	InitDict()
	if _, err := os.Stat("../samples/test2.dcm"); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}
	if _, err := os.Stat("../samples/test2-2.dcm"); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}
	if _, err := os.Stat("../samples/test2-3.dcm"); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}

	type args struct {
		fileName string
	}
	tests := []struct {
		name          string
		args          args
		wantTagsCount int
		wantErr       bool
	}{
		{
			name:          "Should load DICOM file from bugged DICOM written by us",
			args:          args{fileName: "../samples/test2-2.dcm"},
			wantTagsCount: 116,
			wantErr:       false,
		},
		{
			name:          "Should load DICOM file from post bugged DICOM written by us",
			args:          args{fileName: "../samples/test2-3.dcm"},
			wantTagsCount: 116,
			wantErr:       false,
		},
		{
			name:          "Should load DICOM file",
			args:          args{fileName: "../samples/test2.dcm"},
			wantTagsCount: 116,
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dicomObject, err := NewDCMObjFromFile(tt.args.fileName)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDCMObjFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(dicomObject.GetTags()) != tt.wantTagsCount {
				t.Errorf("NewDCMObjFromFile() count = %v, wantTagsCount %v", len(dicomObject.GetTags()), tt.wantTagsCount)
				return
			}
		})
	}
}

func Test_dcmObj_ChangeTransferSyntax(t *testing.T) {
	if _, err := os.Stat("../samples/test2.dcm"); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}
	if _, err := os.Stat("../samples/jpeg8.dcm"); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}

	type args struct {
		outTS *transfersyntax.TransferSyntax
	}
	tests := []struct {
		name     string
		fileName string
		args     args
		wantErr  bool
	}{
		{
			name:     "Should change transfer syntax to ImplicitVRLittleEndian",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.ImplicitVRLittleEndian},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to ExplicitVRLittleEndian",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.ExplicitVRLittleEndian},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to EncapsulatedUncompressedExplicitVRLittleEndian",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to DeflatedImageFrameCompression",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.DeflatedImageFrameCompression},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to ExplicitVRBigEndian",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.ExplicitVRBigEndian},
			wantErr:  true,
		},
		{
			name:     "Should change transfer syntax to RLELossless",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.RLELossless},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGLosslessSV1",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPEGLosslessSV1},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGBaseline8Bit",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPEGBaseline8Bit},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGExtended12Bit",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPEGExtended12Bit},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGLSLossless",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPEGLSLossless},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGLSNearLossless",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPEGLSNearLossless},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEG2000Lossless",
			fileName: "../samples/jpeg8.dcm",
			args:     args{transfersyntax.JPEG2000Lossless},
			wantErr:  true,
		},
		{
			name:     "Should change transfer syntax to JPEG2000",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPEG2000},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEG2000MCLossless",
			fileName: "../samples/jpeg8.dcm",
			args:     args{transfersyntax.JPEG2000MCLossless},
			wantErr:  true,
		},
		{
			name:     "Should change transfer syntax to JPEG2000MC",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPEG2000MC},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to HTJ2KLossless",
			fileName: "../samples/jpeg8.dcm",
			args:     args{transfersyntax.HTJ2KLossless},
			wantErr:  true,
		},
		{
			name:     "Should change transfer syntax to HTJ2KLosslessRPCL",
			fileName: "../samples/jpeg8.dcm",
			args:     args{transfersyntax.HTJ2KLosslessRPCL},
			wantErr:  true,
		},
		{
			name:     "Should change transfer syntax to HTJ2K",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.HTJ2K},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGXLLossless",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPEGXLLossless},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGXLJPEGRecompression",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPEGXLJPEGRecompression},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGXL",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPEGXL},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG2MPML",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.MPEG2MPML},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG2MPMLF",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.MPEG2MPMLF},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG2MPHL",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.MPEG2MPHL},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG2MPHLF",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.MPEG2MPHLF},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG4HP41",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.MPEG4HP41},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG4HP41F",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.MPEG4HP41F},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to HEVCMP51",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.HEVCMP51},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to HEVCM10P51",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.HEVCM10P51},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to SMPTEST211020UncompressedProgressiveActiveVideo",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to SMPTEST211020UncompressedInterlacedActiveVideo",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.SMPTEST211020UncompressedInterlacedActiveVideo},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to SMPTEST211030PCMDigitalAudio",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.SMPTEST211030PCMDigitalAudio},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPIPHTJ2KReferenced",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPIPHTJ2KReferenced},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPIPHTJ2KReferencedDeflate",
			fileName: "../samples/test2.dcm",
			args:     args{transfersyntax.JPIPHTJ2KReferencedDeflate},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dicomObject, err := NewDCMObjFromFile(tt.fileName)
			if err != nil {
				panic(err)
			}
			if err := dicomObject.ChangeTransferSyntax(tt.args.outTS); (err != nil) != tt.wantErr {
				t.Errorf("dicomObject.ChangeTransferSyntax() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeflatedExplicitVRLittleEndianRoundTrip(t *testing.T) {
	InitDict()

	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.DeflatedExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.WriteString(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.1")
	obj.WriteString(tags.PatientName, "DOE^JOHN")
	obj.WriteString(tags.PatientID, "12345")

	data := obj.WriteToBytes()
	if len(data) == 0 {
		t.Fatal("expected non-empty encoded bytes")
	}

	parsed, err := NewDCMObjFromBytes(data)
	if err != nil {
		t.Fatalf("NewDCMObjFromBytes failed: %v", err)
	}

	if parsed.GetTransferSyntax() == nil || parsed.GetTransferSyntax().UID != transfersyntax.DeflatedExplicitVRLittleEndian.UID {
		t.Fatalf("unexpected transfer syntax: %v", parsed.GetTransferSyntax())
	}

	if got := parsed.GetString(tags.PatientName); got != "DOE^JOHN" {
		t.Fatalf("PatientName mismatch: got=%q", got)
	}

	if got := parsed.GetString(tags.PatientID); got != "12345" {
		t.Fatalf("PatientID mismatch: got=%q", got)
	}
}

func TestRLELosslessMultiFrameRoundTrip(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.WriteString(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.2")
	obj.WriteStringGE(0x0028, 0x0004, "CS", "MONOCHROME2")
	obj.WriteUint16GE(0x0028, 0x0006, "US", 0)
	obj.WriteStringGE(0x0028, 0x0008, "IS", "2")
	obj.WriteUint16GE(0x0028, 0x0010, "US", 1)
	obj.WriteUint16GE(0x0028, 0x0011, "US", 2)
	obj.WriteUint16GE(0x0028, 0x0100, "US", 8)
	obj.WriteUint16GE(0x0028, 0x0101, "US", 8)
	obj.WriteUint16GE(0x0028, 0x0103, "US", 0)

	pixel := &DICOMTag{
		Group:     0x7FE0,
		Element:   0x0010,
		Length:    4,
		VR:        "OB",
		Data:      []byte{1, 2, 3, 4},
		BigEndian: false,
	}
	FillTag(pixel)
	obj.Add(pixel)

	if err := obj.ChangeTransferSyntax(transfersyntax.RLELossless); err != nil {
		t.Fatalf("ChangeTransferSyntax to RLELossless failed: %v", err)
	}

	frameItemCount := 0
	for i := 0; i < obj.TagCount(); i++ {
		tag := obj.GetTagAt(i)
		if tag != nil && tag.Group == 0xFFFE && tag.Element == 0xE000 && tag.Length > 0 {
			frameItemCount++
		}
	}
	if frameItemCount != 2 {
		t.Fatalf("expected 2 non-empty encapsulated frame items, got %d", frameItemCount)
	}

	if err := obj.ChangeTransferSyntax(transfersyntax.ExplicitVRLittleEndian); err != nil {
		t.Fatalf("ChangeTransferSyntax back to ExplicitVRLittleEndian failed: %v", err)
	}

	frame0, err := obj.GetPixelData(0)
	if err != nil {
		t.Fatalf("GetPixelData(frame0) failed: %v", err)
	}
	if len(frame0) != 2 || frame0[0] != 1 || frame0[1] != 2 {
		t.Fatalf("unexpected frame0 data after roundtrip: %v", frame0)
	}

	frame1, err := obj.GetPixelData(1)
	if err != nil {
		t.Fatalf("GetPixelData(frame1) failed: %v", err)
	}
	if len(frame1) != 2 || frame1[0] != 3 || frame1[1] != 4 {
		t.Fatalf("unexpected frame1 data after roundtrip: %v", frame1)
	}
}

func newRGBRoundTripObject() DICOMObject {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.WriteString(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.3")
	obj.WriteUint16GE(0x0028, 0x0002, "US", 3)
	obj.WriteStringGE(0x0028, 0x0004, "CS", "RGB")
	obj.WriteUint16GE(0x0028, 0x0006, "US", 0)
	obj.WriteStringGE(0x0028, 0x0008, "IS", "1")
	obj.WriteUint16GE(0x0028, 0x0010, "US", 1)
	obj.WriteUint16GE(0x0028, 0x0011, "US", 2)
	obj.WriteUint16GE(0x0028, 0x0100, "US", 8)
	obj.WriteUint16GE(0x0028, 0x0101, "US", 8)
	obj.WriteUint16GE(0x0028, 0x0103, "US", 0)

	pixel := &DICOMTag{
		Group:     0x7FE0,
		Element:   0x0010,
		Length:    6,
		VR:        "OB",
		Data:      []byte{10, 20, 30, 40, 50, 60},
		BigEndian: false,
	}
	FillTag(pixel)
	obj.Add(pixel)
	return obj
}

func TestGetPixelData_UncompressedMultiFrameReturnsRequestedFrame(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.WriteStringGE(0x0028, 0x0004, "CS", "MONOCHROME2")
	obj.WriteStringGE(0x0028, 0x0008, "IS", "2")
	obj.WriteUint16GE(0x0028, 0x0010, "US", 1)
	obj.WriteUint16GE(0x0028, 0x0011, "US", 2)
	obj.WriteUint16GE(0x0028, 0x0100, "US", 8)
	obj.WriteUint16GE(0x0028, 0x0101, "US", 8)
	obj.WriteUint16GE(0x0028, 0x0103, "US", 0)

	pixel := &DICOMTag{
		Group:     0x7FE0,
		Element:   0x0010,
		Length:    4,
		VR:        "OB",
		Data:      []byte{10, 20, 30, 40},
		BigEndian: false,
	}
	FillTag(pixel)
	obj.Add(pixel)

	frame0, err := obj.GetPixelData(0)
	if err != nil {
		t.Fatalf("GetPixelData(frame0) failed: %v", err)
	}
	if len(frame0) != 2 || frame0[0] != 10 || frame0[1] != 20 {
		t.Fatalf("unexpected frame0 data: %v", frame0)
	}

	frame1, err := obj.GetPixelData(1)
	if err != nil {
		t.Fatalf("GetPixelData(frame1) failed: %v", err)
	}
	if len(frame1) != 2 || frame1[0] != 30 || frame1[1] != 40 {
		t.Fatalf("unexpected frame1 data: %v", frame1)
	}
}

func TestGetPixelData_EncapsulatedSingleFrameWithMultipleFragments(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.JPEGBaseline8Bit)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.WriteStringGE(0x0028, 0x0004, "CS", "MONOCHROME2")
	obj.WriteStringGE(0x0028, 0x0008, "IS", "1")
	obj.WriteUint16GE(0x0028, 0x0010, "US", 1)
	obj.WriteUint16GE(0x0028, 0x0011, "US", 4)
	obj.WriteUint16GE(0x0028, 0x0100, "US", 8)

	obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL", BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 2, VR: "DL", Data: []byte{1, 2}, BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 2, VR: "DL", Data: []byte{3, 4}, BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

	frame0, err := obj.GetPixelData(0)
	if err != nil {
		t.Fatalf("GetPixelData(frame0) failed: %v", err)
	}
	if len(frame0) != 4 || frame0[0] != 1 || frame0[1] != 2 || frame0[2] != 3 || frame0[3] != 4 {
		t.Fatalf("unexpected frame0 data: %v", frame0)
	}
}

func TestGetPixelData_EncapsulatedMultiFrameWithFragmentedFrameUsesBOT(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.JPEGBaseline8Bit)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.WriteStringGE(0x0028, 0x0004, "CS", "MONOCHROME2")
	obj.WriteStringGE(0x0028, 0x0008, "IS", "2")
	obj.WriteUint16GE(0x0028, 0x0010, "US", 1)
	obj.WriteUint16GE(0x0028, 0x0011, "US", 4)
	obj.WriteUint16GE(0x0028, 0x0100, "US", 8)

	// BOT offsets are measured from the first fragment item tag after BOT.
	// Frame 0 starts at offset 0; frame 1 starts after two fragments (2*(8 header + 2 payload) = 20).
	bot := []byte{0x00, 0x00, 0x00, 0x00, 0x14, 0x00, 0x00, 0x00}

	obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: uint32(len(bot)), VR: "DL", Data: bot, BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 2, VR: "DL", Data: []byte{10, 11}, BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 2, VR: "DL", Data: []byte{12, 13}, BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 2, VR: "DL", Data: []byte{20, 21}, BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

	frame0, err := obj.GetPixelData(0)
	if err != nil {
		t.Fatalf("GetPixelData(frame0) failed: %v", err)
	}
	if len(frame0) != 4 || frame0[0] != 10 || frame0[1] != 11 || frame0[2] != 12 || frame0[3] != 13 {
		t.Fatalf("unexpected frame0 data: %v", frame0)
	}

	frame1, err := obj.GetPixelData(1)
	if err != nil {
		t.Fatalf("GetPixelData(frame1) failed: %v", err)
	}
	if len(frame1) != 2 || frame1[0] != 20 || frame1[1] != 21 {
		t.Fatalf("unexpected frame1 data: %v", frame1)
	}
}

func TestRGBRoundTripViaEncapsulatedCodecs(t *testing.T) {
	tests := []struct {
		name string
		ts   *transfersyntax.TransferSyntax
	}{
		{name: "RLELossless", ts: transfersyntax.RLELossless},
		{name: "EncapsulatedUncompressedExplicitVRLittleEndian", ts: transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian},
		{name: "DeflatedImageFrameCompression", ts: transfersyntax.DeflatedImageFrameCompression},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := newRGBRoundTripObject()

			if err := obj.ChangeTransferSyntax(tt.ts); err != nil {
				t.Fatalf("ChangeTransferSyntax to %s failed: %v", tt.ts.Name, err)
			}
			if err := obj.ChangeTransferSyntax(transfersyntax.ExplicitVRLittleEndian); err != nil {
				t.Fatalf("ChangeTransferSyntax back to ExplicitVRLittleEndian failed: %v", err)
			}

			out, err := obj.GetPixelData(0)
			if err != nil {
				t.Fatalf("GetPixelData failed: %v", err)
			}
			want := []byte{10, 20, 30, 40, 50, 60}
			if len(out) < len(want) {
				t.Fatalf("expected at least %d bytes of pixel data, got %d", len(want), len(out))
			}
			for i := range want {
				if out[i] != want[i] {
					t.Fatalf("pixel[%d]=%d want=%d full=%v", i, out[i], want[i], out[:len(want)])
				}
			}
		})
	}
}

func forcePassthroughCodecBackends(t *testing.T) {
	forceCodecBackends(t, codecs.BackendConfig{
		JPEG:      "passthrough",
		JPEGLS:    "passthrough",
		JPEG2000:  "passthrough",
		JPEGXL:    "passthrough",
		MPEG:      "passthrough",
		JPIP:      "passthrough",
		SMPTE2110: "passthrough",
	})
}

func forceCodecBackends(t *testing.T, cfg codecs.BackendConfig) {
	t.Helper()

	err := codecs.UseBackends(cfg)
	if err != nil {
		t.Fatalf("failed to configure codec backends: %v", err)
	}

	t.Cleanup(func() {
		if err := codecs.UseBackends(codecs.BackendConfig{
			JPEG:      "passthrough",
			JPEGLS:    "passthrough",
			JPEG2000:  "passthrough",
			JPEGXL:    "passthrough",
			MPEG:      "passthrough",
			JPIP:      "passthrough",
			SMPTE2110: "passthrough",
		}); err != nil {
			t.Fatalf("failed to restore passthrough codec backends: %v", err)
		}
	})
}

func newMonoRoundTripObject(transferSyntax *transfersyntax.TransferSyntax) DICOMObject {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transferSyntax)
	obj.SetExplicitVR(transferSyntax.UID != transfersyntax.ImplicitVRLittleEndian.UID)
	obj.SetBigEndian(transferSyntax.UID == transfersyntax.ExplicitVRBigEndian.UID)

	obj.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.WriteString(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.4")
	obj.WriteString(tags.PatientName, "ROUNDTRIP^MONO")
	obj.WriteString(tags.PatientID, "MONO-01")
	obj.WriteStringGE(0x0028, 0x0004, "CS", "MONOCHROME2")
	obj.WriteUint16GE(0x0028, 0x0006, "US", 0)
	obj.WriteStringGE(0x0028, 0x0008, "IS", "1")
	obj.WriteUint16GE(0x0028, 0x0010, "US", 1)
	obj.WriteUint16GE(0x0028, 0x0011, "US", 4)
	obj.WriteUint16GE(0x0028, 0x0100, "US", 8)
	obj.WriteUint16GE(0x0028, 0x0101, "US", 8)
	obj.WriteUint16GE(0x0028, 0x0103, "US", 0)

	pixel := &DICOMTag{
		Group:     0x7FE0,
		Element:   0x0010,
		Length:    4,
		VR:        "OB",
		Data:      []byte{7, 17, 27, 37},
		BigEndian: false,
	}
	FillTag(pixel)
	obj.Add(pixel)
	return obj
}

func newMono12BitRoundTripObject() DICOMObject {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.WriteString(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.5")
	obj.WriteStringGE(0x0028, 0x0004, "CS", "MONOCHROME2")
	obj.WriteUint16GE(0x0028, 0x0006, "US", 0)
	obj.WriteStringGE(0x0028, 0x0008, "IS", "1")
	obj.WriteUint16GE(0x0028, 0x0010, "US", 1)
	obj.WriteUint16GE(0x0028, 0x0011, "US", 2)
	obj.WriteUint16GE(0x0028, 0x0100, "US", 16)
	obj.WriteUint16GE(0x0028, 0x0101, "US", 12)
	obj.WriteUint16GE(0x0028, 0x0103, "US", 0)

	pixel := &DICOMTag{
		Group:     0x7FE0,
		Element:   0x0010,
		Length:    4,
		VR:        "OW",
		Data:      []byte{0x34, 0x02, 0xAB, 0x03},
		BigEndian: false,
	}
	FillTag(pixel)
	obj.Add(pixel)
	return obj
}

func assertPixelPrefix(t *testing.T, obj DICOMObject, want []byte) {
	t.Helper()

	out, err := obj.GetPixelData(0)
	if err != nil {
		t.Fatalf("GetPixelData failed: %v", err)
	}
	if len(out) < len(want) {
		t.Fatalf("expected at least %d bytes of pixel data, got %d", len(want), len(out))
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("pixel[%d]=%d want=%d full=%v", i, out[i], want[i], out[:len(want)])
		}
	}
}

func assertPixelPrefixWithinDelta(t *testing.T, obj DICOMObject, want []byte, maxDelta byte) {
	t.Helper()

	out, err := obj.GetPixelData(0)
	if err != nil {
		t.Fatalf("GetPixelData failed: %v", err)
	}
	if len(out) < len(want) {
		t.Fatalf("expected at least %d bytes of pixel data, got %d", len(want), len(out))
	}
	for i := range want {
		got := out[i]
		expected := want[i]
		var delta byte
		if got > expected {
			delta = got - expected
		} else {
			delta = expected - got
		}
		if delta > maxDelta {
			t.Fatalf("pixel[%d]=%d want=%d delta=%d full=%v", i, got, expected, delta, out[:len(want)])
		}
	}
}

func TestDatasetTransferSyntaxRoundTripMatrix(t *testing.T) {
	InitDict()

	tests := []struct {
		name string
		ts   *transfersyntax.TransferSyntax
	}{
		{name: "ImplicitVRLittleEndian", ts: transfersyntax.ImplicitVRLittleEndian},
		{name: "ExplicitVRLittleEndian", ts: transfersyntax.ExplicitVRLittleEndian},
		{name: "DeflatedExplicitVRLittleEndian", ts: transfersyntax.DeflatedExplicitVRLittleEndian},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := newMonoRoundTripObject(tt.ts)

			data := obj.WriteToBytes()
			if len(data) == 0 {
				t.Fatal("expected non-empty serialized bytes")
			}

			parsed, err := NewDCMObjFromBytes(data)
			if err != nil {
				t.Fatalf("NewDCMObjFromBytes failed: %v", err)
			}

			if parsed.GetTransferSyntax() == nil || parsed.GetTransferSyntax().UID != tt.ts.UID {
				t.Fatalf("unexpected transfer syntax: %v", parsed.GetTransferSyntax())
			}
			if got := parsed.GetString(tags.PatientName); got != "ROUNDTRIP^MONO" {
				t.Fatalf("PatientName mismatch: got=%q", got)
			}
			if got := parsed.GetString(tags.PatientID); got != "MONO-01" {
				t.Fatalf("PatientID mismatch: got=%q", got)
			}
			assertPixelPrefix(t, parsed, []byte{7, 17, 27, 37})
		})
	}
}

func TestRepresentativePixelTransferSyntaxRoundTrips(t *testing.T) {
	forcePassthroughCodecBackends(t)

	for _, tt := range representativePixelRoundTripCases() {
		t.Run(tt.name, func(t *testing.T) {
			obj := tt.newObj()

			if err := obj.ChangeTransferSyntax(tt.ts); err != nil {
				t.Fatalf("ChangeTransferSyntax to %s failed: %v", tt.ts.Name, err)
			}

			frameItemCount := 0
			for i := 0; i < obj.TagCount(); i++ {
				tag := obj.GetTagAt(i)
				if tag != nil && tag.Group == 0xFFFE && tag.Element == 0xE000 && tag.Length > 0 {
					frameItemCount++
				}
			}
			if frameItemCount == 0 {
				t.Fatalf("expected non-empty encapsulated frame items after converting to %s", tt.ts.Name)
			}

			if tt.nativeFamily != "" {
				if err := obj.ChangeTransferSyntax(transfersyntax.ExplicitVRLittleEndian); err == nil {
					t.Fatalf("expected ChangeTransferSyntax back to ExplicitVRLittleEndian to fail without native %s decode backend", tt.nativeFamily)
				}
				return
			}

			if err := obj.ChangeTransferSyntax(transfersyntax.ExplicitVRLittleEndian); err != nil {
				t.Fatalf("ChangeTransferSyntax back to ExplicitVRLittleEndian failed: %v", err)
			}
			if obj.GetTransferSyntax() == nil || obj.GetTransferSyntax().UID != transfersyntax.ExplicitVRLittleEndian.UID {
				t.Fatalf("unexpected transfer syntax after roundtrip: %v", obj.GetTransferSyntax())
			}
			assertRoundTripByMode(t, obj, tt.want, tt.passthroughMode)
		})
	}
}

// ── DICOMTag.WriteSeq / ReadSeq ───────────────────────────────────────────────

func TestDICOMTag_WriteSeq_NonEmpty(t *testing.T) {
	inner := NewEmptyDCMObj()
	inner.WriteString(tags.PatientID, "PAT001")

	tag := &DICOMTag{}
	tag.WriteSeq(0x0040, 0xA043, inner)

	if tag.Group != 0x0040 || tag.Element != 0xA043 {
		t.Errorf("WriteSeq: unexpected group/element %04X/%04X", tag.Group, tag.Element)
	}
	if tag.VR != "SQ" {
		t.Errorf("WriteSeq: VR = %q, want SQ", tag.VR)
	}
	if tag.Length == 0 {
		t.Error("WriteSeq: Length should be > 0 for non-empty sequence")
	}
}

func TestDICOMTag_WriteSeq_FFFEGroup_HasNoVR(t *testing.T) {
	inner := NewEmptyDCMObj()
	inner.WriteString(tags.PatientID, "X")

	tag := &DICOMTag{}
	tag.WriteSeq(0xFFFE, 0xE000, inner)

	if tag.VR != "" {
		t.Errorf("WriteSeq: FFFE group should have empty VR, got %q", tag.VR)
	}
}

func TestDICOMTag_WriteSeq_EvenLength(t *testing.T) {
	inner := NewEmptyDCMObj()
	inner.WriteString(tags.PatientID, "A") // odd-length value → padding required

	tag := &DICOMTag{}
	tag.WriteSeq(0x0010, 0x0020, inner)

	if tag.Length%2 != 0 {
		t.Errorf("WriteSeq: Length %d should be even (padded)", tag.Length)
	}
}

func TestDICOMTag_ReadSeq_RoundTrip(t *testing.T) {
	inner := NewEmptyDCMObj()
	inner.WriteString(tags.PatientID, "PAT123")

	tag := &DICOMTag{}
	tag.WriteSeq(0x0040, 0xA043, inner)

	result := tag.ReadSeq(false)
	if result == nil {
		t.Fatal("ReadSeq: returned nil")
	}
	if result.TagCount() == 0 {
		t.Error("ReadSeq: should have at least one tag")
	}
}

// ── AddConceptNameSeq ─────────────────────────────────────────────────────────

func TestAddConceptNameSeq_AddsTag(t *testing.T) {
	obj := NewEmptyDCMObj()
	before := obj.TagCount()

	obj.AddConceptNameSeq(0x0040, 0xA043, "1111", "Radiology Report")

	if obj.TagCount() <= before {
		t.Error("AddConceptNameSeq: no tag was added to object")
	}
}

func TestAddConceptNameSeq_TagGroupElement(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.AddConceptNameSeq(0x0040, 0xA043, "CODE1", "Test Concept")

	tag := obj.GetTagAt(obj.TagCount() - 1)
	if tag.Group != 0x0040 || tag.Element != 0xA043 {
		t.Errorf("AddConceptNameSeq: tag at %04X/%04X, want 0040/A043", tag.Group, tag.Element)
	}
}

// ── AddSRText ─────────────────────────────────────────────────────────────────

func TestAddSRText_AddsTag(t *testing.T) {
	obj := NewEmptyDCMObj()
	before := obj.TagCount()

	obj.AddSRText("This is the report body.")

	if obj.TagCount() <= before {
		t.Error("AddSRText: no tag was added to object")
	}
}

func TestAddSRText_EmptyText(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.AddSRText("")
	if obj.TagCount() == 0 {
		t.Error("AddSRText: should have added at least one tag even for empty text")
	}
}

// ── CreateSR ──────────────────────────────────────────────────────────────────

func testStudy() DICOMStudy {
	return DICOMStudy{
		PatientID:          "P001",
		PatientName:        "Doe^John",
		PatientBirthDate:   "19800101",
		PatientSex:         "M",
		ReferringPhysician: "Dr. Smith",
		StudyDate:          "20240101",
		AccessionNumber:    "ACC001",
		InstitutionName:    "Test Hospital",
		Description:        "Chest X-Ray",
		StudyInstanceUID:   "1.2.3.4.5.6",
		ReportText:         "Normal study.",
		ObserverName:       "Dr. Observer",
	}
}

func TestCreateSR_PopulatesRequiredTags(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.CreateSR(testStudy(), "1.2.3.4.5.6.1", "1.2.3.4.5.6.1.1")

	if uid := obj.GetString(tags.SOPInstanceUID); uid != "1.2.3.4.5.6.1.1" {
		t.Errorf("CreateSR: SOPInstanceUID = %q", uid)
	}
	if patID := obj.GetString(tags.PatientID); patID != "P001" {
		t.Errorf("CreateSR: PatientID = %q", patID)
	}
	if mod := obj.GetString(tags.Modality); mod != "SR" {
		t.Errorf("CreateSR: Modality = %q, want SR", mod)
	}
	if obj.TagCount() == 0 {
		t.Error("CreateSR: no tags created")
	}
}

func TestCreateSR_SeriesAndStudyUIDs(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.CreateSR(testStudy(), "9.8.7.6", "9.8.7.6.1")

	if series := obj.GetString(tags.SeriesInstanceUID); series != "9.8.7.6" {
		t.Errorf("CreateSR: SeriesInstanceUID = %q, want 9.8.7.6", series)
	}
	if study := obj.GetString(tags.StudyInstanceUID); study != "1.2.3.4.5.6" {
		t.Errorf("CreateSR: StudyInstanceUID = %q", study)
	}
}

// ── CreatePDF ─────────────────────────────────────────────────────────────────

func TestCreatePDF_PopulatesRequiredTags(t *testing.T) {
	f, err := os.CreateTemp("", "test*.pdf")
	if err != nil {
		t.Fatalf("CreatePDF: failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.Write([]byte{0x25, 0x50, 0x44, 0x46}) // %PDF
	f.Close()

	obj := NewEmptyDCMObj()
	obj.CreatePDF(testStudy(), "1.2.3.100", "1.2.3.100.1", f.Name())

	if patID := obj.GetString(tags.PatientID); patID != "P001" {
		t.Errorf("CreatePDF: PatientID = %q", patID)
	}
	if mod := obj.GetString(tags.Modality); mod != "OT" {
		t.Errorf("CreatePDF: Modality = %q, want OT", mod)
	}
	if obj.TagCount() == 0 {
		t.Error("CreatePDF: no tags created")
	}
}

func TestCreatePDF_OddLengthFileGetsZeroPadded(t *testing.T) {
	f, err := os.CreateTemp("", "test*.pdf")
	if err != nil {
		t.Fatalf("CreatePDF: failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.Write([]byte{0x01, 0x02, 0x03}) // odd length
	f.Close()

	obj := NewEmptyDCMObj()
	obj.CreatePDF(testStudy(), "1.2.3.200", "1.2.3.200.1", f.Name())

	if obj.TagCount() == 0 {
		t.Error("CreatePDF: no tags created for odd-length file")
	}
}

// ── GetStringGE edge case ─────────────────────────────────────────────────────

func TestGetStringGE_ReturnsEmptyWhenTagAbsent(t *testing.T) {
	obj := NewEmptyDCMObj()
	got := obj.GetStringGE(0x9999, 0x9999)
	if got != "" {
		t.Errorf("GetStringGE: want empty string for absent tag, got %q", got)
	}
}

// ── GetPixelData / GetDecompressedFrame large-frame-count tests ──────────────

// TestGetPixelData_LargeEncapsulatedTotalSizeNoLongerRejected verifies that
// GetPixelData succeeds for encapsulated pixel data even when the theoretical
// uncompressed total size exceeds maxPixelDataBytes. The function should return
// the raw compressed bytes for the requested frame without allocating the full
// uncompressed buffer.
func TestGetPixelData_LargeEncapsulatedTotalSizeNoLongerRejected(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.JPEGBaseline8Bit)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	// 16384×16384 pixels × 3 bytes × 5000 frames ≈ 4 TB uncompressed — well above 512 MiB cap.
	obj.WriteStringGE(0x0028, 0x0004, "CS", "MONOCHROME2")
	obj.WriteStringGE(0x0028, 0x0008, "IS", "5000")
	obj.WriteUint16GE(0x0028, 0x0010, "US", 16384)
	obj.WriteUint16GE(0x0028, 0x0011, "US", 16384)
	obj.WriteUint16GE(0x0028, 0x0100, "US", 8)

	compressedBytes := []byte{0xFF, 0xD8, 0x01, 0x02} // mock compressed frame
	obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL", BigEndian: false}) // empty BOT
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: uint32(len(compressedBytes)), VR: "DL", Data: compressedBytes, BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

	frame0, err := obj.GetPixelData(0)
	if err != nil {
		t.Fatalf("GetPixelData on large encapsulated object failed: %v", err)
	}
	if len(frame0) != len(compressedBytes) {
		t.Fatalf("expected %d bytes, got %d", len(compressedBytes), len(frame0))
	}
}

// TestGetDecompressedFrame_UncompressedMultiFrame verifies that GetDecompressedFrame
// correctly slices individual frames from flat uncompressed pixel data.
func TestGetDecompressedFrame_UncompressedMultiFrame(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	frame0Data := []byte{10, 20, 30, 40}
	frame1Data := []byte{50, 60, 70, 80}
	pixelData := append(frame0Data, frame1Data...)

	obj.WriteStringGE(0x0028, 0x0004, "CS", "MONOCHROME2")
	obj.WriteStringGE(0x0028, 0x0008, "IS", "2")
	obj.WriteUint16GE(0x0028, 0x0010, "US", 2) // rows
	obj.WriteUint16GE(0x0028, 0x0011, "US", 2) // cols
	obj.WriteUint16GE(0x0028, 0x0100, "US", 8) // bits allocated
	obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: uint32(len(pixelData)), VR: "OB", Data: pixelData, BigEndian: false})

	ctx := context.Background()

	f0, err := obj.GetDecompressedFrame(ctx, 0)
	if err != nil {
		t.Fatalf("GetDecompressedFrame(0) failed: %v", err)
	}
	if !bytes.Equal(f0, frame0Data) {
		t.Fatalf("frame0: got %v, want %v", f0, frame0Data)
	}

	f1, err := obj.GetDecompressedFrame(ctx, 1)
	if err != nil {
		t.Fatalf("GetDecompressedFrame(1) failed: %v", err)
	}
	if !bytes.Equal(f1, frame1Data) {
		t.Fatalf("frame1: got %v, want %v", f1, frame1Data)
	}
}

// TestGetDecompressedFrame_LargeEncapsulatedTotalSizeOK verifies that
// GetDecompressedFrame succeeds for an encapsulated object whose theoretical
// total uncompressed size exceeds maxPixelDataBytes — the key scenario that
// triggered the original "invalid pixel data size" error.
//
// Because the test uses ExplicitVRLittleEndian for the target after mocking, we
// use EncapsulatedUncompressedExplicitVRLittleEndian so decompressSingleFrame
// can copy without needing an external codec.
func TestGetDecompressedFrame_LargeEncapsulatedTotalSizeOK(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	// 512×512 × 8 bits × 8192 frames = 2 GiB uncompressed — exceeds 512 MiB cap.
	const rows, cols, bits, frames = 512, 512, 8, 8192
	framePayload := make([]byte, rows*cols) // one real frame of zeros
	for i := range framePayload {
		framePayload[i] = byte(i % 256)
	}

	obj.WriteStringGE(0x0028, 0x0004, "CS", "MONOCHROME2")
	obj.WriteStringGE(0x0028, 0x0008, "IS", "8192")
	obj.WriteUint16GE(0x0028, 0x0010, "US", rows)
	obj.WriteUint16GE(0x0028, 0x0011, "US", cols)
	obj.WriteUint16GE(0x0028, 0x0100, "US", bits)

	obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL", BigEndian: false}) // empty BOT
	// Add only one real fragment — requesting frame 0 should use the single fragment.
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: uint32(len(framePayload)), VR: "DL", Data: framePayload, BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

	out, err := obj.GetDecompressedFrame(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetDecompressedFrame failed for large encapsulated object: %v", err)
	}
	if len(out) != rows*cols {
		t.Fatalf("expected %d bytes, got %d", rows*cols, len(out))
	}
	if out[255] != framePayload[255] {
		t.Fatalf("pixel mismatch at index 255: got %d, want %d", out[255], framePayload[255])
	}
}

// TestGetDecompressedFrame_NonConformantUncompressedEncapsulated verifies that
// GetDecompressedFrame tolerates non-conformant DICOM files that declare an
// uncompressed transfer syntax (Explicit/Implicit VR Little Endian) but store
// pixel data as encapsulated fragments (tag.Length == 0xFFFFFFFF). Real-world
// scanners occasionally produce such files.
func TestGetDecompressedFrame_NonConformantUncompressedEncapsulated(t *testing.T) {
	const rows, cols, bits = 8, 8, 8
	framePayload := make([]byte, rows*cols)
	for i := range framePayload {
		framePayload[i] = byte(i)
	}

	for _, ts := range []*transfersyntax.TransferSyntax{
		transfersyntax.ExplicitVRLittleEndian,
		transfersyntax.ImplicitVRLittleEndian,
	} {
		t.Run(ts.Name, func(t *testing.T) {
			obj := NewEmptyDCMObj()
			obj.SetTransferSyntax(ts)
			obj.SetExplicitVR(ts == transfersyntax.ExplicitVRLittleEndian)
			obj.SetBigEndian(false)

			obj.WriteStringGE(0x0028, 0x0004, "CS", "MONOCHROME2")
			obj.WriteStringGE(0x0028, 0x0008, "IS", "1")
			obj.WriteUint16GE(0x0028, 0x0010, "US", rows)
			obj.WriteUint16GE(0x0028, 0x0011, "US", cols)
			obj.WriteUint16GE(0x0028, 0x0100, "US", bits)

			obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
			obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL", BigEndian: false}) // empty BOT
			obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: uint32(len(framePayload)), VR: "DL", Data: framePayload, BigEndian: false})
			obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

			out, err := obj.GetDecompressedFrame(context.Background(), 0)
			if err != nil {
				t.Fatalf("GetDecompressedFrame failed: %v", err)
			}
			if len(out) != rows*cols {
				t.Fatalf("expected %d bytes, got %d", rows*cols, len(out))
			}
			for i, b := range out {
				if b != framePayload[i] {
					t.Fatalf("pixel mismatch at index %d: got %d, want %d", i, b, framePayload[i])
				}
			}
		})
	}
}
