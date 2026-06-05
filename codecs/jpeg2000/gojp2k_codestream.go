package jpeg2000

// Pure-Go JPEG 2000 (ISO/IEC 15444-1, ITU-T T.800) codestream decoder.
//
// This file implements stage 1: parsing the JPEG 2000 codestream marker
// segments (main header + tile-part headers) into a structured set of coding
// parameters. Later stages consume these to decode packets (tier-2), code-blocks
// (tier-1 / EBCOT), invert the wavelet transform, and apply the component
// transform.
//
// Scope note: the parser targets the DICOM JPEG 2000 transfer syntaxes, which
// carry a raw codestream (no JP2 boxes). Optional/rare features (POC, PPM/PPT
// packed headers, TLM/PLM length markers) are recognized and skipped where they
// do not affect the baseline single-tile decode path; unsupported essentials
// return errJ2KUnsupported so callers can fall back rather than mis-decode.

import (
	"encoding/binary"
	"errors"
)

var (
	errJ2KMalformed   = errors.New("jpeg2000: malformed codestream")
	errJ2KUnsupported = errors.New("jpeg2000: unsupported codestream feature")
)

// Marker codes (T.800 Table A.2). All markers are 0xFF-prefixed.
const (
	mSOC = 0xFF4F // start of codestream
	mSIZ = 0xFF51 // image and tile size
	mCOD = 0xFF52 // coding style default
	mCOC = 0xFF53 // coding style component
	mTLM = 0xFF55 // tile-part lengths
	mPLM = 0xFF57 // packet length, main header
	mPLT = 0xFF58 // packet length, tile-part header
	mQCD = 0xFF5C // quantization default
	mQCC = 0xFF5D // quantization component
	mRGN = 0xFF5E // region of interest
	mPOC = 0xFF5F // progression order change
	mPPM = 0xFF60 // packed packet headers, main header
	mPPT = 0xFF61 // packed packet headers, tile-part header
	mCRG = 0xFF63 // component registration
	mCOM = 0xFF64 // comment
	mSOT = 0xFF90 // start of tile-part
	mSOP = 0xFF91 // start of packet
	mEPH = 0xFF92 // end of packet header
	mSOD = 0xFF93 // start of data
	mEOC = 0xFFD9 // end of codestream
)

// componentInfo holds the per-component geometry from the SIZ marker.
type componentInfo struct {
	precision int  // bit depth (1..38)
	signed    bool // sample sign
	dx, dy    int  // sub-sampling (separation) factors
}

// codingStyle captures COD/COC parameters for a component (or the default).
type codingStyle struct {
	// Scod flags
	usePrecincts bool
	useSOP       bool
	useEPH       bool

	progression int // 0=LRCP 1=RLCP 2=RPCL 3=PCRL 4=CPRL (COD only; default)
	numLayers   int // (COD only; default)
	mct         int // 0=none 1=applied (COD only; default)

	decompLevels int   // number of wavelet decomposition levels (NL)
	cbW, cbH     int   // code-block dimensions (2^(exp+2))
	cbStyle      int   // code-block style flags
	transform    int   // 0=9/7 irreversible, 1=5/3 reversible
	precinctW    []int // per-resolution precinct width  (2^exp), low-to-high
	precinctH    []int // per-resolution precinct height (2^exp), low-to-high
}

// quantStyle captures QCD/QCC parameters for a component (or the default).
type quantStyle struct {
	style     int   // Sqcd low 5 bits: 0=none(reversible) 1=scalar derived 2=scalar expounded
	guardBits int   // Sqcd high 3 bits
	exponents []int // per-subband exponent (reversible: only exponents used)
	mantissas []int // per-subband mantissa (irreversible)
}

// j2kCodestream is the parsed main header plus enough tile bookkeeping for the
// baseline single-tile-part decode path.
type j2kCodestream struct {
	// SIZ
	xsiz, ysiz     int // image area lower-right
	xOsiz, yOsiz   int // image area upper-left offset
	xtsiz, ytsiz   int // nominal tile size
	xtOsiz, ytOsiz int
	comps          []componentInfo

	// Coding/quant: index 0 is the default (COD/QCD); per-component overrides
	// (COC/QCC) replace the corresponding entry.
	cod codingStyle
	coc []codingStyle // len == numComps; defaults copied from cod
	qcd quantStyle
	qcc []quantStyle // len == numComps; defaults copied from qcd

	// Tiles: byte ranges of each tile-part's packet data (after SOD).
	tileParts []tilePart
}

type tilePart struct {
	tileIndex int
	dataStart int // offset of first packet byte (after SOD)
	dataEnd   int // exclusive
}

func (c *j2kCodestream) numComps() int { return len(c.comps) }

// numTilesX/Y compute the tile grid dimensions.
func (c *j2kCodestream) numTilesX() int {
	return ceilDiv(c.xsiz-c.xtOsiz, c.xtsiz)
}
func (c *j2kCodestream) numTilesY() int {
	return ceilDiv(c.ysiz-c.ytOsiz, c.ytsiz)
}

func ceilDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// parseCodestream parses the main header and tile-part headers of a raw J2K
// codestream into a j2kCodestream.
func parseCodestream(data []byte) (*j2kCodestream, error) {
	if len(data) < 4 || be16(data, 0) != mSOC {
		return nil, errJ2KMalformed
	}
	cs := &j2kCodestream{}
	pos := 2

	// Main header: a sequence of marker segments until the first SOT.
	sizSeen := false

	for pos+2 <= len(data) {
		marker := be16(data, pos)
		if marker == mSOT {
			break
		}
		if marker>>8 != 0xFF {
			return nil, errJ2KMalformed
		}
		pos += 2
		// SOC has no length; everything else in the main header carries Lmar.
		segLen := int(be16(data, pos))
		if segLen < 2 || pos+segLen > len(data) {
			return nil, errJ2KMalformed
		}
		seg := data[pos+2 : pos+segLen]
		body := seg
		switch marker {
		case mSIZ:
			if err := cs.parseSIZ(body); err != nil {
				return nil, err
			}
			sizSeen = true
		case mCOD:
			if err := cs.parseCOD(body); err != nil {
				return nil, err
			}
		case mCOC:
			if err := cs.parseCOC(body); err != nil {
				return nil, err
			}
		case mQCD:
			if err := cs.parseQCD(body); err != nil {
				return nil, err
			}
		case mQCC:
			if err := cs.parseQCC(body); err != nil {
				return nil, err
			}
		case mCOM, mCRG:
			// informational — ignore
		case mPOC, mPPM, mTLM, mPLM, mRGN:
			// Recognized but not supported in the baseline path. RGN/POC/PPM
			// change decoding semantics; refuse rather than mis-decode.
			if marker == mPOC || marker == mPPM || marker == mRGN {
				return nil, errJ2KUnsupported
			}
		default:
			return nil, errJ2KUnsupported
		}
		pos += segLen
	}
	if !sizSeen {
		return nil, errJ2KMalformed
	}

	// Tile-part headers + data.
	if err := cs.parseTileParts(data, pos); err != nil {
		return nil, err
	}
	return cs, nil
}

func (cs *j2kCodestream) parseSIZ(b []byte) error {
	if len(b) < 36 {
		return errJ2KMalformed
	}
	// b[0:2] Rsiz (capabilities) — ignored for baseline.
	cs.xsiz = int(be32(b, 2))
	cs.ysiz = int(be32(b, 6))
	cs.xOsiz = int(be32(b, 10))
	cs.yOsiz = int(be32(b, 14))
	cs.xtsiz = int(be32(b, 18))
	cs.ytsiz = int(be32(b, 22))
	cs.xtOsiz = int(be32(b, 26))
	cs.ytOsiz = int(be32(b, 30))
	n := int(be16(b, 34))
	if n < 1 || n > 16384 || len(b) < 36+3*n {
		return errJ2KMalformed
	}
	if cs.xsiz <= cs.xOsiz || cs.ysiz <= cs.yOsiz || cs.xtsiz <= 0 || cs.ytsiz <= 0 {
		return errJ2KMalformed
	}
	cs.comps = make([]componentInfo, n)
	for i := 0; i < n; i++ {
		ssiz := b[36+3*i]
		cs.comps[i] = componentInfo{
			precision: int(ssiz&0x7F) + 1,
			signed:    ssiz&0x80 != 0,
			dx:        int(b[36+3*i+1]),
			dy:        int(b[36+3*i+2]),
		}
		if cs.comps[i].dx == 0 || cs.comps[i].dy == 0 {
			return errJ2KMalformed
		}
	}
	return nil
}

// parseSPcod parses the SPcod/SPcoc body shared by COD and COC, filling a
// codingStyle's decomposition/code-block/transform/precinct fields. usePrecincts
// indicates whether trailing per-resolution precinct bytes are present.
func parseSPcod(cs *codingStyle, b []byte, usePrecincts bool) error {
	if len(b) < 5 {
		return errJ2KMalformed
	}
	cs.decompLevels = int(b[0])
	cs.cbW = 1 << (int(b[1]&0x0F) + 2)
	cs.cbH = 1 << (int(b[2]&0x0F) + 2)
	cs.cbStyle = int(b[3])
	cs.transform = int(b[4])
	if cs.decompLevels > 32 || cs.cbW > 1024 || cs.cbH > 1024 || cs.cbW*cs.cbH > 4096 {
		return errJ2KMalformed
	}
	nres := cs.decompLevels + 1
	if usePrecincts {
		if len(b) < 5+nres {
			return errJ2KMalformed
		}
		cs.precinctW = make([]int, nres)
		cs.precinctH = make([]int, nres)
		for r := 0; r < nres; r++ {
			v := b[5+r]
			cs.precinctW[r] = 1 << int(v&0x0F)
			cs.precinctH[r] = 1 << int(v>>4)
		}
	} else {
		// Default maximal precincts: 2^15 in each resolution.
		cs.precinctW = make([]int, nres)
		cs.precinctH = make([]int, nres)
		for r := 0; r < nres; r++ {
			cs.precinctW[r] = 1 << 15
			cs.precinctH[r] = 1 << 15
		}
	}
	return nil
}

func (cs *j2kCodestream) parseCOD(b []byte) error {
	if len(b) < 5 {
		return errJ2KMalformed
	}
	scod := b[0]
	c := codingStyle{
		usePrecincts: scod&0x01 != 0,
		useSOP:       scod&0x02 != 0,
		useEPH:       scod&0x04 != 0,
		progression:  int(b[1]),
		numLayers:    int(be16(b, 2)),
		mct:          int(b[4]),
	}
	if err := parseSPcod(&c, b[5:], c.usePrecincts); err != nil {
		return err
	}
	cs.cod = c
	return nil
}

func (cs *j2kCodestream) parseCOC(b []byte) error {
	if len(cs.comps) == 0 {
		return errJ2KMalformed
	}
	// Ccoc is 1 byte if numComps < 257, else 2.
	idxLen := 1
	if cs.numComps() >= 257 {
		idxLen = 2
	}
	if len(b) < idxLen+1 {
		return errJ2KMalformed
	}
	var cidx int
	if idxLen == 1 {
		cidx = int(b[0])
	} else {
		cidx = int(be16(b, 0))
	}
	scoc := b[idxLen]
	c := cs.cod // inherit defaults
	c.usePrecincts = scoc&0x01 != 0
	if err := parseSPcod(&c, b[idxLen+1:], c.usePrecincts); err != nil {
		return err
	}
	if cidx < 0 || cidx >= cs.numComps() {
		return errJ2KMalformed
	}
	if cs.coc == nil {
		cs.coc = make([]codingStyle, cs.numComps())
	}
	cs.coc[cidx] = c
	return nil
}

func parseQuant(q *quantStyle, b []byte) error {
	if len(b) < 1 {
		return errJ2KMalformed
	}
	sqc := b[0]
	q.style = int(sqc & 0x1F)
	q.guardBits = int(sqc >> 5)
	rest := b[1:]
	switch q.style {
	case 0: // no quantization (reversible): one byte per subband, exponent in high 5 bits
		q.exponents = make([]int, len(rest))
		for i, v := range rest {
			q.exponents[i] = int(v >> 3)
		}
	case 1, 2: // scalar derived / expounded: two bytes per subband
		if len(rest)%2 != 0 {
			return errJ2KMalformed // odd body length cannot be whole 16-bit step sizes
		}
		n := len(rest) / 2
		q.exponents = make([]int, n)
		q.mantissas = make([]int, n)
		for i := 0; i < n; i++ {
			v := be16(rest, 2*i)
			q.exponents[i] = int(v >> 11)
			q.mantissas[i] = int(v & 0x07FF)
		}
	default:
		return errJ2KUnsupported
	}
	return nil
}

func (cs *j2kCodestream) parseQCD(b []byte) error {
	return parseQuant(&cs.qcd, b)
}

func (cs *j2kCodestream) parseQCC(b []byte) error {
	if len(cs.comps) == 0 {
		return errJ2KMalformed
	}
	idxLen := 1
	if cs.numComps() >= 257 {
		idxLen = 2
	}
	if len(b) < idxLen+1 {
		return errJ2KMalformed
	}
	var cidx int
	if idxLen == 1 {
		cidx = int(b[0])
	} else {
		cidx = int(be16(b, 0))
	}
	if cidx < 0 || cidx >= cs.numComps() {
		return errJ2KMalformed
	}
	var q quantStyle
	if err := parseQuant(&q, b[idxLen:]); err != nil {
		return err
	}
	if cs.qcc == nil {
		cs.qcc = make([]quantStyle, cs.numComps())
	}
	cs.qcc[cidx] = q
	return nil
}

// parseTileParts walks SOT/SOD segments, recording each tile-part's packet-data
// byte range. Psot (in the SOT marker) is the length from the first byte of the
// SOT marker to the end of the tile-part's data, so the tile-part data ends at
// sotOff+Psot (Psot==0 means the last tile-part runs to EOC).
func (cs *j2kCodestream) parseTileParts(data []byte, pos int) error {
	for pos+2 <= len(data) {
		if be16(data, pos) == mEOC {
			break
		}
		sotOff := pos
		if be16(data, pos) != mSOT {
			return errJ2KMalformed
		}
		pos += 2
		segLen := int(be16(data, pos))
		if segLen < 10 || pos+segLen > len(data) {
			return errJ2KMalformed
		}
		body := data[pos+2 : pos+segLen]
		isot := int(be16(body, 0))
		psot := int(be32(body, 2))
		pos += segLen

		// Tile-part header markers until SOD.
		for pos+2 <= len(data) {
			m := be16(data, pos)
			if m == mSOD {
				pos += 2
				break
			}
			if m>>8 != 0xFF {
				return errJ2KMalformed
			}
			pos += 2
			l := int(be16(data, pos))
			if l < 2 || pos+l > len(data) {
				return errJ2KMalformed
			}
			tb := data[pos+2 : pos+l]
			switch m {
			case mCOD:
				if err := cs.parseCOD(tb); err != nil {
					return err
				}
			case mCOC:
				if err := cs.parseCOC(tb); err != nil {
					return err
				}
			case mQCD:
				if err := cs.parseQCD(tb); err != nil {
					return err
				}
			case mQCC:
				if err := cs.parseQCC(tb); err != nil {
					return err
				}
			case mCOM, mPLT:
				// ignore
			case mRGN, mPOC, mPPT:
				return errJ2KUnsupported
			default:
				return errJ2KUnsupported
			}
			pos += l
		}

		dataStart := pos
		var dataEnd int
		if psot == 0 {
			dataEnd = scanToNextTilePartOrEOC(data, dataStart)
		} else {
			dataEnd = sotOff + psot
			if dataEnd < dataStart || dataEnd > len(data) {
				return errJ2KMalformed
			}
		}
		cs.tileParts = append(cs.tileParts, tilePart{tileIndex: isot, dataStart: dataStart, dataEnd: dataEnd})
		pos = dataEnd
	}
	if len(cs.tileParts) == 0 {
		return errJ2KMalformed
	}
	return nil
}

// scanToNextTilePartOrEOC finds the offset of the next SOT/EOC marker at/after
// pos (fallback for streams with Psot==0).
func scanToNextTilePartOrEOC(data []byte, pos int) int {
	for i := pos; i+1 < len(data); i++ {
		if data[i] == 0xFF {
			m := be16(data, i)
			if m == mSOT || m == mEOC {
				return i
			}
		}
	}
	return len(data)
}

// be16/be32 read big-endian integers with bounds already guaranteed by callers.
func be16(b []byte, off int) uint16 { return binary.BigEndian.Uint16(b[off:]) }
func be32(b []byte, off int) uint32 { return binary.BigEndian.Uint32(b[off:]) }
