package gojxl

import "testing"

// TestBitWriterRoundTrip writes values with each field coder and reads them
// back with the decoder, verifying the writer is the exact inverse.
func TestBitWriterRoundTrip(t *testing.T) {
	w := newBitWriter()
	w.WriteBool(true)
	w.WriteBits(0x2A, 6)
	w.WriteBool(false)
	w.WriteBits(0x1234, 16)
	// U32 with a representative distribution.
	u32vals := []uint32{0, 1, 8, 9, 255, 256, 1023, 100000}
	for _, v := range u32vals {
		w.WriteU32(v, u32Val(0), u32Val(1), u32Off(8, 2), u32Off(20, 258))
	}
	u64vals := []uint64{0, 1, 16, 17, 272, 273, 1 << 20, 1 << 40}
	for _, v := range u64vals {
		w.WriteU64(v)
	}
	enumVals := []uint32{0, 1, 2, 13, 17, 18, 50, 81}
	for _, v := range enumVals {
		w.WriteEnum(v)
	}
	data := w.Bytes()

	b := newBitReader(data)
	if !b.ReadBool() {
		t.Fatal("bool 1")
	}
	if got := b.ReadBits(6); got != 0x2A {
		t.Fatalf("bits6: %x", got)
	}
	if b.ReadBool() {
		t.Fatal("bool 2")
	}
	if got := b.ReadBits(16); got != 0x1234 {
		t.Fatalf("bits16: %x", got)
	}
	for _, want := range u32vals {
		if got := b.ReadU32(u32Val(0), u32Val(1), u32Off(8, 2), u32Off(20, 258)); got != want {
			t.Fatalf("u32: got %d want %d", got, want)
		}
	}
	for _, want := range u64vals {
		if got := b.ReadU64(); got != want {
			t.Fatalf("u64: got %d want %d", got, want)
		}
	}
	for _, want := range enumVals {
		if got := b.ReadEnum(); got != want {
			t.Fatalf("enum: got %d want %d", got, want)
		}
	}
}

// TestBitWriterByteAlign checks ZeroPadToByte aligns and pads with zeros.
func TestBitWriterByteAlign(t *testing.T) {
	w := newBitWriter()
	w.WriteBits(0b101, 3)
	w.ZeroPadToByte()
	w.WriteBits(0xFF, 8)
	data := w.Bytes()
	if len(data) != 2 || data[0] != 0b101 || data[1] != 0xFF {
		t.Fatalf("got % x", data)
	}
}
