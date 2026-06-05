package jpeg2000

// MQ arithmetic decoder (ITU-T T.800 Annex C / T.88), the entropy coder used by
// JPEG 2000 tier-1 (EBCOT) code-block decoding. This follows the standard's
// "software conventions" (Figures C.15–C.19): the code register C holds the
// active bits with Chigh = C>>16, A is the interval, CT counts available bits.

// qeEntry is one row of the probability estimation state table (T.800 Table C.2).
type qeEntry struct {
	qe         uint32
	nmps, nlps uint8
	sw         uint8 // SWITCH
}

var mqQe = [47]qeEntry{
	{0x5601, 1, 1, 1}, {0x3401, 2, 6, 0}, {0x1801, 3, 9, 0}, {0x0AC1, 4, 12, 0},
	{0x0521, 5, 29, 0}, {0x0221, 38, 33, 0}, {0x5601, 7, 6, 1}, {0x5401, 8, 14, 0},
	{0x4801, 9, 14, 0}, {0x3801, 10, 14, 0}, {0x3001, 11, 17, 0}, {0x2401, 12, 18, 0},
	{0x1C01, 13, 20, 0}, {0x1601, 29, 21, 0}, {0x5601, 15, 14, 1}, {0x5401, 16, 14, 0},
	{0x5101, 17, 15, 0}, {0x4801, 18, 16, 0}, {0x3801, 19, 17, 0}, {0x3401, 20, 18, 0},
	{0x3001, 21, 19, 0}, {0x2801, 22, 19, 0}, {0x2401, 23, 20, 0}, {0x2201, 24, 21, 0},
	{0x1C01, 25, 22, 0}, {0x1801, 26, 23, 0}, {0x1601, 27, 24, 0}, {0x1401, 28, 25, 0},
	{0x1201, 29, 26, 0}, {0x1101, 30, 27, 0}, {0x0AC1, 31, 28, 0}, {0x09C1, 32, 29, 0},
	{0x08A1, 33, 30, 0}, {0x0521, 34, 31, 0}, {0x0441, 35, 32, 0}, {0x02A1, 36, 33, 0},
	{0x0221, 37, 34, 0}, {0x0141, 38, 35, 0}, {0x0111, 39, 36, 0}, {0x0085, 40, 37, 0},
	{0x0049, 41, 38, 0}, {0x0025, 42, 39, 0}, {0x0015, 43, 40, 0}, {0x0009, 44, 41, 0},
	{0x0005, 45, 42, 0}, {0x0001, 45, 43, 0}, {0x5601, 46, 46, 0},
}

// mqContext holds per-context state: the Qe-table index and the MPS symbol.
type mqContext struct {
	index uint8
	mps   uint8
}

type mqDecoder struct {
	data []byte
	bp   int // index of the current byte B
	c    uint32
	a    uint32
	ct   int32
	end  int // exclusive end of code data
}

// b returns the current byte B, padding past the end of the segment with 0xFF
// (T.800 C.3.4 — the decoder behaves as if 0xFF bytes follow the data).
func (d *mqDecoder) b() uint32 {
	if d.bp >= d.end {
		return 0xFF
	}
	return uint32(d.data[d.bp])
}

func (d *mqDecoder) b1() uint32 {
	if d.bp+1 >= d.end {
		return 0xFF
	}
	return uint32(d.data[d.bp+1])
}

// newMQDecoder initializes the decoder over data[start:end] (INITDEC, C.3.5).
func newMQDecoder(data []byte, start, end int) *mqDecoder {
	d := &mqDecoder{data: data, bp: start, end: end}
	d.c = d.b() << 16
	d.bytein()
	d.c <<= 7
	d.ct -= 7
	d.a = 0x8000
	return d
}

// bytein implements BYTEIN (T.800 C.3.4).
func (d *mqDecoder) bytein() {
	if d.b() == 0xFF {
		if d.b1() > 0x8F {
			d.c += 0xFF00
			d.ct = 8
		} else {
			d.bp++
			d.c += d.b() << 9
			d.ct = 7
		}
	} else {
		d.bp++
		d.c += d.b() << 8
		d.ct = 8
	}
}

func (d *mqDecoder) renormd() {
	for {
		if d.ct == 0 {
			d.bytein()
		}
		d.a <<= 1
		d.c <<= 1
		d.ct--
		if d.a&0x8000 != 0 {
			break
		}
	}
}

// decode decodes one binary decision for the given context (DECODE, C.3.2).
func (d *mqDecoder) decode(cx *mqContext) int {
	qe := mqQe[cx.index].qe
	d.a -= qe
	var decision int
	if (d.c >> 16) < qe {
		// LPS exchange
		if d.a < qe {
			d.a = qe
			decision = int(cx.mps)
			cx.index = mqQe[cx.index].nmps
		} else {
			d.a = qe
			decision = int(1 - cx.mps)
			if mqQe[cx.index].sw == 1 {
				cx.mps = 1 - cx.mps
			}
			cx.index = mqQe[cx.index].nlps
		}
		d.renormd()
	} else {
		d.c -= qe << 16
		if d.a&0x8000 == 0 {
			// MPS exchange
			if d.a < qe {
				decision = int(1 - cx.mps)
				if mqQe[cx.index].sw == 1 {
					cx.mps = 1 - cx.mps
				}
				cx.index = mqQe[cx.index].nlps
			} else {
				decision = int(cx.mps)
				cx.index = mqQe[cx.index].nmps
			}
			d.renormd()
		} else {
			decision = int(cx.mps)
		}
	}
	return decision
}
