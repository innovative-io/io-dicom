package media

import (
	"errors"
	"os"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
)

// sampleFilesForIndex are relative paths to sample files used for index tests.
// All exist in the testdata/ directory checked in alongside the tests.
var sampleFilesForIndex = []string{
	"../testdata/jpeg8.dcm",
	"../testdata/test2.dcm",
	"../testdata/cornerstone-CTImage-explicit-le.dcm",
	"../testdata/cornerstone-CTImage-implicit-le.dcm",
	"../testdata/cornerstone-CTImage-big-endian-explicit.dcm",
	"../testdata/cornerstone-CTImage-jpeg2000.dcm",
	"../testdata/cornerstone-CTImage-jpegls-lossless.dcm",
	"../testdata/cornerstone-CTImage-rle-lossless.dcm",
	"../testdata/highdicom-ct_image.dcm",
}

// TestParseIndexFromFile_Smoke verifies basic success on well-known sample files.
func TestParseIndexFromFile_Smoke(t *testing.T) {
	if _, err := os.Stat(sampleFilesForIndex[0]); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}
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
	if _, err := os.Stat(sampleFilesForIndex[0]); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}
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
	if _, err := os.Stat(sampleFilesForIndex[0]); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}
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

			assertField := func(fieldName, got string, tag *tags.Tag) {
				t.Helper()
				want := obj.GetString(tag)
				if got != want {
					t.Errorf("%s: ParseIndex=%q, FullParse=%q", fieldName, got, want)
				}
			}

			// Transfer syntax comes from group 0002 — compare against TS UID.
			if want := obj.GetTransferSyntax(); want != nil && rec.TransferSyntaxUID != want.UID {
				t.Errorf("TransferSyntaxUID: ParseIndex=%q, FullParse=%q", rec.TransferSyntaxUID, want.UID)
			}

			assertField("SOPClassUID", rec.SOPClassUID, tags.SOPClassUID)
			assertField("SOPInstanceUID", rec.SOPInstanceUID, tags.SOPInstanceUID)
			assertField("StudyDate", rec.StudyDate, tags.StudyDate)
			assertField("SeriesDate", rec.SeriesDate, tags.SeriesDate)
			assertField("StudyTime", rec.StudyTime, tags.StudyTime)
			assertField("AccessionNumber", rec.AccessionNumber, tags.AccessionNumber)
			assertField("Modality", rec.Modality, tags.Modality)
			assertField("StudyDescription", rec.StudyDescription, tags.StudyDescription)
			assertField("SeriesDescription", rec.SeriesDescription, tags.SeriesDescription)
			assertField("PatientName", rec.PatientName, tags.PatientName)
			assertField("PatientID", rec.PatientID, tags.PatientID)
			assertField("PatientBirthDate", rec.PatientBirthDate, tags.PatientBirthDate)
			assertField("PatientSex", rec.PatientSex, tags.PatientSex)
			assertField("StudyInstanceUID", rec.StudyInstanceUID, tags.StudyInstanceUID)
			assertField("SeriesInstanceUID", rec.SeriesInstanceUID, tags.SeriesInstanceUID)
			assertField("SeriesNumber", rec.SeriesNumber, tags.SeriesNumber)
			assertField("InstanceNumber", rec.InstanceNumber, tags.InstanceNumber)
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
	path := "../testdata/jpeg8.dcm"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("sample fixtures unavailable: %v", err)
	}
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
