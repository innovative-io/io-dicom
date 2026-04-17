package transfersyntax

type TransferSyntax struct {
	UID         string
	Name        string
	Description string
	Type        string
}

var supportedTransferSyntaxes = []*TransferSyntax{
	ImplicitVRLittleEndian,
	ExplicitVRLittleEndian,
	EncapsulatedUncompressedExplicitVRLittleEndian,
	DeflatedExplicitVRLittleEndian,
	DeflatedImageFrameCompression,
	RLELossless,
	JPEGLossless,
	JPEGLosslessSV1,
	JPEGBaseline8Bit,
	JPEGExtended12Bit,
	JPEGLSLossless,
	JPEGLSNearLossless,
	JPEG2000Lossless,
	JPEG2000,
	JPEG2000MCLossless,
	JPEG2000MC,
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
	SMPTEST211020UncompressedProgressiveActiveVideo,
	SMPTEST211020UncompressedInterlacedActiveVideo,
	SMPTEST211030PCMDigitalAudio,
	JPEGXLLossless,
	JPEGXLJPEGRecompression,
	JPEGXL,
	HTJ2KLossless,
	HTJ2KLosslessRPCL,
	HTJ2K,
	JPIPHTJ2KReferenced,
	JPIPHTJ2KReferencedDeflate,
}

func GetTransferSyntaxFromName(name string) *TransferSyntax {
	for _, ts := range transferSyntaxes {
		if ts.Name == name {
			return ts
		}
	}
	return nil
}

func GetTransferSyntaxFromUID(uid string) *TransferSyntax {
	for _, ts := range transferSyntaxes {
		if ts.UID == uid {
			return ts
		}
	}
	// Extra loop to fix old bug
	uid = string([]rune(uid)[:len(uid)-1])
	for _, ts := range transferSyntaxes {
		if ts.UID == uid {
			return ts
		}
	}
	return nil
}

func SupportedTransferSyntax(uid string) bool {
	for _, ts := range supportedTransferSyntaxes {
		if ts.UID == uid {
			return true
		}
	}
	return false
}

func GetSupportedTransferSyntaxUIDs() []string {
	uids := make([]string, 0, len(supportedTransferSyntaxes))
	for _, ts := range supportedTransferSyntaxes {
		uids = append(uids, ts.UID)
	}
	return uids
}
