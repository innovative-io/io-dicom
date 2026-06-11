package gojxl

// acTokensByGroupJPEGCtx tokenizes the AC coefficients per group like
// acTokensByGroupJPEG, but additionally computes each token's raw entropy
// context exactly as decodeACGroup derives it on the decode side: the number-of-
// non-zeros token uses nonZeroContext(predicted, blockCtx); each coefficient
// token uses the zero-density context for its scan position and the running
// remaining-non-zeros count. The per-channel non-zero prediction grid is rebuilt
// per group (resolution = group block size >> subsampling shift), matching the
// decoder. maxCtx is the largest context index actually emitted.
func acTokensByGroupJPEGCtx(fd frameDimensions, bw, bh int, acCoeff [3][][]int32, order []int, csHShift, csVShift [3]int) (out [][]encTokenized, maxTok, maxCtx int) {
	m := defaultBlockCtxMap()
	gdb := int(fd.groupDim) / 8 // AC group dimension in 8x8 blocks
	xg := int(fd.xsizeGroups)
	out = make([][]encTokenized, fd.numGroups)
	for g := 0; g < int(fd.numGroups); g++ {
		bx0 := (g % xg) * gdb
		by0 := (g / xg) * gdb
		bx1 := minInt(bx0+gdb, bw)
		by1 := minInt(by0+gdb, bh)
		gw := bx1 - bx0
		gh := by1 - by0

		// Per-channel non-zero prediction grids at the channel's (subsampled)
		// resolution within the group.
		var nzGrid [3][]int32
		var ngw [3]int
		for c := 0; c < 3; c++ {
			ngw[c] = divCeilInt(gw, 1<<uint(csHShift[c]))
			ngh := divCeilInt(gh, 1<<uint(csVShift[c]))
			nzGrid[c] = make([]int32, ngw[c]*ngh)
		}

		var toks []encToken
		for by := by0; by < by1; by++ {
			ly := by - by0
			for bx := bx0; bx < bx1; bx++ {
				lx := bx - bx0
				for _, c := range [3]int{1, 0, 2} {
					hs, vs := csHShift[c], csVShift[c]
					sbx, sby := bx>>uint(hs), by>>uint(vs)
					if (sbx<<uint(hs) != bx) || (sby<<uint(vs) != by) {
						continue
					}
					slx, sly := lx>>uint(hs), ly>>uint(vs)
					cbw := bw >> uint(hs)
					blk := acCoeff[c][sby*cbw+sbx]
					blockCtx := m.blockContext(0, 0, 0, c)

					ng, ngwc := nzGrid[c], ngw[c]
					var rowTop []int32
					if sly > 0 {
						rowTop = ng[(sly-1)*ngwc : sly*ngwc]
					}
					row := ng[sly*ngwc : (sly+1)*ngwc]
					predicted := int(predictFromTopAndLeft(rowTop, row, slx, 32))

					nzCount := 0
					for k := 1; k < 64; k++ {
						if blk[order[k]] != 0 {
							nzCount++
						}
					}
					nzeroCtx := m.nonZeroContext(predicted, blockCtx)
					if nzeroCtx > maxCtx {
						maxCtx = nzeroCtx
					}
					toks = append(toks, encToken{ctx: nzeroCtx, value: uint32(nzCount)})
					ng[sly*ngwc+slx] = int32(nzCount) // coveredBlocks=1, log2cov=0

					zdOffset := m.zeroDensityContextsOffset(blockCtx)
					prev := 0
					if nzCount <= 64/16 {
						prev = 1
					}
					rem := nzCount
					for k := 1; k < 64 && rem != 0; k++ {
						ctx := zdOffset + zeroDensityContext(rem, k, 1, 0, prev)
						if ctx > maxCtx {
							maxCtx = ctx
						}
						v := blk[order[k]]
						toks = append(toks, encToken{ctx: ctx, value: packSigned(v)})
						if v != 0 {
							prev = 1
							rem--
						} else {
							prev = 0
						}
					}
				}
			}
		}
		tks, mx := tokenizeAll(toks)
		out[g] = tks
		if mx > maxTok {
			maxTok = mx
		}
	}
	return out, maxTok, maxCtx
}
