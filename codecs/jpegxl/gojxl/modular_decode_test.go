package gojxl

import (
	"os"
	"testing"
)

// TestModularDecodeGray8 decodes the gray8 fixture end-to-end (container →
// headers → frame header → TOC → LfGlobal preamble → MA tree → channel decode
// with the weighted predictor) and verifies every pixel against the original
// image. This exercises the entire pure-Go pipeline for the single-channel,
// no-transform, single-Weighted-leaf case.
func TestModularDecodeGray8(t *testing.T) {
	data, err := os.ReadFile("testdata/gray8_8x4_lossless.jxl")
	if err != nil {
		t.Fatal(err)
	}
	cs, err := codestream(data)
	if err != nil {
		t.Fatal(err)
	}
	b := newBitReader(cs[2:])
	sh, err := readSizeHeader(b)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := readImageMetadata(b)
	if err != nil {
		t.Fatal(err)
	}
	readTransformData(b, meta.XYBEncoded)
	if err := b.JumpToByteBoundary(); err != nil {
		t.Fatal(err)
	}
	if _, err := readFrameHeader(b, &meta); err != nil {
		t.Fatal(err)
	}
	if _, err := readTOC(b, 1); err != nil {
		t.Fatal(err)
	}

	// LfGlobal: flags==0 (no patches/splines/noise) → DequantMatrices DC bit.
	readDequantMatricesDC(b)
	hasTree := b.ReadBits(1) == 1
	if !hasTree {
		t.Fatal("expected global tree")
	}
	tree, err := decodeTree(b, 1<<20)
	if err != nil {
		t.Fatalf("decodeTree: %v", err)
	}
	if len(tree) != 1 || tree[0].property != -1 || tree[0].predictor != predWeighted {
		t.Fatalf("unexpected tree: %+v", tree)
	}
	code, chanCtxMap, err := decodeHistograms(b, (len(tree)+1)/2, false)
	if err != nil {
		t.Fatalf("global histograms: %v", err)
	}
	gh, err := readGroupHeader(b)
	if err != nil {
		t.Fatalf("group header: %v", err)
	}
	if len(gh.transforms) != 0 {
		t.Fatalf("expected no transforms, got %d", len(gh.transforms))
	}

	w, h := int(sh.Xsize), int(sh.Ysize)
	ctx := int(chanCtxMap[tree[0].lchild])
	reader := newANSSymbolReader(code, b, w)
	pix := decodeWPChannel(reader, b, ctx, w, h, gh.wp)
	if !reader.checkFinalState() {
		t.Fatal("ANS final state check failed")
	}

	want := []int32{
		0x00, 0x20, 0x40, 0x60, 0x80, 0xA0, 0xC0, 0xE0,
		0x10, 0x30, 0x50, 0x70, 0x90, 0xB0, 0xD0, 0xF0,
		0x08, 0x18, 0x28, 0x38, 0x48, 0x58, 0x68, 0x78,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
	}
	for i := range want {
		if pix[i] != want[i] {
			t.Fatalf("pixel %d: got %d want %d", i, pix[i], want[i])
		}
	}
}
