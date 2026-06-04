package jpegls

// Pure-Go JPEG-LS (ISO/IEC 14495-1, ITU-T T.87) decoder — the LOCO-I algorithm
// — for the DICOM JPEG-LS Lossless transfer syntax (1.2.840.10008.1.2.4.80).
// It lets CGO_ENABLED=0 builds decode lossless JPEG-LS, and is byte-for-byte
// identical to charls on lossless streams.
//
// Supports 2–16 bit precision and single-component (grayscale) images with
// regular and run mode. Near-lossless (.81) and interleaved multi-component
// scans are not yet handled here and are deferred to charls (the scan parser
// returns an unsupported error for them so the higher-priority charls backend
// takes over when built with -tags charls).

import (
	"errors"
)

var (
	errJLSMalformed   = errors.New("gojpegls: malformed JPEG-LS payload")
	errJLSUnsupported = errors.New("gojpegls: unsupported JPEG-LS variant")
	errJLSOutputSize  = errors.New("gojpegls: decoded payload does not fit output buffer")
)

const (
	jlsSOI   = 0xD8
	jlsEOI   = 0xD9
	jlsSOF55 = 0xF7 // JPEG-LS frame
	jlsLSE   = 0xF8 // JPEG-LS preset parameters
	jlsSOS   = 0xDA
)

// jlsRunJ is the run-length order table J (T.87 A.6 / Table A.2).
var jlsRunJ = [32]int{0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3,
	4, 4, 5, 5, 6, 6, 7, 7, 8, 9, 10, 11, 12, 13, 14, 15}

type jlsComponent struct {
	id   byte
	h, v int
}

type jlsFrame struct {
	precision int
	width     int
	height    int
	comps     []jlsComponent

	near   int
	ilv    int // interleave: 0 none, 1 line, 2 sample
	maxval int

	t1, t2, t3 int
	reset      int
}

// jlsBitReader reads MSB-first bits with JPEG-LS bit stuffing: when the encoder
// emits a 0xFF data byte it inserts a 0 bit immediately after, so on decode the
// bit following a 0xFF byte is a stuffed bit to skip (the next byte yields only
// 7 payload bits, bits 6..0). Past end-of-data, zero bits are returned.
type jlsBitReader struct {
	data      []byte
	pos       int
	cur       byte
	bitpos    int  // remaining unread bits in cur (0 = need next byte)
	lastWasFF bool // the previously loaded byte was 0xFF
}

func (br *jlsBitReader) readBit() int {
	if br.bitpos == 0 {
		if br.pos >= len(br.data) {
			return 0 // pad with zeros past EOF
		}
		b := br.data[br.pos]
		br.pos++
		br.cur = b
		if br.lastWasFF {
			br.bitpos = 7 // skip the stuffed MSB
		} else {
			br.bitpos = 8
		}
		br.lastWasFF = b == 0xFF
	}
	br.bitpos--
	return int((br.cur >> br.bitpos) & 1)
}

func (br *jlsBitReader) readBits(n int) int {
	v := 0
	for i := 0; i < n; i++ {
		v = v<<1 | br.readBit()
	}
	return v
}

// ceilLog2 returns ceil(log2(n)) for n >= 1.
func ceilLog2(n int) int {
	k := 0
	for (1 << k) < n {
		k++
	}
	return k
}

func clampThreshold(i, j, maxval int) int {
	if i > maxval || i < j {
		return j
	}
	return i
}

// computeDefaultParams fills maxval/thresholds/reset when not preset (T.87 C.2.4.1.1.1).
func (f *jlsFrame) computeDefaultParams() {
	if f.maxval == 0 {
		f.maxval = (1 << f.precision) - 1
	}
	maxval := f.maxval
	near := f.near
	const basicT1, basicT2, basicT3 = 3, 7, 21
	if maxval >= 128 {
		factor := (min(maxval, 4095) + 128) / 256
		if f.t1 == 0 {
			f.t1 = clampThreshold(factor*(basicT1-2)+2+3*near, near+1, maxval)
		}
		if f.t2 == 0 {
			f.t2 = clampThreshold(factor*(basicT2-3)+3+5*near, f.t1, maxval)
		}
		if f.t3 == 0 {
			f.t3 = clampThreshold(factor*(basicT3-4)+4+7*near, f.t2, maxval)
		}
	} else {
		factor := 256 / (maxval + 1)
		if f.t1 == 0 {
			f.t1 = clampThreshold(max(2, basicT1/factor+3*near), near+1, maxval)
		}
		if f.t2 == 0 {
			f.t2 = clampThreshold(max(3, basicT2/factor+5*near), f.t1, maxval)
		}
		if f.t3 == 0 {
			f.t3 = clampThreshold(max(4, basicT3/factor+7*near), f.t2, maxval)
		}
	}
	if f.reset == 0 {
		f.reset = 64
	}
}

// jlsDecoder holds the per-scan decode state.
type jlsDecoder struct {
	f      *jlsFrame
	br     *jlsBitReader
	range_ int
	qbpp   int
	bpp    int
	limit  int

	a, b, n, c []int // context arrays, length 367
	nn         []int // negative-count for run-interruption contexts 365/366
	runIndex   int
}

func newJLSDecoder(f *jlsFrame, entropy []byte) *jlsDecoder {
	d := &jlsDecoder{f: f, br: &jlsBitReader{data: entropy}}
	d.range_ = (f.maxval+2*f.near)/(2*f.near+1) + 1
	d.qbpp = ceilLog2(d.range_)
	d.bpp = max(2, ceilLog2(f.maxval+1))
	d.limit = 2 * (d.bpp + max(8, d.bpp))
	aInit := max(2, (d.range_+32)/64)
	d.a = make([]int, 367)
	d.b = make([]int, 367)
	d.n = make([]int, 367)
	d.c = make([]int, 367)
	d.nn = make([]int, 367)
	for i := range d.a {
		d.a[i] = aInit
		d.n[i] = 1
	}
	return d
}

func (d *jlsDecoder) quantize(diff int) int {
	near, t1, t2, t3 := d.f.near, d.f.t1, d.f.t2, d.f.t3
	switch {
	case diff <= -t3:
		return -4
	case diff <= -t2:
		return -3
	case diff <= -t1:
		return -2
	case diff < -near:
		return -1
	case diff <= near:
		return 0
	case diff < t1:
		return 1
	case diff < t2:
		return 2
	case diff < t3:
		return 3
	default:
		return 4
	}
}

// decodeValue decodes a limited-length Golomb code with parameter k and the
// given limit (CharLS DecodeValue / T.87 A.5.3).
func (d *jlsDecoder) decodeValue(k, limit int) int {
	high := 0
	for d.br.readBit() == 0 {
		high++
		if high > limit+d.qbpp+64 {
			return 0 // guard against runaway on malformed input
		}
	}
	if high >= limit-(d.qbpp+1) {
		return d.br.readBits(d.qbpp) + 1
	}
	if k == 0 {
		return high
	}
	return high<<k + d.br.readBits(k)
}

// unmapErrval inverts the regular-mode error mapping (0,-1,1,-2,2,...).
func unmapErrval(merr int) int {
	if merr&1 != 0 {
		return -(merr + 1) / 2
	}
	return merr / 2
}

// computeRecon reconstructs a sample from a (sign-applied) error value, applying
// near-lossless dequantization and the modulo-range fix-up (CharLS
// ComputeReconstructedSample / FixReconstructedValue).
func (d *jlsDecoder) computeRecon(px, errval int) int {
	near := d.f.near
	v := px + errval*(2*near+1)
	if v < -near {
		v += d.range_ * (2*near + 1)
	} else if v > d.f.maxval+near {
		v -= d.range_ * (2*near + 1)
	}
	if v < 0 {
		v = 0
	} else if v > d.f.maxval {
		v = d.f.maxval
	}
	return v
}

// decodeRegular decodes one regular-mode sample given the context Q, the context
// sign, and the sign-corrected prediction px.
func (d *jlsDecoder) decodeRegular(q, sign, px int) int {
	k := 0
	for (d.n[q] << k) < d.a[q] {
		k++
	}
	errval := unmapErrval(d.decodeValue(k, d.limit))
	// k==0 bias flip (GetErrorCorrection: ErrVal ^= -1 when 2B+N-1 < 0).
	if k == 0 && 2*d.b[q]+d.n[q]-1 < 0 {
		errval = -errval - 1
	}
	rx := d.computeRecon(px, sign*errval)
	d.updateRegular(q, errval)
	return rx
}

// updateRegular updates the regular context A/B/N and the bias correction C.
func (d *jlsDecoder) updateRegular(q, errval int) {
	d.a[q] += abs(errval)
	d.b[q] += errval * (2*d.f.near + 1)
	if d.n[q] == d.f.reset {
		d.a[q] >>= 1
		if d.b[q] >= 0 {
			d.b[q] >>= 1
		} else {
			d.b[q] = -((1 - d.b[q]) >> 1)
		}
		d.n[q] >>= 1
	}
	d.n[q]++
	if d.b[q] <= -d.n[q] {
		d.c[q]--
		if d.c[q] < -128 {
			d.c[q] = -128
		}
		d.b[q] += d.n[q]
		if d.b[q] <= -d.n[q] {
			d.b[q] = -d.n[q] + 1
		}
	} else if d.b[q] > 0 {
		d.c[q]++
		if d.c[q] > 127 {
			d.c[q] = 127
		}
		d.b[q] -= d.n[q]
		if d.b[q] > 0 {
			d.b[q] = 0
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func predict(a, b, c int) int {
	if c >= max(a, b) {
		return min(a, b)
	}
	if c <= min(a, b) {
		return max(a, b)
	}
	return a + b - c
}
