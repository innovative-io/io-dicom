package transfersyntax

// ImplicitVRLittleEndian - (1.2.840.10008.1.2) Implicit VR Little Endian
var ImplicitVRLittleEndian = &TransferSyntax{
	UID:         "1.2.840.10008.1.2",
	Name:        "ImplicitVRLittleEndian",
	Description: "Implicit VR Little Endian",
	Type:        "Transfer Syntax",
}

// ExplicitVRLittleEndian - (1.2.840.10008.1.2.1) Explicit VR Little Endian
var ExplicitVRLittleEndian = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.1",
	Name:        "ExplicitVRLittleEndian",
	Description: "Explicit VR Little Endian",
	Type:        "Transfer Syntax",
}

// EncapsulatedUncompressedExplicitVRLittleEndian - (1.2.840.10008.1.2.1.98) Encapsulated Uncompressed Explicit VR Little Endian
var EncapsulatedUncompressedExplicitVRLittleEndian = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.1.98",
	Name:        "EncapsulatedUncompressedExplicitVRLittleEndian",
	Description: "Encapsulated Uncompressed Explicit VR Little Endian",
	Type:        "Transfer Syntax",
}

// DeflatedExplicitVRLittleEndian - (1.2.840.10008.1.2.1.99) Deflated Explicit VR Little Endian
var DeflatedExplicitVRLittleEndian = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.1.99",
	Name:        "DeflatedExplicitVRLittleEndian",
	Description: "Deflated Explicit VR Little Endian",
	Type:        "Transfer Syntax",
}

// ExplicitVRBigEndian - (1.2.840.10008.1.2.2) Explicit VR Big Endian
var ExplicitVRBigEndian = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.2",
	Name:        "ExplicitVRBigEndian",
	Description: "Explicit VR Big Endian",
	Type:        "Transfer Syntax",
}

// JPEGBaseline8Bit - (1.2.840.10008.1.2.4.50) JPEG Baseline (Process 1)
var JPEGBaseline8Bit = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.50",
	Name:        "JPEGBaseline8Bit",
	Description: "JPEG Baseline (Process 1)",
	Type:        "Transfer Syntax",
}

// JPEGExtended12Bit - (1.2.840.10008.1.2.4.51) JPEG Extended (Process 2 and 4)
var JPEGExtended12Bit = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.51",
	Name:        "JPEGExtended12Bit",
	Description: "JPEG Extended (Process 2 and 4)",
	Type:        "Transfer Syntax",
}

// JPEGExtended35 - (1.2.840.10008.1.2.4.52) JPEG Extended (Process 3 and 5)
var JPEGExtended35 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.52",
	Name:        "JPEGExtended35",
	Description: "JPEG Extended (Process 3 and 5)",
	Type:        "Transfer Syntax",
}

// JPEGSpectralSelectionNonHierarchical68 - (1.2.840.10008.1.2.4.53) JPEG Spectral Selection, Non-Hierarchical (Process 6 and 8)
var JPEGSpectralSelectionNonHierarchical68 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.53",
	Name:        "JPEGSpectralSelectionNonHierarchical68",
	Description: "JPEG Spectral Selection, Non-Hierarchical (Process 6 and 8)",
	Type:        "Transfer Syntax",
}

// JPEGSpectralSelectionNonHierarchical79 - (1.2.840.10008.1.2.4.54) JPEG Spectral Selection, Non-Hierarchical (Process 7 and 9)
var JPEGSpectralSelectionNonHierarchical79 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.54",
	Name:        "JPEGSpectralSelectionNonHierarchical79",
	Description: "JPEG Spectral Selection, Non-Hierarchical (Process 7 and 9)",
	Type:        "Transfer Syntax",
}

// JPEGFullProgressionNonHierarchical1012 - (1.2.840.10008.1.2.4.55) JPEG Full Progression, Non-Hierarchical (Process 10 and 12)
var JPEGFullProgressionNonHierarchical1012 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.55",
	Name:        "JPEGFullProgressionNonHierarchical1012",
	Description: "JPEG Full Progression, Non-Hierarchical (Process 10 and 12)",
	Type:        "Transfer Syntax",
}

// JPEGFullProgressionNonHierarchical1113 - (1.2.840.10008.1.2.4.56) JPEG Full Progression, Non-Hierarchical (Process 11 and 13)
var JPEGFullProgressionNonHierarchical1113 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.56",
	Name:        "JPEGFullProgressionNonHierarchical1113",
	Description: "JPEG Full Progression, Non-Hierarchical (Process 11 and 13)",
	Type:        "Transfer Syntax",
}

// JPEGLossless - (1.2.840.10008.1.2.4.57) JPEG Lossless, Non-Hierarchical (Process 14)
var JPEGLossless = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.57",
	Name:        "JPEGLossless",
	Description: "JPEG Lossless, Non-Hierarchical (Process 14)",
	Type:        "Transfer Syntax",
}

// JPEGLosslessNonHierarchical15 - (1.2.840.10008.1.2.4.58) JPEG Lossless, Non-Hierarchical (Process 15)
var JPEGLosslessNonHierarchical15 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.58",
	Name:        "JPEGLosslessNonHierarchical15",
	Description: "JPEG Lossless, Non-Hierarchical (Process 15)",
	Type:        "Transfer Syntax",
}

// JPEGExtendedHierarchical1618 - (1.2.840.10008.1.2.4.59) JPEG Extended, Hierarchical (Process 16 and 18)
var JPEGExtendedHierarchical1618 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.59",
	Name:        "JPEGExtendedHierarchical1618",
	Description: "JPEG Extended, Hierarchical (Process 16 and 18)",
	Type:        "Transfer Syntax",
}

// JPEGExtendedHierarchical1719 - (1.2.840.10008.1.2.4.60) JPEG Extended, Hierarchical (Process 17 and 19)
var JPEGExtendedHierarchical1719 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.60",
	Name:        "JPEGExtendedHierarchical1719",
	Description: "JPEG Extended, Hierarchical (Process 17 and 19)",
	Type:        "Transfer Syntax",
}

// JPEGSpectralSelectionHierarchical2022 - (1.2.840.10008.1.2.4.61) JPEG Spectral Selection, Hierarchical (Process 20 and 22)
var JPEGSpectralSelectionHierarchical2022 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.61",
	Name:        "JPEGSpectralSelectionHierarchical2022",
	Description: "JPEG Spectral Selection, Hierarchical (Process 20 and 22)",
	Type:        "Transfer Syntax",
}

// JPEGSpectralSelectionHierarchical2123 - (1.2.840.10008.1.2.4.62) JPEG Spectral Selection, Hierarchical (Process 21 and 23)
var JPEGSpectralSelectionHierarchical2123 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.62",
	Name:        "JPEGSpectralSelectionHierarchical2123",
	Description: "JPEG Spectral Selection, Hierarchical (Process 21 and 23)",
	Type:        "Transfer Syntax",
}

// JPEGFullProgressionHierarchical2426 - (1.2.840.10008.1.2.4.63) JPEG Full Progression, Hierarchical (Process 24 and 26)
var JPEGFullProgressionHierarchical2426 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.63",
	Name:        "JPEGFullProgressionHierarchical2426",
	Description: "JPEG Full Progression, Hierarchical (Process 24 and 26)",
	Type:        "Transfer Syntax",
}

// JPEGFullProgressionHierarchical2527 - (1.2.840.10008.1.2.4.64) JPEG Full Progression, Hierarchical (Process 25 and 27)
var JPEGFullProgressionHierarchical2527 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.64",
	Name:        "JPEGFullProgressionHierarchical2527",
	Description: "JPEG Full Progression, Hierarchical (Process 25 and 27)",
	Type:        "Transfer Syntax",
}

// JPEGLosslessHierarchical28 - (1.2.840.10008.1.2.4.65) JPEG Lossless, Hierarchical (Process 28)
var JPEGLosslessHierarchical28 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.65",
	Name:        "JPEGLosslessHierarchical28",
	Description: "JPEG Lossless, Hierarchical (Process 28)",
	Type:        "Transfer Syntax",
}

// JPEGLosslessHierarchical29 - (1.2.840.10008.1.2.4.66) JPEG Lossless, Hierarchical (Process 29)
var JPEGLosslessHierarchical29 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.66",
	Name:        "JPEGLosslessHierarchical29",
	Description: "JPEG Lossless, Hierarchical (Process 29)",
	Type:        "Transfer Syntax",
}

// JPEGLosslessSV1 - (1.2.840.10008.1.2.4.70) JPEG Lossless, Non-Hierarchical, First-Order Prediction (Process 14 [Selection Value 1])
var JPEGLosslessSV1 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.70",
	Name:        "JPEGLosslessSV1",
	Description: "JPEG Lossless, Non-Hierarchical, First-Order Prediction (Process 14 [Selection Value 1])",
	Type:        "Transfer Syntax",
}

// JPEGLSLossless - (1.2.840.10008.1.2.4.80) JPEG-LS Lossless Image Compression
var JPEGLSLossless = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.80",
	Name:        "JPEGLSLossless",
	Description: "JPEG-LS Lossless Image Compression",
	Type:        "Transfer Syntax",
}

// JPEGLSNearLossless - (1.2.840.10008.1.2.4.81) JPEG-LS Lossy (Near-Lossless) Image Compression
var JPEGLSNearLossless = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.81",
	Name:        "JPEGLSNearLossless",
	Description: "JPEG-LS Lossy (Near-Lossless) Image Compression",
	Type:        "Transfer Syntax",
}

// JPEG2000Lossless - (1.2.840.10008.1.2.4.90) JPEG 2000 Image Compression (Lossless Only)
var JPEG2000Lossless = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.90",
	Name:        "JPEG2000Lossless",
	Description: "JPEG 2000 Image Compression (Lossless Only)",
	Type:        "Transfer Syntax",
}

// JPEG2000 - (1.2.840.10008.1.2.4.91) JPEG 2000 Image Compression
var JPEG2000 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.91",
	Name:        "JPEG2000",
	Description: "JPEG 2000 Image Compression",
	Type:        "Transfer Syntax",
}

// JPEG2000MCLossless - (1.2.840.10008.1.2.4.92) JPEG 2000 Part 2 Multi-component Image Compression (Lossless Only)
var JPEG2000MCLossless = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.92",
	Name:        "JPEG2000MCLossless",
	Description: "JPEG 2000 Part 2 Multi-component Image Compression (Lossless Only)",
	Type:        "Transfer Syntax",
}

// JPEG2000MC - (1.2.840.10008.1.2.4.93) JPEG 2000 Part 2 Multi-component Image Compression
var JPEG2000MC = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.93",
	Name:        "JPEG2000MC",
	Description: "JPEG 2000 Part 2 Multi-component Image Compression",
	Type:        "Transfer Syntax",
}

// JPIPReferenced - (1.2.840.10008.1.2.4.94) JPIP Referenced
var JPIPReferenced = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.94",
	Name:        "JPIPReferenced",
	Description: "JPIP Referenced",
	Type:        "Transfer Syntax",
}

// JPIPReferencedDeflate - (1.2.840.10008.1.2.4.95) JPIP Referenced Deflate
var JPIPReferencedDeflate = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.95",
	Name:        "JPIPReferencedDeflate",
	Description: "JPIP Referenced Deflate",
	Type:        "Transfer Syntax",
}

// MPEG2MPML - (1.2.840.10008.1.2.4.100) MPEG2 Main Profile / Main Level
var MPEG2MPML = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.100",
	Name:        "MPEG2MPML",
	Description: "MPEG2 Main Profile / Main Level",
	Type:        "Transfer Syntax",
}

// MPEG2MPMLF - (1.2.840.10008.1.2.4.100.1) Fragmentable MPEG2 Main Profile / Main Level
var MPEG2MPMLF = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.100.1",
	Name:        "MPEG2MPMLF",
	Description: "Fragmentable MPEG2 Main Profile / Main Level",
	Type:        "Transfer Syntax",
}

// MPEG2MPHL - (1.2.840.10008.1.2.4.101) MPEG2 Main Profile / High Level
var MPEG2MPHL = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.101",
	Name:        "MPEG2MPHL",
	Description: "MPEG2 Main Profile / High Level",
	Type:        "Transfer Syntax",
}

// MPEG2MPHLF - (1.2.840.10008.1.2.4.101.1) Fragmentable MPEG2 Main Profile / High Level
var MPEG2MPHLF = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.101.1",
	Name:        "MPEG2MPHLF",
	Description: "Fragmentable MPEG2 Main Profile / High Level",
	Type:        "Transfer Syntax",
}

// MPEG4HP41 - (1.2.840.10008.1.2.4.102) MPEG-4 AVC/H.264 High Profile / Level 4.1
var MPEG4HP41 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.102",
	Name:        "MPEG4HP41",
	Description: "MPEG-4 AVC/H.264 High Profile / Level 4.1",
	Type:        "Transfer Syntax",
}

// MPEG4HP41F - (1.2.840.10008.1.2.4.102.1) Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.1
var MPEG4HP41F = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.102.1",
	Name:        "MPEG4HP41F",
	Description: "Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.1",
	Type:        "Transfer Syntax",
}

// MPEG4HP41BD - (1.2.840.10008.1.2.4.103) MPEG-4 AVC/H.264 BD-compatible High Profile / Level 4.1
var MPEG4HP41BD = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.103",
	Name:        "MPEG4HP41BD",
	Description: "MPEG-4 AVC/H.264 BD-compatible High Profile / Level 4.1",
	Type:        "Transfer Syntax",
}

// MPEG4HP41BDF - (1.2.840.10008.1.2.4.103.1) Fragmentable MPEG-4 AVC/H.264 BD-compatible High Profile / Level 4.1
var MPEG4HP41BDF = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.103.1",
	Name:        "MPEG4HP41BDF",
	Description: "Fragmentable MPEG-4 AVC/H.264 BD-compatible High Profile / Level 4.1",
	Type:        "Transfer Syntax",
}

// MPEG4HP422D - (1.2.840.10008.1.2.4.104) MPEG-4 AVC/H.264 High Profile / Level 4.2 For 2D Video
var MPEG4HP422D = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.104",
	Name:        "MPEG4HP422D",
	Description: "MPEG-4 AVC/H.264 High Profile / Level 4.2 For 2D Video",
	Type:        "Transfer Syntax",
}

// MPEG4HP422DF - (1.2.840.10008.1.2.4.104.1) Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.2 For 2D Video
var MPEG4HP422DF = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.104.1",
	Name:        "MPEG4HP422DF",
	Description: "Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.2 For 2D Video",
	Type:        "Transfer Syntax",
}

// MPEG4HP423D - (1.2.840.10008.1.2.4.105) MPEG-4 AVC/H.264 High Profile / Level 4.2 For 3D Video
var MPEG4HP423D = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.105",
	Name:        "MPEG4HP423D",
	Description: "MPEG-4 AVC/H.264 High Profile / Level 4.2 For 3D Video",
	Type:        "Transfer Syntax",
}

// MPEG4HP423DF - (1.2.840.10008.1.2.4.105.1) Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.2 For 3D Video
var MPEG4HP423DF = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.105.1",
	Name:        "MPEG4HP423DF",
	Description: "Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.2 For 3D Video",
	Type:        "Transfer Syntax",
}

// MPEG4HP42STEREO - (1.2.840.10008.1.2.4.106) MPEG-4 AVC/H.264 Stereo High Profile / Level 4.2
var MPEG4HP42STEREO = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.106",
	Name:        "MPEG4HP42STEREO",
	Description: "MPEG-4 AVC/H.264 Stereo High Profile / Level 4.2",
	Type:        "Transfer Syntax",
}

// MPEG4HP42STEREOF - (1.2.840.10008.1.2.4.106.1) Fragmentable MPEG-4 AVC/H.264 Stereo High Profile / Level 4.2
var MPEG4HP42STEREOF = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.106.1",
	Name:        "MPEG4HP42STEREOF",
	Description: "Fragmentable MPEG-4 AVC/H.264 Stereo High Profile / Level 4.2",
	Type:        "Transfer Syntax",
}

// HEVCMP51 - (1.2.840.10008.1.2.4.107) HEVC/H.265 Main Profile / Level 5.1
var HEVCMP51 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.107",
	Name:        "HEVCMP51",
	Description: "HEVC/H.265 Main Profile / Level 5.1",
	Type:        "Transfer Syntax",
}

// HEVCM10P51 - (1.2.840.10008.1.2.4.108) HEVC/H.265 Main 10 Profile / Level 5.1
var HEVCM10P51 = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.108",
	Name:        "HEVCM10P51",
	Description: "HEVC/H.265 Main 10 Profile / Level 5.1",
	Type:        "Transfer Syntax",
}

// JPEGXLLossless - (1.2.840.10008.1.2.4.110) JPEG XL Lossless
var JPEGXLLossless = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.110",
	Name:        "JPEGXLLossless",
	Description: "JPEG XL Lossless",
	Type:        "Transfer Syntax",
}

// JPEGXLJPEGRecompression - (1.2.840.10008.1.2.4.111) JPEG XL JPEG Recompression
var JPEGXLJPEGRecompression = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.111",
	Name:        "JPEGXLJPEGRecompression",
	Description: "JPEG XL JPEG Recompression",
	Type:        "Transfer Syntax",
}

// JPEGXL - (1.2.840.10008.1.2.4.112) JPEG XL
var JPEGXL = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.112",
	Name:        "JPEGXL",
	Description: "JPEG XL",
	Type:        "Transfer Syntax",
}

// HTJ2KLossless - (1.2.840.10008.1.2.4.201) High-Throughput JPEG 2000 Image Compression (Lossless Only)
var HTJ2KLossless = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.201",
	Name:        "HTJ2KLossless",
	Description: "High-Throughput JPEG 2000 Image Compression (Lossless Only)",
	Type:        "Transfer Syntax",
}

// HTJ2KLosslessRPCL - (1.2.840.10008.1.2.4.202) High-Throughput JPEG 2000 with RPCL Options Image Compression (Lossless Only)
var HTJ2KLosslessRPCL = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.202",
	Name:        "HTJ2KLosslessRPCL",
	Description: "High-Throughput JPEG 2000 with RPCL Options Image Compression (Lossless Only)",
	Type:        "Transfer Syntax",
}

// HTJ2K - (1.2.840.10008.1.2.4.203) High-Throughput JPEG 2000 Image Compression
var HTJ2K = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.203",
	Name:        "HTJ2K",
	Description: "High-Throughput JPEG 2000 Image Compression",
	Type:        "Transfer Syntax",
}

// JPIPHTJ2KReferenced - (1.2.840.10008.1.2.4.204) JPIP HTJ2K Referenced
var JPIPHTJ2KReferenced = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.204",
	Name:        "JPIPHTJ2KReferenced",
	Description: "JPIP HTJ2K Referenced",
	Type:        "Transfer Syntax",
}

// JPIPHTJ2KReferencedDeflate - (1.2.840.10008.1.2.4.205) JPIP HTJ2K Referenced Deflate
var JPIPHTJ2KReferencedDeflate = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.4.205",
	Name:        "JPIPHTJ2KReferencedDeflate",
	Description: "JPIP HTJ2K Referenced Deflate",
	Type:        "Transfer Syntax",
}

// RLELossless - (1.2.840.10008.1.2.5) RLE Lossless
var RLELossless = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.5",
	Name:        "RLELossless",
	Description: "RLE Lossless",
	Type:        "Transfer Syntax",
}

// RFC2557MIMEEncapsulation - (1.2.840.10008.1.2.6.1) RFC 2557 MIME encapsulation
var RFC2557MIMEEncapsulation = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.6.1",
	Name:        "RFC2557MIMEEncapsulation",
	Description: "RFC 2557 MIME encapsulation",
	Type:        "Transfer Syntax",
}

// XMLEncoding - (1.2.840.10008.1.2.6.2) XML Encoding
var XMLEncoding = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.6.2",
	Name:        "XMLEncoding",
	Description: "XML Encoding",
	Type:        "Transfer Syntax",
}

// SMPTEST211020UncompressedProgressiveActiveVideo - (1.2.840.10008.1.2.7.1) SMPTE ST 2110-20 Uncompressed Progressive Active Video
var SMPTEST211020UncompressedProgressiveActiveVideo = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.7.1",
	Name:        "SMPTEST211020UncompressedProgressiveActiveVideo",
	Description: "SMPTE ST 2110-20 Uncompressed Progressive Active Video",
	Type:        "Transfer Syntax",
}

// SMPTEST211020UncompressedInterlacedActiveVideo - (1.2.840.10008.1.2.7.2) SMPTE ST 2110-20 Uncompressed Interlaced Active Video
var SMPTEST211020UncompressedInterlacedActiveVideo = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.7.2",
	Name:        "SMPTEST211020UncompressedInterlacedActiveVideo",
	Description: "SMPTE ST 2110-20 Uncompressed Interlaced Active Video",
	Type:        "Transfer Syntax",
}

// SMPTEST211030PCMDigitalAudio - (1.2.840.10008.1.2.7.3) SMPTE ST 2110-30 PCM Digital Audio
var SMPTEST211030PCMDigitalAudio = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.7.3",
	Name:        "SMPTEST211030PCMDigitalAudio",
	Description: "SMPTE ST 2110-30 PCM Digital Audio",
	Type:        "Transfer Syntax",
}

// DeflatedImageFrameCompression - (1.2.840.10008.1.2.8.1) Deflated Image Frame Compression
var DeflatedImageFrameCompression = &TransferSyntax{
	UID:         "1.2.840.10008.1.2.8.1",
	Name:        "DeflatedImageFrameCompression",
	Description: "Deflated Image Frame Compression",
	Type:        "Transfer Syntax",
}

// Papyrus3ImplicitVRLittleEndian - (1.2.840.10008.1.20) Papyrus 3 Implicit VR Little Endian
var Papyrus3ImplicitVRLittleEndian = &TransferSyntax{
	UID:         "1.2.840.10008.1.20",
	Name:        "Papyrus3ImplicitVRLittleEndian",
	Description: "Papyrus 3 Implicit VR Little Endian",
	Type:        "Transfer Syntax",
}

var transferSyntaxes = []*TransferSyntax{
	ImplicitVRLittleEndian,
	ExplicitVRLittleEndian,
	EncapsulatedUncompressedExplicitVRLittleEndian,
	DeflatedExplicitVRLittleEndian,
	ExplicitVRBigEndian,
	JPEGBaseline8Bit,
	JPEGExtended12Bit,
	JPEGExtended35,
	JPEGSpectralSelectionNonHierarchical68,
	JPEGSpectralSelectionNonHierarchical79,
	JPEGFullProgressionNonHierarchical1012,
	JPEGFullProgressionNonHierarchical1113,
	JPEGLossless,
	JPEGLosslessNonHierarchical15,
	JPEGExtendedHierarchical1618,
	JPEGExtendedHierarchical1719,
	JPEGSpectralSelectionHierarchical2022,
	JPEGSpectralSelectionHierarchical2123,
	JPEGFullProgressionHierarchical2426,
	JPEGFullProgressionHierarchical2527,
	JPEGLosslessHierarchical28,
	JPEGLosslessHierarchical29,
	JPEGLosslessSV1,
	JPEGLSLossless,
	JPEGLSNearLossless,
	JPEG2000Lossless,
	JPEG2000,
	JPEG2000MCLossless,
	JPEG2000MC,
	JPIPReferenced,
	JPIPReferencedDeflate,
	MPEG2MPML,
	MPEG2MPMLF,
	MPEG2MPHL,
	MPEG2MPHLF,
	MPEG4HP41,
	MPEG4HP41F,
	MPEG4HP41BD,
	MPEG4HP41BDF,
	MPEG4HP422D,
	MPEG4HP422DF,
	MPEG4HP423D,
	MPEG4HP423DF,
	MPEG4HP42STEREO,
	MPEG4HP42STEREOF,
	HEVCMP51,
	HEVCM10P51,
	JPEGXLLossless,
	JPEGXLJPEGRecompression,
	JPEGXL,
	HTJ2KLossless,
	HTJ2KLosslessRPCL,
	HTJ2K,
	JPIPHTJ2KReferenced,
	JPIPHTJ2KReferencedDeflate,
	RLELossless,
	RFC2557MIMEEncapsulation,
	XMLEncoding,
	SMPTEST211020UncompressedProgressiveActiveVideo,
	SMPTEST211020UncompressedInterlacedActiveVideo,
	SMPTEST211030PCMDigitalAudio,
	DeflatedImageFrameCompression,
	Papyrus3ImplicitVRLittleEndian,
}
