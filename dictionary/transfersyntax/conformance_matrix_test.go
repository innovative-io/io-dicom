package transfersyntax

import "testing"

type conformanceExpectation struct {
	transferSyntax *TransferSyntax
	wantSupported  bool
	status         string
}

var conformanceMatrix = []conformanceExpectation{
	{transferSyntax: ImplicitVRLittleEndian, wantSupported: true, status: "native"},
	{transferSyntax: ExplicitVRLittleEndian, wantSupported: true, status: "native"},
	{transferSyntax: EncapsulatedUncompressedExplicitVRLittleEndian, wantSupported: true, status: "native"},
	{transferSyntax: DeflatedExplicitVRLittleEndian, wantSupported: true, status: "native"},
	{transferSyntax: DeflatedImageFrameCompression, wantSupported: true, status: "native"},
	{transferSyntax: RLELossless, wantSupported: true, status: "native"},
	{transferSyntax: JPEGBaseline8Bit, wantSupported: true, status: "exact-family"},
	{transferSyntax: JPEGExtended12Bit, wantSupported: true, status: "exact-family"},
	{transferSyntax: JPEGLossless, wantSupported: true, status: "exact-family"},
	{transferSyntax: JPEGLosslessSV1, wantSupported: true, status: "exact-family"},
	{transferSyntax: JPEGLSLossless, wantSupported: true, status: "exact-family"},
	{transferSyntax: JPEGLSNearLossless, wantSupported: true, status: "exact-family"},
	{transferSyntax: JPEG2000Lossless, wantSupported: true, status: "exact-family"},
	{transferSyntax: JPEG2000, wantSupported: true, status: "exact-family"},
	{transferSyntax: JPEG2000MCLossless, wantSupported: true, status: "shared-family"},
	{transferSyntax: JPEG2000MC, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG2MPML, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG2MPMLF, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG2MPHL, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG2MPHLF, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG4HP41, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG4HP41F, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG4HP41BD, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG4HP41BDF, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG4HP422D, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG4HP422DF, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG4HP423D, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG4HP423DF, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG4HP42STEREO, wantSupported: true, status: "shared-family"},
	{transferSyntax: MPEG4HP42STEREOF, wantSupported: true, status: "shared-family"},
	{transferSyntax: HEVCMP51, wantSupported: true, status: "shared-family"},
	{transferSyntax: HEVCM10P51, wantSupported: true, status: "shared-family"},
	{transferSyntax: SMPTEST211020UncompressedProgressiveActiveVideo, wantSupported: true, status: "shared-family"},
	{transferSyntax: SMPTEST211020UncompressedInterlacedActiveVideo, wantSupported: true, status: "shared-family"},
	{transferSyntax: SMPTEST211030PCMDigitalAudio, wantSupported: true, status: "shared-family"},
	{transferSyntax: JPEGXLLossless, wantSupported: true, status: "exact-family"},
	{transferSyntax: JPEGXLJPEGRecompression, wantSupported: true, status: "shared-family"},
	{transferSyntax: JPEGXL, wantSupported: true, status: "exact-family"},
	{transferSyntax: HTJ2KLossless, wantSupported: true, status: "shared-family"},
	{transferSyntax: HTJ2KLosslessRPCL, wantSupported: true, status: "shared-family"},
	{transferSyntax: HTJ2K, wantSupported: true, status: "shared-family"},
	{transferSyntax: JPIPHTJ2KReferenced, wantSupported: true, status: "shared-family"},
	{transferSyntax: JPIPHTJ2KReferencedDeflate, wantSupported: true, status: "shared-family"},
	{transferSyntax: ExplicitVRBigEndian, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGExtended35, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGSpectralSelectionNonHierarchical68, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGSpectralSelectionNonHierarchical79, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGFullProgressionNonHierarchical1012, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGFullProgressionNonHierarchical1113, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGExtendedHierarchical1618, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGExtendedHierarchical1719, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGSpectralSelectionHierarchical2022, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGSpectralSelectionHierarchical2123, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGFullProgressionHierarchical2426, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGFullProgressionHierarchical2527, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGLosslessHierarchical28, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPEGLosslessHierarchical29, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPIPReferenced, wantSupported: false, status: "unsupported"},
	{transferSyntax: JPIPReferencedDeflate, wantSupported: false, status: "unsupported"},
	{transferSyntax: RFC2557MIMEEncapsulation, wantSupported: false, status: "unsupported"},
	{transferSyntax: XMLEncoding, wantSupported: false, status: "unsupported"},
}

func TestTransferSyntaxConformanceMatrixMatchesSupportGate(t *testing.T) {
	seen := make(map[string]conformanceExpectation, len(conformanceMatrix))

	for _, entry := range conformanceMatrix {
		if entry.transferSyntax == nil {
			t.Fatal("conformance matrix contains nil transfer syntax")
		}
		if entry.status == "" {
			t.Fatalf("conformance matrix entry %q is missing a status", entry.transferSyntax.UID)
		}
		if _, exists := seen[entry.transferSyntax.UID]; exists {
			t.Fatalf("duplicate conformance matrix entry for %s", entry.transferSyntax.UID)
		}
		seen[entry.transferSyntax.UID] = entry

		got := SupportedTransferSyntax(entry.transferSyntax.UID)
		if got != entry.wantSupported {
			t.Fatalf("SupportedTransferSyntax(%s) = %v, want %v", entry.transferSyntax.UID, got, entry.wantSupported)
		}
	}

	for _, transferSyntax := range supportedTransferSyntaxes {
		entry, exists := seen[transferSyntax.UID]
		if !exists {
			t.Fatalf("supported transfer syntax %s missing from conformance matrix", transferSyntax.UID)
		}
		if !entry.wantSupported {
			t.Fatalf("supported transfer syntax %s marked unsupported in conformance matrix", transferSyntax.UID)
		}
	}
}