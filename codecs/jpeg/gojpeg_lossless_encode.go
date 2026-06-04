package jpeg

// Pure-Go encoder for JPEG Lossless (ITU-T T.81 process 14, SOF3), the inverse
// of the decoder in gojpeg_lossless.go. It lets CGO_ENABLED=0 builds produce
// valid lossless JPEG (e.g. for transcoding) instead of the previous
// passthrough behaviour. Uses predictor 1 (Ra) with per-image optimal Huffman
// coding; any conforming decoder (incl. libjpeg) reconstructs the input exactly.

import "errors"

var errGoJPEGEncodeInput = errors.New("gojpeg: invalid input for lossless JPEG encode")

// jpegBitWriter writes MSB-first bits and applies JPEG byte stuffing (a 0x00 is
// inserted after every 0xFF emitted to the entropy stream).
type jpegBitWriter struct {
	out   []byte
	acc   uint32
	nbits uint
}

func (w *jpegBitWriter) writeBits(code uint32, n uint) {
	w.acc |= code << (32 - w.nbits - n)
	w.nbits += n
	for w.nbits >= 8 {
		b := byte(w.acc >> 24)
		w.out = append(w.out, b)
		if b == 0xFF {
			w.out = append(w.out, 0x00) // byte stuffing
		}
		w.acc <<= 8
		w.nbits -= 8
	}
}

func (w *jpegBitWriter) flush() {
	if w.nbits > 0 {
		// pad the final partial byte with exactly (8-nbits) 1 bits (JPEG
		// convention). The pad value must be masked to that width or it would
		// overlap and corrupt the real trailing bits already in the accumulator.
		n := 8 - w.nbits
		w.writeBits((1<<n)-1, n)
	}
}

// buildEncodeHuffTable builds canonical Huffman code lengths/codes for symbols
// 0..16 from their frequencies, limited to 16-bit codes (JPEG Annex K.2).
func buildEncodeHuffTable(freq [17]int) (sizes [17]uint, codes [17]uint32, bits [17]int, vals []byte) {
	// freq2 includes a reserved sentinel symbol (index 17) with frequency 1 so
	// no real symbol ever gets the all-ones code.
	const nsym = 18
	var f [nsym + 1]int
	for i := 0; i < 17; i++ {
		f[i] = freq[i]
	}
	f[17] = 1 // reserved
	codesize := make([]int, nsym+1)
	others := make([]int, nsym+1)
	for i := range others {
		others[i] = -1
	}
	for {
		// find two smallest non-zero frequencies
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
	// limit to 16 bits (Annex K.2 figure K.3)
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
	// remove the reserved sentinel (longest code)
	for i := 32; i > 0; i-- {
		if bitsCount[i] > 0 {
			bitsCount[i]--
			break
		}
	}
	for i := 1; i <= 16; i++ {
		bits[i] = bitsCount[i] // bits[0] unused; index = code length
	}
	// HUFFVAL: symbols sorted by code length then value
	for l := 1; l <= 32; l++ {
		for sym := 0; sym < 17; sym++ {
			if codesize[sym] == l {
				vals = append(vals, byte(sym))
			}
		}
	}
	// assign canonical codes
	var huffsize []uint
	for l := 1; l <= 16; l++ {
		for k := 0; k < bits[l]; k++ {
			huffsize = append(huffsize, uint(l))
		}
	}
	var code uint32
	var si uint
	if len(huffsize) > 0 {
		si = huffsize[0]
	}
	ci := 0
	codeList := make([]uint32, len(huffsize))
	sizeList := make([]uint, len(huffsize))
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

// ssssCategory returns the magnitude category of a lossless difference.
func ssssCategory(diff int) int {
	a := diff
	if a < 0 {
		a = -a
	}
	s := 0
	for a > 0 {
		s++
		a >>= 1
	}
	return s
}

func appendMarker(out []byte, marker byte, payload []byte) []byte {
	out = append(out, 0xFF, marker)
	n := len(payload) + 2
	out = append(out, byte(n>>8), byte(n))
	return append(out, payload...)
}

// encodeLosslessJPEG encodes raw samples (little-endian if bytesPerSample==2)
// as a lossless SOF3 JPEG using predictor 1.
func encodeLosslessJPEG(raw []byte, width, height, samples, precision int) ([]byte, error) {
	if width <= 0 || height <= 0 || (samples != 1 && samples != 3) {
		return nil, errGoJPEGEncodeInput
	}
	if precision < 2 || precision > 16 {
		return nil, errGoJPEGEncodeInput
	}
	bps := 1
	if precision > 8 {
		bps = 2
	}
	if width*height*samples*bps != len(raw) {
		return nil, errGoJPEGEncodeInput
	}

	// Load samples into per-component planes.
	getSample := func(i int) int {
		if bps == 1 {
			return int(raw[i])
		}
		return int(raw[i*2]) | int(raw[i*2+1])<<8
	}
	planes := make([][]int, samples)
	for c := 0; c < samples; c++ {
		planes[c] = make([]int, width*height)
	}
	for px := 0; px < width*height; px++ {
		for c := 0; c < samples; c++ {
			planes[c][px] = getSample(px*samples + c)
		}
	}

	defaultPx := 1 << (precision - 1)
	// Differences are reduced modulo 2^16 into [-32768, 32767] (T.81 lossless),
	// not modulo 2^precision — for precision < 16 this is a no-op since the true
	// difference already fits, and for 16-bit it produces SSSS=16 only for the
	// single value -32768 (encoded with no extra bits).
	const diffHalf = 1 << 15
	const diffMask = (1 << 16) - 1

	// Compute diffs (predictor 1) and frequencies.
	diffs := make([][]int, samples)
	var freq [17]int
	for c := 0; c < samples; c++ {
		diffs[c] = make([]int, width*height)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				cur := planes[c][y*width+x]
				var pred int
				switch {
				case x == 0 && y == 0:
					pred = defaultPx
				case y == 0:
					pred = planes[c][y*width+x-1] // Ra
				case x == 0:
					pred = planes[c][(y-1)*width+x] // Rb
				default:
					pred = planes[c][y*width+x-1] // predictor 1 = Ra
				}
				d := cur - pred
				d = ((d + diffHalf) & diffMask) - diffHalf // reduce mod 2^16
				diffs[c][y*width+x] = d
				freq[ssssCategory(d)]++
			}
		}
	}

	sizes, codes, bits, vals := buildEncodeHuffTable(freq)

	out := []byte{0xFF, mSOI}

	// SOF3
	sof := []byte{byte(precision), byte(height >> 8), byte(height), byte(width >> 8), byte(width), byte(samples)}
	for c := 0; c < samples; c++ {
		sof = append(sof, byte(c+1), 0x11, 0x00)
	}
	out = appendMarker(out, mSOF3, sof)

	// DHT (class 0, table 0)
	dht := []byte{0x00}
	for l := 1; l <= 16; l++ {
		dht = append(dht, byte(bits[l]))
	}
	dht = append(dht, vals...)
	out = appendMarker(out, mDHT, dht)

	// SOS: Ns, per-comp (Cs, Td<<4|Ta), Ss=1 (predictor), Se=0, Ah/Al=0
	sos := []byte{byte(samples)}
	for c := 0; c < samples; c++ {
		sos = append(sos, byte(c+1), 0x00)
	}
	sos = append(sos, 0x01, 0x00, 0x00)
	out = appendMarker(out, mSOS, sos)

	// Entropy-coded data.
	w := &jpegBitWriter{out: out}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			for c := 0; c < samples; c++ {
				d := diffs[c][y*width+x]
				s := ssssCategory(d)
				w.writeBits(codes[s], sizes[s])
				if s > 0 && s < 16 {
					v := d
					if v < 0 {
						v += (1 << s) - 1
					}
					w.writeBits(uint32(v&((1<<s)-1)), uint(s))
				}
			}
		}
	}
	w.flush()
	out = w.out
	out = append(out, 0xFF, mEOI)
	return out, nil
}
