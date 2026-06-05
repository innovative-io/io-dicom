package jpeg2000

// Inverse reversible 5/3 wavelet transform (ITU-T T.800 Annex F, reversible
// filter). Reconstructs a tile-component from its subband coefficients, level by
// level, using integer lifting with whole-sample-symmetric boundary extension.

// reflectIdx maps an index to the symmetric (mirror, edge-not-repeated) extension
// of a length-n signal: ..., 2,1,0,1,2,...,n-1,n-2,...
func reflectIdx(k, n int) int {
	if n == 1 {
		return 0
	}
	period := 2 * (n - 1)
	k = ((k % period) + period) % period
	if k >= n {
		k = period - k
	}
	return k
}

// idwt53_1d performs the 1-D inverse 5/3 transform in place on a[0..len), whose
// samples occupy absolute positions [i0, i0+len). Even absolute positions are
// low-pass, odd are high-pass (T.800 F.3.4).
func idwt53_1d(a []int32, i0 int) {
	n := len(a)
	if n == 0 {
		return
	}
	if n == 1 {
		// Single low-pass sample at an even position needs the DC scaling only in
		// the cas==1 single-high edge case; for the common low-only case it passes
		// through unchanged.
		if (i0 & 1) == 1 {
			a[0] /= 2
		}
		return
	}
	at := func(k int) int32 { return a[reflectIdx(k, n)] }
	// Even (low) update: y[2n] -= (y[2n-1] + y[2n+1] + 2) >> 2
	for k := 0; k < n; k++ {
		if (i0+k)&1 == 0 {
			a[k] -= (at(k-1) + at(k+1) + 2) >> 2
		}
	}
	// Odd (high) predict: y[2n+1] += (y[2n] + y[2n+2]) >> 1
	for k := 0; k < n; k++ {
		if (i0+k)&1 == 1 {
			a[k] += (at(k-1) + at(k+1)) >> 1
		}
	}
}

// idwt53 runs the full multi-level inverse 5/3 transform for a tile-component,
// given each subband's decoded coefficients (band[expIdx] indexed by subband
// expIdx). Returns the full-resolution samples (row-major, size of resolution NL).
func idwt53(tc *tileComp, band [][]int32) []int32 {
	NL := tc.style.decompLevels

	// Resolution 0 is the LL band itself.
	llSb := &tc.resolutions[0].subbands[0]
	cur := append([]int32(nil), band[llSb.expIdx]...)
	curW, curH := llSb.w(), llSb.h()

	for r := 1; r <= NL; r++ {
		res := &tc.resolutions[r]
		rw := res.x1 - res.x0
		rh := res.y1 - res.y0
		casx := res.x0 & 1
		casy := res.y0 & 1

		g := make([]int32, rw*rh)
		put := func(x, y int, v int32) {
			if x >= 0 && x < rw && y >= 0 && y < rh {
				g[y*rw+x] = v
			}
		}
		// Place the 4 bands into the deinterleaved grid.
		// LL = previous reconstruction (cur, curW×curH).
		for iy := 0; iy < curH; iy++ {
			for ix := 0; ix < curW; ix++ {
				put(casx+2*ix, casy+2*iy, cur[iy*curW+ix])
			}
		}
		hl := &res.subbands[0] // HL: high-x, low-y
		lh := &res.subbands[1] // LH: low-x, high-y
		hh := &res.subbands[2] // HH: high-x, high-y
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

		// Horizontal inverse on each row, then vertical on each column.
		row := make([]int32, rw)
		for y := 0; y < rh; y++ {
			copy(row, g[y*rw:y*rw+rw])
			idwt53_1d(row, res.x0)
			copy(g[y*rw:y*rw+rw], row)
		}
		col := make([]int32, rh)
		for x := 0; x < rw; x++ {
			for y := 0; y < rh; y++ {
				col[y] = g[y*rw+x]
			}
			idwt53_1d(col, res.y0)
			for y := 0; y < rh; y++ {
				g[y*rw+x] = col[y]
			}
		}

		cur, curW, curH = g, rw, rh
	}
	return cur
}
