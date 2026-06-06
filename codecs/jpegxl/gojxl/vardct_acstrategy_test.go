package gojxl

import (
	"math/bits"
	"testing"
)

// TestAcStrategyGeometry validates internal consistency of the covered-block
// LUTs and a few named strategies against their RowsXCols meaning.
func TestAcStrategyGeometry(t *testing.T) {
	for i := 0; i < acNumValidStrategies; i++ {
		s := acStrategyType(i)
		cx, cy := s.coveredBlocksX(), s.coveredBlocksY()
		if cx <= 0 || cy <= 0 {
			t.Fatalf("strategy %d: bad covered blocks %dx%d", i, cx, cy)
		}
		// cx and cy are powers of two; log2_covered == log2(cx*cy).
		if cx&(cx-1) != 0 || cy&(cy-1) != 0 {
			t.Errorf("strategy %d: covered blocks not powers of two (%d,%d)", i, cx, cy)
		}
		wantLog2 := bits.TrailingZeros(uint(cx * cy))
		if s.log2CoveredBlocks() != wantLog2 {
			t.Errorf("strategy %d: log2CoveredBlocks=%d, want %d (cx*cy=%d)", i, s.log2CoveredBlocks(), wantLog2, cx*cy)
		}
	}

	// Named-strategy checks: covered blocks are (width_blocks, height_blocks),
	// names are HeightxWidth.
	cases := []struct {
		t          acStrategyType
		wPx, hPx   int
		plain      bool
	}{
		{acDCT, 8, 8, true},
		{acDCT16X16, 16, 16, true},
		{acDCT32X32, 32, 32, true},
		{acDCT16X8, 8, 16, true},   // 16 tall, 8 wide
		{acDCT8X16, 16, 8, true},   // 8 tall, 16 wide
		{acDCT64X64, 64, 64, true},
		{acDCT256X256, 256, 256, true},
		{acDCT256X128, 128, 256, true}, // 256 tall, 128 wide
		{acIDENTITY, 8, 8, false},
		{acDCT2X2, 8, 8, false},
		{acDCT4X4, 8, 8, false},
		{acAFV0, 8, 8, false},
	}
	for _, c := range cases {
		if c.t.pixelWidth() != c.wPx || c.t.pixelHeight() != c.hPx {
			t.Errorf("strategy %d: pixel extent %dx%d, want %dx%d",
				c.t, c.t.pixelWidth(), c.t.pixelHeight(), c.wPx, c.hPx)
		}
		if c.t.usesPlainDCT() != c.plain {
			t.Errorf("strategy %d: usesPlainDCT=%v, want %v", c.t, c.t.usesPlainDCT(), c.plain)
		}
		if c.t.numCoeffs() != c.wPx*c.hPx {
			t.Errorf("strategy %d: numCoeffs=%d, want %d", c.t, c.t.numCoeffs(), c.wPx*c.hPx)
		}
	}
}

// TestAcStrategyNaturalOrder checks that naturalCoeffOrder accepts each
// strategy's covered-block geometry and yields a full permutation.
func TestAcStrategyNaturalOrder(t *testing.T) {
	for i := 0; i < acNumValidStrategies; i++ {
		s := acStrategyType(i)
		order := naturalCoeffOrder(s.coveredBlocksX(), s.coveredBlocksY())
		if len(order) != s.numCoeffs() {
			t.Fatalf("strategy %d: order len %d, want %d", i, len(order), s.numCoeffs())
		}
		seen := make([]bool, len(order))
		for _, v := range order {
			if v < 0 || v >= len(order) || seen[v] {
				t.Fatalf("strategy %d: order not a permutation", i)
			}
			seen[v] = true
		}
	}
}
