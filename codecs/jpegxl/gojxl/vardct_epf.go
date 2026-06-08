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
	sharpLut        [kEpfSharpEntries]float32
	channelScale    [3]float32
	pass1Zeroflush  float32
	pass2Zeroflush  float32
	quantMul        float32
	borderSadMul    float32
	pass2SigmaScale float32
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
	p.pass2SigmaScale = 6.5
	return p
}

// epfMirror reflects an out-of-range coordinate into [0,n) (image_ops.h Mirror).
func epfMirror(x, n int) int {
	for x < 0 || x >= n {
		if x < 0 {
			x = -x - 1
		} else {
			x = 2*n - 1 - x
		}
	}
	return x
}

// epfInvSigma returns the per-pixel inverse sigma: the block sigma scaled by the
// position-dependent sad multiplier (reduced on 8x8 block borders).
func epfInvSigma(blockSigma, sm, bsm float32, ix, iy int) float32 {
	smv := sm
	if iy == 0 || iy == 7 || ix == 0 || ix == 7 {
		smv = bsm
	}
	return blockSigma * smv
}

// applyEPF1 is the EPF pass-1 (5-tap directional-SAD bilateral) filter on the
// three XYB planes (stage_epf.cc EPF1Stage). Returns new planes.
func applyEPF1(pl *[3][]float32, w, h int, sigmaGrid []float32, bw int, p *epfParams) [3][]float32 {
	const sm = float32(1.65)
	bsm := sm * p.borderSadMul
	at := func(c, x, y int) float32 { return pl[c][epfMirror(y, h)*w+epfMirror(x, w)] }
	var out [3][]float32
	for c := range out {
		out[c] = make([]float32, w*h)
	}
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			bsigma := sigmaGrid[(py/8)*bw+(px/8)]
			if bsigma < kMinSigma {
				for c := 0; c < 3; c++ {
					out[c][py*w+px] = pl[c][py*w+px]
				}
				continue
			}
			invSigma := epfInvSigma(bsigma, sm, bsm, px%8, py%8)
			var sad [4]float32
			for c := 0; c < 3; c++ {
				p20, p21 := at(c, px, py-2), at(c, px, py-1)
				p11, p31 := at(c, px-1, py-1), at(c, px+1, py-1)
				p02, p12 := at(c, px-2, py), at(c, px-1, py)
				p22, p32, p42 := at(c, px, py), at(c, px+1, py), at(c, px+2, py)
				p13, p23, p33 := at(c, px-1, py+1), at(c, px, py+1), at(c, px+1, py+1)
				p24 := at(c, px, py+2)
				s0 := absf(p20-p21) + absf(p11-p12) + absf(p22-p21) + absf(p31-p32) + absf(p22-p23)
				s1 := absf(p11-p21) + absf(p02-p12) + absf(p12-p22) + absf(p22-p32) + absf(p13-p23)
				s2 := absf(p31-p21) + absf(p12-p22) + absf(p22-p32) + absf(p42-p32) + absf(p33-p23)
				s3 := absf(p22-p21) + absf(p13-p12) + absf(p22-p23) + absf(p33-p32) + absf(p24-p23)
				sc := p.channelScale[c]
				sad[0] += s0 * sc
				sad[1] += s1 * sc
				sad[2] += s2 * sc
				sad[3] += s3 * sc
			}
			// Neighbors: top, left, right, bottom (matching sad indices 0..3).
			nx := [4]int{px, px - 1, px + 1, px}
			ny := [4]int{py - 1, py, py, py + 1}
			for c := 0; c < 3; c++ {
				wsum := float32(1)
				acc := pl[c][py*w+px]
				for d := 0; d < 4; d++ {
					wt := epfWeight(sad[d], invSigma)
					wsum += wt
					acc += wt * at(c, nx[d], ny[d])
				}
				out[c][py*w+px] = acc / wsum
			}
		}
	}
	return out
}

// applyEPF2 is the EPF pass-2 (3x3-plus bilateral) filter (stage_epf.cc
// EPF2Stage): each neighbor weighted by its SAD to the center.
func applyEPF2(pl *[3][]float32, w, h int, sigmaGrid []float32, bw int, p *epfParams) [3][]float32 {
	sm := p.pass2SigmaScale * 1.65
	bsm := sm * p.borderSadMul
	at := func(c, x, y int) float32 { return pl[c][epfMirror(y, h)*w+epfMirror(x, w)] }
	var out [3][]float32
	for c := range out {
		out[c] = make([]float32, w*h)
	}
	nx := [4]int{0, -1, 1, 0}
	ny := [4]int{-1, 0, 0, 1}
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			bsigma := sigmaGrid[(py/8)*bw+(px/8)]
			if bsigma < kMinSigma {
				for c := 0; c < 3; c++ {
					out[c][py*w+px] = pl[c][py*w+px]
				}
				continue
			}
			invSigma := epfInvSigma(bsigma, sm, bsm, px%8, py%8)
			cx, cy, cb := pl[0][py*w+px], pl[1][py*w+px], pl[2][py*w+px]
			wsum := float32(1)
			var ax, ay, ab float32 = cx, cy, cb
			for d := 0; d < 4; d++ {
				vx, vy, vb := at(0, px+nx[d], py+ny[d]), at(1, px+nx[d], py+ny[d]), at(2, px+nx[d], py+ny[d])
				sad := p.channelScale[0]*absf(vx-cx) + p.channelScale[1]*absf(vy-cy) + p.channelScale[2]*absf(vb-cb)
				wt := epfWeight(sad, invSigma)
				wsum += wt
				ax += wt * vx
				ay += wt * vy
				ab += wt * vb
			}
			out[0][py*w+px] = ax / wsum
			out[1][py*w+px] = ay / wsum
			out[2][py*w+px] = ab / wsum
		}
	}
	return out
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
