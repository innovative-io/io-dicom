package jpeg2000

import (
	"os"
	"testing"
)

func TestGoJ2KGeometryCTFixture(t *testing.T) {
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
		t.Fatalf("buildTileComponent: %v", err)
	}
	// 512x512, 5 levels → resolutions r0..r5; r0 size 16x16 (512/2^5).
	if len(tc.resolutions) != 6 {
		t.Fatalf("resolutions = %d, want 6", len(tc.resolutions))
	}
	r0 := tc.resolutions[0]
	if r0.subbands[0].orient != bandLL || r0.subbands[0].w() != 16 || r0.subbands[0].h() != 16 {
		t.Errorf("r0 LL = %dx%d, want 16x16", r0.subbands[0].w(), r0.subbands[0].h())
	}
	rTop := tc.resolutions[5]
	if len(rTop.subbands) != 3 {
		t.Errorf("r5 subbands = %d, want 3 (HL,LH,HH)", len(rTop.subbands))
	}
	// r5 detail bands are at decomposition level 1 → 256x256 each.
	for _, sb := range rTop.subbands {
		if sb.w() != 256 || sb.h() != 256 {
			t.Errorf("r5 band %d = %dx%d, want 256x256", sb.orient, sb.w(), sb.h())
		}
		// 256/64 = 4 → 4x4 code-blocks
		if sb.cbCols != 4 || sb.cbRows != 4 {
			t.Errorf("r5 band %d code-blocks = %dx%d, want 4x4", sb.orient, sb.cbCols, sb.cbRows)
		}
	}
	// log the full structure
	total := 0
	for _, r := range tc.resolutions {
		for _, sb := range r.subbands {
			total += len(sb.blocks)
			t.Logf("r%d band%d %dx%d cb=%dx%d (%d blocks) expIdx=%d gain=%d",
				r.level, sb.orient, sb.w(), sb.h(), sb.cbCols, sb.cbRows, len(sb.blocks), sb.expIdx, sb.gain)
		}
	}
	t.Logf("total code-blocks = %d", total)
}
