package gojxl

import (
	"math"
	"sync"
)

// Separable orthonormal DCT used by VarDCT inverse transforms. libjxl's
// dct-inl.h implements the same cosine transform via a fast recursive butterfly
// with its normalization folded into the dequantization scales (DCTTotalScale).
// Here the kernel is the plain orthonormal DCT-II (forward) / DCT-III (inverse)
// pair, which is a true round-trip inverse; the VarDCT integration applies
// libjxl's per-size scale factor at the dequant boundary so the two agree.
//
// Cosine bases are precomputed per block dimension (8, 16, 32 are the square
// VarDCT sizes; larger/rect sizes reuse the same 1D kernel).

type dctTable struct {
	n     int
	basis []float32 // [k*n + x] = alpha(k) * cos(pi*(2x+1)*k/(2n))
}

// dctTableCache is precomputed once at package initialization for the 1D sizes
// VarDCT uses (covered-block axis lengths 1,2,4,8 and pixel axis lengths
// 8,16,32,64). It is never mutated afterwards, so concurrent reads (e.g.
// multi-frame transcoding) are race-free without locking.
var dctTableCache = func() map[int]*dctTable {
	m := make(map[int]*dctTable)
	for _, n := range []int{1, 2, 4, 8, 16, 32, 64, 128, 256} {
		m[n] = computeDCTTable(n)
	}
	return m
}()

func computeDCTTable(n int) *dctTable {
	t := &dctTable{n: n, basis: make([]float32, n*n)}
	for k := 0; k < n; k++ {
		alpha := math.Sqrt(2.0 / float64(n))
		if k == 0 {
			alpha = math.Sqrt(1.0 / float64(n))
		}
		for x := 0; x < n; x++ {
			t.basis[k*n+x] = float32(alpha * math.Cos(math.Pi*(2*float64(x)+1)*float64(k)/(2*float64(n))))
		}
	}
	return t
}

func getDCTTable(n int) *dctTable {
	if t, ok := dctTableCache[n]; ok {
		return t
	}
	// Unexpected size (the reconstruction guard restricts transforms to the
	// precomputed set): compute a fresh table without mutating the shared cache,
	// keeping reads lock-free and race-free.
	return computeDCTTable(n)
}

// idct1d computes the inverse (DCT-III): out[x] = sum_k coeff[k]*basis[k][x].
func idct1d(t *dctTable, coeff, out []float32) {
	n := t.n
	for x := 0; x < n; x++ {
		var s float32
		for k := 0; k < n; k++ {
			s += coeff[k] * t.basis[k*n+x]
		}
		out[x] = s
	}
}

// dct1d computes the forward (DCT-II): out[k] = sum_x in[x]*basis[k][x].
func dct1d(t *dctTable, in, out []float32) {
	n := t.n
	for k := 0; k < n; k++ {
		var s float32
		row := t.basis[k*n : k*n+n]
		for x := 0; x < n; x++ {
			s += in[x] * row[x]
		}
		out[k] = s
	}
}

// idct2d transforms an h*w block of DCT coefficients (row-major) to pixels,
// applying the 1D inverse along columns then rows. in and out may be the same
// slice. Used for the square DCT8x8 / DCT16x16 / DCT32x32 VarDCT blocks (and the
// rectangular DCTs via differing tw/th).
// dctScratch holds the working buffers idct2d needs. They were allocated fresh
// on every call, which made idct2d the single largest allocation site in JPEG XL
// decoding — 196653 objects and 48 MB on one 512x512 lossy decode, and a fifth
// of decode CPU spent in runtime.madvise returning the churn to the OS.
//
// A sync.Pool rather than shared package state: frames are decoded concurrently
// (runFrameJobs), so a single shared buffer would be a data race.
//
// Recycled buffers are not zeroed because every element is written before it is
// read: idct1d assigns out[x] for all x, the column loop fills every tmp entry,
// and rowIn is fully overwritten by copy.
type dctScratch struct {
	col, tmpc, tmp, rowIn, rowOut []float32
}

var dctScratchPool = sync.Pool{New: func() any { return new(dctScratch) }}

func growF32(b []float32, n int) []float32 {
	if cap(b) < n {
		return make([]float32, n)
	}
	return b[:n]
}

func idct2d(coeff []float32, w, h int) []float32 {
	out := make([]float32, w*h) // returned to the caller, so always fresh
	tcol := getDCTTable(h)
	trow := getDCTTable(w)

	s := dctScratchPool.Get().(*dctScratch)
	defer dctScratchPool.Put(s)
	s.col = growF32(s.col, h)
	s.tmpc = growF32(s.tmpc, h)
	s.tmp = growF32(s.tmp, w*h)
	s.rowIn = growF32(s.rowIn, w)
	s.rowOut = growF32(s.rowOut, w)
	col, tmpc, tmp, rowIn, rowOut := s.col, s.tmpc, s.tmp, s.rowIn, s.rowOut

	// Columns: inverse-transform each column of length h.
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			col[y] = coeff[y*w+x]
		}
		idct1d(tcol, col, tmpc)
		for y := 0; y < h; y++ {
			tmp[y*w+x] = tmpc[y]
		}
	}
	// Rows: inverse-transform each row of length w.
	for y := 0; y < h; y++ {
		copy(rowIn, tmp[y*w:y*w+w])
		idct1d(trow, rowIn, rowOut)
		copy(out[y*w:y*w+w], rowOut)
	}
	return out
}

// dct2d is the forward of idct2d (pixels -> coefficients), used for testing.
func dct2d(pix []float32, w, h int) []float32 {
	out := make([]float32, w*h)
	tcol := getDCTTable(h)
	trow := getDCTTable(w)
	tmp := make([]float32, w*h)

	rowIn := make([]float32, w)
	rowOut := make([]float32, w)
	for y := 0; y < h; y++ {
		copy(rowIn, pix[y*w:y*w+w])
		dct1d(trow, rowIn, rowOut)
		copy(tmp[y*w:y*w+w], rowOut)
	}
	col := make([]float32, h)
	tmpc := make([]float32, h)
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			col[y] = tmp[y*w+x]
		}
		dct1d(tcol, col, tmpc)
		for y := 0; y < h; y++ {
			out[y*w+x] = tmpc[y]
		}
	}
	return out
}
