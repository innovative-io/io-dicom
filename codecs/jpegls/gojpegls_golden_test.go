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
