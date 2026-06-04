package media_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/innovative-io/io-dicom/codecs"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/transcoder"
)

func TestNewDCMObjFromFile(t *testing.T) {
	if _, err := os.Stat("../testdata/test2.dcm"); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}
	if _, err := os.Stat("../testdata/test2-2.dcm"); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}
	if _, err := os.Stat("../testdata/test2-3.dcm"); err != nil {
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
			args:          args{fileName: "../testdata/test2-2.dcm"},
			wantTagsCount: 116,
			wantErr:       false,
		},
		{
			name:          "Should load DICOM file from post bugged DICOM written by us",
			args:          args{fileName: "../testdata/test2-3.dcm"},
			wantTagsCount: 116,
			wantErr:       false,
		},
		{
			name:          "Should load DICOM file",
			args:          args{fileName: "../testdata/test2.dcm"},
			wantTagsCount: 116,
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dicomObject, err := media.NewDCMObjFromFile(tt.args.fileName)
			if (err != nil) != tt.wantErr {
				t.Errorf("media.NewDCMObjFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(dicomObject.GetTags()) != tt.wantTagsCount {
				t.Errorf("media.NewDCMObjFromFile() count = %v, wantTagsCount %v", len(dicomObject.GetTags()), tt.wantTagsCount)
				return
			}
		})
	}
}

func Test_dcmObj_ChangeTransferSyntax(t *testing.T) {
	if _, err := os.Stat("../testdata/test2.dcm"); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}
	if _, err := os.Stat("../testdata/jpeg8.dcm"); err != nil {
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
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.ImplicitVRLittleEndian},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to ExplicitVRLittleEndian",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.ExplicitVRLittleEndian},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to EncapsulatedUncompressedExplicitVRLittleEndian",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to DeflatedImageFrameCompression",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.DeflatedImageFrameCompression},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to ExplicitVRBigEndian",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.ExplicitVRBigEndian},
			wantErr:  true,
		},
		{
			name:     "Should change transfer syntax to RLELossless",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.RLELossless},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGLosslessSV1",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPEGLosslessSV1},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGBaseline8Bit",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPEGBaseline8Bit},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGExtended12Bit",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPEGExtended12Bit},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGLSLossless",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPEGLSLossless},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPEGLSNearLossless",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPEGLSNearLossless},
			wantErr:  false, // pure-Go JPEG-LS near-lossless encoder
		},
		{
			name:     "Should change transfer syntax to JPEG2000Lossless",
			fileName: "../testdata/jpeg8.dcm",
			args:     args{transfersyntax.JPEG2000Lossless},
			wantErr:  true,
		},
		{
			name:     "Should change transfer syntax to JPEG2000",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPEG2000},
			wantErr:  true, // no pure-Go JPEG 2000 encoder
		},
		{
			name:     "Should change transfer syntax to JPEG2000MCLossless",
			fileName: "../testdata/jpeg8.dcm",
			args:     args{transfersyntax.JPEG2000MCLossless},
			wantErr:  true,
		},
		{
			name:     "Should change transfer syntax to JPEG2000MC",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPEG2000MC},
			wantErr:  true, // no pure-Go JPEG 2000 encoder
		},
		{
			name:     "Should change transfer syntax to HTJ2KLossless",
			fileName: "../testdata/jpeg8.dcm",
			args:     args{transfersyntax.HTJ2KLossless},
			wantErr:  true,
		},
		{
			name:     "Should change transfer syntax to HTJ2KLosslessRPCL",
			fileName: "../testdata/jpeg8.dcm",
			args:     args{transfersyntax.HTJ2KLosslessRPCL},
			wantErr:  true,
		},
		{
			name:     "Should change transfer syntax to HTJ2K",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.HTJ2K},
			wantErr:  true, // no pure-Go HTJ2K encoder
		},
		{
			name:     "Should change transfer syntax to JPEGXLLossless",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPEGXLLossless},
			wantErr:  true, // no pure-Go JPEG XL encoder
		},
		{
			name:     "Should change transfer syntax to JPEGXLJPEGRecompression",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPEGXLJPEGRecompression},
			wantErr:  true, // no pure-Go JPEG XL encoder
		},
		{
			name:     "Should change transfer syntax to JPEGXL",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPEGXL},
			wantErr:  true, // no pure-Go JPEG XL encoder
		},
		{
			name:     "Should change transfer syntax to MPEG2MPML",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.MPEG2MPML},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG2MPMLF",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.MPEG2MPMLF},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG2MPHL",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.MPEG2MPHL},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG2MPHLF",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.MPEG2MPHLF},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG4HP41",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.MPEG4HP41},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to MPEG4HP41F",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.MPEG4HP41F},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to HEVCMP51",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.HEVCMP51},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to HEVCM10P51",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.HEVCM10P51},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to SMPTEST211020UncompressedProgressiveActiveVideo",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to SMPTEST211020UncompressedInterlacedActiveVideo",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.SMPTEST211020UncompressedInterlacedActiveVideo},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to SMPTEST211030PCMDigitalAudio",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.SMPTEST211030PCMDigitalAudio},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPIPHTJ2KReferenced",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPIPHTJ2KReferenced},
			wantErr:  false,
		},
		{
			name:     "Should change transfer syntax to JPIPHTJ2KReferencedDeflate",
			fileName: "../testdata/test2.dcm",
			args:     args{transfersyntax.JPIPHTJ2KReferencedDeflate},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dicomObject, err := media.NewDCMObjFromFile(tt.fileName)
			if err != nil {
				t.Fatal(err)
			}
			gotErr := transcoder.ChangeTransferSyntax(dicomObject, tt.args.outTS)
			t.Logf("MAP %-50s want=%v err=%v", tt.name, tt.wantErr, gotErr != nil)
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("dicomObject.ChangeTransferSyntax() error = %v, wantErr %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestDeflatedExplicitVRLittleEndianRoundTrip(t *testing.T) {

	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.DeflatedExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.1")
	obj.Write(tags.PatientName, "DOE^JOHN")
	obj.Write(tags.PatientID, "12345")

	data := obj.WriteToBytes()
	if len(data) == 0 {
		t.Fatal("expected non-empty encoded bytes")
	}

	parsed, err := media.NewDCMObjFromBytes(data)
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
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.2")
	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.PlanarConfiguration, 0)
	obj.Write(tags.NumberOfFrames, "2")
	obj.Write(tags.Rows, 1)
	obj.Write(tags.Columns, 2)
	obj.Write(tags.BitsAllocated, 8)
	obj.Write(tags.BitsStored, 8)
	obj.Write(tags.PixelRepresentation, 0)

	pixel := &media.DICOMTag{
		Group:     0x7FE0,
		Element:   0x0010,
		Length:    4,
		VR:        "OB",
		Data:      []byte{1, 2, 3, 4},
		BigEndian: false,
	}
	media.FillTag(pixel)
	obj.Add(pixel)

	if err := transcoder.ChangeTransferSyntax(obj, transfersyntax.RLELossless); err != nil {
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

	if err := transcoder.ChangeTransferSyntax(obj, transfersyntax.ExplicitVRLittleEndian); err != nil {
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

func newRGBRoundTripObject() media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.3")
	obj.Write(tags.SamplesPerPixel, 3)
	obj.Write(tags.PhotometricInterpretation, "RGB")
	obj.Write(tags.PlanarConfiguration, 0)
	obj.Write(tags.NumberOfFrames, "1")
	obj.Write(tags.Rows, 1)
	obj.Write(tags.Columns, 2)
	obj.Write(tags.BitsAllocated, 8)
	obj.Write(tags.BitsStored, 8)
	obj.Write(tags.PixelRepresentation, 0)

	pixel := &media.DICOMTag{
		Group:     0x7FE0,
		Element:   0x0010,
		Length:    6,
		VR:        "OB",
		Data:      []byte{10, 20, 30, 40, 50, 60},
		BigEndian: false,
	}
	media.FillTag(pixel)
	obj.Add(pixel)
	return obj
}

func TestGetPixelData_UncompressedMultiFrameReturnsRequestedFrame(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.NumberOfFrames, "2")
	obj.Write(tags.Rows, 1)
	obj.Write(tags.Columns, 2)
	obj.Write(tags.BitsAllocated, 8)
	obj.Write(tags.BitsStored, 8)
	obj.Write(tags.PixelRepresentation, 0)

	pixel := &media.DICOMTag{
		Group:     0x7FE0,
		Element:   0x0010,
		Length:    4,
		VR:        "OB",
		Data:      []byte{10, 20, 30, 40},
		BigEndian: false,
	}
	media.FillTag(pixel)
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
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.JPEGBaseline8Bit)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.NumberOfFrames, "1")
	obj.Write(tags.Rows, 1)
	obj.Write(tags.Columns, 4)
	obj.Write(tags.BitsAllocated, 8)

	obj.Add(&media.DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL", BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 2, VR: "DL", Data: []byte{1, 2}, BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 2, VR: "DL", Data: []byte{3, 4}, BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

	frame0, err := obj.GetPixelData(0)
	if err != nil {
		t.Fatalf("GetPixelData(frame0) failed: %v", err)
	}
	if len(frame0) != 4 || frame0[0] != 1 || frame0[1] != 2 || frame0[2] != 3 || frame0[3] != 4 {
		t.Fatalf("unexpected frame0 data: %v", frame0)
	}
}

func TestGetPixelData_EncapsulatedMultiFrameWithFragmentedFrameUsesBOT(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.JPEGBaseline8Bit)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.NumberOfFrames, "2")
	obj.Write(tags.Rows, 1)
	obj.Write(tags.Columns, 4)
	obj.Write(tags.BitsAllocated, 8)

	// BOT offsets are measured from the first fragment item tag after BOT.
	// Frame 0 starts at offset 0; frame 1 starts after two fragments (2*(8 header + 2 payload) = 20).
	bot := []byte{0x00, 0x00, 0x00, 0x00, 0x14, 0x00, 0x00, 0x00}

	obj.Add(&media.DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: uint32(len(bot)), VR: "DL", Data: bot, BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 2, VR: "DL", Data: []byte{10, 11}, BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 2, VR: "DL", Data: []byte{12, 13}, BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 2, VR: "DL", Data: []byte{20, 21}, BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

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

			if err := transcoder.ChangeTransferSyntax(obj, tt.ts); err != nil {
				t.Fatalf("ChangeTransferSyntax to %s failed: %v", tt.ts.Name, err)
			}
			if err := transcoder.ChangeTransferSyntax(obj, transfersyntax.ExplicitVRLittleEndian); err != nil {
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

func newMonoRoundTripObject(transferSyntax *transfersyntax.TransferSyntax) media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transferSyntax)
	obj.SetExplicitVR(transferSyntax.UID != transfersyntax.ImplicitVRLittleEndian.UID)
	obj.SetBigEndian(transferSyntax.UID == transfersyntax.ExplicitVRBigEndian.UID)

	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.4")
	obj.Write(tags.PatientName, "ROUNDTRIP^MONO")
	obj.Write(tags.PatientID, "MONO-01")
	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.PlanarConfiguration, 0)
	obj.Write(tags.NumberOfFrames, "1")
	obj.Write(tags.Rows, 1)
	obj.Write(tags.Columns, 4)
	obj.Write(tags.BitsAllocated, 8)
	obj.Write(tags.BitsStored, 8)
	obj.Write(tags.PixelRepresentation, 0)

	pixel := &media.DICOMTag{
		Group:     0x7FE0,
		Element:   0x0010,
		Length:    4,
		VR:        "OB",
		Data:      []byte{7, 17, 27, 37},
		BigEndian: false,
	}
	media.FillTag(pixel)
	obj.Add(pixel)
	return obj
}

func newMono12BitRoundTripObject() media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.5")
	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.PlanarConfiguration, 0)
	obj.Write(tags.NumberOfFrames, "1")
	obj.Write(tags.Rows, 1)
	obj.Write(tags.Columns, 2)
	obj.Write(tags.BitsAllocated, 16)
	obj.Write(tags.BitsStored, 12)
	obj.Write(tags.PixelRepresentation, 0)

	pixel := &media.DICOMTag{
		Group:     0x7FE0,
		Element:   0x0010,
		Length:    4,
		VR:        "OW",
		Data:      []byte{0x34, 0x02, 0xAB, 0x03},
		BigEndian: false,
	}
	media.FillTag(pixel)
	obj.Add(pixel)
	return obj
}

func assertPixelPrefix(t *testing.T, obj media.DICOMObject, want []byte) {
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

func assertPixelPrefixWithinDelta(t *testing.T, obj media.DICOMObject, want []byte, maxDelta byte) {
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

			parsed, err := media.NewDCMObjFromBytes(data)
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

			if tt.passthroughCannotEncode {
				// No pure-Go encoder for this syntax: the forward transcode must
				// fail rather than emit raw bytes as a fake compressed stream.
				if err := transcoder.ChangeTransferSyntax(obj, tt.ts); err == nil {
					t.Fatalf("expected ChangeTransferSyntax to %s to fail without a pure-Go encoder", tt.ts.Name)
				}
				return
			}

			if err := transcoder.ChangeTransferSyntax(obj, tt.ts); err != nil {
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
				if err := transcoder.ChangeTransferSyntax(obj, transfersyntax.ExplicitVRLittleEndian); err == nil {
					t.Fatalf("expected ChangeTransferSyntax back to ExplicitVRLittleEndian to fail without native %s decode backend", tt.nativeFamily)
				}
				return
			}

			if err := transcoder.ChangeTransferSyntax(obj, transfersyntax.ExplicitVRLittleEndian); err != nil {
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
	inner := media.NewEmptyDCMObj()
	inner.Write(tags.PatientID, "PAT001")

	tag := &media.DICOMTag{}
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
	inner := media.NewEmptyDCMObj()
	inner.Write(tags.PatientID, "X")

	tag := &media.DICOMTag{}
	tag.WriteSeq(0xFFFE, 0xE000, inner)

	if tag.VR != "" {
		t.Errorf("WriteSeq: FFFE group should have empty VR, got %q", tag.VR)
	}
}

func TestDICOMTag_WriteSeq_EvenLength(t *testing.T) {
	inner := media.NewEmptyDCMObj()
	inner.Write(tags.PatientID, "A") // odd-length value → padding required

	tag := &media.DICOMTag{}
	tag.WriteSeq(0x0010, 0x0020, inner)

	if tag.Length%2 != 0 {
		t.Errorf("WriteSeq: Length %d should be even (padded)", tag.Length)
	}
}

func TestDICOMTag_ReadSeq_RoundTrip(t *testing.T) {
	inner := media.NewEmptyDCMObj()
	inner.Write(tags.PatientID, "PAT123")

	tag := &media.DICOMTag{}
	tag.WriteSeq(0x0040, 0xA043, inner)

	result := tag.ReadSeq(false)
	if result == nil {
		t.Fatal("ReadSeq: returned nil")
	}
	if result.TagCount() == 0 {
		t.Error("ReadSeq: should have at least one tag")
	}
}

// ── GetString edge case ──────────────────────────────────────────────────────

func TestGetString_ReturnsEmptyWhenTagAbsent(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	got := obj.GetString(tags.SOPClassUID)
	if got != "" {
		t.Errorf("GetString: want empty string for absent tag, got %q", got)
	}
}

// ── GetPixelData / GetDecompressedFrame large-frame-count tests ──────────────

// TestGetPixelData_LargeEncapsulatedTotalSizeNoLongerRejected verifies that
// GetPixelData succeeds for encapsulated pixel data even when the theoretical
// uncompressed total size exceeds maxPixelDataBytes. The function should return
// the raw compressed bytes for the requested frame without allocating the full
// uncompressed buffer.
func TestGetPixelData_LargeEncapsulatedTotalSizeNoLongerRejected(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.JPEGBaseline8Bit)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	// 16384×16384 pixels × 3 bytes × 5000 frames ≈ 4 TB uncompressed — well above 512 MiB cap.
	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.NumberOfFrames, "5000")
	obj.Write(tags.Rows, 16384)
	obj.Write(tags.Columns, 16384)
	obj.Write(tags.BitsAllocated, 8)

	compressedBytes := []byte{0xFF, 0xD8, 0x01, 0x02} // mock compressed frame
	obj.Add(&media.DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL", BigEndian: false}) // empty BOT
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: uint32(len(compressedBytes)), VR: "DL", Data: compressedBytes, BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

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
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	frame0Data := []byte{10, 20, 30, 40}
	frame1Data := []byte{50, 60, 70, 80}
	pixelData := append(frame0Data, frame1Data...)

	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.NumberOfFrames, "2")
	obj.Write(tags.Rows, 2)          // rows
	obj.Write(tags.Columns, 2)       // cols
	obj.Write(tags.BitsAllocated, 8) // bits allocated
	obj.Add(&media.DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: uint32(len(pixelData)), VR: "OB", Data: pixelData, BigEndian: false})

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
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	// 512×512 × 8 bits × 8192 frames = 2 GiB uncompressed — exceeds 512 MiB cap.
	const rows, cols, bits, frames = 512, 512, 8, 8192
	framePayload := make([]byte, rows*cols) // one real frame of zeros
	for i := range framePayload {
		framePayload[i] = byte(i % 256)
	}

	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.NumberOfFrames, "8192")
	obj.Write(tags.Rows, rows)
	obj.Write(tags.Columns, cols)
	obj.Write(tags.BitsAllocated, bits)

	obj.Add(&media.DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL", BigEndian: false}) // empty BOT
	// Add only one real fragment — requesting frame 0 should use the single fragment.
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: uint32(len(framePayload)), VR: "DL", Data: framePayload, BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

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
			obj := media.NewEmptyDCMObj()
			obj.SetTransferSyntax(ts)
			obj.SetExplicitVR(ts == transfersyntax.ExplicitVRLittleEndian)
			obj.SetBigEndian(false)

			obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
			obj.Write(tags.NumberOfFrames, "1")
			obj.Write(tags.Rows, rows)
			obj.Write(tags.Columns, cols)
			obj.Write(tags.BitsAllocated, bits)

			obj.Add(&media.DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
			obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL", BigEndian: false}) // empty BOT
			obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: uint32(len(framePayload)), VR: "DL", Data: framePayload, BigEndian: false})
			obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

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

// TestChangeTransferSyntax_NoPixelData verifies that ChangeTransferSyntax
// succeeds for DICOM objects that have no pixel data element (e.g. non-image
// SOP classes such as Structured Reports and Key Object Selection Documents),
// as long as both source and target TSes share the same tag encoding.
func TestChangeTransferSyntax_NoPixelData(t *testing.T) {

	// Build a minimal non-image object (no Pixel Data tag) in EVLE.
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.88.11") // Basic Text SR
	obj.Write(tags.SOPInstanceUID, "1.2.3.4.5")
	obj.Write(tags.PatientID, "P-001")

	// All compressed TSes share explicit-VR little-endian tag encoding, so
	// converting a no-pixel-data object to any of them must succeed.
	compressedTargets := []*transfersyntax.TransferSyntax{
		transfersyntax.JPEGLosslessSV1,
		transfersyntax.JPEGLossless,
		transfersyntax.JPEG2000Lossless,
		transfersyntax.JPEGLSLossless,
		transfersyntax.RLELossless,
	}
	for _, ts := range compressedTargets {
		// Clone obj so each sub-test starts fresh.
		clone := media.NewEmptyDCMObj()
		clone.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
		clone.SetExplicitVR(true)
		clone.SetBigEndian(false)
		clone.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.88.11")
		clone.Write(tags.SOPInstanceUID, "1.2.3.4.5")
		clone.Write(tags.PatientID, "P-001")

		if err := transcoder.ChangeTransferSyntax(clone, ts); err != nil {
			t.Errorf("ChangeTransferSyntax(no-pixel-data, %s) error = %v, want nil", ts.Name, err)
			continue
		}
		if got := clone.GetTransferSyntax(); got == nil || got.UID != ts.UID {
			t.Errorf("after ChangeTransferSyntax(%s): TS = %v, want %s", ts.Name, got, ts.UID)
		}
	}

	// Converting between TSes with different tag encodings (EVLE → ILE) still
	// requires pixel data when both sides are "image" TSes, but for a
	// no-pixel-data object the same-encoding check applies (both are LE).
	// EVLE → ILE changes implicit/explicit VR so it MUST still fail (we cannot
	// re-encode VR names without the data dictionary at this level).
	isCrossEncodingErr := false
	crossEncObj := media.NewEmptyDCMObj()
	crossEncObj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	crossEncObj.SetExplicitVR(true)
	crossEncObj.SetBigEndian(false)
	crossEncObj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.88.11")
	if err := transcoder.ChangeTransferSyntax(crossEncObj, transfersyntax.ImplicitVRLittleEndian); err != nil {
		isCrossEncodingErr = true
	}
	if !isCrossEncodingErr {
		t.Error("ChangeTransferSyntax(EVLE→ILE, no pixel data) should fail for cross-encoding conversion")
	}
}

func TestDICOMObject_TypeSafeWriteOverloads(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)

	// WriteString
	obj.WriteString(tags.PatientID, "P-SAFE-001")
	if got := obj.GetString(tags.PatientID); got != "P-SAFE-001" {
		t.Errorf("WriteString: got %q, want %q", got, "P-SAFE-001")
	}

	// WriteUint16
	obj.WriteUint16(tags.BitsAllocated, 16)
	if got := obj.GetUint16(tags.BitsAllocated); got != 16 {
		t.Errorf("WriteUint16: got %d, want 16", got)
	}

	// WriteUint32 — CommandGroupLength (0000,0000) is a UL tag.
	obj.WriteUint32(tags.CommandGroupLength, 512)
	if got := obj.GetUint32(tags.CommandGroupLength); got != 512 {
		t.Errorf("WriteUint32: got %d, want 512", got)
	}

	// WriteTime (DA tag)
	timeObj := media.NewEmptyDCMObj()
	timeObj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	timeObj.SetExplicitVR(true)
	timeObj.WriteTime(tags.StudyDate, mustParseTime(t, "20060102", "20231015"))
	if got := timeObj.GetString(tags.StudyDate); got != "20231015" {
		t.Errorf("WriteTime(DA): got %q, want %q", got, "20231015")
	}

	// WriteTime (TM tag)
	timeObj.WriteTime(tags.StudyTime, mustParseTime(t, "150405", "143000"))
	if got := timeObj.GetString(tags.StudyTime); got != "143000" {
		t.Errorf("WriteTime(TM): got %q, want %q", got, "143000")
	}

	// WriteDateRange
	dr := media.DateRange{
		Start: mustParseTime(t, "20060102", "20230101"),
		End:   mustParseTime(t, "20060102", "20231231"),
	}
	drObj := media.NewEmptyDCMObj()
	drObj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	drObj.SetExplicitVR(true)
	drObj.WriteDateRange(tags.StudyDate, dr)
	if got := drObj.GetString(tags.StudyDate); got != "20230101-20231231" {
		t.Errorf("WriteDateRange: got %q, want %q", got, "20230101-20231231")
	}
}

func mustParseTime(t *testing.T, layout, value string) time.Time {
	t.Helper()
	tm, err := time.Parse(layout, value)
	if err != nil {
		t.Fatalf("mustParseTime(%q, %q): %v", layout, value, err)
	}
	return tm
}

func TestDICOMObject_GetDateMethod(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetExplicitVR(true)
	obj.WriteString(tags.StudyDate, "20231015")

	got := obj.GetDate(tags.StudyDate)
	want := mustParseTime(t, "20060102", "20231015")
	if !got.Equal(want) {
		t.Errorf("GetDate = %v, want %v", got, want)
	}
}

func TestDICOMObject_GetTimeMethod(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetExplicitVR(true)
	obj.WriteString(tags.StudyTime, "143000")

	got := obj.GetTime(tags.StudyTime)
	want := mustParseTime(t, "150405", "143000")
	if !got.Equal(want) {
		t.Errorf("GetTime = %v, want %v", got, want)
	}
}

func TestDICOMObject_GetDateMethod_ZeroOnMissing(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	if got := obj.GetDate(tags.StudyDate); !got.IsZero() {
		t.Errorf("GetDate on missing tag = %v, want zero", got)
	}
}

func TestDICOMObject_GetTimeMethod_ZeroOnMissing(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	if got := obj.GetTime(tags.StudyTime); !got.IsZero() {
		t.Errorf("GetTime on missing tag = %v, want zero", got)
	}
}

func TestDICOMObject_WriteReadFloat32(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetExplicitVR(true)
	const want float32 = 3.14
	obj.WriteFloat32(tags.SelectorFLValue, want)

	got := obj.GetFloat32(tags.SelectorFLValue)
	if got != want {
		t.Errorf("WriteFloat32/GetFloat32 = %v, want %v", got, want)
	}
}

func TestDICOMObject_WriteReadFloat64(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetExplicitVR(true)
	const want float64 = 2.718281828459045
	obj.WriteFloat64(tags.SelectorFDValue, want)

	got := obj.GetFloat64(tags.SelectorFDValue)
	if got != want {
		t.Errorf("WriteFloat64/GetFloat64 = %v, want %v", got, want)
	}
}

func TestDICOMObject_GetFloat32_ZeroOnMissing(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	if got := obj.GetFloat32(tags.SelectorFLValue); got != 0 {
		t.Errorf("GetFloat32 on missing tag = %v, want 0", got)
	}
}

func TestDICOMObject_GetFloat64_ZeroOnMissing(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	if got := obj.GetFloat64(tags.SelectorFDValue); got != 0 {
		t.Errorf("GetFloat64 on missing tag = %v, want 0", got)
	}
}

func TestDICOMObject_Clone_IndependentData(t *testing.T) {
	orig := media.NewEmptyDCMObj()
	orig.SetExplicitVR(true)
	orig.WriteString(tags.PatientName, "Smith^John")
	orig.WriteUint16(tags.BitsAllocated, 16)

	cloned := orig.Clone()

	// Cloned object should have the same values.
	if got := cloned.GetString(tags.PatientName); got != "Smith^John" {
		t.Errorf("Clone PatientName = %q, want %q", got, "Smith^John")
	}
	if got := cloned.GetUint16(tags.BitsAllocated); got != 16 {
		t.Errorf("Clone BitsAllocated = %d, want 16", got)
	}

	// Mutating clone must not affect original.
	cloned.WriteString(tags.PatientName, "Doe^Jane")
	if got := orig.GetString(tags.PatientName); got != "Smith^John" {
		t.Errorf("Mutation of clone changed original: got %q", got)
	}
}

func TestDICOMObject_Clone_EmptyObject(t *testing.T) {
	orig := media.NewEmptyDCMObj()
	cloned := orig.Clone()
	if cloned.TagCount() != 0 {
		t.Errorf("Clone of empty object has %d tags, want 0", cloned.TagCount())
	}
}

func TestDICOMObject_Clone_PreservesMetadata(t *testing.T) {
	orig := media.NewEmptyDCMObj()
	orig.SetBigEndian(true)
	orig.SetExplicitVR(true)

	cloned := orig.Clone()
	if !cloned.IsBigEndian() {
		t.Error("Clone did not preserve BigEndian=true")
	}
	if !cloned.IsExplicitVR() {
		t.Error("Clone did not preserve ExplicitVR=true")
	}
}

// ── ValidateFileWrite ─────────────────────────────────────────────────────────

func validatableObj() media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.2")
	obj.WriteString(tags.SOPInstanceUID, "1.2.3.4.5")
	return obj
}

func TestValidateFileWrite_OK(t *testing.T) {
	if err := media.ValidateFileWrite(validatableObj()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateFileWrite_MetaConsistency_OK(t *testing.T) {
	obj := validatableObj()
	obj.WriteString(tags.MediaStorageSOPClassUID, "1.2.840.10008.5.1.4.1.1.2")
	obj.WriteString(tags.MediaStorageSOPInstanceUID, "1.2.3.4.5")
	if err := media.ValidateFileWrite(obj); err != nil {
		t.Fatalf("unexpected error with matching meta: %v", err)
	}
}

func TestValidateFileWrite_MetaClassMismatch(t *testing.T) {
	obj := validatableObj()
	obj.WriteString(tags.MediaStorageSOPClassUID, "9.9.9.9")
	if err := media.ValidateFileWrite(obj); err == nil {
		t.Fatal("want error when MediaStorageSOPClassUID mismatches SOPClassUID")
	}
}

func TestValidateFileWrite_MetaInstanceMismatch(t *testing.T) {
	obj := validatableObj()
	obj.WriteString(tags.MediaStorageSOPInstanceUID, "9.9.9.9")
	if err := media.ValidateFileWrite(obj); err == nil {
		t.Fatal("want error when MediaStorageSOPInstanceUID mismatches SOPInstanceUID")
	}
}

// ── DICOMObject.Merge ─────────────────────────────────────────────────────────

func TestDICOMObject_Merge_AddsMissingTags(t *testing.T) {
	dst := media.NewEmptyDCMObj()
	dst.WriteString(tags.PatientName, "Smith^John")

	src := media.NewEmptyDCMObj()
	src.WriteString(tags.PatientID, "12345")
	src.WriteString(tags.PatientName, "OTHER^NAME") // already in dst — should not overwrite

	dst.Merge(src)

	if got := dst.GetString(tags.PatientID); got != "12345" {
		t.Errorf("Merge: PatientID = %q, want %q", got, "12345")
	}
	// PatientName must remain unchanged (not overwritten by src).
	if got := dst.GetString(tags.PatientName); got != "Smith^John" {
		t.Errorf("Merge overwrote existing tag: PatientName = %q, want %q", got, "Smith^John")
	}
}

func TestDICOMObject_Merge_Independent(t *testing.T) {
	dst := media.NewEmptyDCMObj()
	src := media.NewEmptyDCMObj()
	src.WriteString(tags.PatientID, "ABC")

	dst.Merge(src)

	// Mutating src after merge must not affect dst.
	src.WriteString(tags.PatientID, "CHANGED")
	if got := dst.GetString(tags.PatientID); got != "ABC" {
		t.Errorf("Merge: post-merge mutation of src affected dst: got %q", got)
	}
}

func TestDICOMObject_Merge_NilSrc(t *testing.T) {
	dst := media.NewEmptyDCMObj()
	dst.WriteString(tags.PatientName, "Smith^John")
	dst.Merge(nil) // must not panic
	if got := dst.GetString(tags.PatientName); got != "Smith^John" {
		t.Errorf("Merge(nil) changed dst: got %q", got)
	}
}
