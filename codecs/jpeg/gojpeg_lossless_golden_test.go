//go:build libjpeg && cgo

package jpeg

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestGoJPEGLosslessMatchesLibjpeg proves the pure-Go lossless decoder is
// byte-for-byte identical to libjpeg on the lossless SOF3 fixtures. Lossless
// means there is exactly one correct answer, so any mismatch is a real bug.
func TestGoJPEGLosslessMatchesLibjpeg(t *testing.T) {
	cases := []string{
		"../../testdata/cornerstone-CTImage-jpeg-process14.dcm",
		"../../testdata/cornerstone-CTImage-jpeg-process14sv1.dcm",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			dcm := loadBytesFromFile(path, t)
			frame := extractFirstDICOMEncapsulatedFrame(t, dcm)

			f, _, err := decodeLossless(frame)
			if err != nil {
				t.Fatalf("decodeLossless: %v", err)
			}
			bps := 1
			if f.precision > 8 {
				bps = 2
			}
			size := f.width * f.height * len(f.comps) * bps

			decode := func(backend string) []byte {
				if err := UseBackend(backend); err != nil {
					t.Skipf("backend %s unavailable: %v", backend, err)
				}
				out := make([]byte, size)
				var derr error
				switch bps {
				case 1:
					derr = DIJG8decode(frame, uint32(len(frame)), out, uint32(size))
				default:
					derr = DIJG16decode(frame, uint32(len(frame)), out, uint32(size))
				}
				if derr != nil {
					t.Fatalf("%s decode: %v", backend, derr)
				}
				return out
			}

			t.Cleanup(func() { SetBackend(nil) })
			want := decode("libjpeg")
			got := decode("gojpeg")
			if !bytes.Equal(want, got) {
				n := 0
				for i := range want {
					if want[i] != got[i] {
						n++
					}
				}
				t.Fatalf("pure-Go decode differs from libjpeg in %d/%d bytes", n, len(want))
			}
		})
	}
}

// TestGoJPEGDCT12CloseToLibjpeg checks the pure-Go 12-bit DCT decoder against
// libjpeg on the Extended (SOF1) fixtures. DCT is lossy and the IDCT rounding
// differs between decoders, so this compares with a small tolerance rather than
// requiring bit-exact equality.
func TestGoJPEGDCT12CloseToLibjpeg(t *testing.T) {
	cases := []string{
		"../../testdata/cornerstone-CTImage-jpeg-process2-4.dcm",
		"../../testdata/pydicom-JPGExtended.dcm",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			dcm := loadBytesFromFile(path, t)
			frame := extractFirstDICOMEncapsulatedFrame(t, dcm)
			f, err := decodeDCT(frame)
			if err != nil {
				t.Fatalf("decodeDCT: %v", err)
			}
			size := f.width * f.height * len(f.comps) * 2

			t.Cleanup(func() { SetBackend(nil) })
			decode := func(backend string) []byte {
				if err := UseBackend(backend); err != nil {
					t.Skipf("backend %s unavailable: %v", backend, err)
				}
				out := make([]byte, size)
				if err := DIJG12decode(frame, uint32(len(frame)), out, uint32(size)); err != nil {
					t.Fatalf("%s decode: %v", backend, err)
				}
				return out
			}
			want := decode("libjpeg")
			got := decode("gojpeg")

			n := size / 2
			var maxDiff, sumDiff int64
			for i := 0; i < n; i++ {
				a := int64(binary.LittleEndian.Uint16(want[i*2:]))
				b := int64(binary.LittleEndian.Uint16(got[i*2:]))
				d := a - b
				if d < 0 {
					d = -d
				}
				if d > maxDiff {
					maxDiff = d
				}
				sumDiff += d
			}
			mean := float64(sumDiff) / float64(n)
			t.Logf("%s: maxDiff=%d meanDiff=%.4f over %d samples", path, maxDiff, mean, n)
			if maxDiff > 4 {
				t.Fatalf("max abs diff %d exceeds tolerance (4)", maxDiff)
			}
			if mean > 0.5 {
				t.Fatalf("mean abs diff %.4f exceeds tolerance (0.5)", mean)
			}
		})
	}
}

// TestGoJPEGEncodeDecodesInLibjpeg confirms the pure-Go lossless encoder emits
// standard-conformant output by decoding it with libjpeg and requiring an exact
// round trip.
func TestGoJPEGEncodeDecodesInLibjpeg(t *testing.T) {
	for _, tc := range []struct {
		name             string
		w, h, samples, p int
	}{
		{"gray12", 64, 40, 1, 12},
		{"gray16", 51, 33, 1, 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := make([]byte, tc.w*tc.h*tc.samples*2)
			maxv := (1 << tc.p) - 1
			for i := 0; i < tc.w*tc.h*tc.samples; i++ {
				v := (i*11 + (i*i)%17) % (maxv + 1)
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
			enc, err := encodeLosslessJPEG(raw, tc.w, tc.h, tc.samples, tc.p)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			t.Cleanup(func() { SetBackend(nil) })
			if err := UseBackend("libjpeg"); err != nil {
				t.Skipf("libjpeg unavailable: %v", err)
			}
			out := make([]byte, len(raw))
			var derr error
			if tc.p > 12 {
				derr = DIJG16decode(enc, uint32(len(enc)), out, uint32(len(out)))
			} else {
				derr = DIJG12decode(enc, uint32(len(enc)), out, uint32(len(out)))
			}
			if derr != nil {
				t.Fatalf("libjpeg decode of pure-Go output: %v", derr)
			}
			if !bytes.Equal(out, raw) {
				t.Fatal("libjpeg did not round-trip the pure-Go encoded stream")
			}
		})
	}
}
