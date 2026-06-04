//go:build charls && cgo

package jpegls

import (
	"bytes"
	"testing"
)

// TestGoJLSLosslessMatchesCharls proves the pure-Go lossless JPEG-LS decoder is
// byte-for-byte identical to charls on lossless fixtures. Lossless decoding has
// exactly one correct output, so any mismatch is a real bug.
func TestGoJLSLosslessMatchesCharls(t *testing.T) {
	cases := []string{
		"../../testdata/cornerstone-CTImage-jpegls-lossless.dcm",
		"../../testdata/highdicom-sm_image_jpegls.dcm",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			dcm := loadDCM(t, path)
			frame := extractFirstFrame(t, dcm)
			f, _, err := parseJLS(frame)
			if err != nil {
				t.Skipf("not a pure-Go-supported lossless stream: %v", err)
			}
			bps := 1
			if f.precision > 8 {
				bps = 2
			}
			size := f.width * f.height * len(f.comps) * bps

			t.Cleanup(func() { SetBackend(nil) })
			decode := func(backend string) []byte {
				if err := UseBackend(backend); err != nil {
					t.Skipf("backend %s unavailable: %v", backend, err)
				}
				out := make([]byte, size)
				if err := JLSdecode(frame, uint32(len(frame)), out); err != nil {
					t.Fatalf("%s decode: %v", backend, err)
				}
				return out
			}
			want := decode("charls")
			got := decode("gojpegls")
			if !bytes.Equal(want, got) {
				n := 0
				for i := range want {
					if want[i] != got[i] {
						n++
					}
				}
				t.Fatalf("pure-Go decode differs from charls in %d/%d bytes", n, len(want))
			}
		})
	}
}

// TestGoJLSEncodeDecodesInCharls confirms the pure-Go lossless JPEG-LS encoder
// emits standard-conformant output by decoding it with charls (exact round trip).
func TestGoJLSEncodeDecodesInCharls(t *testing.T) {
	t.Cleanup(func() { SetBackend(nil) })
	for _, tc := range []struct{ w, h, p int }{
		{16, 12, 8}, {40, 30, 12}, {50, 40, 16}, {64, 64, 12},
	} {
		bps := 1
		if tc.p > 8 {
			bps = 2
		}
		maxv := (1 << tc.p) - 1
		raw := make([]byte, tc.w*tc.h*bps)
		for i := 0; i < tc.w*tc.h; i++ {
			v := (i*5 + (i*i)%13) % (maxv + 1)
			if bps == 1 {
				raw[i] = byte(v)
			} else {
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
		}
		enc, err := encodeJLS(raw, tc.w, tc.h, 1, tc.p)
		if err != nil {
			t.Fatalf("p%d encode: %v", tc.p, err)
		}
		if err := UseBackend("charls"); err != nil {
			t.Skipf("charls unavailable: %v", err)
		}
		out := make([]byte, len(raw))
		if err := JLSdecode(enc, uint32(len(enc)), out); err != nil {
			t.Fatalf("p%d charls decode of pure-Go output: %v", tc.p, err)
		}
		if !bytes.Equal(out, raw) {
			t.Fatalf("p%d charls did not round-trip the pure-Go encoded stream", tc.p)
		}
	}
}
