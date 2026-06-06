package gojxl

import (
	"errors"
	"os"
	"testing"
)

// TestVarDCTLfGlobalParse checks that the VarDCT frame decoder parses the
// LfGlobal global sections of a real cjxl lossy file without desync. The
// strongest signal is that the modular global tree decodes to a small, valid
// tree: a bit-misaligned quantizer/block-context/CfL parse would garble it.
func TestVarDCTLfGlobalParse(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_rgb16x16.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	st, err := decodeVarDCTFrame(data)
	if err != nil && !errors.Is(err, errVarDCTIncomplete) {
		t.Fatalf("LfGlobal parse failed: %v", err)
	}
	if st == nil {
		t.Fatal("nil state")
	}
	// 16x16 XYB VarDCT, single group/pass.
	if st.sh.Xsize != 16 || st.sh.Ysize != 16 {
		t.Errorf("size = %dx%d, want 16x16", st.sh.Xsize, st.sh.Ysize)
	}
	if !st.meta.XYBEncoded {
		t.Error("expected XYB-encoded frame")
	}
	// Quantizer: plausible global scale for a d1.0 lossy encode.
	if st.quant.globalScale <= 0 || st.quant.quantDC <= 0 {
		t.Errorf("implausible quantizer: global_scale=%d quant_dc=%d", st.quant.globalScale, st.quant.quantDC)
	}
	// Default block context map and CfL for this simple image.
	if st.blockCtx.numCtxs != 15 || st.blockCtx.numDCCtxs != 1 {
		t.Errorf("block ctx: numCtxs=%d numDCCtxs=%d, want 15/1", st.blockCtx.numCtxs, st.blockCtx.numDCCtxs)
	}
	if st.cmap.colorFactor != kDefaultColorFactor || st.cmap.baseX != 0 {
		t.Errorf("unexpected non-default CfL: %+v", st.cmap)
	}
	// The global tree decoded to a small valid tree (proves alignment).
	if len(st.tree) == 0 || len(st.tree) > 1024 {
		t.Errorf("global tree size %d looks wrong (desync?)", len(st.tree))
	}
	if st.code == nil || len(st.ctxMap) == 0 {
		t.Error("histograms not decoded")
	}
}

// TestVarDCTDCImage checks the LF/DC image decodes correctly: the ANS stream
// finalizes cleanly and the quantized DC values match the source structure of
// the 16x16 gradient (R=x*16, G=y*16, B=(x+y)*8) at 1/8 resolution (2x2 DC).
func TestVarDCTDCImage(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_rgb16x16.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	st, err := decodeVarDCTFrame(data)
	if err != nil && !errors.Is(err, errVarDCTIncomplete) {
		t.Fatalf("decode failed before DC: %v", err)
	}
	if st.dc == nil {
		t.Fatal("DC image not decoded")
	}
	if st.dc.w != 2 || st.dc.h != 2 {
		t.Fatalf("DC size %dx%d, want 2x2", st.dc.w, st.dc.h)
	}
	// Y (luma) increases with image brightness; blocks are row-major
	// (0,0),(1,0),(0,1),(1,1).
	for i := 1; i < 4; i++ {
		if st.dc.y[i] <= st.dc.y[i-1] {
			t.Errorf("DC luma not increasing: %v", st.dc.y)
			break
		}
	}
	// X (red-green): block (1,0) is red-dominant (R>G) -> positive; block (0,1)
	// is green-dominant -> negative.
	if st.dc.x[1] <= 0 {
		t.Errorf("DC X[red-dominant block] = %d, want positive", st.dc.x[1])
	}
	if st.dc.x[2] >= 0 {
		t.Errorf("DC X[green-dominant block] = %d, want negative", st.dc.x[2])
	}
}
