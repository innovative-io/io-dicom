package brotli

// Insert-length codes (24 entries): extra bits and base offset (RFC 7932 §5).
var insExtra = [24]uint{0, 0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 7, 8, 9, 10, 12, 14, 24}
var insOffset = [24]uint32{0, 1, 2, 3, 4, 5, 6, 8, 10, 14, 18, 26, 34, 50, 66, 98, 130, 194, 322, 578, 1090, 2114, 6210, 22594}

// Copy-length codes (24 entries): extra bits and base offset (RFC 7932 §5).
var copyExtra = [24]uint{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 7, 8, 9, 10, 24}
var copyOffset = [24]uint32{2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 14, 18, 22, 30, 38, 54, 70, 102, 134, 198, 326, 582, 1094, 2118}

// Block-length codes (26 entries): extra bits and base offset (RFC 7932 §6).
var blockLenExtra = [26]uint{2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 7, 8, 9, 10, 11, 12, 13, 24}
var blockLenOffset = [26]uint32{1, 5, 9, 13, 17, 25, 33, 41, 49, 65, 81, 97, 113, 145, 177, 209, 241, 305, 369, 497, 753, 1265, 2289, 4337, 8433, 16625}

// cmdLutEntry decodes one insert-and-copy command symbol.
type cmdLutEntry struct {
	insExtra  uint
	insOffset uint32
	cpExtra   uint
	cpOffset  uint32
	dist0     bool // command uses the implicit (last) distance, no distance code
}

// kCmdLut maps each of the 704 insert-and-copy length codes to its component
// insert and copy length codes plus the implicit-distance flag. It is derived
// from the 11 command-range blocks defined in RFC 7932 §5.
var kCmdLut [704]cmdLutEntry

func init() {
	// Per command block (cmd_code>>6): insert-code base, copy-code base, dist0.
	type blk struct {
		iB, cB int
		d0     bool
	}
	blocks := [11]blk{
		{0, 0, true},
		{0, 8, true},
		{0, 0, false},
		{0, 8, false},
		{8, 0, false},
		{8, 8, false},
		{0, 16, false},
		{16, 0, false},
		{8, 16, false},
		{16, 8, false},
		{16, 16, false},
	}
	for code := 0; code < 704; code++ {
		bl := blocks[code>>6]
		insCode := bl.iB + ((code >> 3) & 7)
		cpCode := bl.cB + (code & 7)
		kCmdLut[code] = cmdLutEntry{
			insExtra:  insExtra[insCode],
			insOffset: insOffset[insCode],
			cpExtra:   copyExtra[cpCode],
			cpOffset:  copyOffset[cpCode],
			dist0:     bl.d0,
		}
	}
}

// distanceShortCode describes one of the 16 "last distance" reference codes as
// (ring index, signed offset). Ring index selects one of the four most recent
// distances; the offset adjusts it.
var distanceShortCode = [16]struct {
	ring   int
	offset int
}{
	{0, 0}, {1, 0}, {2, 0}, {3, 0},
	{0, -1}, {0, 1}, {0, -2}, {0, 2},
	{0, -3}, {0, 3}, {1, -1}, {1, 1},
	{1, -2}, {1, 2}, {1, -3}, {1, 3},
}
