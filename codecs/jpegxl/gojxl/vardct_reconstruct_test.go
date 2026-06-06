package gojxl

import (
	"os"
	"testing"
)

// TestVarDCTReconstruct decodes the lossy VarDCT fixture end-to-end and checks
// block-interior pixels against the djxl reference. Interiors are far from block
// boundaries, where the not-yet-implemented Gaborish/EPF loop filters would
// alter values, so they should match a conforming decoder closely.
func TestVarDCTReconstruct(t *testing.T) {
	data, err := os.ReadFile("testdata/vardct_rgb16x16.jxl")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	img, err := DecodeVarDCT(data)
	if err != nil {
		t.Fatalf("DecodeVarDCT: %v", err)
	}
	if img.W != 16 || img.H != 16 || img.Channels != 3 {
		t.Fatalf("got %dx%d ch=%d, want 16x16x3", img.W, img.H, img.Channels)
	}
	get := func(x, y int) (int, int, int) {
		i := (y*img.W + x) * 3
		return int(img.Pixels[i]), int(img.Pixels[i+1]), int(img.Pixels[i+2])
	}
	// Block-center references from djxl (the conforming decoder).
	type ref struct {
		x, y    int
		r, g, b int
	}
	refs := []ref{
		{4, 4, 66, 65, 60},
		{12, 4, 188, 63, 127},
		{4, 12, 64, 191, 129},
		{12, 12, 192, 192, 191},
	}
	const tol = 5
	for _, rf := range refs {
		r, g, b := get(rf.x, rf.y)
		if abs(r-rf.r) > tol || abs(g-rf.g) > tol || abs(b-rf.b) > tol {
			t.Errorf("pixel (%d,%d) = (%d,%d,%d), djxl (%d,%d,%d), tol %d",
				rf.x, rf.y, r, g, b, rf.r, rf.g, rf.b, tol)
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
