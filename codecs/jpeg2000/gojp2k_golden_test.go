//go:build openjpeg && cgo

package jpeg2000

import (
	"os"
	"testing"
)

// TestGoJ2KMatchesOpenJPEG decodes lossless (5/3) JPEG 2000 codestreams with both
// the pure-Go decoder and openjpeg and requires a byte-exact match.
func TestGoJ2KMatchesOpenJPEG(t *testing.T) {
	for _, path := range []string{
		"../../testdata/cornerstone-CTImage-jpeg2000-lossless.dcm",
	} {
		dcm, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("fixture unavailable: %v", err)
		}
		frame := extractFirstJ2KFrame(t, dcm)
		cs, err := parseCodestream(frame)
		if err != nil {
			t.Fatalf("%s parse: %v", path, err)
		}
		bps := 1
		if cs.comps[0].precision > 8 {
			bps = 2
		}
		size := (cs.xsiz - cs.xOsiz) * (cs.ysiz - cs.yOsiz) * cs.numComps() * bps

		want := make([]byte, size)
		if err := J2Kdecode(frame, uint32(len(frame)), want); err != nil {
			t.Fatalf("openjpeg decode: %v", err)
		}
		got := make([]byte, size)
		if err := goJ2Kdecode(frame, got); err != nil {
			t.Fatalf("pure-Go decode: %v", err)
		}
		nd, first := 0, -1
		for i := range want {
			if want[i] != got[i] {
				if first < 0 {
					first = i
				}
				nd++
			}
		}
		if nd != 0 {
			t.Fatalf("%s: %d/%d bytes differ (first at %d: got %d want %d)", path[13:], nd, len(want), first, got[first], want[first])
		}
		t.Logf("%s: byte-exact (%d bytes)", path[13:], size)
	}
}

// TestGoJ2KLossy97 decodes lossy (irreversible 9/7) JPEG 2000 with both the
// pure-Go decoder and openjpeg. Because the 9/7 inverse transform and scalar
// dequantization are floating-point, the result is compared within a small
// absolute tolerance rather than byte-for-byte.
func TestGoJ2KLossy97(t *testing.T) {
	for _, tc := range []struct {
		path      string
		maxAbs    int   // max allowed per-sample absolute difference
		maxOutlie int   // allowed count of samples exceeding maxAbs (rounding edges)
	}{
		{"../../testdata/pydicom-JPEG2000.dcm", 1, 0},
	} {
		dcm, err := os.ReadFile(tc.path)
		if err != nil {
			t.Skipf("fixture unavailable: %v", err)
		}
		frame := extractFirstJ2KFrame(t, dcm)
		cs, err := parseCodestream(frame)
		if err != nil {
			t.Fatalf("%s parse: %v", tc.path, err)
		}
		comp := cs.comps[0]
		bps := 1
		if comp.precision > 8 {
			bps = 2
		}
		nc := cs.numComps()
		nsamp := (cs.xsiz - cs.xOsiz) * (cs.ysiz - cs.yOsiz) * nc
		size := nsamp * bps
		t.Logf("%s: %dx%d nc=%d prec=%d signed=%v transform=%d qstyle=%d",
			tc.path[13:], cs.xsiz-cs.xOsiz, cs.ysiz-cs.yOsiz, nc, comp.precision,
			comp.signed, cs.cod.transform, cs.qcd.style)

		want := make([]byte, size)
		if err := J2Kdecode(frame, uint32(len(frame)), want); err != nil {
			t.Fatalf("openjpeg decode: %v", err)
		}
		got := make([]byte, size)
		if err := goJ2Kdecode(frame, got); err != nil {
			t.Fatalf("pure-Go decode: %v", err)
		}

		readSample := func(b []byte, i int) int {
			if bps == 1 {
				return int(b[i])
			}
			v := int(b[i*2]) | int(b[i*2+1])<<8
			if comp.signed {
				v = int(int16(uint16(v)))
			}
			return v
		}
		outliers, maxd, sumd := 0, 0, 0
		for i := 0; i < nsamp; i++ {
			d := readSample(want, i) - readSample(got, i)
			if d < 0 {
				d = -d
			}
			sumd += d
			if d > maxd {
				maxd = d
			}
			if d > tc.maxAbs {
				outliers++
			}
		}
		t.Logf("%s: maxDiff=%d meanDiff=%.4f outliers(>%d)=%d/%d",
			tc.path[13:], maxd, float64(sumd)/float64(nsamp), tc.maxAbs, outliers, nsamp)
		if outliers > tc.maxOutlie {
			t.Fatalf("%s: %d samples exceed |Δ|=%d (allowed %d); maxDiff=%d",
				tc.path[13:], outliers, tc.maxAbs, tc.maxOutlie, maxd)
		}
	}
}

// TestGoJ2KRGBLossless covers the 3-component lossless RGB codestream (test.j2k).
func TestGoJ2KRGBLossless(t *testing.T) {
	frame, err := os.ReadFile("../../testdata/test.j2k")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	cs, err := parseCodestream(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	size := (cs.xsiz - cs.xOsiz) * (cs.ysiz - cs.yOsiz) * cs.numComps()
	want := make([]byte, size)
	if err := J2Kdecode(frame, uint32(len(frame)), want); err != nil {
		t.Skipf("openjpeg decode: %v", err)
	}
	got := make([]byte, size)
	if err := goJ2Kdecode(frame, got); err != nil {
		t.Fatalf("pure-Go decode: %v", err)
	}
	nd := 0
	for i := range want {
		if want[i] != got[i] {
			nd++
		}
	}
	if nd != 0 {
		t.Fatalf("test.j2k RGB: %d/%d bytes differ", nd, len(want))
	}
	t.Logf("test.j2k RGB: byte-exact (%d bytes)", size)
}
