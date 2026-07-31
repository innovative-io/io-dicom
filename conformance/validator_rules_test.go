package conformance

import (
	"testing"
)

// report runs the pixel-data rules and returns the summary counts.
func pixelReport(t *testing.T, pd map[string]interface{}) (errors int, compliant bool) {
	t.Helper()
	v := NewConformanceValidator()
	rep := v.GetConformanceReport(v.ValidatePixelData(pd))
	return rep["error_count"].(int), rep["compliant"].(bool)
}

// TestImpossiblePixelGeometryIsNotCompliant is the headline correction: the
// validator used to return compliant/PASS for geometry that cannot describe an
// image. Rows was tested with `== 0`, so negative values passed; SamplesPerPixel
// and BitsAllocated were presence-only despite advertising value constraints.
//
// {Rows:-5, Columns:1e18} is exactly the shape that overflows a downstream
// rows*cols*bits computation, and this validator blessed it.
func TestImpossiblePixelGeometryIsNotCompliant(t *testing.T) {
	cases := []struct {
		name string
		pd   map[string]interface{}
	}{
		{"negative rows and absurd columns", map[string]interface{}{
			"Rows": float64(-5), "Columns": float64(1e18),
			"SamplesPerPixel": float64(7), "BitsAllocated": float64(3),
			"BitsStored": float64(3), "PhotometricInterpretation": "MONOCHROME2",
		}},
		{"zero rows", map[string]interface{}{
			"Rows": float64(0), "Columns": float64(512),
			"SamplesPerPixel": float64(1), "BitsAllocated": float64(16),
			"BitsStored": float64(12),
		}},
		{"samples per pixel of 7", map[string]interface{}{
			"Rows": float64(512), "Columns": float64(512),
			"SamplesPerPixel": float64(7), "BitsAllocated": float64(8),
			"BitsStored": float64(8),
		}},
		{"bits allocated of 3", map[string]interface{}{
			"Rows": float64(512), "Columns": float64(512),
			"SamplesPerPixel": float64(1), "BitsAllocated": float64(3),
			"BitsStored": float64(3),
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs, compliant := pixelReport(t, tc.pd)
			if compliant || errs == 0 {
				t.Fatalf("impossible geometry reported compliant=%v with %d errors", compliant, errs)
			}
		})
	}
}

// TestLegalPixelGeometryStillPasses guards the tightened rules from rejecting
// real objects, including the 1-bit and 64-bit cases PS3.5 §8.1.1 permits.
func TestLegalPixelGeometryStillPasses(t *testing.T) {
	cases := []struct {
		name string
		pd   map[string]interface{}
	}{
		{"8-bit grayscale", map[string]interface{}{
			"Rows": float64(512), "Columns": float64(512),
			"SamplesPerPixel": float64(1), "BitsAllocated": float64(8),
			"BitsStored": float64(8), "PhotometricInterpretation": "MONOCHROME2",
		}},
		{"16-bit grayscale", map[string]interface{}{
			"Rows": float64(512), "Columns": float64(512),
			"SamplesPerPixel": float64(1), "BitsAllocated": float64(16),
			"BitsStored": float64(12), "PhotometricInterpretation": "MONOCHROME1",
		}},
		{"1-bit segmentation", map[string]interface{}{
			"Rows": float64(256), "Columns": float64(256),
			"SamplesPerPixel": float64(1), "BitsAllocated": float64(1),
			"BitsStored": float64(1), "PhotometricInterpretation": "MONOCHROME2",
		}},
		{"RGB colour", map[string]interface{}{
			"Rows": float64(640), "Columns": float64(480),
			"SamplesPerPixel": float64(3), "BitsAllocated": float64(8),
			"BitsStored": float64(8), "PhotometricInterpretation": "RGB",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if errs, compliant := pixelReport(t, tc.pd); !compliant || errs != 0 {
				t.Fatalf("valid geometry rejected: compliant=%v errors=%d", compliant, errs)
			}
		})
	}
}

// TestPhotometricInterpretationEnumeration covers the values PS3.3 C.7.6.3.1.2
// enumerates. The previous whitelist omitted four of them — flagging the common
// JPEG colour case YBR_FULL_422 as non-conformant — while accepting
// "YBR_PARTIAL", which is not an enumerated value.
func TestPhotometricInterpretationEnumeration(t *testing.T) {
	base := func(pi string) map[string]interface{} {
		return map[string]interface{}{
			"Rows": float64(64), "Columns": float64(64),
			"SamplesPerPixel": float64(3), "BitsAllocated": float64(8),
			"BitsStored": float64(8), "PhotometricInterpretation": pi,
		}
	}
	for _, pi := range []string{
		"MONOCHROME1", "MONOCHROME2", "PALETTE COLOR", "RGB",
		"YBR_FULL", "YBR_FULL_422", "YBR_PARTIAL_422", "YBR_PARTIAL_420",
		"YBR_ICT", "YBR_RCT",
	} {
		v := NewConformanceValidator()
		for _, f := range v.ValidatePixelData(base(pi)) {
			if f.RuleID == "DICOM-P007" {
				t.Errorf("standard photometric interpretation %q was flagged", pi)
			}
		}
	}
	// Not an enumerated value; it must be flagged.
	v := NewConformanceValidator()
	flagged := false
	for _, f := range v.ValidatePixelData(base("YBR_PARTIAL")) {
		if f.RuleID == "DICOM-P007" {
			flagged = true
		}
	}
	if !flagged {
		t.Error(`"YBR_PARTIAL" is not an enumerated value but was accepted`)
	}
}

// TestUIDSyntaxValidation covers the SOP Class rule. The old prefix test had no
// separating dot, so a different registration arc passed, and no rule checked
// UID syntax at all — a path-traversal string was certified conformant.
func TestUIDSyntaxValidation(t *testing.T) {
	invalid := []string{
		"1.2.840.10008/../../../etc/passwd",
		"1.2.840.10008EVIL",
		"not-a-uid",
		"1.2.3.",
		"",
	}
	for _, uid := range invalid {
		if isValidUID(uid) {
			t.Errorf("invalid UID %q accepted", uid)
		}
	}
	valid := []string{
		"1.2.840.10008.5.1.4.1.1.2",
		"1.3.46.670589.11.0.0.12.1",    // Philips private
		"1.2.826.0.1.3680043.2.1125.1", // Medical Connections private
		"1",
	}
	for _, uid := range valid {
		if !isValidUID(uid) {
			t.Errorf("valid UID %q rejected", uid)
		}
	}
	// A UID over 64 characters is invalid per PS3.5 §9.1.
	long := "1." + string(make([]byte, 0))
	for len(long) <= 64 {
		long += "1"
	}
	if isValidUID(long) {
		t.Errorf("UID of %d characters accepted", len(long))
	}
}

// TestPrivateSOPClassIsNotAnError pins the correction that private and vendor
// SOP Classes are legal DICOM. Reporting them as errors gave a blanket FAIL to
// any archive holding vendor-private objects.
func TestPrivateSOPClassIsNotAnError(t *testing.T) {
	v := NewConformanceValidator()
	findings := v.ValidateDICOMMetadata(map[string]interface{}{
		"SOPClassUID": "1.3.46.670589.11.0.0.12.1",
	})
	for _, f := range findings {
		if f.RuleID == "DICOM-004" && f.Severity == "error" {
			t.Fatalf("a legal private SOP Class was reported as an error: %s", f.Description)
		}
	}
}

// TestDateAndTimeValidation covers the anchoring and range fixes. The time
// pattern was unanchored at the end, so any 6..14 byte value starting with six
// digits was "valid"; the date pattern accepted impossible calendar values.
func TestDateAndTimeValidation(t *testing.T) {
	for _, bad := range []string{"123456<script>", "1234567890abcd", "999999", "246199", "12345"} {
		if isValidTime(bad) {
			t.Errorf("invalid time %q accepted", bad)
		}
	}
	for _, good := range []string{"000000", "235959", "120000.123456", "235960"} {
		if !isValidTime(good) {
			t.Errorf("valid time %q rejected", good)
		}
	}
	for _, bad := range []string{"99999999", "00000000", "20231345", "2023121"} {
		if isValidDate(bad) {
			t.Errorf("invalid date %q accepted", bad)
		}
	}
	for _, good := range []string{"20231215", "20000229"} { // 2000 was a leap year
		if !isValidDate(good) {
			t.Errorf("valid date %q rejected", good)
		}
	}
}

// TestUnknownSeverityFailsClosed pins the severity switch default. An
// unrecognised severity previously counted toward nothing, so it could never
// fail a verdict while still appearing in total_findings.
func TestUnknownSeverityFailsClosed(t *testing.T) {
	v := NewConformanceValidator()
	rep := v.GetConformanceReport([]Finding{
		{RuleID: "X1", Severity: "ERROR"},    // wrong case
		{RuleID: "X2", Severity: "critical"}, // unknown level
	})
	if rep["compliant"].(bool) {
		t.Fatalf("unknown severities were treated as non-blocking: %v", rep)
	}
}
