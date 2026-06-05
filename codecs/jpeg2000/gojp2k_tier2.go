package jpeg2000

// Tier-2 packet decoding (ITU-T T.800 §B.9–B.10): walk packets in progression
// order, parse each packet header (code-block inclusion + zero-bitplane
// tag-trees, coding-pass counts, length codes), and slice the packet body into
// per-code-block coded-data segments for tier-1.
//
// Baseline scope: single tile, single layer, LRCP progression, default maximal
// precincts (one precinct per resolution), no SOP/EPH dependence, no per-pass
// termination (one segment per code-block). These cover the DICOM J2K fixtures.

import "math/bits"

// bioReader reads bits MSB-first from a packet header with JPEG 2000 bit
// un-stuffing, mirroring the reference (openjpeg opj_bio): a 16-bit window holds
// the previous and current bytes; when the previous byte was 0xFF the current
// byte contributes only 7 bits (its MSB is a stuffed 0).
type bioReader struct {
	data []byte
	bp   int // next byte index
	end  int
	buf  uint32
	ct   int
}

func newBIO(data []byte, start, end int) *bioReader {
	return &bioReader{data: data, bp: start, end: end}
}

func (b *bioReader) bytein() {
	b.buf = (b.buf << 8) & 0xFFFF
	if b.buf == 0xFF00 {
		b.ct = 7
	} else {
		b.ct = 8
	}
	nb := uint32(0xFF)
	if b.bp < b.end {
		nb = uint32(b.data[b.bp])
	}
	b.buf |= nb
	b.bp++
}

func (b *bioReader) readBit() int {
	if b.ct == 0 {
		b.bytein()
	}
	b.ct--
	return int((b.buf >> uint(b.ct)) & 1)
}

func (b *bioReader) read(n int) int {
	v := 0
	for i := 0; i < n; i++ {
		v = (v << 1) | b.readBit()
	}
	return v
}

// inalign byte-aligns the reader at the end of a packet header, consuming the
// trailing stuff byte when the last header byte was 0xFF. Returns the byte
// offset where the packet body begins.
func (b *bioReader) inalign() int {
	if (b.buf & 0xFF) == 0xFF {
		b.bytein()
	}
	b.ct = 0
	return b.bp
}

// tagTree is the JPEG 2000 tag tree (T.800 B.10.2) over a w×h leaf grid.
type tagTree struct {
	w, h    int
	nlevels int
	// nodes per level; level 0 = leaves, top level = single root.
	levelW []int
	levelH []int
	off    []int // start index in nodes[] for each level
	value  []int // current lower bound per node
	fixed  []bool
}

func newTagTree(w, h int) *tagTree {
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	t := &tagTree{w: w, h: h}
	lw, lh := w, h
	total := 0
	for {
		t.levelW = append(t.levelW, lw)
		t.levelH = append(t.levelH, lh)
		t.off = append(t.off, total)
		total += lw * lh
		if lw == 1 && lh == 1 {
			break
		}
		lw = (lw + 1) / 2
		lh = (lh + 1) / 2
	}
	t.nlevels = len(t.levelW)
	t.value = make([]int, total)
	t.fixed = make([]bool, total)
	return t
}

// decode refines the value of leaf (m,n) until it is known or known to be
// >= threshold, reading bits from bio. Returns the (possibly partial) value.
func (t *tagTree) decode(bio *bioReader, m, n, threshold int) int {
	minVal := 0
	for level := t.nlevels - 1; level >= 0; level-- {
		mm := m >> level
		nn := n >> level
		idx := t.off[level] + mm*t.levelW[level] + nn
		if t.value[idx] < minVal {
			t.value[idx] = minVal
		}
		for t.value[idx] < threshold && !t.fixed[idx] {
			if bio.readBit() == 1 {
				t.fixed[idx] = true
			} else {
				t.value[idx]++
			}
		}
		minVal = t.value[idx]
	}
	idx := t.off[0] + m*t.w + n
	return t.value[idx]
}

// precinctTrees holds the two tag-trees per subband within a precinct.
type precinctTrees struct {
	inclusion []*tagTree // per subband
	zeroBP    []*tagTree
}

// decodeTileTier2 parses all packets for the tile and fills each code-block's
// nzeroBP, npasses, and coded-data segment.
func decodeTileTier2(cs *j2kCodestream, comps []*tileComp, data []byte, start, end int) error {
	if cs.cod.progression != 0 {
		return errJ2KUnsupported // only LRCP for now
	}
	numLayers := cs.cod.numLayers
	NL := cs.cod.decompLevels

	// One precinctTrees per (component, resolution).
	trees := make([][]*precinctTrees, len(comps))
	for ci, tc := range comps {
		trees[ci] = make([]*precinctTrees, len(tc.resolutions))
		for ri := range tc.resolutions {
			res := &tc.resolutions[ri]
			pt := &precinctTrees{}
			for si := range res.subbands {
				sb := &res.subbands[si]
				pt.inclusion = append(pt.inclusion, newTagTree(sb.cbCols, sb.cbRows))
				pt.zeroBP = append(pt.zeroBP, newTagTree(sb.cbCols, sb.cbRows))
			}
			trees[ci][ri] = pt
		}
	}

	pos := start
	// LRCP: layer → resolution → component → precinct(1).
	for layer := 0; layer < numLayers; layer++ {
		for r := 0; r <= NL; r++ {
			for ci, tc := range comps {
				if r >= len(tc.resolutions) {
					continue
				}
				np, err := decodeOnePacket(cs, tc, trees[ci][r], r, layer, data, &pos, end)
				if err != nil {
					return err
				}
				_ = np
			}
		}
	}
	return nil
}

// decodeOnePacket parses one packet (header + body) starting at *pos, advancing
// *pos past the packet body.
func decodeOnePacket(cs *j2kCodestream, tc *tileComp, pt *precinctTrees, r, layer int, data []byte, pos *int, end int) (int, error) {
	res := &tc.resolutions[r]

	// Optional SOP marker (0xFF91) — skip if present. SOP is a fixed 6-byte
	// segment; require the whole thing to be in range before advancing so a
	// truncated tail cannot push the read position past end.
	p := *pos
	if cs.cod.useSOP && p+1 < end && data[p] == 0xFF && data[p+1] == 0x91 {
		if p+6 > end {
			return 0, errJ2KMalformed
		}
		p += 6
	}

	bio := newBIO(data, p, end)
	// First bit: packet non-empty flag.
	if bio.readBit() == 0 {
		// Empty packet. Header is byte-aligned; advance.
		*pos = bio.inalign()
		return 0, nil
	}

	type cbContrib struct {
		sb     *subband
		blk    *codeBlock
		length int
	}
	var contribs []cbContrib

	for si := range res.subbands {
		sb := &res.subbands[si]
		incl := pt.inclusion[si]
		zbp := pt.zeroBP[si]
		for by := 0; by < sb.cbRows; by++ {
			for bx := 0; bx < sb.cbCols; bx++ {
				blk := &sb.blocks[by*sb.cbCols+bx]
				included := false
				if !blk.included {
					// inclusion tag-tree: value == first-inclusion layer.
					v := incl.decode(bio, by, bx, layer+1)
					included = v <= layer
				} else {
					included = bio.readBit() == 1
				}
				if !included {
					continue
				}
				if !blk.included {
					// zero bit-plane count via its tag tree (read until fixed).
					nz := zbp.decode(bio, by, bx, 1<<30)
					blk.nzeroBP = nz
					blk.included = true
				}
				passes := readNumPasses(bio)
				var length int
				if cs.cod.htCodeblocks {
					// HT length signaling (ISO 15444-15): placeholder passes fold
					// into missing-MSBs, then one cleanup length and (for >1 pass) a
					// refinement length, with HT-specific bit widths.
					numPhld := (passes - 1) / 3
					blk.nzeroBP += numPhld
					eff := passes - numPhld*3
					for bio.readBit() == 1 {
						blk.lblock++
					}
					bits0 := blk.lblock + 31 - bits.LeadingZeros32(uint32(numPhld+1))
					len0 := bio.read(bits0)
					len1 := 0
					if eff > 1 {
						bits1 := blk.lblock
						if eff > 2 {
							bits1++
						}
						len1 = bio.read(bits1)
					}
					blk.npasses += eff
					blk.htLen1 = len0
					blk.htLen2 = len1
					length = len0 + len1
				} else {
					// Lblock increment (comma code).
					for bio.readBit() == 1 {
						blk.lblock++
					}
					lenBits := blk.lblock + floorLog2(passes)
					length = bio.read(lenBits)
					blk.npasses += passes
				}
				contribs = append(contribs, cbContrib{sb: sb, blk: blk, length: length})
			}
		}
	}

	// EPH marker handling + byte alignment of the header.
	bp := bio.inalign()
	if cs.cod.useEPH && bp+1 < end && data[bp] == 0xFF && data[bp+1] == 0x92 {
		bp += 2
	}

	// Packet body: code-block data segments, in the order contributions appear.
	for i := range contribs {
		c := &contribs[i]
		if bp+c.length > end {
			return 0, errJ2KMalformed
		}
		seg := data[bp : bp+c.length]
		c.blk.segs = append(c.blk.segs, seg)
		bp += c.length
	}
	*pos = bp
	return len(contribs), nil
}

// readNumPasses decodes the number-of-coding-passes code (T.800 Table B.4),
// matching the reference (openjpeg) decode.
func readNumPasses(b *bioReader) int {
	if b.readBit() == 0 {
		return 1
	}
	if b.readBit() == 0 {
		return 2
	}
	if n := b.read(2); n != 3 {
		return n + 3
	}
	if n := b.read(5); n != 31 {
		return n + 6
	}
	return b.read(7) + 37
}

func floorLog2(v int) int {
	if v <= 0 {
		return 0
	}
	n := 0
	for v > 1 {
		v >>= 1
		n++
	}
	return n
}
