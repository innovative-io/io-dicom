package jpeg2000

import (
	"os"
	"testing"
)

func TestGoJ2KTier2CTFixture(t *testing.T) {
	dcm, err := os.ReadFile("../../testdata/cornerstone-CTImage-jpeg2000-lossless.dcm")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	frame := extractFirstJ2KFrame(t, dcm)
	cs, err := parseCodestream(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tc, err := cs.buildTileComponent(0, 0)
	if err != nil {
		t.Fatalf("geometry: %v", err)
	}
	tp := cs.tileParts[0]
	if err := decodeTileTier2(cs, []*tileComp{tc}, frame, tp.dataStart, tp.dataEnd); err != nil {
		t.Fatalf("tier-2: %v", err)
	}

	totalBlocks, included, segBytes, withPasses := 0, 0, 0, 0
	for _, r := range tc.resolutions {
		for _, sb := range r.subbands {
			for i := range sb.blocks {
				blk := &sb.blocks[i]
				totalBlocks++
				if blk.included {
					included++
				}
				if blk.npasses > 0 {
					withPasses++
				}
				for _, s := range blk.segs {
					segBytes += len(s)
				}
			}
		}
	}
	tileLen := tp.dataEnd - tp.dataStart
	t.Logf("blocks=%d included=%d withPasses=%d segBytes=%d / tileDataLen=%d",
		totalBlocks, included, withPasses, segBytes, tileLen)
	if included == 0 || segBytes == 0 {
		t.Fatal("no code-blocks decoded — tier-2 produced nothing")
	}
	// A full-quality lossless stream should include essentially every block.
	if included < totalBlocks {
		t.Logf("note: %d/%d blocks included (some empty subband regions can be excluded)", included, totalBlocks)
	}
	// Segment bytes should be a large fraction of the tile data (headers are small).
	if segBytes < tileLen/2 {
		t.Errorf("segBytes=%d suspiciously small vs tileLen=%d (packet parsing likely wrong)", segBytes, tileLen)
	}
}
