package gojxl

// Gaborish loop filter (render_pipeline/stage_gaborish.cc): a normalized 3x3
// convolution applied to each XYB plane after the inverse DCT, smoothing block
// boundaries. The kernel is w0 at the center, w1 at the 4 edge neighbors and w2
// at the 4 corners, normalized by w0 + 4(w1+w2). Borders replicate the edge
// pixel (Mirror with radius 1).
func applyGaborish(plane []float32, w, h int, w1, w2 float32) []float32 {
	div := 1.0 + 4*(w1+w2)
	n0 := 1.0 / div
	n1 := w1 / div
	n2 := w2 / div

	clamp := func(v, hi int) int {
		if v < 0 {
			return 0
		}
		if v >= hi {
			return hi - 1
		}
		return v
	}
	at := func(x, y int) float32 {
		return plane[clamp(y, h)*w+clamp(x, w)]
	}

	out := make([]float32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m := plane[y*w+x]
			side := at(x, y-1) + at(x, y+1) + at(x-1, y) + at(x+1, y)
			diag := at(x-1, y-1) + at(x+1, y-1) + at(x-1, y+1) + at(x+1, y+1)
			out[y*w+x] = n0*m + n1*side + n2*diag
		}
	}
	return out
}
