package transfersyntax

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func conformanceEntryByUID(uid string) (ConformanceExpectation, bool) {
	for _, entry := range ConformanceMatrix {
		if entry.TransferSyntax != nil && entry.TransferSyntax.UID == uid {
			return entry, true
		}
	}
	return ConformanceExpectation{}, false
}

func TestTransferSyntaxConformanceMatrixMatchesSupportGate(t *testing.T) {
	seen := make(map[string]ConformanceExpectation, len(ConformanceMatrix))

	for _, entry := range ConformanceMatrix {
		if entry.TransferSyntax == nil {
			t.Fatal("conformance matrix contains nil transfer syntax")
		}
		if entry.Status == "" {
			t.Fatalf("conformance matrix entry %q is missing a status", entry.TransferSyntax.UID)
		}
		if entry.Note == "" {
			t.Fatalf("conformance matrix entry %q is missing a note", entry.TransferSyntax.UID)
		}
		if _, exists := seen[entry.TransferSyntax.UID]; exists {
			t.Fatalf("duplicate conformance matrix entry for %s", entry.TransferSyntax.UID)
		}
		seen[entry.TransferSyntax.UID] = entry

		got := SupportedTransferSyntax(entry.TransferSyntax.UID)
		if got != entry.Supported {
			t.Fatalf("SupportedTransferSyntax(%s) = %v, want %v", entry.TransferSyntax.UID, got, entry.Supported)
		}
	}

	for _, transferSyntax := range supportedTransferSyntaxes {
		entry, exists := seen[transferSyntax.UID]
		if !exists {
			t.Fatalf("supported transfer syntax %s missing from conformance matrix", transferSyntax.UID)
		}
		if !entry.Supported {
			t.Fatalf("supported transfer syntax %s marked unsupported in conformance matrix", transferSyntax.UID)
		}
	}
}

func TestTransferSyntaxConformanceMatrixCoversKnownTransferSyntaxes(t *testing.T) {
	seen := make(map[string]struct{}, len(ConformanceMatrix))
	for _, entry := range ConformanceMatrix {
		seen[entry.TransferSyntax.UID] = struct{}{}
	}

	for _, transferSyntax := range transferSyntaxes {
		if _, exists := seen[transferSyntax.UID]; !exists {
			t.Fatalf("transfer syntax %s missing from conformance matrix", transferSyntax.UID)
		}
	}
}

func TestTransferSyntaxSupportMatrixDocumentIsGeneratedFromSource(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current test file path")
	}

	docPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "docs", "transfer-syntax-support-matrix.md")
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("failed to read support matrix document: %v", err)
	}

	want := RenderSupportMatrixMarkdown()
	if got := strings.ReplaceAll(string(content), "\r\n", "\n"); got != want {
		t.Fatalf("transfer syntax support matrix doc is out of date; regenerate %s", docPath)
	}
}

func TestTransferSyntaxBehavioralSummaryDocumentIsGeneratedFromSource(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current test file path")
	}

	docPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "docs", "transfer-syntax-behavioral-summary.md")
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("failed to read behavioral summary document: %v", err)
	}

	want := RenderBehavioralSummaryMarkdown()
	if got := strings.ReplaceAll(string(content), "\r\n", "\n"); got != want {
		t.Fatalf("transfer syntax behavioral summary doc is out of date; regenerate %s", docPath)
	}
}

func TestRepresentativeSyntaxesDeclareBehavioralQuality(t *testing.T) {
	requiredPassthrough := []string{
		EncapsulatedUncompressedExplicitVRLittleEndian.UID,
		DeflatedImageFrameCompression.UID,
		RLELossless.UID,
		JPEGBaseline8Bit.UID,
		JPEGLosslessSV1.UID,
		JPEGExtended12Bit.UID,
		JPEGLSLossless.UID,
		JPEG2000Lossless.UID,
		JPEGXL.UID,
		JPEGXLLossless.UID,
		JPIPHTJ2KReferenced.UID,
		MPEG2MPML.UID,
		SMPTEST211020UncompressedProgressiveActiveVideo.UID,
	}

	requiredNative := []string{
		JPEGExtended12Bit.UID,
		JPEGLSLossless.UID,
		JPEG2000Lossless.UID,
		JPEGXLLossless.UID,
		JPIPHTJ2KReferenced.UID,
		MPEG2MPML.UID,
		SMPTEST211020UncompressedProgressiveActiveVideo.UID,
	}

	for _, uid := range requiredPassthrough {
		entry, exists := conformanceEntryByUID(uid)
		if !exists {
			t.Fatalf("missing conformance matrix entry for %s", uid)
		}
		if !entry.Supported {
			t.Fatalf("expected representative passthrough syntax %s to be supported", uid)
		}
		if entry.PassthroughBehavioralQuality == "" {
			t.Fatalf("missing passthrough behavioral quality for representative syntax %s", uid)
		}
	}

	for _, uid := range requiredNative {
		entry, exists := conformanceEntryByUID(uid)
		if !exists {
			t.Fatalf("missing conformance matrix entry for %s", uid)
		}
		if !entry.Supported {
			t.Fatalf("expected representative native syntax %s to be supported", uid)
		}
		if entry.NativeBehavioralQuality == "" {
			t.Fatalf("missing native behavioral quality for representative syntax %s", uid)
		}
	}
}

func TestBehavioralQualityValuesAreKnown(t *testing.T) {
	known := map[BehavioralQuality]struct{}{
		BehavioralQualityExact:    {},
		BehavioralQualityDelta3:   {},
		BehavioralQualityNonEmpty: {},
	}

	for _, entry := range ConformanceMatrix {
		if entry.PassthroughBehavioralQuality != "" {
			if _, ok := known[entry.PassthroughBehavioralQuality]; !ok {
				t.Fatalf("unknown passthrough behavioral quality %q for %s", entry.PassthroughBehavioralQuality, entry.TransferSyntax.UID)
			}
		}

		if entry.NativeBehavioralQuality != "" {
			if _, ok := known[entry.NativeBehavioralQuality]; !ok {
				t.Fatalf("unknown native behavioral quality %q for %s", entry.NativeBehavioralQuality, entry.TransferSyntax.UID)
			}
		}
	}
}
