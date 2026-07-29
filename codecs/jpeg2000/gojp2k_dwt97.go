package jpeg2000

// Inverse irreversible 9/7 wavelet transform (ITU-T T.800 Annex F, irreversible
// filter) for lossy JPEG 2000. Float lifting with the standard constants,
// applied per resolution level after scalar dequantization. Because the
// transform is floating-point, the result matches the openjpeg reference within
// a small rounding tolerance rather than bit-exactly.

const (
	dwtAlpha = float32(-1.586134342)
	dwtBeta  = float32(-0.052980118)
	dwtGamma = float32(0.882911075)
	dwtDelta = float32(0.443506852)
	dwtK     = float32(1.230174105)
	// openjpeg scales the high-pass on inverse by 2/K (two_invK), not 1/K, and
	// compensates by forcing the sub-band gain to 0 in the dequant step (see
	// BUG_WEIRD_TWO_INVK in openjpeg dwt.c/tcd.c). We mirror that exact scheme so
	// our float results track openjpeg's within rounding tolerance.
	dwtTwoInvK = float32(1.625732422)
)

// idwt97_1d performs the 1-D inverse 9/7 transform in place on a[0..len), whose
// samples occupy absolute positions [i0, i0+len). Even absolute positions are
// low-pass, odd are high-pass. Mirrors the reverse of openjpeg's forward lifting.
func idwt97_1d(a []float32, i0 int) {
	n := len(a)
	if n == 0 {
		return
	}
	if n == 1 {
		// openjpeg returns early for a lone sample (no scaling applied).
		return
	}
	// Undo scaling: even (low) *= K, odd (high) *= 2/K (openjpeg two_invK).
	// Strided by parity so the per-sample parity test disappears.
	evenStart := 0
	if (i0 & 1) != 0 {
		evenStart = 1
	}
	for k := evenStart; k < n; k += 2 {
		a[k] *= dwtK
	}
	for k := 1 - evenStart; k < n; k += 2 {
		a[k] *= dwtTwoInvK
	}
	// Inverse lifting, undoing the forward steps in reverse. The forward 9/7
	// applies alpha→odd, beta→even, gamma→odd, delta→even (T.800; openjpeg
	// opj_dwt_encode_1_real), so the inverse undoes delta→even, gamma→odd,
	// beta→even, alpha→odd. (Low-pass = even absolute positions.)
	//
	liftInverse97(a, i0, 0, dwtDelta) // undo delta: even -= delta*(odd-1 + odd+1)
	liftInverse97(a, i0, 1, dwtGamma) // undo gamma: odd  -= gamma*(even-1 + even+1)
	liftInverse97(a, i0, 0, dwtBeta)  // undo beta:  even -= beta*(odd-1 + odd+1)
	liftInverse97(a, i0, 1, dwtAlpha) // undo alpha: odd  -= alpha*(even-1 + even+1)
}

// liftInverse97 applies one inverse lifting step — a[k] -= coef*(a[k-1]+a[k+1])
// — to every sample whose absolute position has the given parity.
//
// Only k==0 and the final sample can reflect past the array bounds, so the ends
// are peeled and the interior runs on direct indices. The previous form routed
// every neighbour access through reflectIdx, paying two integer divisions per
// access (eight per sample across the four steps) to handle two edge cases.
// Passing coef by value rather than closing over a helper also keeps this
// inlinable and allocation-free.
func liftInverse97(a []float32, i0, parity int, coef float32) {
	n := len(a)
	k := 0
	if (i0 & 1) != parity {
		k = 1
	}
	if k >= n {
		return
	}
	if k == 0 {
		a[0] -= coef * (a[reflectIdx(-1, n)] + a[reflectIdx(1, n)])
		k = 2
	}
	for ; k+1 < n; k += 2 {
		a[k] -= coef * (a[k-1] + a[k+1])
	}
	if k < n {
		a[k] -= coef * (a[k-1] + a[reflectIdx(k+1, n)])
	}
}

// idwt97 runs the full multi-level inverse 9/7 transform for a tile-component,
// given each subband's dequantized float coefficients (band[expIdx]). Returns the
// full-resolution float samples (row-major, size of resolution NL).
func idwt97(tc *tileComp, band [][]float32) []float32 {
	NL := tc.style.decompLevels
	llSb := &tc.resolutions[0].subbands[0]
	cur := append([]float32(nil), band[llSb.expIdx]...)
	curW, curH := llSb.w(), llSb.h()

	for r := 1; r <= NL; r++ {
		res := &tc.resolutions[r]
		rw := res.x1 - res.x0
		rh := res.y1 - res.y0
		casx := res.x0 & 1
		casy := res.y0 & 1

		g := make([]float32, rw*rh)
		put := func(x, y int, v float32) {
			if x >= 0 && x < rw && y >= 0 && y < rh {
				g[y*rw+x] = v
			}
		}
		for iy := 0; iy < curH; iy++ {
			for ix := 0; ix < curW; ix++ {
				put(casx+2*ix, casy+2*iy, cur[iy*curW+ix])
			}
		}
		hl := &res.subbands[0]
		lh := &res.subbands[1]
		hh := &res.subbands[2]
		hlc, lhc, hhc := band[hl.expIdx], band[lh.expIdx], band[hh.expIdx]
		for iy := 0; iy < hl.h(); iy++ {
			for ix := 0; ix < hl.w(); ix++ {
				put((1-casx)+2*ix, casy+2*iy, hlc[iy*hl.w()+ix])
			}
		}
		for iy := 0; iy < lh.h(); iy++ {
			for ix := 0; ix < lh.w(); ix++ {
				put(casx+2*ix, (1-casy)+2*iy, lhc[iy*lh.w()+ix])
			}
		}
		for iy := 0; iy < hh.h(); iy++ {
			for ix := 0; ix < hh.w(); ix++ {
				put((1-casx)+2*ix, (1-casy)+2*iy, hhc[iy*hh.w()+ix])
			}
		}

		row := make([]float32, rw)
		for y := 0; y < rh; y++ {
			copy(row, g[y*rw:y*rw+rw])
			idwt97_1d(row, res.x0)
			copy(g[y*rw:y*rw+rw], row)
		}
		col := make([]float32, rh)
		for x := 0; x < rw; x++ {
			for y := 0; y < rh; y++ {
				col[y] = g[y*rw+x]
			}
			idwt97_1d(col, res.y0)
			for y := 0; y < rh; y++ {
				g[y*rw+x] = col[y]
			}
		}
		cur, curW, curH = g, rw, rh
	}
	return cur
}
