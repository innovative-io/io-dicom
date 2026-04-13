package media

import (
	"errors"
	"os"
	"testing"
)

// sampleFilesForIndex are relative paths to sample files used for index tests.
// All exist in the samples/ directory checked in alongside the tests.
var sampleFilesForIndex = []string{
	"../samples/jpeg8.dcm",
	"../samples/test2.dcm",
	"../samples/cornerstone-CTImage-explicit-le.dcm",
	"../samples/cornerstone-CTImage-implicit-le.dcm",
	"../samples/cornerstone-CTImage-big-endian-explicit.dcm",
	"../samples/cornerstone-CTImage-jpeg2000.dcm",
	"../samples/cornerstone-CTImage-jpegls-lossless.dcm",
	"../samples/cornerstone-CTImage-rle-lossless.dcm",
	"../samples/highdicom-ct_image.dcm",
}

// TestParseIndexFromFile_Smoke verifies basic success on well-known sample files.
func TestParseIndexFromFile_Smoke(t *testing.T) {
	for _, path := range sampleFilesForIndex {
		t.Run(path, func(t *testing.T) {
			rec, err := ParseIndexFromFile(path)
			if err != nil {
				t.Fatalf("ParseIndexFromFile(%q): %v", path, err)
			}
			if rec == nil {
				t.Fatalf("ParseIndexFromFile(%q): returned nil record", path)
			}
			if rec.FilePath != path {
				t.Errorf("FilePath: got %q, want %q", rec.FilePath, path)
			}
			if rec.TransferSyntaxUID == "" {
				t.Errorf("TransferSyntaxUID is empty for %q", path)
			}
		})
	}
}

// TestParseIndexFromBytes_Smoke verifies ParseIndexFromBytes on the same set.
func TestParseIndexFromBytes_Smoke(t *testing.T) {
	for _, path := range sampleFilesForIndex {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q): %v", path, err)
			}
			rec, err := ParseIndexFromBytes(data)
			if err != nil {
				t.Fatalf("ParseIndexFromBytes(%q): %v", path, err)
			}
			if rec == nil {
				t.Fatalf("ParseIndexFromBytes(%q): returned nil record", path)
			}
			if rec.TransferSyntaxUID == "" {
				t.Errorf("TransferSyntaxUID is empty for %q", path)
			}
		})
	}
}

// TestParseIndexMatchesFullParse is the correctness test: for each sample file
// it compares every IndexRecord field against the corresponding tag extracted
// by a full NewDCMObjFromFile parse.
func TestParseIndexMatchesFullParse(t *testing.T) {
	for _, path := range sampleFilesForIndex {
		t.Run(path, func(t *testing.T) {
			rec, err := ParseIndexFromFile(path)
			if err != nil {
				t.Fatalf("ParseIndexFromFile: %v", err)
			}

			obj, err := NewDCMObjFromFile(path)
			if err != nil {
				t.Fatalf("NewDCMObjFromFile: %v", err)
			}

			assertField := func(fieldName, got, group, elem string) {
				t.Helper()
				want := obj.GetStringGE(parseHex(t, group), parseHex(t, elem))
				if got != want {
					t.Errorf("%s: ParseIndex=%q, FullParse=%q", fieldName, got, want)
				}
			}

			// Transfer syntax comes from group 0002 — compare against TS UID.
			if want := obj.GetTransferSyntax(); want != nil && rec.TransferSyntaxUID != want.UID {
				t.Errorf("TransferSyntaxUID: ParseIndex=%q, FullParse=%q", rec.TransferSyntaxUID, want.UID)
			}

			assertField("SOPClassUID", rec.SOPClassUID, "0008", "0016")
			assertField("SOPInstanceUID", rec.SOPInstanceUID, "0008", "0018")
			assertField("StudyDate", rec.StudyDate, "0008", "0020")
			assertField("SeriesDate", rec.SeriesDate, "0008", "0021")
			assertField("StudyTime", rec.StudyTime, "0008", "0030")
			assertField("AccessionNumber", rec.AccessionNumber, "0008", "0050")
			assertField("Modality", rec.Modality, "0008", "0060")
			assertField("StudyDescription", rec.StudyDescription, "0008", "1030")
			assertField("SeriesDescription", rec.SeriesDescription, "0008", "103E")
			assertField("PatientName", rec.PatientName, "0010", "0010")
			assertField("PatientID", rec.PatientID, "0010", "0020")
			assertField("PatientBirthDate", rec.PatientBirthDate, "0010", "0030")
			assertField("PatientSex", rec.PatientSex, "0010", "0040")
			assertField("StudyInstanceUID", rec.StudyInstanceUID, "0020", "000D")
			assertField("SeriesInstanceUID", rec.SeriesInstanceUID, "0020", "000E")
			assertField("SeriesNumber", rec.SeriesNumber, "0020", "0011")
			assertField("InstanceNumber", rec.InstanceNumber, "0020", "0013")
		})
	}
}

// TestParseIndexFromFile_NotDICOM ensures ErrNotDICOM is returned for non-DICOM data.
func TestParseIndexFromFile_NotDICOM(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "notdicom*.bin")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Write(make([]byte, 256)) // all-zero file, no DICM magic
	tmp.Close()

	_, err = ParseIndexFromFile(tmp.Name())
	if err == nil {
		t.Fatal("expected error for non-DICOM file, got nil")
	}
	if !errors.Is(err, ErrNotDICOM) {
		t.Errorf("expected ErrNotDICOM in error chain, got: %v", err)
	}
}

// TestParseIndexFromBytes_TooSmall ensures an error is returned for tiny payloads.
func TestParseIndexFromBytes_TooSmall(t *testing.T) {
	_, err := ParseIndexFromBytes([]byte("tiny"))
	if err == nil {
		t.Fatal("expected error for undersized payload")
	}
}

// TestParseIndexFromBytes_FileBytesMatchFile verifies that ParseIndexFromBytes
// and ParseIndexFromFile produce identical records for the same file.
func TestParseIndexFromBytes_FileBytesMatchFile(t *testing.T) {
	path := "../samples/jpeg8.dcm"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	recFile, err := ParseIndexFromFile(path)
	if err != nil {
		t.Fatalf("ParseIndexFromFile: %v", err)
	}
	recBytes, err := ParseIndexFromBytes(data)
	if err != nil {
		t.Fatalf("ParseIndexFromBytes: %v", err)
	}

	// FilePath differs by design (ParseIndexFromBytes doesn't know the path).
	recBytes.FilePath = path

	if *recFile != *recBytes {
		t.Errorf("mismatch:\n  file:  %+v\n  bytes: %+v", recFile, recBytes)
	}
}

// parseHex converts a 4-character hex string to uint16 for tag lookups in tests.
func parseHex(t *testing.T, s string) uint16 {
	t.Helper()
	var v uint16
	for _, c := range s {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= uint16(c - '0')
		case c >= 'a' && c <= 'f':
			v |= uint16(c-'a') + 10
		case c >= 'A' && c <= 'F':
			v |= uint16(c-'A') + 10
		default:
			t.Fatalf("invalid hex char %q in %q", c, s)
		}
	}
	return v
}
