package gojxl

import "math"

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
	for _, n := range []int{1, 2, 4, 8, 16, 32, 64} {
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
func idct2d(coeff []float32, w, h int) []float32 {
	out := make([]float32, w*h)
	tcol := getDCTTable(h)
	trow := getDCTTable(w)
	col := make([]float32, h)
	tmpc := make([]float32, h)
	tmp := make([]float32, w*h)

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
	rowIn := make([]float32, w)
	rowOut := make([]float32, w)
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
