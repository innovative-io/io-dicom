package main

import (
	"strings"
	"testing"
)

func TestParsePythonTuple(t *testing.T) {
	tuple := `('LO', '1', "Patient's Name", '', 'PatientsName')`
	values, err := parsePythonTuple(tuple)
	if err != nil {
		t.Fatalf("parsePythonTuple returned error: %v", err)
	}

	if len(values) != 5 {
		t.Fatalf("expected 5 values, got %d", len(values))
	}
	if values[2] != "Patient's Name" {
		t.Fatalf("unexpected description %q", values[2])
	}
	if values[4] != "PatientsName" {
		t.Fatalf("unexpected keyword %q", values[4])
	}
}

func TestParseDicomDictionary(t *testing.T) {
	source := `"""DICOM data dictionary"""
DicomDictionary: dict[int, tuple[str, str, str, str, str]] = {
    0x00020010: ('UI', '1', "Transfer Syntax UID", '', 'TransferSyntaxUID'),
    0x00020012: ('UI', '1', "Implementation Class UID", '', 'ImplementationClassUID'),
}
`

	entries, err := parseDicomDictionary(source)
	if err != nil {
		t.Fatalf("parseDicomDictionary returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Tag != 0x00020010 {
		t.Fatalf("unexpected first tag %#x", entries[0].Tag)
	}
	if entries[1].Keyword != "ImplementationClassUID" {
		t.Fatalf("unexpected second keyword %q", entries[1].Keyword)
	}
}

func TestBuildTransferSyntaxFileFiltersAndSorts(t *testing.T) {
	entries := []uidEntry{
		{UID: "1.2.840.10008.5.1.4.1.1.2", Name: "CT Image Storage", Type: "SOP Class", Keyword: "CTImageStorage"},
		{UID: "1.2.840.10008.1.2.4.50", Name: "JPEG Baseline (Process 1)", Type: "Transfer Syntax", Keyword: "JPEGBaseline8Bit"},
		{UID: "1.2.840.10008.1.2", Name: "Implicit VR Little Endian", Type: "Transfer Syntax", Keyword: "ImplicitVRLittleEndian"},
	}

	content, err := buildTransferSyntaxFile(entries)
	if err != nil {
		t.Fatalf("buildTransferSyntaxFile returned error: %v", err)
	}

	source := string(content)
	if strings.Contains(source, "CTImageStorage") {
		t.Fatal("non-transfer-syntax entry should not be included")
	}
	first := strings.Index(source, "ImplicitVRLittleEndian")
	second := strings.Index(source, "JPEGBaseline8Bit")
	if first < 0 || second < 0 {
		t.Fatalf("expected identifiers in generated source: %s", source)
	}
	if first > second {
		t.Fatal("transfer syntaxes were not sorted by UID")
	}
}

func TestBuildTagsFileDeduplicatesIdentifiers(t *testing.T) {
	entries := []tagEntry{
		{Tag: 0x00100010, VR: "PN", VM: "1", Description: "Patient Name", Keyword: "PatientName"},
		{Tag: 0x00100020, VR: "LO", VM: "1", Description: "Patient ID", Keyword: "PatientName"},
	}

	content, err := buildTagsFile(entries)
	if err != nil {
		t.Fatalf("buildTagsFile returned error: %v", err)
	}

	source := string(content)
	if !strings.Contains(source, "var PatientName = &Tag{") {
		t.Fatalf("expected original identifier in source: %s", source)
	}
	if !strings.Contains(source, "var PatientName00100020 = &Tag{") {
		t.Fatalf("expected deduplicated identifier in source: %s", source)
	}
	if !strings.Contains(source, "PatientName00100020,") {
		t.Fatalf("expected deduplicated identifier in tags slice: %s", source)
	}
}
