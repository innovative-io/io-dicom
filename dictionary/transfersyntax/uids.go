package transfersyntax

type TransferSyntax struct {
	UID         string
	Name        string
	Description string
	Type        string
}

// supportedTransferSyntaxes lists every transfer syntax this implementation
// can encode and decode. The ORDER matters: GetSupportedTransferSyntaxUIDs
// returns UIDs in this order, which is used as the preference list when
// proposing storage presentation contexts in a C-GET SCU association request.
// The remote SCP (selectPreferredTransferSyntax) accepts the first TS from
// this list that it supports, so placing lossless-compressed formats before
// uncompressed ones means the SCP will deliver files in their native compressed
// format (e.g., JPEG Lossless) without transcoding, rather than always
// inflating to ILE. ILE is placed last as the universal legacy fallback.
//
// Preference order: lossless compressed → uncompressed explicit → lossy compressed
// → video/audio → ILE (legacy implicit, last resort).
var supportedTransferSyntaxes = []*TransferSyntax{
	JPEGLosslessSV1,
	JPEGLossless,
	JPEG2000Lossless,
	HTJ2KLossless,
	HTJ2KLosslessRPCL,
	JPEGLSLossless,
	RLELossless,
	JPEGXLLossless,
	ExplicitVRLittleEndian,
	EncapsulatedUncompressedExplicitVRLittleEndian,
	DeflatedExplicitVRLittleEndian,
	DeflatedImageFrameCompression,
	JPEGBaseline8Bit,
	JPEGExtended12Bit,
	JPEGLSNearLossless,
	JPEG2000,
	HTJ2K,
	JPEGXLJPEGRecompression,
	JPEGXL,
	JPEG2000MCLossless,
	JPEG2000MC,
	JPIPHTJ2KReferenced,
	JPIPHTJ2KReferencedDeflate,
	SMPTEST211020UncompressedProgressiveActiveVideo,
	SMPTEST211020UncompressedInterlacedActiveVideo,
	SMPTEST211030PCMDigitalAudio,
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
	ImplicitVRLittleEndian,
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
	// Retry without a trailing byte to tolerate a UID that still carries an
	// odd-length null/space pad byte the caller did not trim. Guard len > 0:
	// an empty UID (e.g. from malformed/truncated file meta) must not slice to
	// a negative bound.
	if len(uid) > 0 {
		trimmed := uid[:len(uid)-1]
		for _, ts := range transferSyntaxes {
			if ts.UID == trimmed {
				return ts
			}
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
