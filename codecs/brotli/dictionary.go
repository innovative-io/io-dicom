package brotli

import (
	"bytes"
	"compress/flate"
	_ "embed"
	"io"
	"sync"
)

//go:embed dictionary.bin.flate
var dictFlate []byte

//go:embed ctxlut.bin
var ctxLut []byte // 2048 bytes: 8 sub-tables of 256 (see context())

var (
	dictOnce sync.Once
	dictData []byte
)

// dictionary returns the 122,784-byte Brotli static dictionary, inflating the
// embedded copy on first use.
func dictionary() []byte {
	dictOnce.Do(func() {
		r := flate.NewReader(bytes.NewReader(dictFlate))
		defer r.Close()
		buf, err := io.ReadAll(r)
		if err != nil || len(buf) != 122784 {
			panic("brotli: corrupt embedded dictionary")
		}
		dictData = buf
	})
	return dictData
}

// Per-word-length offset into the dictionary data and the number of words of
// that length, expressed as a bit count (RFC 7932 Appendix A). Indexed by word
// length 0..24; lengths below 4 are unused.
var dictOffsetByLength = [25]uint32{
	0, 0, 0, 0, 0, 4096, 9216, 21504, 35840, 44032, 53248, 63488, 74752,
	87040, 93696, 100864, 104704, 106752, 108928, 113536, 115968, 118528,
	119872, 121280, 122016,
}
var dictSizeBitsByLength = [25]uint8{
	0, 0, 0, 0, 10, 10, 11, 11, 10, 10, 10, 10, 10, 9, 9, 8, 7, 7, 8, 7, 7,
	6, 6, 5, 5,
}

// context computes the literal context id (0..63) from the two preceding bytes
// p1 (most recent) and p2 under the given context mode (0=LSB6, 1=MSB6, 2=UTF8,
// 3=Signed). ctxLut holds eight 256-entry sub-tables: for mode m, the p1
// sub-table is at m*512 and the p2 sub-table at m*512+256.
func context(mode, p1, p2 int) int {
	base := mode * 512
	return int(ctxLut[base+p1]) | int(ctxLut[base+256+p2])
}
