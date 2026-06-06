package jpeg2000

import (
	"encoding/binary"
	"os"
	"testing"
)

// TestHTEncodeEmitForOjph writes HTJ2K codestreams plus matching raw PGM/PPM
// references to /tmp so an out-of-band ojph_expand run can confirm the encoder
// is conformant. Gated behind HT_EMIT=1 so it never runs in CI.
func TestHTEncodeEmitForOjph(t *testing.T) {
	if os.Getenv("HT_EMIT") != "1" {
		t.Skip("set HT_EMIT=1 to emit HTJ2K test files")
	}
	cases := []struct {
		name           string
		w, h, nc, prec int
	}{
		{"gray8", 96, 72, 1, 8},
		{"gray16", 100, 70, 1, 16},
		{"rgb8", 64, 48, 3, 8},
		{"gray10", 200, 13, 1, 10},
	}
	for _, c := range cases {
		bps := 1
		if c.prec > 8 {
			bps = 2
		}
		raw := make([]byte, c.w*c.h*c.nc*bps)
		for y := 0; y < c.h; y++ {
			for x := 0; x < c.w; x++ {
				for k := 0; k < c.nc; k++ {
					v := (x*5 + y*3 + k*37) & ((1 << uint(c.prec)) - 1)
					i := (y*c.w + x) * c.nc * bps
					if bps == 1 {
						raw[i+k] = byte(v)
					} else {
						binary.LittleEndian.PutUint16(raw[i+k*2:], uint16(v))
					}
				}
			}
		}
		stream, err := goHTJ2Kencode(raw, c.w, c.h, c.nc, c.prec)
		if err != nil {
			t.Fatalf("%s encode: %v", c.name, err)
		}
		j2c := "/tmp/htgo_" + c.name + ".j2c"
		if err := os.WriteFile(j2c, stream, 0o644); err != nil {
			t.Fatal(err)
		}

		// Reference as binary PGM/PPM (P5/P6). 16-bit PGM/PPM is big-endian.
		ref := "/tmp/htgo_" + c.name + ".ref.pgm"
		magic := "P5"
		if c.nc == 3 {
			magic = "P6"
			ref = "/tmp/htgo_" + c.name + ".ref.ppm"
		}
		maxval := (1 << uint(c.prec)) - 1
		var pnm []byte
		pnm = append(pnm, []byte(magic+"\n")...)
		pnm = append(pnm, []byte(itoa(c.w)+" "+itoa(c.h)+"\n"+itoa(maxval)+"\n")...)
		for y := 0; y < c.h; y++ {
			for x := 0; x < c.w; x++ {
				for k := 0; k < c.nc; k++ {
					i := (y*c.w + x) * c.nc * bps
					if bps == 1 {
						pnm = append(pnm, raw[i+k])
					} else {
						v := binary.LittleEndian.Uint16(raw[i+k*2:])
						pnm = append(pnm, byte(v>>8), byte(v)) // PNM is big-endian
					}
				}
			}
		}
		if err := os.WriteFile(ref, pnm, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d bytes) and %s", j2c, len(stream), ref)
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
