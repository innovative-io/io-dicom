package conformance

import (
	"fmt"
	"regexp"
	"strings"
)

// ConformanceValidator validates DICOM files for standards compliance
type ConformanceValidator struct{}

// Finding represents a compliance validation finding
type Finding struct {
	RuleID      string
	Severity    string // "error", "warning", "info"
	Element     string
	Expected    string
	Actual      string
	Description string
}

// NewConformanceValidator creates a new validator instance
func NewConformanceValidator() *ConformanceValidator {
	return &ConformanceValidator{}
}

// ValidateDICOMMetadata validates DICOM metadata for conformance
func (cv *ConformanceValidator) ValidateDICOMMetadata(metadata map[string]interface{}) []Finding {
	findings := []Finding{}

	// Rule 1: Patient ID (mandatory)
	if patientID, ok := metadata["PatientID"].(string); !ok || patientID == "" {
		findings = append(findings, Finding{
			RuleID:      "DICOM-001",
			Severity:    "error",
			Element:     "Patient ID (0010,0020)",
			Expected:    "non-empty string",
			Actual:      "missing or empty",
			Description: "Patient ID is mandatory in DICOM metadata",
		})
	}

	// Rule 2: Study Date format (mandatory, YYYYMMDD)
	if studyDate, ok := metadata["StudyDate"].(string); !ok || studyDate == "" {
		findings = append(findings, Finding{
			RuleID:      "DICOM-002",
			Severity:    "error",
			Element:     "Study Date (0008,0020)",
			Expected:    "YYYYMMDD format",
			Actual:      "missing",
			Description: "Study Date is mandatory and must be in YYYYMMDD format",
		})
	} else if !isValidDate(studyDate) {
		findings = append(findings, Finding{
			RuleID:      "DICOM-002",
			Severity:    "error",
			Element:     "Study Date (0008,0020)",
			Expected:    "YYYYMMDD format",
			Actual:      studyDate,
			Description: "Study Date must be in YYYYMMDD format (e.g., 20231215)",
		})
	}

	// Rule 3: Study Time format (mandatory, HHMMSS)
	if studyTime, ok := metadata["StudyTime"].(string); !ok || studyTime == "" {
		findings = append(findings, Finding{
			RuleID:      "DICOM-003",
			Severity:    "error",
			Element:     "Study Time (0008,0030)",
			Expected:    "HHMMSS format",
			Actual:      "missing",
			Description: "Study Time is mandatory and must be in HHMMSS format",
		})
	} else if !isValidTime(studyTime) {
		findings = append(findings, Finding{
			RuleID:      "DICOM-003",
			Severity:    "error",
			Element:     "Study Time (0008,0030)",
			Expected:    "HHMMSS format",
			Actual:      studyTime,
			Description: "Study Time must be in HHMMSS format (e.g., 140000)",
		})
	}

	// Rule 4: SOP Class UID (mandatory, must start with 1.2.840.10008)
	if sopClassUID, ok := metadata["SOPClassUID"].(string); !ok || sopClassUID == "" {
		findings = append(findings, Finding{
			RuleID:      "DICOM-004",
			Severity:    "error",
			Element:     "SOP Class UID (0008,0016)",
			Expected:    "1.2.840.10008.* UID",
			Actual:      "missing",
			Description: "SOP Class UID is mandatory and must be a valid DICOM UID",
		})
	} else if !strings.HasPrefix(sopClassUID, "1.2.840.10008") {
		findings = append(findings, Finding{
			RuleID:      "DICOM-004",
			Severity:    "error",
			Element:     "SOP Class UID (0008,0016)",
			Expected:    "1.2.840.10008.* UID",
			Actual:      sopClassUID,
			Description: "SOP Class UID must be a valid DICOM UID starting with 1.2.840.10008",
		})
	}

	// Rule 5: Bits Allocated (mandatory, must be 8, 16, or 32)
	if bitsAlloc, ok := metadata["BitsAllocated"].(float64); !ok {
		findings = append(findings, Finding{
			RuleID:      "DICOM-005",
			Severity:    "error",
			Element:     "Bits Allocated (0028,0100)",
			Expected:    "8, 16, or 32",
			Actual:      "missing or invalid type",
			Description: "Bits Allocated is mandatory and must be 8, 16, or 32",
		})
	} else if bitsAlloc != 8 && bitsAlloc != 16 && bitsAlloc != 32 {
		findings = append(findings, Finding{
			RuleID:      "DICOM-005",
			Severity:    "error",
			Element:     "Bits Allocated (0028,0100)",
			Expected:    "8, 16, or 32",
			Actual:      fmt.Sprintf("%.0f", bitsAlloc),
			Description: "Bits Allocated must be 8, 16, or 32",
		})
	}

	// Rule 6: Patient Name (recommended)
	if patientName, ok := metadata["PatientName"].(string); !ok || patientName == "" {
		findings = append(findings, Finding{
			RuleID:      "DICOM-006",
			Severity:    "warning",
			Element:     "Patient Name (0010,0010)",
			Expected:    "non-empty string",
			Actual:      "missing or empty",
			Description: "Patient Name is recommended but may be unavailable due to privacy",
		})
	}

	// Rule 7: Modalities (recommended)
	if modalities, ok := metadata["Modalities"].(string); !ok || modalities == "" {
		findings = append(findings, Finding{
			RuleID:      "DICOM-007",
			Severity:    "warning",
			Element:     "Modalities (0008,0061)",
			Expected:    "modality code (CT, MR, US, etc.)",
			Actual:      "missing or empty",
			Description: "Modalities is recommended for clinical workflow",
		})
	}

	// Rule 8: Image Dimensions (recommended, 64-8192 pixels)
	if rows, rowsOK := metadata["Rows"].(float64); rowsOK {
		if cols, colsOK := metadata["Columns"].(float64); colsOK {
			if rows < 64 || rows > 8192 || cols < 64 || cols > 8192 {
				findings = append(findings, Finding{
					RuleID:      "DICOM-008",
					Severity:    "warning",
					Element:     "Image Dimensions (Rows/Columns)",
					Expected:    "64-8192 pixels per dimension",
					Actual:      fmt.Sprintf("%.0f x %.0f", rows, cols),
					Description: "Image dimensions outside typical range may indicate compression or quality issues",
				})
			}
		}
	}

	// Rule 9: Accession Number (optional)
	if accNum, ok := metadata["AccessionNumber"].(string); !ok || accNum == "" {
		findings = append(findings, Finding{
			RuleID:      "DICOM-009",
			Severity:    "info",
			Element:     "Accession Number (0008,0050)",
			Expected:    "procedure accession number",
			Actual:      "not provided",
			Description: "Accession Number connects study to hospital information system workflow",
		})
	}

	// Rule 10: Referring Physician (optional)
	if refPhys, ok := metadata["ReferringPhysician"].(string); !ok || refPhys == "" {
		findings = append(findings, Finding{
			RuleID:      "DICOM-010",
			Severity:    "info",
			Element:     "Referring Physician (0008,0090)",
			Expected:    "physician name",
			Actual:      "not provided",
			Description: "Referring Physician facilitates clinical communication",
		})
	}

	// Rule 11: Color space (recommended for color images)
	if photoInterp, ok := metadata["PhotometricInterpretation"].(string); ok && isColorModality(photoInterp) {
		if _, hasICCProfile := metadata["ICCProfile"]; !hasICCProfile {
			findings = append(findings, Finding{
				RuleID:      "DICOM-011",
				Severity:    "warning",
				Element:     "ICC Profile (optional for RGB)",
				Expected:    "ICC color profile for consistent rendering",
				Actual:      "not provided",
				Description: "ICC Profile recommended for proper color image rendering",
			})
		}
	}

	return findings
}

// ValidatePixelData validates DICOM pixel data conformance
func (cv *ConformanceValidator) ValidatePixelData(pixelData map[string]interface{}) []Finding {
	findings := []Finding{}

	// Rule P1: Rows and Columns present
	if rows, ok := pixelData["Rows"].(float64); !ok || rows == 0 {
		findings = append(findings, Finding{
			RuleID:      "DICOM-P001",
			Severity:    "error",
			Element:     "Rows (0028,0010)",
			Expected:    "positive integer",
			Actual:      "missing or zero",
			Description: "Rows must be specified for pixel data",
		})
	}

	if cols, ok := pixelData["Columns"].(float64); !ok || cols == 0 {
		findings = append(findings, Finding{
			RuleID:      "DICOM-P002",
			Severity:    "error",
			Element:     "Columns (0028,0011)",
			Expected:    "positive integer",
			Actual:      "missing or zero",
			Description: "Columns must be specified for pixel data",
		})
	}

	// Rule P2: Bits Allocated consistency with Bits Stored
	if bitsAlloc, allocOK := pixelData["BitsAllocated"].(float64); allocOK {
		if bitsStored, storedOK := pixelData["BitsStored"].(float64); storedOK {
			if bitsStored > bitsAlloc {
				findings = append(findings, Finding{
					RuleID:      "DICOM-P003",
					Severity:    "error",
					Element:     "Bits Stored (0028,0101)",
					Expected:    fmt.Sprintf("<= %.0f bits", bitsAlloc),
					Actual:      fmt.Sprintf("%.0f bits", bitsStored),
					Description: "Bits Stored cannot exceed Bits Allocated",
				})
			}
		}
	}

	// Rule P3: Photometric Interpretation valid
	if photoInterp, ok := pixelData["PhotometricInterpretation"].(string); ok {
		validInterps := map[string]bool{
			"MONOCHROME1": true,
			"MONOCHROME2": true,
			"RGB":         true,
			"YBR_FULL":    true,
			"YBR_PARTIAL": true,
			"YBR_ICT":     true,
			"YBR_RCT":     true,
		}
		if !validInterps[photoInterp] {
			findings = append(findings, Finding{
				RuleID:      "DICOM-P004",
				Severity:    "warning",
				Element:     "Photometric Interpretation (0028,0004)",
				Expected:    "standard DICOM photometric value",
				Actual:      photoInterp,
				Description: "Non-standard photometric interpretation may cause display issues",
			})
		}
	}

	return findings
}

// GetConformanceReport generates a compliance report from findings
func (cv *ConformanceValidator) GetConformanceReport(findings []Finding) map[string]interface{} {
	errorCount := 0
	warningCount := 0
	infoCount := 0

	for _, f := range findings {
		switch f.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		}
	}

	compliant := errorCount == 0
	complianceScore := 1.0
	if len(findings) > 0 {
		complianceScore = float64(100-errorCount*10-warningCount*3) / 100.0
		if complianceScore < 0 {
			complianceScore = 0
		}
	}

	status := "PASS"
	if !compliant {
		status = "FAIL"
	}

	return map[string]interface{}{
		"compliant":        compliant,
		"status":           status,
		"compliance_score": complianceScore,
		"error_count":      errorCount,
		"warning_count":    warningCount,
		"info_count":       infoCount,
		"total_findings":   len(findings),
		"findings":         findings,
	}
}

// Helper functions

func isValidDate(dateStr string) bool {
	if len(dateStr) != 8 {
		return false
	}
	matched, _ := regexp.MatchString(`^\d{8}$`, dateStr)
	return matched
}

func isValidTime(timeStr string) bool {
	if len(timeStr) < 6 || len(timeStr) > 14 {
		return false
	}
	matched, _ := regexp.MatchString(`^\d{6}`, timeStr)
	return matched
}

func isColorModality(photoInterp string) bool {
	colorModes := map[string]bool{
		"RGB":      true,
		"YBR_FULL": true,
		"YBR_ICT":  true,
		"YBR_RCT":  true,
	}
	return colorModes[photoInterp]
}
