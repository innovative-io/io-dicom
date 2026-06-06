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
	"math/bits"
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
//
// Bits are refilled into a 64-bit accumulator a byte at a time, with the stuffed
// bit removed at fill time, so the accumulator is a clean stuffing-free bit
// stream. This lets readBits extract n bits with a single shift and lets the
// unary decoder count zero runs with a hardware leading-zeros instruction rather
// than one method call per bit.
type jlsBitReader struct {
	data      []byte
	pos       int
	acc       uint64 // valid bits live in the low nbits; next bit is bit (nbits-1)
	nbits     int    // number of valid bits currently in acc (0..64)
	lastWasFF bool   // the previously loaded byte was 0xFF
}

// fill loads bytes into acc until it holds at least 56 bits or the data is
// exhausted. Stuffed bits (the MSB following a 0xFF byte) are dropped here.
func (br *jlsBitReader) fill() {
	for br.nbits <= 56 && br.pos < len(br.data) {
		b := br.data[br.pos]
		br.pos++
		if br.lastWasFF {
			br.acc = br.acc<<7 | uint64(b&0x7F)
			br.nbits += 7
		} else {
			br.acc = br.acc<<8 | uint64(b)
			br.nbits += 8
		}
		br.lastWasFF = b == 0xFF
	}
}

func (br *jlsBitReader) readBit() int {
	if br.nbits == 0 {
		br.fill()
		if br.nbits == 0 {
			return 0 // pad with zeros past EOF
		}
	}
	br.nbits--
	return int((br.acc >> br.nbits) & 1)
}

func (br *jlsBitReader) readBits(n int) int {
	if n == 0 {
		return 0
	}
	for br.nbits < n {
		before := br.nbits
		br.fill()
		if br.nbits == before {
			// EOF: take what remains, pad the rest with zero bits (MSB-first).
			v := int(br.acc&mask(br.nbits)) << (n - br.nbits)
			br.nbits = 0
			return v
		}
	}
	br.nbits -= n
	return int((br.acc >> br.nbits) & mask(n))
}

// readZeros counts and consumes a run of zero bits terminated by a 1 bit (which
// is also consumed), capping the count at cap to bound malformed input. It is
// the unary-prefix fast path for decodeValue and is exactly equivalent to
// calling readBit until it returns 1.
func (br *jlsBitReader) readZeros(cap int) int {
	count := 0
	for {
		if br.nbits == 0 {
			br.fill()
			if br.nbits == 0 {
				return cap // past EOF: treat as an over-long (capped) run
			}
		}
		// Left-align the nbits valid bits so the first unread bit is bit 63.
		w := br.acc << (64 - br.nbits)
		lz := bits.LeadingZeros64(w)
		if lz >= br.nbits {
			count += br.nbits
			br.nbits = 0
			if count >= cap {
				return cap
			}
			continue
		}
		count += lz
		br.nbits -= lz + 1 // consume the zeros and the terminating 1
		if count >= cap {
			return cap
		}
		return count
	}
}

// mask returns a bit mask with the low n bits set (n in 0..64).
func mask(n int) uint64 {
	if n >= 64 {
		return ^uint64(0)
	}
	return 1<<uint(n) - 1
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

	ctx      []ctxState // per-context model state, length 367
	runIndex int

	qLUT []int8 // quantize lookup: qLUT[diff+qOff] for diff in [-maxval, maxval]
	qOff int    // = maxval; bias to index qLUT by a signed diff
}

// ctxState is the per-context model state. Grouping a/b/c/n (and nn for the two
// run-interruption contexts) into one struct keeps a context's fields on a
// single cache line, since every regular-mode sample reads and writes all of
// them together.
type ctxState struct {
	a, b, c, n, nn int
}

func newJLSDecoder(f *jlsFrame, entropy []byte) *jlsDecoder {
	d := &jlsDecoder{f: f, br: &jlsBitReader{data: entropy}}
	d.range_ = (f.maxval+2*f.near)/(2*f.near+1) + 1
	d.qbpp = ceilLog2(d.range_)
	d.bpp = max(2, ceilLog2(f.maxval+1))
	d.limit = 2 * (d.bpp + max(8, d.bpp))
	aInit := max(2, (d.range_+32)/64)
	d.ctx = make([]ctxState, 367)
	for i := range d.ctx {
		d.ctx[i].a = aInit
		d.ctx[i].n = 1
	}
	d.buildQuantizeLUT()
	return d
}

// buildQuantizeLUT precomputes the gradient-quantization region for every
// possible neighbor difference (diff in [-maxval, maxval]), so the per-sample
// quantize() is a single array lookup instead of a threshold-ladder branch.
func (d *jlsDecoder) buildQuantizeLUT() {
	d.qOff = d.f.maxval
	d.qLUT = make([]int8, 2*d.f.maxval+1)
	for diff := -d.f.maxval; diff <= d.f.maxval; diff++ {
		d.qLUT[diff+d.qOff] = int8(quantizeDiff(diff, d.f.near, d.f.t1, d.f.t2, d.f.t3))
	}
}

// quantizeDiff is the scalar gradient quantizer (T.87 A.3.3); buildQuantizeLUT
// tabulates it.
func quantizeDiff(diff, near, t1, t2, t3 int) int {
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

func (d *jlsDecoder) quantize(diff int) int {
	return int(d.qLUT[diff+d.qOff])
}

// decodeValue decodes a limited-length Golomb code with parameter k and the
// given limit (CharLS DecodeValue / T.87 A.5.3).
func (d *jlsDecoder) decodeValue(k, limit int) int {
	high := d.br.readZeros(limit + d.qbpp + 64)
	if high >= limit+d.qbpp+64 {
		return 0 // guard against runaway on malformed input
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
	cs := &d.ctx[q]
	k := 0
	for (cs.n << k) < cs.a {
		k++
	}
	errval := unmapErrval(d.decodeValue(k, d.limit))
	// k==0 bias flip (GetErrorCorrection: ErrVal ^= -1 when 2B+N-1 < 0).
	// CharLS calls get_error_correction(near_lossless), which returns 0 whenever
	// its argument is nonzero — so the flip applies in pure-lossless mode only
	// (NEAR==0). In near-lossless mode the correction is never applied.
	if k == 0 && d.f.near == 0 && 2*cs.b+cs.n-1 < 0 {
		errval = -errval - 1
	}
	rx := d.computeRecon(px, sign*errval)
	updateRegular(cs, errval, d.f.near, d.f.reset)
	return rx
}

// updateRegular updates the regular context A/B/N and the bias correction C.
func updateRegular(cs *ctxState, errval, near, reset int) {
	cs.a += abs(errval)
	cs.b += errval * (2*near + 1)
	if cs.n == reset {
		cs.a >>= 1
		if cs.b >= 0 {
			cs.b >>= 1
		} else {
			cs.b = -((1 - cs.b) >> 1)
		}
		cs.n >>= 1
	}
	cs.n++
	if cs.b <= -cs.n {
		cs.c--
		if cs.c < -128 {
			cs.c = -128
		}
		cs.b += cs.n
		if cs.b <= -cs.n {
			cs.b = -cs.n + 1
		}
	} else if cs.b > 0 {
		cs.c++
		if cs.c > 127 {
			cs.c = 127
		}
		cs.b -= cs.n
		if cs.b > 0 {
			cs.b = 0
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
