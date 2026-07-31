package jpeg2000

import (
	"bytes"
	"testing"
)

// setCODMCT flips the multi-component-transform flag in the codestream's COD
// segment. Layout per PS3.5 / ITU-T T.800 A.6.1: marker FF52, then a two-byte
// segment length, then Scod, SGcod (progression, layers, MCT). parseCOD reads
// the segment body from just past the length, so MCT is body[4].
func setCODMCT(t *testing.T, cs []byte, v byte) []byte {
	t.Helper()
	out := append([]byte(nil), cs...)
	for i := 0; i+1 < len(out); i++ {
		if out[i] != 0xFF || out[i+1] != 0x52 {
			continue
		}
		body := i + 4 // skip marker (2) + Lcod (2)
		if body+4 >= len(out) {
			t.Fatalf("COD segment at %d is truncated", i)
		}
		out[body+4] = v
		return out
	}
	t.Fatalf("no COD marker (FF52) found in codestream")
	return nil
}

// TestInverseMCTIsApplied pins that the decoder honours the multi-component
// transform flag it parses.
//
// parseCOD reads Scod/SGcod into codingStyle.mct, but nothing on the decode
// path ever reads that field back — there is no inverse ICT or RCT in the
// package. Colour JPEG 2000 encoded with the MCT (the usual case for
// YBR_ICT and YBR_RCT) therefore decodes to raw, unconverted component planes
// and renders with badly wrong colours.
//
// The encoder in this package always writes mct=0, which is why round-trip
// tests never exposed this. The test instead takes a known-good mct=0
// codestream and flips only that one byte: a decoder that honours the flag must
// produce different output, because it would now apply an inverse transform
// that was never applied at encode time.
func TestInverseMCTIsApplied(t *testing.T) {
	// Pin the pure-Go backend: other tests in this package leave SetBackend(nil)
	// installed, under which J2Kencode passes bytes through and emits no
	// codestream at all.
	if err := UseBackend("gojpeg2000"); err != nil {
		t.Fatalf("UseBackend(gojpeg2000): %v", err)
	}
	t.Cleanup(func() { SetBackend(nil) })

	const w, h = 16, 16
	// Strongly non-grey samples: an inverse RCT/ICT on neutral data is close to
	// a no-op and would hide the difference.
	src := make([]byte, w*h*3)
	for i := 0; i < w*h; i++ {
		src[3*i] = byte(200 - i%97)
		src[3*i+1] = byte(20 + i%53)
		src[3*i+2] = byte(120 + i%31)
	}

	var enc []byte
	var encSize int
	if err := J2Kencode(src, w, h, 3, 8, &enc, &encSize, 0); err != nil {
		t.Skipf("J2K encoder unavailable: %v", err)
	}
	enc = enc[:encSize]

	plain := make([]byte, len(src))
	if err := J2Kdecode(enc, uint32(len(enc)), plain); err != nil {
		t.Fatalf("decode of mct=0 codestream: %v", err)
	}

	withMCT := setCODMCT(t, enc, 1)
	got := make([]byte, len(src))
	if err := J2Kdecode(withMCT, uint32(len(withMCT)), got); err != nil {
		t.Fatalf("decode of mct=1 codestream: %v", err)
	}

	if bytes.Equal(got, plain) {
		t.Fatalf("setting the COD multi-component-transform flag changed nothing: "+
			"the decoder parses mct (gojp2k_codestream.go parseCOD) and never "+
			"applies the inverse transform, so colour J2K encoded with the MCT — "+
			"the usual case for YBR_ICT and YBR_RCT — decodes to unconverted "+
			"component planes (first sample %d,%d,%d)", got[0], got[1], got[2])
	}
}

// forwardRCT is the encoder-side reversible transform from T.800 G.2, written
// here so the inverse can be checked for exactness rather than merely for
// "output changed".
func forwardRCT(r, g, b int32) (y0, y1, y2 int32) {
	y0 = (r + 2*g + b) >> 2
	y1 = b - g
	y2 = r - g
	return
}

// TestInverseRCTIsExact pins the reversible path as a true inverse. The RCT is
// lossless by construction, so forward-then-inverse must reproduce every
// sample exactly — including negatives, where a `/4` would truncate toward zero
// and T.800's floor requires an arithmetic shift.
func TestInverseRCTIsExact(t *testing.T) {
	var rs, gs, bs []int32
	for r := int32(-128); r < 128; r += 7 {
		for g := int32(-128); g < 128; g += 11 {
			for b := int32(-128); b < 128; b += 13 {
				rs = append(rs, r)
				gs = append(gs, g)
				bs = append(bs, b)
			}
		}
	}
	y0 := make([]int32, len(rs))
	y1 := make([]int32, len(rs))
	y2 := make([]int32, len(rs))
	for i := range rs {
		y0[i], y1[i], y2[i] = forwardRCT(rs[i], gs[i], bs[i])
	}

	inverseMCT([][]int32{y0, y1, y2}, true)

	for i := range rs {
		if y0[i] != rs[i] || y1[i] != gs[i] || y2[i] != bs[i] {
			t.Fatalf("RCT round-trip failed at sample %d: sent (%d,%d,%d), got (%d,%d,%d)",
				i, rs[i], gs[i], bs[i], y0[i], y1[i], y2[i])
		}
	}
	t.Logf("verified %d samples", len(rs))
}

// TestInverseICTKnownValues checks the irreversible path against the T.800 G.1
// matrix. Chroma is DC-centred here (no +128 offset): the inverse MCT runs
// before the DC level shift.
func TestInverseICTKnownValues(t *testing.T) {
	cases := []struct{ y, cb, cr, wr, wg, wb int32 }{
		{0, 0, 0, 0, 0, 0},       // neutral stays neutral
		{50, 0, 0, 50, 50, 50},   // no chroma -> grey
		{0, 0, 100, 140, -71, 0}, // Cr drives red
		{0, 100, 0, 0, -34, 177}, // Cb drives blue
	}
	for _, tc := range cases {
		y0 := []int32{tc.y}
		y1 := []int32{tc.cb}
		y2 := []int32{tc.cr}
		inverseMCT([][]int32{y0, y1, y2}, false)
		if abs32(y0[0]-tc.wr) > 1 || abs32(y1[0]-tc.wg) > 1 || abs32(y2[0]-tc.wb) > 1 {
			t.Errorf("ICT(%d,%d,%d) = (%d,%d,%d), want (%d,%d,%d)",
				tc.y, tc.cb, tc.cr, y0[0], y1[0], y2[0], tc.wr, tc.wg, tc.wb)
		}
	}
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
