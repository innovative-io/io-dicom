package gojxl

import (
	"errors"
	"fmt"
)

// decodeDequantMatrices reads the non-default AC dequant matrices
// (DequantMatrices::Decode). JPEG-reconstruction frames encode table 0 (the
// DCT-8 table) with the RAW mode — the actual JPEG quantization table — and
// leave the remaining tables at their library defaults.
func decodeDequantMatrices(b *bitReader, st *vardctState) error {
	for idx := 0; idx < kNumQuantTables; idx++ {
		mode := int(b.ReadBits(3))
		switch mode {
		case quantModeLibrary:
			// predefined index uses ceil(log2(kNumPredefinedTables)) = 0 bits;
			// the library default is already in st.quantLib[idx].
		case quantModeRAW:
			enc, err := decodeRawQuantTable(b, st, idx)
			if err != nil {
				return err
			}
			st.quantLib[idx] = enc
		default:
			return fmt.Errorf("gojxl: dequant matrix mode %d not yet supported", mode)
		}
	}
	return nil
}

// decodeRawQuantTable reads a RAW quant encoding: an F16 denominator followed by
// an 8x8x3 Modular sub-image holding the integer quantization values
// (ModularFrameDecoder::DecodeQuantTable). The sub-image uses the frame's global
// modular tree/histograms.
func decodeRawQuantTable(b *bitReader, st *vardctState, idx int) (*quantEncoding, error) {
	den, err := b.ReadF16()
	if err != nil {
		return nil, err
	}
	if den < 1e-8 {
		return nil, errors.New("gojxl: invalid qtable denominator")
	}

	const w, h, nchan = 8, 8, 3
	img := &modImage{bitdepth: 8}
	for i := 0; i < nchan; i++ {
		img.channel = append(img.channel, modChannel{w: w, h: h, pix: make([]int32, w*h)})
	}
	gh, err := readGroupHeader(b)
	if err != nil {
		return nil, err
	}
	for i := range gh.transforms {
		if err := metaApplyTransform(img, &gh.transforms[i]); err != nil {
			return nil, err
		}
	}
	tree, code, ctxMap, err := groupTree(b, st, gh.useGlobalTree)
	if err != nil {
		return nil, err
	}
	streamID := 1 + 3*int(st.fd.numDCGroups) + idx
	reader := newANSSymbolReader(code, b, maxChannelWidth(img.channel))
	for ci := range img.channel {
		decodeChannel(reader, b, tree, ctxMap, img.channel, ci, streamID, gh.wp)
	}
	if !reader.checkFinalState() {
		return nil, errors.New("gojxl: quant table ANS final state failed")
	}
	for i := len(gh.transforms) - 1; i >= 0; i-- {
		if err := inverseTransform(img, gh.transforms[i], gh.wp); err != nil {
			return nil, err
		}
	}

	enc := &quantEncoding{mode: quantModeRAW, rawDen: den, rawQtable: make([]int32, 3*64)}
	base := img.nbMeta
	for c := 0; c < 3; c++ {
		src := img.channel[base+c].pix
		for i := 0; i < 64; i++ {
			if src[i] <= 0 {
				return nil, errors.New("gojxl: invalid raw quantization value")
			}
			enc.rawQtable[c*64+i] = src[i]
		}
	}
	return enc, nil
}
