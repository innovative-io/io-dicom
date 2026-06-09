package jpegxl

import "github.com/innovative-io/io-dicom/codecs/jpegxl/gojxl"

// IsJPEGRecompression reports whether the payload is a JPEG XL container that
// carries a JPEG reconstruction (jbrd) box — i.e. a losslessly transcoded JPEG
// (DICOM transfer syntax 1.2.840.10008.1.2.4.111).
func IsJPEGRecompression(encoded []byte) bool {
	return gojxl.IsJPEGRecompression(encoded)
}

// ReconstructJPEG reconstructs the original JPEG file bytes from a JPEG XL
// JPEG-recompression payload using the pure-Go decoder. This is a lossless,
// byte-exact inverse of cjxl's JPEG transcoding. It supports baseline-sequential
// JPEGs (4:4:4 / 4:2:2 / 4:2:0 and grayscale); progressive scans return an error.
func ReconstructJPEG(encoded []byte) ([]byte, error) {
	return gojxl.ReconstructJPEG(encoded)
}
