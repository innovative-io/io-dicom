package gojxl

import (
	"testing"
)

// TestXYBRoundTrip verifies xybToLinearRGB is the inverse of linearRGBToXYB
// across the [0,1] linear-RGB cube. The opsin bias cancels analytically, so the
// only error is float rounding; tolerate a small epsilon.
func TestXYBRoundTrip(t *testing.T) {
	const eps = 2e-4
	steps := []float32{0, 0.1, 0.25, 0.5, 0.75, 0.9, 1.0}
	maxErr := float32(0)
	for _, r := range steps {
		for _, g := range steps {
			for _, b := range steps {
				x, y, bb := linearRGBToXYB(r, g, b)
				gr, gg, gb := xybToLinearRGB(x, y, bb)
				for _, d := range []float32{absf(gr - r), absf(gg - g), absf(gb - b)} {
					if d > maxErr {
						maxErr = d
					}
				}
				if absf(gr-r) > eps || absf(gg-g) > eps || absf(gb-b) > eps {
					t.Errorf("round trip RGB(%.3g,%.3g,%.3g) -> XYB(%.4g,%.4g,%.4g) -> RGB(%.5g,%.5g,%.5g)",
						r, g, b, x, y, bb, gr, gg, gb)
				}
			}
		}
	}
	t.Logf("max XYB round-trip error: %.2e", maxErr)
}

// TestXYBNeutralGray checks that a neutral gray (R=G=B) maps to XYB with X≈0
// (the red-green axis is zero for achromatic colors) and inverts to the same
// gray.
func TestXYBNeutralGray(t *testing.T) {
	for _, v := range []float32{0.05, 0.2, 0.5, 0.8} {
		x, y, b := linearRGBToXYB(v, v, v)
		if absf(x) > 1e-6 {
			t.Errorf("gray %.2f: expected X≈0, got %.6f", v, x)
		}
		gr, gg, gb := xybToLinearRGB(x, y, b)
		if absf(gr-v) > 2e-4 || absf(gg-v) > 2e-4 || absf(gb-v) > 2e-4 {
			t.Errorf("gray %.2f: inverse gave (%.5f,%.5f,%.5f)", v, gr, gg, gb)
		}
	}
}

// TestSRGBTransfer checks the sRGB transfer function against known anchors and
// round-trips it.
func TestSRGBTransfer(t *testing.T) {
	cases := []struct{ lin, srgb float32 }{
		{0, 0},
		{1, 1},
		{0.0031308, 0.04045}, // the breakpoint
	}
	for _, c := range cases {
		got := linearToSRGB(c.lin)
		if absf(got-c.srgb) > 1e-4 {
			t.Errorf("linearToSRGB(%.7f) = %.6f, want %.6f", c.lin, got, c.srgb)
		}
	}
	for _, v := range []float32{0.001, 0.01, 0.1, 0.3, 0.6, 0.95} {
		if d := absf(srgbToLinear(linearToSRGB(v)) - v); d > 1e-4 {
			t.Errorf("sRGB round trip %.4f: err %.2e", v, d)
		}
	}
}
