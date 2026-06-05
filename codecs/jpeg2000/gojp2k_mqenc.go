package jpeg2000

// MQ arithmetic encoder (ITU-T T.800 Annex C), the inverse of mqDecoder. It
// follows openjpeg's software conventions (Figures C.4–C.11) so the produced
// segment decodes with the matching mqDecoder and any conformant decoder. The
// probability-estimation state table (mqQe) and context model (mqContext) are
// shared with the decoder.

type mqEncoder struct {
	buf []byte // buf[0] is the "start-1" sentinel byte (0); output begins at [1]
	bp  int    // index of the current byte in buf
	a   uint32
	c   uint32
	ct  int32
}

func newMQEncoder() *mqEncoder {
	return &mqEncoder{buf: []byte{0x00}, bp: 0, a: 0x8000, ct: 12}
}

// byteout removes a byte of compressed data from C (T.800 C.2.10).
func (e *mqEncoder) byteout() {
	if e.buf[e.bp] == 0xFF {
		e.bp++
		e.buf = append(e.buf, byte(e.c>>20))
		e.c &= 0xFFFFF
		e.ct = 7
	} else if e.c&0x8000000 == 0 {
		e.bp++
		e.buf = append(e.buf, byte(e.c>>19))
		e.c &= 0x7FFFF
		e.ct = 8
	} else {
		e.buf[e.bp]++
		if e.buf[e.bp] == 0xFF {
			e.c &= 0x7FFFFFF
			e.bp++
			e.buf = append(e.buf, byte(e.c>>20))
			e.c &= 0xFFFFF
			e.ct = 7
		} else {
			e.bp++
			e.buf = append(e.buf, byte(e.c>>19))
			e.c &= 0x7FFFF
			e.ct = 8
		}
	}
}

func (e *mqEncoder) renorme() {
	for {
		e.a <<= 1
		e.c <<= 1
		e.ct--
		if e.ct == 0 {
			e.byteout()
		}
		if e.a&0x8000 != 0 {
			break
		}
	}
}

// encode codes one binary decision d for the given context (ENCODE, C.2.5).
func (e *mqEncoder) encode(cx *mqContext, d int) {
	qe := mqQe[cx.index].qe
	if int(cx.mps) == d {
		// CODEMPS
		e.a -= qe
		if e.a&0x8000 == 0 {
			if e.a < qe {
				e.a = qe
			} else {
				e.c += qe
			}
			cx.index = mqQe[cx.index].nmps
			e.renorme()
		} else {
			e.c += qe
		}
	} else {
		// CODELPS
		e.a -= qe
		if e.a < qe {
			e.c += qe
		} else {
			e.a = qe
		}
		if mqQe[cx.index].sw == 1 {
			cx.mps = 1 - cx.mps
		}
		cx.index = mqQe[cx.index].nlps
		e.renorme()
	}
}

// setbits is the SETBITS step of FLUSH (Figure C.11).
func (e *mqEncoder) setbits() {
	tempc := e.c + e.a
	e.c |= 0xFFFF
	if e.c >= tempc {
		e.c -= 0x8000
	}
}

// flush terminates the segment (FLUSH, C.2.9) and returns the coded bytes.
func (e *mqEncoder) flush() []byte {
	e.setbits()
	e.c <<= uint(e.ct)
	e.byteout()
	e.c <<= uint(e.ct)
	e.byteout()
	// A coding pass must not end with 0xFF; advance so numbytes is valid.
	if e.buf[e.bp] != 0xFF {
		e.bp++
	}
	return e.buf[1:e.bp]
}
