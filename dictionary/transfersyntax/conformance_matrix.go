package transfersyntax

import (
	"fmt"
	"strings"
)

type ConformanceStatus string

type BehavioralQuality string

const (
	ConformanceStatusNative       ConformanceStatus = "native"
	ConformanceStatusExactFamily  ConformanceStatus = "exact-family"
	ConformanceStatusSharedFamily ConformanceStatus = "shared-family"
	ConformanceStatusUnsupported  ConformanceStatus = "unsupported"

	BehavioralQualityExact    BehavioralQuality = "exact"
	BehavioralQualityDelta3   BehavioralQuality = "delta-3"
	BehavioralQualityNonEmpty BehavioralQuality = "non-empty"
)

type ConformanceExpectation struct {
	TransferSyntax               *TransferSyntax
	Supported                    bool
	Status                       ConformanceStatus
	Note                         string
	PassthroughBehavioralQuality BehavioralQuality
	NativeBehavioralQuality      BehavioralQuality
}

func (entry ConformanceExpectation) DocumentationStatus() string {
	switch entry.Status {
	case ConformanceStatusNative, ConformanceStatusExactFamily:
		return "supported"
	case ConformanceStatusSharedFamily:
		return "supported-via-shared-family"
	case ConformanceStatusUnsupported:
		return "unsupported"
	default:
		return string(entry.Status)
	}
}

func (entry ConformanceExpectation) DocumentationBehavioralQuality() string {
	if entry.PassthroughBehavioralQuality == "" && entry.NativeBehavioralQuality == "" {
		return "-"
	}

	passthrough := string(entry.PassthroughBehavioralQuality)
	if passthrough == "" {
		passthrough = "-"
	}
	native := string(entry.NativeBehavioralQuality)
	if native == "" {
		native = "-"
	}
	return fmt.Sprintf("%s / %s", passthrough, native)
}

var ConformanceMatrix = []ConformanceExpectation{
	{TransferSyntax: ImplicitVRLittleEndian, Supported: true, Status: ConformanceStatusNative, Note: "Native uncompressed parsing/writing"},
	{TransferSyntax: ExplicitVRLittleEndian, Supported: true, Status: ConformanceStatusNative, Note: "Native uncompressed parsing/writing"},
	{TransferSyntax: EncapsulatedUncompressedExplicitVRLittleEndian, Supported: true, Status: ConformanceStatusNative, Note: "Encapsulation path in media layer", PassthroughBehavioralQuality: BehavioralQualityExact},
	{TransferSyntax: DeflatedExplicitVRLittleEndian, Supported: true, Status: ConformanceStatusNative, Note: "Dataset deflate/inflate path"},
	{TransferSyntax: ExplicitVRBigEndian, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired; not in supported transfer syntax set"},
	{TransferSyntax: JPEGBaseline8Bit, Supported: true, Status: ConformanceStatusExactFamily, Note: "JPEG family path", PassthroughBehavioralQuality: BehavioralQualityDelta3},
	{TransferSyntax: JPEGExtended12Bit, Supported: true, Status: ConformanceStatusExactFamily, Note: "JPEG family path", PassthroughBehavioralQuality: BehavioralQualityExact, NativeBehavioralQuality: BehavioralQualityExact},
	{TransferSyntax: JPEGExtended35, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired JPEG process variant not implemented"},
	{TransferSyntax: JPEGSpectralSelectionNonHierarchical68, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired JPEG process variant not implemented"},
	{TransferSyntax: JPEGSpectralSelectionNonHierarchical79, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired JPEG process variant not implemented"},
	{TransferSyntax: JPEGFullProgressionNonHierarchical1012, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired JPEG process variant not implemented"},
	{TransferSyntax: JPEGFullProgressionNonHierarchical1113, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired JPEG process variant not implemented"},
	{TransferSyntax: JPEGLossless, Supported: true, Status: ConformanceStatusExactFamily, Note: "JPEG family path"},
	{TransferSyntax: JPEGLosslessNonHierarchical15, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired JPEG process variant not implemented"},
	{TransferSyntax: JPEGExtendedHierarchical1618, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired hierarchical JPEG variant not implemented"},
	{TransferSyntax: JPEGExtendedHierarchical1719, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired hierarchical JPEG variant not implemented"},
	{TransferSyntax: JPEGSpectralSelectionHierarchical2022, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired hierarchical JPEG variant not implemented"},
	{TransferSyntax: JPEGSpectralSelectionHierarchical2123, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired hierarchical JPEG variant not implemented"},
	{TransferSyntax: JPEGFullProgressionHierarchical2426, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired hierarchical JPEG variant not implemented"},
	{TransferSyntax: JPEGFullProgressionHierarchical2527, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired hierarchical JPEG variant not implemented"},
	{TransferSyntax: JPEGLosslessHierarchical28, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired hierarchical JPEG variant not implemented"},
	{TransferSyntax: JPEGLosslessHierarchical29, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired hierarchical JPEG variant not implemented"},
	{TransferSyntax: JPEGLosslessSV1, Supported: true, Status: ConformanceStatusExactFamily, Note: "JPEG family path", PassthroughBehavioralQuality: BehavioralQualityDelta3},
	{TransferSyntax: JPEGLSLossless, Supported: true, Status: ConformanceStatusExactFamily, Note: "JPEG-LS backend path", PassthroughBehavioralQuality: BehavioralQualityExact, NativeBehavioralQuality: BehavioralQualityExact},
	{TransferSyntax: JPEGLSNearLossless, Supported: true, Status: ConformanceStatusExactFamily, Note: "JPEG-LS backend path"},
	{TransferSyntax: JPEG2000Lossless, Supported: true, Status: ConformanceStatusExactFamily, Note: "JPEG 2000 family path", PassthroughBehavioralQuality: BehavioralQualityExact, NativeBehavioralQuality: BehavioralQualityExact},
	{TransferSyntax: JPEG2000, Supported: true, Status: ConformanceStatusExactFamily, Note: "JPEG 2000 family path"},
	{TransferSyntax: JPEG2000MCLossless, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through JPEG 2000 codec path"},
	{TransferSyntax: JPEG2000MC, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through JPEG 2000 codec path"},
	{TransferSyntax: JPIPReferenced, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired JPIP syntax excluded by supported set"},
	{TransferSyntax: JPIPReferencedDeflate, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired JPIP syntax excluded by supported set"},
	{TransferSyntax: MPEG2MPML, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path", PassthroughBehavioralQuality: BehavioralQualityExact, NativeBehavioralQuality: BehavioralQualityNonEmpty},
	{TransferSyntax: MPEG2MPMLF, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG2MPHL, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG2MPHLF, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG4HP41, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG4HP41F, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG4HP41BD, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG4HP41BDF, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG4HP422D, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG4HP422DF, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG4HP423D, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG4HP423DF, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG4HP42STEREO, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: MPEG4HP42STEREOF, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: HEVCMP51, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: HEVCM10P51, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through FFmpeg-backed MPEG path"},
	{TransferSyntax: JPEGXLLossless, Supported: true, Status: ConformanceStatusExactFamily, Note: "JPEG XL family path", PassthroughBehavioralQuality: BehavioralQualityExact, NativeBehavioralQuality: BehavioralQualityExact},
	{TransferSyntax: JPEGXLJPEGRecompression, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through JPEG XL family path"},
	{TransferSyntax: JPEGXL, Supported: true, Status: ConformanceStatusExactFamily, Note: "JPEG XL family path", PassthroughBehavioralQuality: BehavioralQualityExact},
	{TransferSyntax: HTJ2KLossless, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through JPEG 2000 family path"},
	{TransferSyntax: HTJ2KLosslessRPCL, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through JPEG 2000 family path"},
	{TransferSyntax: HTJ2K, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through JPEG 2000 family path"},
	{TransferSyntax: JPIPHTJ2KReferenced, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through JPIP family path", PassthroughBehavioralQuality: BehavioralQualityExact, NativeBehavioralQuality: BehavioralQualityExact},
	{TransferSyntax: JPIPHTJ2KReferencedDeflate, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through JPIP family path"},
	{TransferSyntax: RLELossless, Supported: true, Status: ConformanceStatusNative, Note: "Dedicated RLE transcoder implementation", PassthroughBehavioralQuality: BehavioralQualityExact},
	{TransferSyntax: RFC2557MIMEEncapsulation, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired / not implemented"},
	{TransferSyntax: XMLEncoding, Supported: false, Status: ConformanceStatusUnsupported, Note: "Retired / not implemented"},
	{TransferSyntax: SMPTEST211020UncompressedProgressiveActiveVideo, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through SMPTE 2110 family path", PassthroughBehavioralQuality: BehavioralQualityExact, NativeBehavioralQuality: BehavioralQualityNonEmpty},
	{TransferSyntax: SMPTEST211020UncompressedInterlacedActiveVideo, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through SMPTE 2110 family path"},
	{TransferSyntax: SMPTEST211030PCMDigitalAudio, Supported: true, Status: ConformanceStatusSharedFamily, Note: "Routed through SMPTE 2110 family path"},
	{TransferSyntax: DeflatedImageFrameCompression, Supported: true, Status: ConformanceStatusNative, Note: "Dedicated frame deflate/inflate path", PassthroughBehavioralQuality: BehavioralQualityExact},
	{TransferSyntax: Papyrus3ImplicitVRLittleEndian, Supported: false, Status: ConformanceStatusUnsupported, Note: "Legacy Papyrus transfer syntax not implemented"},
}

func RenderSupportMatrixMarkdown() string {
	var builder strings.Builder

	builder.WriteString("# Transfer Syntax Support Matrix\n\n")
	builder.WriteString("This document is generated from `dictionary/transfersyntax.ConformanceMatrix`.\n")
	builder.WriteString("Do not edit it manually; regenerate it from source instead.\n\n")
	builder.WriteString("It distinguishes between:\n\n")
	builder.WriteString("- fully supported uncompressed/native syntax handling,\n")
	builder.WriteString("- supported syntaxes that are implemented through a shared codec family path,\n")
	builder.WriteString("- intentionally unsupported or retired syntaxes.\n\n")
	builder.WriteString("## Supported\n\n")
	builder.WriteString("| UID | Name | Status | Behavioral (passthrough/native) | Notes |\n")
	builder.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, entry := range ConformanceMatrix {
		if !entry.Supported {
			continue
		}
		builder.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", entry.TransferSyntax.UID, entry.TransferSyntax.Description, entry.DocumentationStatus(), entry.DocumentationBehavioralQuality(), entry.Note))
	}

	builder.WriteString("\n## Intentionally Unsupported / Retired\n\n")
	builder.WriteString("| UID | Name | Reason |\n")
	builder.WriteString("| --- | --- | --- |\n")
	for _, entry := range ConformanceMatrix {
		if entry.Supported {
			continue
		}
		builder.WriteString(fmt.Sprintf("| %s | %s | %s |\n", entry.TransferSyntax.UID, entry.TransferSyntax.Description, entry.Note))
	}

	builder.WriteString("\n## Notes\n\n")
	builder.WriteString("- `supported-via-shared-family` means the syntax is accepted and transcoded, but it is handled by a broader codec implementation rather than a syntax-unique engine.\n")
	builder.WriteString("- The canonical support gate is `SupportedTransferSyntax` in `dictionary/transfersyntax/uids.go`.\n")
	builder.WriteString("- The canonical conformance inventory is `ConformanceMatrix` in `dictionary/transfersyntax/conformance_matrix.go`.\n")
	builder.WriteString("- Behavioral quality uses `exact`, `delta-3`, and `non-empty` to encode representative expectations for passthrough and native backend paths.\n")
	builder.WriteString("- The actual encode/decode switch logic lives in `media/dicom_object.go`.\n")
	builder.WriteString("- The support contract is guarded by tests in both `dictionary/transfersyntax` and `media` so supported syntaxes must stay aligned with media-layer handling.\n")
	builder.WriteString("- Representative behavioral roundtrip tests in `media/dicom_object_test.go` verify that native dataset syntaxes and representative pixel-data syntax families can roundtrip through the media layer with deterministic passthrough-backed codec selection.\n")

	return builder.String()
}

func RenderBehavioralSummaryMarkdown() string {
	var builder strings.Builder

	writeBehavioralSection := func(title string, quality BehavioralQuality) {
		builder.WriteString(fmt.Sprintf("### %s\n\n", title))
		builder.WriteString("| UID | Name | Status | Notes |\n")
		builder.WriteString("| --- | --- | --- | --- |\n")

		count := 0
		for _, entry := range ConformanceMatrix {
			if !entry.Supported || entry.PassthroughBehavioralQuality != quality {
				continue
			}
			builder.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", entry.TransferSyntax.UID, entry.TransferSyntax.Description, entry.DocumentationStatus(), entry.Note))
			count++
		}

		if count == 0 {
			builder.WriteString("| - | - | - | no transfer syntaxes currently tagged with this behavioral quality |\n")
		}
		builder.WriteString("\n")
	}

	writeNativeBehavioralSection := func(title string, quality BehavioralQuality) {
		builder.WriteString(fmt.Sprintf("### %s\n\n", title))
		builder.WriteString("| UID | Name | Status | Notes |\n")
		builder.WriteString("| --- | --- | --- | --- |\n")

		count := 0
		for _, entry := range ConformanceMatrix {
			if !entry.Supported || entry.NativeBehavioralQuality != quality {
				continue
			}
			builder.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", entry.TransferSyntax.UID, entry.TransferSyntax.Description, entry.DocumentationStatus(), entry.Note))
			count++
		}

		if count == 0 {
			builder.WriteString("| - | - | - | no transfer syntaxes currently tagged with this behavioral quality |\n")
		}
		builder.WriteString("\n")
	}

	builder.WriteString("# Transfer Syntax Behavioral Summary\n\n")
	builder.WriteString("This document is generated from `dictionary/transfersyntax.ConformanceMatrix`.\n")
	builder.WriteString("Do not edit it manually; regenerate it from source instead.\n\n")
	builder.WriteString("Behavioral quality labels summarize representative media-layer roundtrip expectations:\n\n")
	builder.WriteString("- `exact`: pixel payload should roundtrip exactly.\n")
	builder.WriteString("- `delta-3`: roundtrip values may differ but stay within absolute per-byte delta <= 3.\n")
	builder.WriteString("- `non-empty`: exact byte match is not required, but decoded payload must be present.\n\n")
	builder.WriteString("## Passthrough Behavioral Quality\n\n")
	writeBehavioralSection("Exact", BehavioralQualityExact)
	writeBehavioralSection("Delta-3", BehavioralQualityDelta3)
	writeBehavioralSection("Non-Empty", BehavioralQualityNonEmpty)

	builder.WriteString("## Native Behavioral Quality\n\n")
	writeNativeBehavioralSection("Exact", BehavioralQualityExact)
	writeNativeBehavioralSection("Delta-3", BehavioralQualityDelta3)
	writeNativeBehavioralSection("Non-Empty", BehavioralQualityNonEmpty)

	builder.WriteString("## Notes\n\n")
	builder.WriteString("- Behavioral expectations are intentionally attached only to representative syntax families.\n")
	builder.WriteString("- Canonical expectations are maintained in `dictionary/transfersyntax/conformance_matrix.go`.\n")
	builder.WriteString("- Representative roundtrip assertions live in media tests and are exercised by both package tests and tagged backend suites.\n")

	return builder.String()
}
