package gojxl

import (
	"math"
	"testing"
)

// TestEncodeVarDCTRoundTrip checks the pure-Go lossy VarDCT encoder: the output
// decodes with this package's (djxl-validated) decoder, the quality scales with
// globalScale, and a flat image is reconstructed exactly.
func TestEncodeVarDCTRoundTrip(t *testing.T) {
	w, h := 96, 80
	raw := make([]byte, w*h*3)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			raw[i] = byte(x * 255 / w)
			raw[i+1] = byte(y * 255 / h)
			raw[i+2] = byte((x + y) * 255 / (w + h))
		}
	}
	psnr := func(enc []byte) float64 {
		img, err := DecodeVarDCT(enc)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if img.W != w || img.H != h || img.Channels != 3 {
			t.Fatalf("geom %dx%d ch=%d", img.W, img.H, img.Channels)
		}
		var se float64
		for i := 0; i < w*h*3; i++ {
			d := float64(int(img.Pixels[i]) - int(raw[i]))
			se += d * d
		}
		return 20 * math.Log10(255/math.Sqrt(se/float64(w*h*3)))
	}
	lo, _ := EncodeVarDCT(raw, w, h, 3, 2048)
	hi, _ := EncodeVarDCT(raw, w, h, 3, 32768)
	plo, phi := psnr(lo), psnr(hi)
	if phi <= plo {
		t.Errorf("quality should improve with globalScale: %0.1f -> %0.1f dB", plo, phi)
	}
	if phi < 30 {
		t.Errorf("high-quality PSNR too low: %.1f dB", phi)
	}

	// Flat image must reconstruct exactly.
	flat := make([]byte, w*h*3)
	for i := range flat {
		flat[i] = 137
	}
	fenc, err := EncodeVarDCT(flat, w, h, 3, 4096)
	if err != nil {
		t.Fatal(err)
	}
	fimg, err := DecodeVarDCT(fenc)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range fimg.Pixels {
		if v != 137 {
			t.Fatalf("flat not exact at %d: %d", i, v)
		}
	}
}

// TestEncodeVarDCTMultiGroup checks the encoder splits images larger than one
// group (256px) into the decoder's section layout and round-trips.
func TestEncodeVarDCTMultiGroup(t *testing.T) {
	for _, sz := range []struct{ w, h int }{{300, 200}, {512, 300}, {200, 600}} {
		w, h := sz.w, sz.h
		raw := make([]byte, w*h*3)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := (y*w + x) * 3
				raw[i] = byte(x * 255 / w)
				raw[i+1] = byte(y * 255 / h)
				raw[i+2] = 128
			}
		}
		enc, err := EncodeVarDCT(raw, w, h, 3, 16384)
		if err != nil {
			t.Fatalf("%dx%d encode: %v", w, h, err)
		}
		img, err := DecodeVarDCT(enc)
		if err != nil {
			t.Fatalf("%dx%d decode: %v", w, h, err)
		}
		if img.W != w || img.H != h || img.Channels != 3 {
			t.Fatalf("%dx%d geom %dx%d ch=%d", w, h, img.W, img.H, img.Channels)
		}
		// Recovered gradient: red ramps left-to-right.
		get := func(x, y, c int) int { return int(img.Pixels[(y*w+x)*3+c]) }
		if get(w-3, h/2, 0)-get(3, h/2, 0) < 180 {
			t.Errorf("%dx%d red gradient not recovered", w, h)
		}
	}
}
