package gojxl

import "math"

// XYB color conversion for VarDCT (xyb_encoded) frames. Ports the frozen opsin
// absorbance model from libjxl (lib/jxl/cms/opsin_params.h, dec_xyb-inl.h). The
// decoder needs the XYB->linear direction; the forward linear->XYB direction is
// kept for round-trip testing. Math is done in float32 to mirror libjxl, since
// matching djxl output bit-for-bit eventually depends on the same precision.

// kOpsinAbsorbanceBias[0..2] (all equal). cbrt of it centers the gamma values.
const opsinBias float32 = 0.0037930732552754493

// opsinCbrtBias = cbrtf(opsinBias). libjxl stores neg_bias_cbrt = -opsinCbrtBias.
var opsinCbrtBias = float32(math.Cbrt(float64(opsinBias)))

// kOpsinAbsorbanceMatrix (forward: linear RGB -> mixed LMS), row-major 3x3.
var fwdOpsinMatrix = [9]float32{
	0.30, 1.0 - 0.078 - 0.30, 0.078,
	0.23, 1.0 - 0.078 - 0.23, 0.078,
	0.24342268924547819, 0.20476744424496821, 1.0 - 0.24342268924547819 - 0.20476744424496821,
}

// kDefaultInverseOpsinAbsorbanceMatrix (inverse: mixed LMS -> linear RGB). This
// is the inverse of fwdOpsinMatrix. It is scaled by 255/intensity_target in
// libjxl; for the default intensity_target (255, SDR) the scale is 1.
var invOpsinMatrix = [9]float32{
	11.031566901960783, -9.866943921568629, -0.16462299647058826,
	-3.254147380392157, 4.418770392156863, -0.16462299647058826,
	-3.6588512862745097, 2.7129230470588235, 1.9459282392156863,
}

// xybToLinearRGB converts a stored XYB pixel to linear sRGB (intensity_target
// 255). Mirror of XybToRgb in dec_xyb-inl.h.
func xybToLinearRGB(x, y, b float32) (r, g, bl float32) {
	// Recombine X (red-green) and Y (luma) into the cube-root LMS gammas, then
	// add cbrt(bias) so that cubing recovers the mixed absorbances.
	gammaL := y + x + opsinCbrtBias
	gammaM := y - x + opsinCbrtBias
	gammaS := b + opsinCbrtBias

	mixedL := gammaL*gammaL*gammaL - opsinBias
	mixedM := gammaM*gammaM*gammaM - opsinBias
	mixedS := gammaS*gammaS*gammaS - opsinBias

	r = invOpsinMatrix[0]*mixedL + invOpsinMatrix[1]*mixedM + invOpsinMatrix[2]*mixedS
	g = invOpsinMatrix[3]*mixedL + invOpsinMatrix[4]*mixedM + invOpsinMatrix[5]*mixedS
	bl = invOpsinMatrix[6]*mixedL + invOpsinMatrix[7]*mixedM + invOpsinMatrix[8]*mixedS
	return r, g, bl
}

// linearRGBToXYB is the forward opsin transform (LinearRGBToXYB in enc_xyb.cc),
// used to validate xybToLinearRGB by round trip.
func linearRGBToXYB(r, g, b float32) (x, y, bl float32) {
	mixedL := fwdOpsinMatrix[0]*r + fwdOpsinMatrix[1]*g + fwdOpsinMatrix[2]*b + opsinBias
	mixedM := fwdOpsinMatrix[3]*r + fwdOpsinMatrix[4]*g + fwdOpsinMatrix[5]*b + opsinBias
	mixedS := fwdOpsinMatrix[6]*r + fwdOpsinMatrix[7]*g + fwdOpsinMatrix[8]*b + opsinBias
	mixedL = zeroIfNeg(mixedL)
	mixedM = zeroIfNeg(mixedM)
	mixedS = zeroIfNeg(mixedS)

	gammaL := float32(math.Cbrt(float64(mixedL))) - opsinCbrtBias
	gammaM := float32(math.Cbrt(float64(mixedM))) - opsinCbrtBias
	gammaS := float32(math.Cbrt(float64(mixedS))) - opsinCbrtBias

	x = 0.5 * (gammaL - gammaM)
	y = 0.5 * (gammaL + gammaM)
	bl = gammaS
	return x, y, bl
}

// absf is the absolute value of a float32.
func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func zeroIfNeg(v float32) float32 {
	if v < 0 {
		return 0
	}
	return v
}

// linearToSRGB applies the sRGB opto-electronic transfer function (linear ->
// non-linear sRGB), clamping to [0,1] as a conforming decoder does for 8-bit
// output. IEC 61966-2-1.
func linearToSRGB(v float32) float32 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*float32(math.Pow(float64(v), 1.0/2.4)) - 0.055
}

// srgbToLinear is the inverse sRGB transfer function (kept for testing).
func srgbToLinear(v float32) float32 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	if v <= 0.04045 {
		return v / 12.92
	}
	return float32(math.Pow(float64((v+0.055)/1.055), 2.4))
}
