package gojxl

import "errors"

var errInvalidPermutation = errors.New("gojxl: invalid coefficient-order permutation")

// readPermutation decodes a Lehmer-coded permutation of [0,size) from an ANS
// stream (ReadPermutation, coeff_order.cc). The first `skip` entries are fixed
// (the low-frequency / DC coefficients) and not transmitted.
func readPermutation(skip, size int, reader *ansSymbolReader, b *bitReader, ctxMap []uint8) ([]uint32, error) {
	lehmer := make([]uint32, size)
	end := int(reader.readHybridUint(int(coeffOrderContext(uint32(size))), b, ctxMap)) + skip
	if end > size {
		return nil, errInvalidPermutation
	}
	last := uint32(0)
	for i := skip; i < end; i++ {
		lehmer[i] = reader.readHybridUint(int(coeffOrderContext(last)), b, ctxMap)
		last = lehmer[i]
		if int(lehmer[i]) >= size-i {
			return nil, errInvalidPermutation
		}
	}
	return decodeLehmerCode(lehmer, size), nil
}

// decodeCoeffOrder reads one transform's coefficient order: a permutation of the
// covered region, composed with the natural scan order (DecodeCoeffOrder).
func decodeCoeffOrder(t acStrategyType, reader *ansSymbolReader, b *bitReader, ctxMap []uint8) ([]int, error) {
	llf := t.coveredBlocksX() * t.coveredBlocksY()
	size := acBlockDim * acBlockDim * llf
	perm, err := readPermutation(llf, size, reader, b, ctxMap)
	if err != nil {
		return nil, err
	}
	natural := naturalCoeffOrder(t.coveredBlocksX(), t.coveredBlocksY())
	order := make([]int, size)
	for k := 0; k < size; k++ {
		order[k] = natural[perm[k]]
	}
	return order, nil
}
