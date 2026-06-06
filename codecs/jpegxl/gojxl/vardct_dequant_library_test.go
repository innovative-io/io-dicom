package gojxl

import "testing"

// TestDefaultQuantLibraryComputes builds the default dequant library and checks
// every populated kind computes a valid inverse-quant table with the
// low-frequency DC region zeroed.
func TestDefaultQuantLibraryComputes(t *testing.T) {
	lib := buildDefaultQuantLibrary()
	supported := []int{
		qtDCT, qtIDENTITY, qtDCT2X2, qtDCT4X4, qtDCT16X16, qtDCT32X32,
		qtDCT8X16, qtDCT8X32, qtDCT16X32, qtDCT4X8,
	}
	for _, kind := range supported {
		enc := lib[kind]
		if enc == nil {
			t.Fatalf("kind %d missing from default library", kind)
		}
		inv, ok := computeInvQuantTable(kind, enc)
		if !ok {
			t.Fatalf("kind %d: computeInvQuantTable failed", kind)
		}
		num := 64 * requiredSizeX[kind] * requiredSizeY[kind]
		if len(inv) != 3*num {
			t.Fatalf("kind %d: inv len %d, want %d", kind, len(inv), 3*num)
		}
		// DC of each channel must be zeroed; some AC weight must be positive.
		for c := 0; c < 3; c++ {
			if inv[c*num] != 0 {
				t.Errorf("kind %d channel %d: DC not zeroed (%g)", kind, c, inv[c*num])
			}
		}
		anyPos := false
		for _, v := range inv {
			if v > 0 {
				anyPos = true
				break
			}
		}
		if !anyPos {
			t.Errorf("kind %d: all-zero inverse table", kind)
		}
	}
}

// TestDefaultLibraryDCTMatchesHardcoded cross-checks the generated DCT(8x8)
// params against the standalone interpolation used elsewhere.
func TestDefaultLibraryDCTMatchesHardcoded(t *testing.T) {
	lib := buildDefaultQuantLibrary()
	enc := lib[qtDCT]
	inv, ok := computeInvQuantTable(qtDCT, enc)
	if !ok {
		t.Fatal("compute failed")
	}
	ref, _ := getQuantWeightsDCT(8, 8, &[3][]float32{
		{3150.0, 0.0, -0.4, -0.4, -0.4, -2.0},
		{560.0, 0.0, -0.3, -0.3, -0.3, -0.3},
		{512.0, -2.0, -1.0, 0.0, -1.0, -2.0},
	}, 6)
	for c := 0; c < 3; c++ {
		for i := 1; i < 64; i++ {
			if absf(inv[c*64+i]-ref[c*64+i]) > 1e-3 {
				t.Fatalf("DCT kind channel %d AC[%d]=%g, want %g", c, i, inv[c*64+i], ref[c*64+i])
			}
		}
	}
}

// TestDefaultLibraryUnsupportedNil documents that AFV and the 128/256 DCTs are
// not yet in the default library (decoder must reject those strategies). The
// DCT64X64 and DCT32X64 kinds are now populated.
func TestDefaultLibraryUnsupportedNil(t *testing.T) {
	lib := buildDefaultQuantLibrary()
	for _, kind := range []int{qtAFV0, qtDCT128X128, qtDCT64X128, qtDCT256X256, qtDCT128X256} {
		if lib[kind] != nil {
			t.Errorf("kind %d unexpectedly populated", kind)
		}
	}
}
