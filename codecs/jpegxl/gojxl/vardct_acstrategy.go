package gojxl

// AC strategy (transform type) table for VarDCT. The 27 strategies and their
// covered-block geometry are ported from AcStrategy in lib/jxl/ac_strategy.h.
// Strategy names are RowsXCols (height x width); covered_blocks_x is the width
// in 8x8 blocks, covered_blocks_y the height.

type acStrategyType uint32

const (
	acDCT                acStrategyType = 0
	acIDENTITY           acStrategyType = 1
	acDCT2X2             acStrategyType = 2
	acDCT4X4             acStrategyType = 3
	acDCT16X16           acStrategyType = 4
	acDCT32X32           acStrategyType = 5
	acDCT16X8            acStrategyType = 6
	acDCT8X16            acStrategyType = 7
	acDCT32X8            acStrategyType = 8
	acDCT8X32            acStrategyType = 9
	acDCT32X16           acStrategyType = 10
	acDCT16X32           acStrategyType = 11
	acDCT4X8             acStrategyType = 12
	acDCT8X4             acStrategyType = 13
	acAFV0               acStrategyType = 14
	acAFV1               acStrategyType = 15
	acAFV2               acStrategyType = 16
	acAFV3               acStrategyType = 17
	acDCT64X64           acStrategyType = 18
	acDCT64X32           acStrategyType = 19
	acDCT32X64           acStrategyType = 20
	acDCT128X128         acStrategyType = 21
	acDCT128X64          acStrategyType = 22
	acDCT64X128          acStrategyType = 23
	acDCT256X256         acStrategyType = 24
	acDCT256X128         acStrategyType = 25
	acDCT128X256         acStrategyType = 26
	acNumValidStrategies                = 27
)

// covered_blocks_x / covered_blocks_y / log2_covered_blocks LUTs (ac_strategy.h).
var acCoveredBlocksX = [acNumValidStrategies]uint8{
	1, 1, 1, 1, 2, 4, 1, 2, 1,
	4, 2, 4, 1, 1, 1, 1, 1, 1,
	8, 4, 8, 16, 8, 16, 32, 16, 32,
}
var acCoveredBlocksY = [acNumValidStrategies]uint8{
	1, 1, 1, 1, 2, 4, 2, 1, 4,
	1, 4, 2, 1, 1, 1, 1, 1, 1,
	8, 8, 4, 16, 16, 8, 32, 32, 16,
}
var acLog2CoveredBlocks = [acNumValidStrategies]uint8{
	0, 0, 0, 0, 2, 4, 1, 1, 2,
	2, 3, 3, 0, 0, 0, 0, 0, 0,
	6, 5, 5, 8, 7, 7, 10, 9, 9,
}

func (t acStrategyType) valid() bool { return uint32(t) < acNumValidStrategies }

func (t acStrategyType) coveredBlocksX() int    { return int(acCoveredBlocksX[t]) }
func (t acStrategyType) coveredBlocksY() int    { return int(acCoveredBlocksY[t]) }
func (t acStrategyType) log2CoveredBlocks() int { return int(acLog2CoveredBlocks[t]) }

// numBlocks is the total 8x8 blocks covered by the strategy.
func (t acStrategyType) numBlocks() int { return t.coveredBlocksX() * t.coveredBlocksY() }

// numCoeffs is the count of transform coefficients (= covered pixels).
func (t acStrategyType) numCoeffs() int { return t.numBlocks() * acBlockDim * acBlockDim }

// pixelWidth / pixelHeight give the transform's pixel extent.
func (t acStrategyType) pixelWidth() int  { return t.coveredBlocksX() * acBlockDim }
func (t acStrategyType) pixelHeight() int { return t.coveredBlocksY() * acBlockDim }

// isMultiblock reports whether the strategy covers more than one 8x8 block.
func (t acStrategyType) isMultiblock() bool { return t.numBlocks() > 1 }

// usesPlainDCT reports whether the inverse transform is a plain separable DCT of
// the covered region (true for the DCT* strategies). The special transforms —
// IDENTITY, DCT2X2, DCT4X4, DCT4X8/8X4 and the AFV corner DCTs — need bespoke
// inverse transforms implemented separately.
func (t acStrategyType) usesPlainDCT() bool {
	switch t {
	case acIDENTITY, acDCT2X2, acDCT4X4, acDCT4X8, acDCT8X4, acAFV0, acAFV1, acAFV2, acAFV3:
		return false
	default:
		return true
	}
}
