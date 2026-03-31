package media

import (
	"testing"

	"github.com/innovative-io/io-dicom/codecs"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

func TestNewDCMObjFromFile(t *testing.T) {
	InitDict()

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

	out, err := obj.GetPixelData(0)
	if err != nil {
		t.Fatalf("GetPixelData failed: %v", err)
	}
	if len(out) < 4 {
		t.Fatalf("expected at least 4 bytes of pixel data, got %d", len(out))
	}
	if out[0] != 1 || out[1] != 2 || out[2] != 3 || out[3] != 4 {
		t.Fatalf("pixel data mismatch after roundtrip: %v", out[:4])
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
