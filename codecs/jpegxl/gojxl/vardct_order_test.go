package gojxl

import "testing"

// TestNaturalCoeffOrder validates the scan-order generator for the square and
// rectangular VarDCT block sizes: the order is a permutation of [0,size), starts
// at DC (index 0), and the order and its LUT are mutual inverses.
func TestNaturalCoeffOrder(t *testing.T) {
	cases := [][2]int{{1, 1}, {2, 2}, {4, 4}, {2, 1}, {1, 2}, {4, 2}, {8, 8}}
	for _, c := range cases {
		cbx, cby := c[0], c[1]
		order := naturalCoeffOrder(cbx, cby)
		lut := naturalCoeffOrderLut(cbx, cby)
		size := cbx * cby * 64
		if len(order) != size || len(lut) != size {
			t.Fatalf("%dx%d: len order=%d lut=%d want %d", cbx, cby, len(order), len(lut), size)
		}
		// Permutation check.
		seen := make([]bool, size)
		for k, v := range order {
			if v < 0 || v >= size || seen[v] {
				t.Fatalf("%dx%d: order not a permutation at scan %d (val %d)", cbx, cby, k, v)
			}
			seen[v] = true
		}
		// DC first.
		if order[0] != 0 {
			t.Errorf("%dx%d: order[0]=%d, want 0 (DC)", cbx, cby, order[0])
		}
		// Mutual inverse: lut[order[k]] == k.
		for k := 0; k < size; k++ {
			if lut[order[k]] != k {
				t.Fatalf("%dx%d: lut[order[%d]]=%d, want %d", cbx, cby, k, lut[order[k]], k)
			}
		}
	}
}

// TestNaturalOrder8x8DC checks that the first several scan positions of an 8x8
// block cover the lowest frequencies (top-left 2x2) as a sanity check that the
// zig-zag starts in the DC corner.
func TestNaturalOrder8x8DC(t *testing.T) {
	order := naturalCoeffOrder(1, 1)
	// The DC (0) and the three next-lowest frequencies (1=right of DC, 8=below
	// DC) must appear within the first four scan positions.
	first4 := map[int]bool{}
	for k := 0; k < 4; k++ {
		first4[order[k]] = true
	}
	for _, idx := range []int{0, 1, 8} {
		if !first4[idx] {
			t.Errorf("expected low-frequency coeff %d within first 4 scan positions; order[:4]=%v", idx, order[:4])
		}
	}
}
