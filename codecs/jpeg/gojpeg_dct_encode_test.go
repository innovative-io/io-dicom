package jpeg

import "testing"

// TestGoJPEGDCTEncodeRoundTrip encodes synthetic 8/12-bit grayscale images with
// the pure-Go DCT encoder and decodes them back with the pure-Go DCT decoder.
// DCT is lossy, so this asserts the round trip stays within a tolerance that a
// high quality factor comfortably meets.
func TestGoJPEGDCTEncodeRoundTrip(t *testing.T) {
	for _, p := range []int{8, 12} {
		w, h := 40, 32
		maxv := (1 << p) - 1
		bps := 1
		if p > 8 {
			bps = 2
		}
		raw := make([]byte, w*h*bps)
		for i := 0; i < w*h; i++ {
			v := (i*7 + (i*i)%23) % (maxv + 1)
			if bps == 1 {
				raw[i] = byte(v)
			} else {
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
		}

		enc, err := encodeDCTJPEG(raw, w, h, p, defaultDCTQuality)
		if err != nil {
			t.Fatalf("p%d encode: %v", p, err)
		}
		f, err := decodeDCT(enc)
		if err != nil {
			t.Fatalf("p%d decodeDCT: %v", p, err)
		}
		if f.width != w || f.height != h || f.precision != p || len(f.comps) != 1 {
			t.Fatalf("p%d geometry mismatch: %dx%d P%d c%d", p, f.width, f.height, f.precision, len(f.comps))
		}
		out := make([]byte, len(raw))
		if err := decodeDCTInto(enc, out); err != nil {
			t.Fatalf("p%d decodeInto: %v", p, err)
		}
		maxDiff := 0
		for i := 0; i < w*h; i++ {
			var a, b int
			if bps == 1 {
				a, b = int(raw[i]), int(out[i])
			} else {
				a = int(raw[i*2]) | int(raw[i*2+1])<<8
				b = int(out[i*2]) | int(out[i*2+1])<<8
			}
			if d := a - b; d > maxDiff {
				maxDiff = d
			} else if -d > maxDiff {
				maxDiff = -d
			}
		}
		if maxDiff > 30 {
			t.Fatalf("p%d DCT round-trip max diff %d exceeds tolerance", p, maxDiff)
		}
	}
}

func TestGoJPEGDCTEncodeRejectsBadInput(t *testing.T) {
	if _, err := encodeDCTJPEG(make([]byte, 10), 4, 4, 12, 90); err == nil {
		t.Fatal("expected size-mismatch error")
	}
	if _, err := encodeDCTJPEG(make([]byte, 16), 4, 4, 10, 90); err == nil {
		t.Fatal("expected unsupported-precision error")
	}
}
