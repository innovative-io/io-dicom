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

// TestGoJLSNearLosslessMatchesCharls proves the pure-Go near-lossless (.81)
// decoder is byte-for-byte identical to charls. The stream is produced by the
// charls encoder (NEAR=1); decoding is deterministic given the stream, so the
// two decoders must agree exactly even though the encode itself is lossy.
func TestGoJLSNearLosslessMatchesCharls(t *testing.T) {
	t.Cleanup(func() { SetBackend(nil) })
	for _, tc := range []struct{ w, h, p int }{
		{16, 16, 8}, {40, 30, 8}, {50, 40, 12}, {64, 64, 16}, {33, 17, 10},
	} {
		bps := 1
		if tc.p > 8 {
			bps = 2
		}
		maxv := (1 << tc.p) - 1
		raw := make([]byte, tc.w*tc.h*bps)
		for i := 0; i < tc.w*tc.h; i++ {
			v := (i*7 + (i*i)%29 + (i*i*i)%17) % (maxv + 1)
			if bps == 1 {
				raw[i] = byte(v)
			} else {
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
		}

		// Encode a NEAR=1 stream with charls.
		if err := UseBackend("charls"); err != nil {
			t.Skipf("charls unavailable: %v", err)
		}
		var enc []byte
		var encSize int
		if err := JLSencode(raw, uint16(tc.w), uint16(tc.h), 1, uint16(tc.p), &enc, &encSize, true); err != nil {
			t.Fatalf("p%d charls near-lossless encode: %v", tc.p, err)
		}
		enc = enc[:encSize]

		f, _, err := parseJLS(enc)
		if err != nil {
			t.Fatalf("p%d parse charls near-lossless stream: %v", tc.p, err)
		}
		if f.near == 0 {
			t.Fatalf("p%d expected NEAR>0 stream from charls", tc.p)
		}
		size := f.width * f.height * len(f.comps) * bps

		decode := func(backend string) []byte {
			if err := UseBackend(backend); err != nil {
				t.Skipf("backend %s unavailable: %v", backend, err)
			}
			out := make([]byte, size)
			if err := JLSdecode(enc, uint32(len(enc)), out); err != nil {
				t.Fatalf("p%d %s decode: %v", tc.p, backend, err)
			}
			return out
		}
		want := decode("charls")
		got := decode("gojpegls")
		if !bytes.Equal(want, got) {
			n, first := 0, -1
			for i := range want {
				if want[i] != got[i] {
					if first < 0 {
						first = i
					}
					n++
				}
			}
			t.Fatalf("p%d pure-Go near-lossless decode differs from charls in %d/%d bytes (first at %d: got %d want %d)",
				tc.p, n, len(want), first, got[first], want[first])
		}
	}
}

// TestGoJLSNearLosslessEncodeDecodesInCharls confirms the pure-Go near-lossless
// encoder emits standard-conformant output: charls must decode the stream to
// exactly the same pixels the pure-Go decoder produces (both decoders are
// deterministic given the stream), and every pixel must be within NEAR of the
// original (the near-lossless guarantee).
func TestGoJLSNearLosslessEncodeDecodesInCharls(t *testing.T) {
	t.Cleanup(func() { SetBackend(nil) })
	const near = 1
	for _, tc := range []struct{ w, h, p int }{
		{16, 16, 8}, {40, 30, 8}, {50, 40, 12}, {64, 64, 16}, {33, 17, 10},
	} {
		bps := 1
		if tc.p > 8 {
			bps = 2
		}
		maxv := (1 << tc.p) - 1
		raw := make([]byte, tc.w*tc.h*bps)
		for i := 0; i < tc.w*tc.h; i++ {
			v := (i*7 + (i*i)%29 + (i*i*i)%17) % (maxv + 1)
			if bps == 1 {
				raw[i] = byte(v)
			} else {
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
		}
		enc, err := encodeJLS(raw, tc.w, tc.h, 1, tc.p, near)
		if err != nil {
			t.Fatalf("p%d pure-Go near-lossless encode: %v", tc.p, err)
		}

		// Pure-Go decode of our own stream (the reference reconstruction).
		SetBackend(nil)
		mine := make([]byte, len(raw))
		if err := decodeJLSInto(enc, mine); err != nil {
			t.Fatalf("p%d pure-Go decode of pure-Go output: %v", tc.p, err)
		}

		// charls must decode the same stream to identical pixels.
		if err := UseBackend("charls"); err != nil {
			t.Skipf("charls unavailable: %v", err)
		}
		theirs := make([]byte, len(raw))
		if err := JLSdecode(enc, uint32(len(enc)), theirs); err != nil {
			t.Fatalf("p%d charls decode of pure-Go output: %v", tc.p, err)
		}
		if !bytes.Equal(mine, theirs) {
			t.Fatalf("p%d charls decode of pure-Go near-lossless stream differs from pure-Go decode", tc.p)
		}

		// Near-lossless guarantee: |original - reconstructed| <= NEAR.
		for i := 0; i < tc.w*tc.h; i++ {
			var o, g int
			if bps == 1 {
				o, g = int(raw[i]), int(theirs[i])
			} else {
				o = int(raw[i*2]) | int(raw[i*2+1])<<8
				g = int(theirs[i*2]) | int(theirs[i*2+1])<<8
			}
			if d := o - g; d < -near || d > near {
				t.Fatalf("p%d sample %d: got %d, want within %d of %d", tc.p, i, g, near, o)
			}
		}
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
		enc, err := encodeJLS(raw, tc.w, tc.h, 1, tc.p, 0)
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
