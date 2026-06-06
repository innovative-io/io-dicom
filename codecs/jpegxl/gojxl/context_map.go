package gojxl

import "errors"

// decodeContextMap reads a context map (dec_context_map.cc), returning the
// number of histograms. context_map is filled in place.
func decodeContextMap(contextMap []uint8, b *bitReader) (int, error) {
	isSimple := b.ReadBits(1) != 0
	if isSimple {
		bitsPerEntry := int(b.ReadBits(2))
		if bitsPerEntry != 0 {
			for i := range contextMap {
				contextMap[i] = uint8(b.ReadBits(bitsPerEntry))
			}
		} else {
			for i := range contextMap {
				contextMap[i] = 0
			}
		}
	} else {
		useMTF := b.ReadBits(1) != 0
		// LZ77 is disallowed when the map is tiny (prevents pathological nesting).
		code, sinkCtxMap, err := decodeHistograms(b, 1, len(contextMap) <= 2)
		if err != nil {
			return 0, err
		}
		reader := newANSSymbolReader(code, b, 0)
		maxsym := uint32(0)
		for i := range contextMap {
			sym := reader.readHybridUint(0, b, sinkCtxMap)
			if sym > maxsym {
				maxsym = sym
			}
			contextMap[i] = uint8(sym)
		}
		if maxsym >= kMaxClusters {
			return 0, errors.New("gojxl: invalid cluster id")
		}
		if !reader.checkFinalState() {
			return 0, errors.New("gojxl: invalid context map ANS state")
		}
		if useMTF {
			inverseMoveToFront(contextMap)
		}
	}
	numHtrees := 0
	for _, v := range contextMap {
		if int(v)+1 > numHtrees {
			numHtrees = int(v) + 1
		}
	}
	if err := verifyContextMap(contextMap, numHtrees); err != nil {
		return 0, err
	}
	return numHtrees, nil
}

func verifyContextMap(contextMap []uint8, numHtrees int) error {
	have := make([]bool, numHtrees)
	found := 0
	for _, h := range contextMap {
		if int(h) >= numHtrees {
			return errors.New("gojxl: context map index out of range")
		}
		if !have[h] {
			have[h] = true
			found++
		}
	}
	if found != numHtrees {
		return errors.New("gojxl: incomplete context map")
	}
	return nil
}

// inverseMoveToFront applies the inverse MTF transform in place
// (inverse_mtf-inl.h).
func inverseMoveToFront(v []uint8) {
	var mtf [256]uint8
	for i := 0; i < 256; i++ {
		mtf[i] = uint8(i)
	}
	for i := range v {
		index := v[i]
		val := mtf[index]
		v[i] = val
		if index != 0 {
			// Move val to front.
			copy(mtf[1:index+1], mtf[0:index])
			mtf[0] = val
		}
	}
}
