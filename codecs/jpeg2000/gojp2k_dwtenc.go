package jpeg2000

// Forward reversible 5/3 wavelet transform (ITU-T T.800 Annex F), the exact
// inverse of idwt53. Encoding analysis: predict (odd) then update (even), the
// reverse of the synthesis lifting, with whole-sample-symmetric boundaries.

// fdwt53_1d performs the 1-D forward 5/3 transform in place on a[0..n), inverse
// of idwt53_1d.
func fdwt53_1d(a []int32, i0 int) {
	n := len(a)
	if n == 0 {
		return
	}
	if n == 1 {
		// Inverse of idwt53_1d's single-sample case (which halves an odd sample).
		if (i0 & 1) == 1 {
			a[0] *= 2
		}
		return
	}
	at := func(k int) int32 { return a[reflectIdx(k, n)] }
	// Predict (odd/high): y[2n+1] -= (y[2n] + y[2n+2]) >> 1
	for k := 0; k < n; k++ {
		if (i0+k)&1 == 1 {
			a[k] -= (at(k-1) + at(k+1)) >> 1
		}
	}
	// Update (even/low): y[2n] += (y[2n-1] + y[2n+1] + 2) >> 2
	for k := 0; k < n; k++ {
		if (i0+k)&1 == 0 {
			a[k] += (at(k-1) + at(k+1) + 2) >> 2
		}
	}
}

// fdwt53 runs the full multi-level forward 5/3 transform on the tile-component
// samples (row-major, full resolution), returning each subband's coefficients
// indexed by subband expIdx — the layout idwt53 consumes.
func fdwt53(tc *tileComp, samples []int32) [][]int32 {
	NL := tc.style.decompLevels

	maxExp := 0
	for _, r := range tc.resolutions {
		for _, sb := range r.subbands {
			if sb.expIdx+1 > maxExp {
				maxExp = sb.expIdx + 1
			}
		}
	}
	band := make([][]int32, maxExp)

	// cur holds the current resolution image; start at full resolution (NL).
	full := &tc.resolutions[NL]
	cur := append([]int32(nil), samples...)
	curW := full.x1 - full.x0
	curH := full.y1 - full.y0

	for r := NL; r >= 1; r-- {
		res := &tc.resolutions[r]
		rw := res.x1 - res.x0
		rh := res.y1 - res.y0
		casx := res.x0 & 1
		casy := res.y0 & 1

		if rw <= 0 || rh <= 0 {
			continue
		}
		// Forward is the reverse of idwt53: vertical then horizontal.
		g := make([]int32, rw*rh)
		copy(g, cur)
		col := make([]int32, rh)
		for x := 0; x < rw; x++ {
			for y := 0; y < rh; y++ {
				col[y] = g[y*rw+x]
			}
			fdwt53_1d(col, res.y0)
			for y := 0; y < rh; y++ {
				g[y*rw+x] = col[y]
			}
		}
		row := make([]int32, rw)
		for y := 0; y < rh; y++ {
			copy(row, g[y*rw:y*rw+rw])
			fdwt53_1d(row, res.x0)
			copy(g[y*rw:y*rw+rw], row)
		}

		// Deinterleave g into LL (→ next coarser cur) and HL/LH/HH bands.
		at := func(x, y int) int32 {
			if x >= 0 && x < rw && y >= 0 && y < rh {
				return g[y*rw+x]
			}
			return 0
		}
		// The LL of resolution r is the resolution-(r-1) image (idwt53 carries it
		// as cur), not a stored subband: subbands[0] is HL for r-1 ≥ 1.
		llRes := &tc.resolutions[r-1]
		llW := llRes.x1 - llRes.x0
		llH := llRes.y1 - llRes.y0
		ll := make([]int32, llW*llH)
		for iy := 0; iy < llH; iy++ {
			for ix := 0; ix < llW; ix++ {
				ll[iy*llW+ix] = at(casx+2*ix, casy+2*iy)
			}
		}
		hl := &res.subbands[0]
		lh := &res.subbands[1]
		hh := &res.subbands[2]
		hlc := make([]int32, hl.w()*hl.h())
		for iy := 0; iy < hl.h(); iy++ {
			for ix := 0; ix < hl.w(); ix++ {
				hlc[iy*hl.w()+ix] = at((1-casx)+2*ix, casy+2*iy)
			}
		}
		lhc := make([]int32, lh.w()*lh.h())
		for iy := 0; iy < lh.h(); iy++ {
			for ix := 0; ix < lh.w(); ix++ {
				lhc[iy*lh.w()+ix] = at(casx+2*ix, (1-casy)+2*iy)
			}
		}
		hhc := make([]int32, hh.w()*hh.h())
		for iy := 0; iy < hh.h(); iy++ {
			for ix := 0; ix < hh.w(); ix++ {
				hhc[iy*hh.w()+ix] = at((1-casx)+2*ix, (1-casy)+2*iy)
			}
		}
		band[hl.expIdx] = hlc
		band[lh.expIdx] = lhc
		band[hh.expIdx] = hhc

		cur, curW, curH = ll, llW, llH
	}
	// Resolution-0 LL band.
	band[tc.resolutions[0].subbands[0].expIdx] = cur
	_ = curW
	_ = curH
	return band
}
