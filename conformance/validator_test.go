package conformance

import (
	"testing"
)

func TestValidateDICOMMetadata(t *testing.T) {
	validator := NewConformanceValidator()

	tests := []struct {
		name           string
		metadata       map[string]interface{}
		expectErrors   int
		expectWarnings int
	}{
		{
			name: "Valid DICOM metadata",
			metadata: map[string]interface{}{
				"PatientID":     "12345",
				"PatientName":   "Doe, John",
				"StudyDate":     "20231215",
				"StudyTime":     "140000",
				"SOPClassUID":   "1.2.840.10008.5.1.4.1.1.2",
				"BitsAllocated": float64(16),
				"Rows":          float64(512),
				"Columns":       float64(512),
			},
			expectErrors:   0,
			expectWarnings: 0,
		},
		{
			name: "Missing PatientID",
			metadata: map[string]interface{}{
				"StudyDate":     "20231215",
				"StudyTime":     "140000",
				"SOPClassUID":   "1.2.840.10008.5.1.4.1.1.2",
				"BitsAllocated": float64(16),
			},
			expectErrors:   1,
			expectWarnings: 0,
		},
		{
			name: "Invalid BitsAllocated",
			metadata: map[string]interface{}{
				"PatientID":     "12345",
				"StudyDate":     "20231215",
				"StudyTime":     "140000",
				"SOPClassUID":   "1.2.840.10008.5.1.4.1.1.2",
				"BitsAllocated": float64(12),
			},
			expectErrors:   1,
			expectWarnings: 0,
		},
		{
			name: "Invalid StudyDate format",
			metadata: map[string]interface{}{
				"PatientID":     "12345",
				"StudyDate":     "2023-12-15",
				"StudyTime":     "140000",
				"SOPClassUID":   "1.2.840.10008.5.1.4.1.1.2",
				"BitsAllocated": float64(16),
			},
			expectErrors:   1,
			expectWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := validator.ValidateDICOMMetadata(tt.metadata)
			report := validator.GetConformanceReport(findings)

			errorCount := report["error_count"].(int)
			if errorCount != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d", tt.expectErrors, errorCount)
			}
		})
	}
}

func TestValidatePixelData(t *testing.T) {
	validator := NewConformanceValidator()

	tests := []struct {
		name           string
		pixelData      map[string]interface{}
		expectErrors   int
		expectWarnings int
	}{
		{
			name: "Valid pixel data",
			pixelData: map[string]interface{}{
				"Rows":                      float64(512),
				"Columns":                   float64(512),
				"SamplesPerPixel":           float64(1),
				"BitsAllocated":             float64(16),
				"BitsStored":                float64(12),
				"PhotometricInterpretation": "MONOCHROME2",
			},
			expectErrors:   0,
			expectWarnings: 0,
		},
		{
			name: "Missing Rows",
			pixelData: map[string]interface{}{
				"Columns":         float64(512),
				"SamplesPerPixel": float64(1),
				"BitsAllocated":   float64(16),
				"BitsStored":      float64(12),
			},
			// Rows is Type 1, so its absence is an error rather than a warning.
			expectErrors:   1,
			expectWarnings: 0,
		},
		{
			name:      "Missing all pixel attributes",
			pixelData: map[string]interface{}{},
			// Rows, Columns, SamplesPerPixel and BitsAllocated are Type 1
			// (PS3.3 C.7.6.3): an object carrying Pixel Data without them is not
			// renderable, so their absence is an error. BitsStored remains a
			// warning. This previously reported 0 errors, which meant an object
			// with no pixel geometry at all was certified compliant.
			expectErrors:   4,
			expectWarnings: 1,
		},
		{
			name: "BitsStored exceeds BitsAllocated",
			pixelData: map[string]interface{}{
				"Rows":            float64(512),
				"Columns":         float64(512),
				"SamplesPerPixel": float64(1),
				"BitsAllocated":   float64(8),
				"BitsStored":      float64(16),
			},
			expectErrors:   1,
			expectWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := validator.ValidatePixelData(tt.pixelData)
			report := validator.GetConformanceReport(findings)

			errorCount := report["error_count"].(int)
			if errorCount != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d", tt.expectErrors, errorCount)
			}
			warningCount := report["warning_count"].(int)
			if warningCount != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d", tt.expectWarnings, warningCount)
			}
		})
	}
}

func TestGetConformanceReport(t *testing.T) {
	validator := NewConformanceValidator()

	findings := []Finding{
		{RuleID: "R1", Severity: "error"},
		{RuleID: "R2", Severity: "warning"},
		{RuleID: "R3", Severity: "info"},
	}

	report := validator.GetConformanceReport(findings)

	if report["compliant"].(bool) != false {
		t.Error("Expected compliant to be false with errors")
	}

	if report["status"].(string) != "FAIL" {
		t.Error("Expected status FAIL with errors")
	}

	if report["error_count"].(int) != 1 {
		t.Error("Expected 1 error")
	}

	if report["warning_count"].(int) != 1 {
		t.Error("Expected 1 warning")
	}
}
