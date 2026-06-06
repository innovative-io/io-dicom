package gojxl

// Edge-preserving filter (EPF) support for VarDCT. This file implements the
// per-block sigma computation (epf.cc ComputeSigma) and the EPF parameter
// defaults; the EPF1/EPF2 convolution passes (render_pipeline/stage_epf.cc) build
// on this and are added separately.
//
// EPF smooths within smooth regions while preserving edges: each output pixel is
// a weighted average of itself and its plus-shaped neighbors, with weights
// w = max(0, sad*inv_sigma + 1) where inv_sigma = (1/sigma)*sad_mul and sad is a
// directional sum of absolute differences. Larger sigma (smoother block) -> more
// smoothing; an edge (large sad) drives the weight to zero.

const (
	// kInvSigmaNum / kMinSigma from epf.h (note: negative by convention).
	kInvSigmaNum = -1.1715728752538099024
	kMinSigma    = -3.90524291751269967465540850526868
	// kEpfSharpEntries from loop_filter.h.
	kEpfSharpEntries = 8
)

// epfParams holds the EPF tuning parameters (LoopFilter defaults).
type epfParams struct {
	sharpLut      [kEpfSharpEntries]float32
	channelScale  [3]float32
	pass1Zeroflush float32
	pass2Zeroflush float32
	quantMul      float32
	borderSadMul  float32
}

func defaultEPFParams() epfParams {
	var p epfParams
	for i := 0; i < kEpfSharpEntries; i++ {
		p.sharpLut[i] = float32(i) / float32(kEpfSharpEntries-1)
	}
	p.channelScale = [3]float32{40.0, 5.0, 3.5}
	p.pass1Zeroflush = 0.45
	p.pass2Zeroflush = 0.6
	p.quantMul = 0.46
	p.borderSadMul = 0.6666666666666666
	return p
}

// computeEPFSigma returns the per-block stored sigma value (1/sigma) used by the
// EPF passes, for a block with the given raw quant field and EPF sharpness.
// quantScale is the quantizer's Scale() (global_scale / 65536). The block is
// skipped by EPF when the result is < kMinSigma.
func computeEPFSigma(quantScale float32, quantField int32, sharpness int, p *epfParams) float32 {
	sigmaQuant := p.quantMul / (quantScale * float32(quantField) * float32(kInvSigmaNum))
	sigma := sigmaQuant * p.sharpLut[sharpness]
	if sigma > -1e-4 {
		sigma = -1e-4 // avoid infinities (min(-1e-4, sigma))
	}
	return 1.0 / sigma
}

// epfWeight is the EPF bilateral weight: max(0, sad*invSigma + 1).
func epfWeight(sad, invSigma float32) float32 {
	v := sad*invSigma + 1
	if v < 0 {
		return 0
	}
	return v
}
