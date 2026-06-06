package gojxl

// Lehmer-code permutation decoding for VarDCT coefficient orders, ported from
// lib/jxl/lehmer_code.h. The coded coefficient order is a permutation of the
// natural order, transmitted as a Lehmer code and reconstructed with an implicit
// order-statistics (Fenwick) tree. Also includes kStrategyOrder (AC strategy ->
// coefficient-order class) and CoeffOrderContext.

// valueOfLowest1Bit returns x & -x (the value of the lowest set bit).
func valueOfLowest1Bit(x uint32) uint32 { return x & (^x + 1) }

// decodeLehmerCode reconstructs a permutation of [0,n) from its Lehmer code
// (DecodeLehmerCode).
func decodeLehmerCode(code []uint32, n int) []uint32 {
	perm := make([]uint32, n)
	log2n := ceilLog2Nonzero(uint32(n))
	paddedN := 1 << uint(log2n)
	temp := make([]uint32, paddedN)
	for i := 0; i < paddedN; i++ {
		temp[i] = valueOfLowest1Bit(uint32(i + 1))
	}
	for i := 0; i < n; i++ {
		rank := code[i] + 1
		bit := paddedN
		next := 0
		for j := 0; j <= log2n; j++ {
			cand := next + bit
			bit >>= 1
			if temp[cand-1] < rank {
				next = cand
				rank -= temp[cand-1]
			}
		}
		perm[i] = uint32(next)
		next++
		for next <= paddedN {
			temp[next-1]--
			next += int(valueOfLowest1Bit(uint32(next)))
		}
	}
	return perm
}

// computeLehmerCode is the forward transform (ComputeLehmerCode), used for
// round-trip testing.
func computeLehmerCode(perm []uint32, n int) []uint32 {
	code := make([]uint32, n)
	temp := make([]uint32, n+1)
	for idx := 0; idx < n; idx++ {
		s := perm[idx]
		penalty := uint32(0)
		i := s + 1
		for i != 0 {
			penalty += temp[i]
			i &= i - 1
		}
		code[idx] = s - penalty
		i = s + 1
		for i < uint32(n)+1 {
			temp[i]++
			i += valueOfLowest1Bit(i)
		}
	}
	return code
}

// kStrategyOrder maps each AC strategy to its coefficient-order class (0..12);
// transposed/related transforms share an order (coeff_order.h).
var kStrategyOrder = [acNumValidStrategies]uint8{
	0, 1, 1, 1, 2, 3, 4, 4, 5, 5, 6, 6, 1, 1,
	1, 1, 1, 1, 7, 8, 8, 9, 10, 10, 11, 12, 12,
}

const kPermutationContexts = 8

// coeffOrderContext = min(HybridUintConfig(0,0,0).Encode(val).token, 7).
func coeffOrderContext(val uint32) uint32 {
	token, _, _ := encodeHybridUint(newHybridUintConfig(0, 0, 0), val)
	if token >= kPermutationContexts {
		return kPermutationContexts - 1
	}
	return token
}
