package jpeg2000

// Pure-Go JPEG 2000 backend. It decodes the codestreams it supports (single
// tile, single layer, LRCP, default precincts, 5/3 reversible and 9/7
// irreversible) entirely in Go,
// and returns errJ2KUnsupported for anything else so the native openjpeg backend
// (when built with -tags openjpeg, registered at a higher priority) handles the
// rest. It is registered at priority -1 so openjpeg always wins when present.

// gojpeg2000Backend is the built-in pure-Go decoder.
type gojpeg2000Backend struct{}

func (gojpeg2000Backend) Name() string { return "gojpeg2000" }

func (gojpeg2000Backend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

// Decode decodes a JPEG 2000 codestream into output. A panic on malformed input
// is converted to an error so the decoder is safe on hostile/fuzzed data.
func (gojpeg2000Backend) Decode(encoded []byte, output []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errJ2KMalformed
		}
	}()
	return goJ2Kdecode(stripJP2(encoded), output)
}

// Encode is not implemented in pure Go; J2K encoding still requires openjpeg.
func (gojpeg2000Backend) Encode(raw []byte, width, height, samples, bitsa uint16, _ int) (data []byte, err error) {
	// Pure-Go encode is lossless 5/3 only; ratio (lossy rate control) is ignored.
	defer func() {
		if r := recover(); r != nil {
			data, err = nil, errJ2KMalformed
		}
	}()
	return goJ2Kencode(raw, int(width), int(height), int(samples), int(bitsa))
}

func registerPureGoBackend() {
	_ = mgr.RegisterWithPriority("gojpeg2000", func() Backend { return gojpeg2000Backend{} }, -1)
}

// stripJP2 returns the raw codestream from data, skipping a leading JP2 box
// wrapper if present (DICOM uses the raw codestream, but be defensive).
func stripJP2(data []byte) []byte {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0x4F {
		return data
	}
	for i := 0; i+1 < len(data); i++ {
		if data[i] == 0xFF && data[i+1] == 0x4F {
			return data[i:]
		}
	}
	return data
}
