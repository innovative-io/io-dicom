package media

import (
	"os"
	"strings"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

func TestSupportedTransferSyntaxesHaveMediaLayerCoverage(t *testing.T) {
	t.Helper()

	supportedSymbols := supportedTransferSyntaxSymbols(t)

	source, err := os.ReadFile("dicom_object.go")
	if err != nil {
		t.Fatalf("failed to read dicom_object.go: %v", err)
	}

	src := string(source)
	compressSection := sourceSection(t, src, "func (obj *dicomObject) compress(", "func (obj *dicomObject) uncompress(")
	uncompressSection := sourceSection(t, src, "func (obj *dicomObject) uncompress(", "")
	parseSection := sourceSection(t, src, "func parseBufData(", "func (obj *dicomObject) WriteToBytes()")
	writeSection := sourceSection(t, src, "func (obj *dicomObject) WriteToBytes()", "func (obj *dicomObject) DumpTags()")

	type nativeDatasetExpectation struct {
		transferSyntax    *transfersyntax.TransferSyntax
		requireParseCheck bool
	}
	nativeDatasetSyntaxes := []nativeDatasetExpectation{
		{transferSyntax: transfersyntax.ImplicitVRLittleEndian, requireParseCheck: true},
		{transferSyntax: transfersyntax.ExplicitVRLittleEndian, requireParseCheck: false},
		{transferSyntax: transfersyntax.DeflatedExplicitVRLittleEndian, requireParseCheck: true},
	}

	explicitPixelSyntaxes := []*transfersyntax.TransferSyntax{
		transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian,
		transfersyntax.DeflatedImageFrameCompression,
		transfersyntax.RLELossless,
		transfersyntax.JPEGBaseline8Bit,
		transfersyntax.JPEGExtended12Bit,
		transfersyntax.JPEGLossless,
		transfersyntax.JPEGLosslessSV1,
		transfersyntax.JPEGLSLossless,
		transfersyntax.JPEGLSNearLossless,
		transfersyntax.JPEG2000Lossless,
		transfersyntax.JPEG2000,
		transfersyntax.JPEG2000MCLossless,
		transfersyntax.JPEG2000MC,
		transfersyntax.HTJ2KLossless,
		transfersyntax.HTJ2KLosslessRPCL,
		transfersyntax.HTJ2K,
		transfersyntax.JPEGXLLossless,
		transfersyntax.JPEGXLJPEGRecompression,
		transfersyntax.JPEGXL,
		transfersyntax.JPIPHTJ2KReferenced,
		transfersyntax.JPIPHTJ2KReferencedDeflate,
		transfersyntax.MPEG2MPML,
		transfersyntax.MPEG2MPMLF,
		transfersyntax.MPEG2MPHL,
		transfersyntax.MPEG2MPHLF,
		transfersyntax.MPEG4HP41,
		transfersyntax.MPEG4HP41F,
		transfersyntax.MPEG4HP41BD,
		transfersyntax.MPEG4HP41BDF,
		transfersyntax.MPEG4HP422D,
		transfersyntax.MPEG4HP422DF,
		transfersyntax.MPEG4HP423D,
		transfersyntax.MPEG4HP423DF,
		transfersyntax.MPEG4HP42STEREO,
		transfersyntax.MPEG4HP42STEREOF,
		transfersyntax.HEVCMP51,
		transfersyntax.HEVCM10P51,
		transfersyntax.SMPTEST211020UncompressedProgressiveActiveVideo,
		transfersyntax.SMPTEST211020UncompressedInterlacedActiveVideo,
		transfersyntax.SMPTEST211030PCMDigitalAudio,
	}

	covered := make(map[string]bool, len(nativeDatasetSyntaxes)+len(explicitPixelSyntaxes))
	coveredBySymbol := make(map[string]bool, len(nativeDatasetSyntaxes)+len(explicitPixelSyntaxes))
	for _, expectation := range nativeDatasetSyntaxes {
		ts := expectation.transferSyntax
		covered[ts.UID] = true
		coveredBySymbol[ts.Name] = true
		if expectation.requireParseCheck {
			assertContainsSymbol(t, parseSection, mediaTransferSyntaxSymbol(ts), "parseBufData")
		}
	}
	assertContainsSymbol(t, writeSection, mediaTransferSyntaxSymbol(transfersyntax.DeflatedExplicitVRLittleEndian), "WriteToBytes")

	for _, ts := range explicitPixelSyntaxes {
		covered[ts.UID] = true
		coveredBySymbol[ts.Name] = true
		symbol := mediaTransferSyntaxSymbol(ts)
		assertContainsSymbol(t, compressSection, symbol, "compress")
		assertContainsSymbol(t, uncompressSection, symbol, "uncompress")
	}

	for _, symbol := range supportedSymbols {
		if !coveredBySymbol[symbol] {
			t.Fatalf("supported transfer syntax %s from dictionary/transfersyntax/uids.go is not classified in media-layer conformance test", symbol)
		}
	}

	if len(coveredBySymbol) != len(supportedSymbols) {
		t.Fatalf("media-layer conformance coverage count = %d, want %d", len(coveredBySymbol), len(supportedSymbols))
	}

	for _, expectation := range nativeDatasetSyntaxes {
		ts := expectation.transferSyntax
		if !transfersyntax.SupportedTransferSyntax(ts.UID) {
			t.Fatalf("expected native dataset transfer syntax %s to be supported", ts.Name)
		}
	}
	for _, ts := range explicitPixelSyntaxes {
		if !transfersyntax.SupportedTransferSyntax(ts.UID) {
			t.Fatalf("expected explicit pixel transfer syntax %s to be supported", ts.Name)
		}
	}
}

func sourceSection(t *testing.T, source string, startMarker string, endMarker string) string {
	t.Helper()

	start := strings.Index(source, startMarker)
	if start == -1 {
		t.Fatalf("could not find start marker %q", startMarker)
	}
	if endMarker == "" {
		return source[start:]
	}

	searchStart := start + len(startMarker)
	endRelative := strings.Index(source[searchStart:], endMarker)
	if endRelative == -1 {
		return source[start:]
	}

	return source[start : searchStart+endRelative]
}

func mediaTransferSyntaxSymbol(ts *transfersyntax.TransferSyntax) string {
	return "transfersyntax." + ts.Name
}

func assertContainsSymbol(t *testing.T, section string, symbol string, sectionName string) {
	t.Helper()
	if !strings.Contains(section, symbol) {
		t.Fatalf("expected %s to contain %s", sectionName, symbol)
	}
}

func supportedTransferSyntaxSymbols(t *testing.T) []string {
	t.Helper()

	source, err := os.ReadFile("../dictionary/transfersyntax/uids.go")
	if err != nil {
		t.Fatalf("failed to read dictionary/transfersyntax/uids.go: %v", err)
	}

	src := string(source)
	block := sourceSection(t, src, "var supportedTransferSyntaxes = []*TransferSyntax{", "}")
	start := strings.Index(block, "{")
	if start == -1 {
		t.Fatal("failed to parse supported transfer syntax block")
	}

	lines := strings.Split(block[start+1:], "\n")
	symbols := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, ","))
		if trimmed == "" || trimmed == "}" {
			continue
		}
		symbols = append(symbols, trimmed)
	}
	if len(symbols) == 0 {
		t.Fatal("no supported transfer syntaxes parsed from dictionary/transfersyntax/uids.go")
	}
	return symbols
}
