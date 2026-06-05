package jpeg2000

// Tile-component geometry (ITU-T T.800 §B.5–B.7): from the parsed coding
// parameters, compute the resolution levels, subbands (LL / HL / LH / HH),
// precinct partition, and code-block partition for one tile-component. Tier-2
// (packet decode) and tier-1 (EBCOT) both consume this structure.

// subband orientations.
const (
	bandLL = 0
	bandHL = 1
	bandLH = 2
	bandHH = 3
)

type codeBlock struct {
	x0, y0, x1, y1 int // coordinates within the subband

	// tier-2 state
	included bool
	nzeroBP  int      // number of all-zero (insignificant) most-significant bitplanes
	lblock   int      // length-indicator state (starts at 3)
	npasses  int      // total coding passes decoded so far
	segs     [][]byte // coded data segments (concatenated per tier-1 needs)

	// HT (ISO 15444-15) split lengths: cleanup (htLen1) and refinement
	// SigProp+MagRef (htLen2) within the single stored segment.
	htLen1 int
	htLen2 int
}

func (cb *codeBlock) w() int { return cb.x1 - cb.x0 }
func (cb *codeBlock) h() int { return cb.y1 - cb.y0 }

type subband struct {
	orient         int
	x0, y0, x1, y1 int // coefficient coordinates
	gain           int // log2 gain (LL/HL/LH=... per quant), used for dequant exponent
	expIdx         int // index into the quant exponent/mantissa arrays

	// code-block grid (anchored to the canonical partition)
	cbCols, cbRows int
	blocks         []codeBlock
}

func (s *subband) w() int { return s.x1 - s.x0 }
func (s *subband) h() int { return s.y1 - s.y0 }

type resolution struct {
	level          int // r
	x0, y0, x1, y1 int // resolution coordinates
	subbands       []subband
	// precinct partition (number of precincts in x/y). Baseline assumes the
	// default maximal precincts → 1×1.
	pcW, pcH int // precinct dims (px, py) as exponents already resolved to sizes
	npx, npy int // number of precincts in x, y
}

type tileComp struct {
	comp           int
	x0, y0, x1, y1 int // tile-component coordinates
	style          codingStyle
	quant          quantStyle
	resolutions    []resolution
}

// ceilDivI computes ceil(a/b) for b>0 and any sign of a.
func ceilDivI(a, b int) int {
	if a >= 0 {
		return (a + b - 1) / b
	}
	return -((-a) / b)
}

// floorDivI computes floor(a/b) for b>0 and any sign of a.
func floorDivI(a, b int) int {
	if a >= 0 {
		return a / b
	}
	return -(((-a) + b - 1) / b)
}

// tileBounds returns the image-area rectangle of tile p (single-tile-part path
// supports arbitrary tile grids, but DICOM streams are single-tile).
func (cs *j2kCodestream) tileBounds(p int) (tx0, ty0, tx1, ty1 int) {
	ntx := cs.numTilesX()
	px := p % ntx
	py := p / ntx
	tx0 = max(cs.xtOsiz+px*cs.xtsiz, cs.xOsiz)
	ty0 = max(cs.ytOsiz+py*cs.ytsiz, cs.yOsiz)
	tx1 = min(cs.xtOsiz+(px+1)*cs.xtsiz, cs.xsiz)
	ty1 = min(cs.ytOsiz+(py+1)*cs.ytsiz, cs.ysiz)
	return
}

// quantFor returns the effective quantization for component c.
func (cs *j2kCodestream) quantFor(c int) quantStyle {
	if cs.qcc != nil && c < len(cs.qcc) && cs.qcc[c].exponents != nil {
		return cs.qcc[c]
	}
	return cs.qcd
}

// buildTileComponent computes the full geometry for tile p, component c.
func (cs *j2kCodestream) buildTileComponent(p, c int) (*tileComp, error) {
	style := cs.cod
	if cs.coc != nil && c < len(cs.coc) && (cs.coc[c].cbW != 0) {
		style = cs.coc[c]
	}
	quant := cs.quantFor(c)
	ci := cs.comps[c]

	tx0, ty0, tx1, ty1 := cs.tileBounds(p)
	tcx0 := ceilDivI(tx0, ci.dx)
	tcy0 := ceilDivI(ty0, ci.dy)
	tcx1 := ceilDivI(tx1, ci.dx)
	tcy1 := ceilDivI(ty1, ci.dy)

	tc := &tileComp{comp: c, x0: tcx0, y0: tcy0, x1: tcx1, y1: tcy1, style: style, quant: quant}
	NL := style.decompLevels

	// Subband index into the quant arrays follows resolution order:
	// r=0: LL; r=1: HL,LH,HH; r=2: HL,LH,HH; ...
	expIdx := 0
	for r := 0; r <= NL; r++ {
		nb := NL - r
		res := resolution{
			level: r,
			x0:    ceilDivI(tcx0, 1<<nb),
			y0:    ceilDivI(tcy0, 1<<nb),
			x1:    ceilDivI(tcx1, 1<<nb),
			y1:    ceilDivI(tcy1, 1<<nb),
		}
		// Precinct count (baseline supports the single-precinct case).
		pw, ph := style.precinctW[r], style.precinctH[r]
		res.pcW, res.pcH = pw, ph
		if res.x1 > res.x0 {
			res.npx = floorDivI(res.x1-1, pw) - floorDivI(res.x0, pw) + 1
		}
		if res.y1 > res.y0 {
			res.npy = floorDivI(res.y1-1, ph) - floorDivI(res.y0, ph) + 1
		}
		if res.npx > 1 || res.npy > 1 {
			return nil, errJ2KUnsupported // baseline: single precinct per resolution
		}
		res.npx, res.npy = 1, 1

		var orients []int
		if r == 0 {
			orients = []int{bandLL}
		} else {
			orients = []int{bandHL, bandLH, bandHH}
		}
		for _, o := range orients {
			sb := buildSubband(o, r, NL, tcx0, tcy0, tcx1, tcy1, style)
			sb.expIdx = expIdx
			expIdx++
			partitionCodeBlocks(&sb, r, style)
			res.subbands = append(res.subbands, sb)
		}
		tc.resolutions = append(tc.resolutions, res)
	}
	return tc, nil
}

// buildSubband computes a subband's coefficient rectangle (T.800 B.5).
func buildSubband(orient, r, NL, tcx0, tcy0, tcx1, tcy1 int, _ codingStyle) subband {
	if r == 0 {
		// LL band at the coarsest decomposition level.
		nb := NL
		return subband{
			orient: bandLL,
			x0:     ceilDivI(tcx0, 1<<nb), y0: ceilDivI(tcy0, 1<<nb),
			x1: ceilDivI(tcx1, 1<<nb), y1: ceilDivI(tcy1, 1<<nb),
			gain: 0,
		}
	}
	nb := NL - r + 1
	xob, yob := 0, 0
	switch orient {
	case bandHL:
		xob = 1
	case bandLH:
		yob = 1
	case bandHH:
		xob, yob = 1, 1
	}
	half := 1 << (nb - 1)
	sb := subband{
		orient: orient,
		x0:     ceilDivI(tcx0-half*xob, 1<<nb),
		y0:     ceilDivI(tcy0-half*yob, 1<<nb),
		x1:     ceilDivI(tcx1-half*xob, 1<<nb),
		y1:     ceilDivI(tcy1-half*yob, 1<<nb),
	}
	// log2 gain: LL=0, HL/LH=1, HH=2.
	switch orient {
	case bandHL, bandLH:
		sb.gain = 1
	case bandHH:
		sb.gain = 2
	}
	return sb
}

// partitionCodeBlocks tiles the subband with the (anchored) code-block grid.
func partitionCodeBlocks(sb *subband, r int, style codingStyle) {
	// Effective code-block size, clamped to the precinct partition. With default
	// maximal precincts this is just the nominal code-block size.
	xcb := log2(style.cbW)
	ycb := log2(style.cbH)
	// For resolutions > 0 the code-block partition is at the precinct grid / 2.
	ppx := log2(style.precinctW[r])
	ppy := log2(style.precinctH[r])
	if r > 0 {
		ppx--
		ppy--
	}
	if xcb > ppx {
		xcb = ppx
	}
	if ycb > ppy {
		ycb = ppy
	}
	cbw := 1 << xcb
	cbh := 1 << ycb

	if sb.x1 <= sb.x0 || sb.y1 <= sb.y0 {
		sb.cbCols, sb.cbRows = 0, 0
		return
	}
	col0 := floorDivI(sb.x0, cbw)
	col1 := floorDivI(sb.x1-1, cbw)
	row0 := floorDivI(sb.y0, cbh)
	row1 := floorDivI(sb.y1-1, cbh)
	sb.cbCols = col1 - col0 + 1
	sb.cbRows = row1 - row0 + 1
	sb.blocks = make([]codeBlock, sb.cbCols*sb.cbRows)
	for ry := row0; ry <= row1; ry++ {
		for rx := col0; rx <= col1; rx++ {
			bx0 := max(sb.x0, rx*cbw)
			by0 := max(sb.y0, ry*cbh)
			bx1 := min(sb.x1, (rx+1)*cbw)
			by1 := min(sb.y1, (ry+1)*cbh)
			sb.blocks[(ry-row0)*sb.cbCols+(rx-col0)] = codeBlock{
				x0: bx0, y0: by0, x1: bx1, y1: by1, lblock: 3,
			}
		}
	}
}

func log2(v int) int {
	n := 0
	for v > 1 {
		v >>= 1
		n++
	}
	return n
}
