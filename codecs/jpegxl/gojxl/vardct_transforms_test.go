package gojxl

import "testing"

func TestIDCT2TopBlockHadamard(t *testing.T) {
	// S=2: a single 2x2 Hadamard on positions (0,0),(0,1),(1,0),(1,1).
	block := make([]float32, 64)
	a, b, c, d := float32(1), float32(2), float32(3), float32(4)
	block[0] = a
	block[1] = b
	block[8] = c
	block[9] = d
	out := make([]float32, 64)
	idct2TopBlock(2, block, acBlockDim, out)
	want := []float32{a + b + c + d, a + b - c - d, a - b + c - d, a - b - c + d}
	got := []float32{out[0], out[1], out[8], out[9]}
	for i := range want {
		if absf(got[i]-want[i]) > 1e-5 {
			t.Errorf("Hadamard[%d]=%g, want %g", i, got[i], want[i])
		}
	}
}

// TestDCT2X2DCFlat: a pure-DC coefficient block produces a flat pixel block.
func TestDCT2X2DCFlat(t *testing.T) {
	for _, v := range []float32{0, 1, 16, -7.5} {
		coeffs := make([]float32, 64)
		coeffs[0] = v
		pix := inverseDCT2X2(coeffs)
		for i, p := range pix {
			if absf(p-v) > 1e-4 {
				t.Fatalf("DC=%g: pixel %d = %g, want flat", v, i, p)
			}
		}
	}
}

// TestIdentityDCFlat: a pure-DC IDENTITY block is also flat.
func TestIdentityDCFlat(t *testing.T) {
	for _, v := range []float32{0, 1, 16, -3.25} {
		coeffs := make([]float32, 64)
		coeffs[0] = v
		pix := inverseIdentity(coeffs)
		for i, p := range pix {
			if absf(p-v) > 1e-4 {
				t.Fatalf("DC=%g: pixel %d = %g, want flat", v, i, p)
			}
		}
	}
}

// TestTransformsLinear: both transforms are linear (T(a+b)=T(a)+T(b)).
func TestTransformsLinear(t *testing.T) {
	mk := func(seed int) []float32 {
		c := make([]float32, 64)
		for i := range c {
			c[i] = float32((i*seed+3)%17) - 8
		}
		return c
	}
	for _, fn := range []func([]float32) []float32{inverseDCT2X2, inverseIdentity} {
		a, b := mk(5), mk(11)
		sum := make([]float32, 64)
		for i := range sum {
			sum[i] = a[i] + b[i]
		}
		ta, tb, tsum := fn(a), fn(b), fn(sum)
		for i := 0; i < 64; i++ {
			if absf(ta[i]+tb[i]-tsum[i]) > 1e-3 {
				t.Fatalf("non-linear at %d: %g + %g != %g", i, ta[i], tb[i], tsum[i])
			}
		}
	}
}
