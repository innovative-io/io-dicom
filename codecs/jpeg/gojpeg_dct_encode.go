package jpeg

// Pure-Go encoder for DCT-based JPEG — the inverse of the decoder in
// gojpeg_dct.go — producing JPEG Extended Sequential (SOF1) streams for the
// DICOM "JPEG Extended (Process 2 & 4) 12-bit" transfer syntax. Single-component
// grayscale, 8 or 12 bit. DCT is lossy, so the output decodes back within a
// small tolerance (controlled by the quantization table).
//
// Reuses zigzag and idctCos from gojpeg_dct.go and the bit writer / Huffman
// helpers from gojpeg_lossless_encode.go.

import "math"

// defaultDCTQuality is the JPEG quality factor used when encoding JPEG Extended
// 12-bit. High quality keeps the lossy DCT error small, which suits medical use.
const defaultDCTQuality = 90

// stdLuminanceQuant is the JPEG Annex K.1 luminance quantization table in
// natural (row-major) order.
var stdLuminanceQuant = [64]int{
	16, 11, 10, 16, 24, 40, 51, 61,
	12, 12, 14, 19, 26, 58, 60, 55,
	14, 13, 16, 24, 40, 57, 69, 56,
	14, 17, 22, 29, 51, 87, 80, 62,
	18, 22, 37, 56, 68, 109, 103, 77,
	24, 35, 55, 64, 81, 104, 113, 92,
	49, 64, 78, 87, 103, 121, 120, 101,
	72, 92, 95, 98, 112, 100, 103, 99,
}

// scaleQuant scales the base table by a JPEG quality factor (1..100); higher
// quality yields smaller divisors and less loss. Values are clamped to [1,255].
func scaleQuant(quality int) [64]int {
	if quality < 1 {
		quality = 1
	} else if quality > 100 {
		quality = 100
	}
	var scale int
	if quality < 50 {
		scale = 5000 / quality
	} else {
		scale = 200 - quality*2
	}
	var q [64]int
	for i, v := range stdLuminanceQuant {
		t := (v*scale + 50) / 100
		if t < 1 {
			t = 1
		} else if t > 255 {
			t = 255
		}
		q[i] = t
	}
	return q
}

// fdct8x8 performs a separable forward DCT (the transpose of idct8x8), writing
// coefficients back into block in natural order.
func fdct8x8(block *[64]float64) {
	var tmp [64]float64
	// rows: F(y,u) = 0.5 * sum_x idctCos[u][x] * f(y,x)
	for y := 0; y < 8; y++ {
		for u := 0; u < 8; u++ {
			s := 0.0
			for x := 0; x < 8; x++ {
				s += idctCos[u][x] * block[y*8+x]
			}
			tmp[y*8+u] = s * 0.5
		}
	}
	// columns
	for u := 0; u < 8; u++ {
		for v := 0; v < 8; v++ {
			s := 0.0
			for y := 0; y < 8; y++ {
				s += idctCos[v][y] * tmp[y*8+u]
			}
			block[v*8+u] = s * 0.5
		}
	}
}

// buildHuffN builds canonical Huffman codes for nsym symbols (0..nsym-1) from
// their frequencies, limited to 16-bit codes (JPEG Annex K.2). Returns per-symbol
// code sizes and codes, the BITS counts (index = code length), and HUFFVAL.
func buildHuffN(freq []int) (sizes []uint, codes []uint32, bits [17]int, vals []byte) {
	nsym := len(freq)
	// freq2 includes a reserved sentinel so no real symbol gets the all-ones code.
	f := make([]int, nsym+1)
	copy(f, freq)
	f[nsym] = 1
	codesize := make([]int, nsym+1)
	others := make([]int, nsym+1)
	for i := range others {
		others[i] = -1
	}
	for {
		c1, c2 := -1, -1
		v := int(^uint(0) >> 1)
		for i := 0; i <= nsym; i++ {
			if f[i] != 0 && f[i] <= v {
				v = f[i]
				c1 = i
			}
		}
		v = int(^uint(0) >> 1)
		for i := 0; i <= nsym; i++ {
			if f[i] != 0 && f[i] <= v && i != c1 {
				v = f[i]
				c2 = i
			}
		}
		if c2 < 0 {
			break
		}
		f[c1] += f[c2]
		f[c2] = 0
		for {
			codesize[c1]++
			if others[c1] < 0 {
				break
			}
			c1 = others[c1]
		}
		others[c1] = c2
		for {
			codesize[c2]++
			if others[c2] < 0 {
				break
			}
			c2 = others[c2]
		}
	}
	var bitsCount [33]int
	for i := 0; i <= nsym; i++ {
		if codesize[i] > 0 {
			bitsCount[codesize[i]]++
		}
	}
	for i := 32; i > 16; i-- {
		for bitsCount[i] > 0 {
			j := i - 2
			for bitsCount[j] == 0 {
				j--
			}
			bitsCount[i] -= 2
			bitsCount[i-1]++
			bitsCount[j+1] += 2
			bitsCount[j]--
		}
	}
	for i := 32; i > 0; i-- {
		if bitsCount[i] > 0 {
			bitsCount[i]-- // drop the reserved sentinel (longest code)
			break
		}
	}
	for i := 1; i <= 16; i++ {
		bits[i] = bitsCount[i]
	}
	for l := 1; l <= 32; l++ {
		for sym := 0; sym < nsym; sym++ {
			if codesize[sym] == l {
				vals = append(vals, byte(sym))
			}
		}
	}
	var huffsize []uint
	for l := 1; l <= 16; l++ {
		for k := 0; k < bits[l]; k++ {
			huffsize = append(huffsize, uint(l))
		}
	}
	sizes = make([]uint, nsym)
	codes = make([]uint32, nsym)
	codeList := make([]uint32, len(huffsize))
	sizeList := make([]uint, len(huffsize))
	var code uint32
	var si uint
	if len(huffsize) > 0 {
		si = huffsize[0]
	}
	ci := 0
	for ci < len(huffsize) {
		for ci < len(huffsize) && huffsize[ci] == si {
			codeList[ci] = code
			sizeList[ci] = si
			code++
			ci++
		}
		code <<= 1
		si++
	}
	for i, sym := range vals {
		sizes[sym] = sizeList[i]
		codes[sym] = codeList[i]
	}
	return sizes, codes, bits, vals
}

// encodeDCTJPEG encodes a single-component grayscale image as JPEG Extended
// Sequential (SOF1). quality is a JPEG quality factor (1..100).
func encodeDCTJPEG(raw []byte, width, height, precision, quality int) ([]byte, error) {
	if width <= 0 || height <= 0 || (precision != 8 && precision != 12) {
		return nil, errGoJPEGEncodeInput
	}
	bps := 1
	if precision > 8 {
		bps = 2
	}
	if width*height*bps != len(raw) {
		return nil, errGoJPEGEncodeInput
	}
	getSample := func(i int) int {
		if bps == 1 {
			return int(raw[i])
		}
		return int(raw[i*2]) | int(raw[i*2+1])<<8
	}

	quant := scaleQuant(quality)
	levelShift := 1 << (precision - 1)
	blocksX := (width + 7) / 8
	blocksY := (height + 7) / 8

	// First pass: forward DCT + quantize every block, gather Huffman frequencies.
	type block struct{ coef [64]int }
	blocks := make([]block, blocksX*blocksY)
	dcFreq := make([]int, 17)
	acFreq := make([]int, 256)
	prevDC := 0
	var fb [64]float64
	for by := 0; by < blocksY; by++ {
		for bx := 0; bx < blocksX; bx++ {
			for yy := 0; yy < 8; yy++ {
				sy := by*8 + yy
				if sy >= height {
					sy = height - 1
				}
				for xx := 0; xx < 8; xx++ {
					sx := bx*8 + xx
					if sx >= width {
						sx = width - 1
					}
					fb[yy*8+xx] = float64(getSample(sy*width+sx) - levelShift)
				}
			}
			fdct8x8(&fb)
			b := &blocks[by*blocksX+bx]
			for k := 0; k < 64; k++ {
				nat := zigzag[k]
				b.coef[k] = int(math.Round(fb[nat] / float64(quant[nat])))
			}
			// frequencies: DC difference category
			diff := b.coef[0] - prevDC
			prevDC = b.coef[0]
			dcFreq[ssssCategory(diff)]++
			// AC run/size
			run := 0
			for k := 1; k < 64; k++ {
				if b.coef[k] == 0 {
					run++
					continue
				}
				for run > 15 {
					acFreq[0xF0]++ // ZRL
					run -= 16
				}
				acFreq[(run<<4)|ssssCategory(b.coef[k])]++
				run = 0
			}
			if run > 0 {
				acFreq[0x00]++ // EOB
			}
		}
	}

	dcSizes, dcCodes, dcBits, dcVals := buildHuffN(dcFreq)
	acSizes, acCodes, acBits, acVals := buildHuffN(acFreq)

	out := []byte{0xFF, mSOI}
	// DQT (8-bit precision table, id 0), in zigzag order.
	dqt := []byte{0x00}
	for k := 0; k < 64; k++ {
		dqt = append(dqt, byte(quant[zigzag[k]]))
	}
	out = appendMarker(out, mDQT, dqt)
	// SOF1: precision, height, width, 1 component (id 1, 1x1 sampling, quant 0).
	sof := []byte{byte(precision), byte(height >> 8), byte(height), byte(width >> 8), byte(width), 0x01, 0x01, 0x11, 0x00}
	out = appendMarker(out, mSOF1, sof)
	// DHT: DC table (class 0, id 0) and AC table (class 1, id 0).
	dht := []byte{0x00}
	for l := 1; l <= 16; l++ {
		dht = append(dht, byte(dcBits[l]))
	}
	dht = append(dht, dcVals...)
	out = appendMarker(out, mDHT, dht)
	dht = []byte{0x10}
	for l := 1; l <= 16; l++ {
		dht = append(dht, byte(acBits[l]))
	}
	dht = append(dht, acVals...)
	out = appendMarker(out, mDHT, dht)
	// SOS: 1 component (id 1, DC table 0 / AC table 0), Ss=0, Se=63, Ah/Al=0.
	out = appendMarker(out, mSOS, []byte{0x01, 0x01, 0x00, 0x00, 0x3F, 0x00})

	// Second pass: entropy-code.
	w := &jpegBitWriter{out: out}
	prevDC = 0
	for i := range blocks {
		b := &blocks[i]
		diff := b.coef[0] - prevDC
		prevDC = b.coef[0]
		s := ssssCategory(diff)
		w.writeBits(dcCodes[s], dcSizes[s])
		if s > 0 {
			writeAmplitude(w, diff, s)
		}
		run := 0
		for k := 1; k < 64; k++ {
			if b.coef[k] == 0 {
				run++
				continue
			}
			for run > 15 {
				w.writeBits(acCodes[0xF0], acSizes[0xF0]) // ZRL
				run -= 16
			}
			s := ssssCategory(b.coef[k])
			rs := (run << 4) | s
			w.writeBits(acCodes[rs], acSizes[rs])
			writeAmplitude(w, b.coef[k], s)
			run = 0
		}
		if run > 0 {
			w.writeBits(acCodes[0x00], acSizes[0x00]) // EOB
		}
	}
	w.flush()
	out = w.out
	out = append(out, 0xFF, mEOI)
	return out, nil
}

// writeAmplitude writes the s low bits of a coefficient using the JPEG
// magnitude-category encoding (negative values are offset by 2^s - 1).
func writeAmplitude(w *jpegBitWriter, v, s int) {
	if v < 0 {
		v += (1 << s) - 1
	}
	w.writeBits(uint32(v&((1<<s)-1)), uint(s))
}
