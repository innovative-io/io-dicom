package gojxl

import "errors"

// ICC profile stream handling (icc_codec.cc / icc_codec_common.cc). The embedded
// ICC profile is colour metadata only — it does not affect the decoded pixel
// values — so the decoder just consumes the stream to stay byte-aligned with the
// frame data that follows, discarding the decoded profile bytes.

const kNumICCContexts = 41

// iccByteKind1 / iccByteKind2 classify a previous byte for the ICC ANS context
// (icc_codec_common.cc ByteKind1 / ByteKind2).
func iccByteKind1(b byte) int {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z':
		return 0
	case b >= '0' && b <= '9', b == '.', b == ',':
		return 1
	case b == 0:
		return 2
	case b == 1:
		return 3
	case b < 16:
		return 4
	case b == 255:
		return 6
	case b > 240:
		return 5
	default:
		return 7
	}
}

func iccByteKind2(b byte) int {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z':
		return 0
	case b >= '0' && b <= '9', b == '.', b == ',':
		return 1
	case b < 16:
		return 2
	case b > 240:
		return 3
	default:
		return 4
	}
}

// iccANSContext is the per-byte context for the ICC ANS stream (ICCANSContext).
func iccANSContext(i uint64, b1, b2 byte) int {
	if i <= 128 {
		return 0
	}
	return 1 + iccByteKind1(b1) + iccByteKind2(b2)*8
}

// consumeICC reads (and discards) the embedded ICC profile stream: a U64
// encoded size, the ANS histograms over kNumICCContexts contexts, then that
// many context-modelled bytes. It leaves the bit reader positioned right after
// the stream so frame decoding can continue.
func consumeICC(b *bitReader) error {
	encSize := b.ReadU64()
	if encSize > 268435456 {
		return errors.New("gojxl: ICC profile too large")
	}
	code, ctxMap, err := decodeHistograms(b, kNumICCContexts, false)
	if err != nil {
		return err
	}
	reader := newANSSymbolReader(code, b, 0)
	var b1, b2 byte
	for i := uint64(0); i < encSize; i++ {
		v := byte(reader.readHybridUint(iccANSContext(i, b1, b2), b, ctxMap))
		b2, b1 = b1, v
	}
	if !reader.checkFinalState() {
		return errors.New("gojxl: ICC ANS final state failed")
	}
	if !b.ok() {
		return errTruncated
	}
	return nil
}
