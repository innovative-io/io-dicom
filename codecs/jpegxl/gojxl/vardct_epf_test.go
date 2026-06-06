package gojxl

import "testing"

func TestComputeEPFSigma(t *testing.T) {
	p := defaultEPFParams()
	// Default sharp LUT is i/7.
	if absf(p.sharpLut[4]-4.0/7.0) > 1e-6 {
		t.Errorf("sharpLut[4]=%g, want 4/7", p.sharpLut[4])
	}
	// Fixture parameters: global_scale 8813 -> quantScale = 8813/65536; quant
	// field 6, sharpness 4. sigma_quant = 0.46 / (qs*6*-1.17157); sigma =
	// sigma_quant*4/7; stored = 1/sigma.
	quantScale := float32(8813) / 65536.0
	got := computeEPFSigma(quantScale, 6, 4, &p)
	// Hand computation: sigma_quant = 0.46/(0.13448*6*-1.17157) = -0.4866;
	// sigma = -0.4866*0.5714 = -0.2781; stored = 1/-0.2781 = -3.596.
	if absf(got-(-3.596)) > 0.05 {
		t.Errorf("stored sigma = %g, want ≈-3.596", got)
	}
	// -3.596 >= kMinSigma (-3.905), so EPF applies (is not skipped) for this block.
	if got < kMinSigma {
		t.Errorf("sigma %g < kMinSigma %g — block would be wrongly skipped", got, kMinSigma)
	}
	// Sharper (lower sharpness index) -> smaller magnitude smoothing.
	if computeEPFSigma(quantScale, 6, 0, &p) != 1.0/-1e-4 {
		t.Errorf("sharpness 0 -> sigma 0 -> clamped to -1e-4, stored %g", computeEPFSigma(quantScale, 6, 0, &p))
	}
}

func TestEPFWeight(t *testing.T) {
	// Smooth (sad=0) -> weight 1 (full averaging).
	if epfWeight(0, -5) != 1 {
		t.Errorf("weight(0,-5)=%g, want 1", epfWeight(0, -5))
	}
	// Edge (large sad, negative inv_sigma) -> clamped to 0.
	if epfWeight(1.0, -5) != 0 {
		t.Errorf("weight(1,-5)=%g, want 0 (edge preserved)", epfWeight(1.0, -5))
	}
	// Mid: sad=0.1, inv_sigma=-5 -> 1 - 0.5 = 0.5.
	if absf(epfWeight(0.1, -5)-0.5) > 1e-6 {
		t.Errorf("weight(0.1,-5)=%g, want 0.5", epfWeight(0.1, -5))
	}
}
