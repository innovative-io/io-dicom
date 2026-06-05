package jpeg2000

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestHTEncodeVsOpenJPH compares encodeHTCleanup byte-for-byte against vectors
// produced by openjph's own ojph_encode_codeblock32 (see /tmp/htenc_cross.cpp).
// The vector file is generated out-of-band; the test is skipped when absent so
// CI (which has no openjph) stays green.
func TestHTEncodeVsOpenJPH(t *testing.T) {
	const path = "/tmp/htenc_vectors.txt"
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("vector file %s not present: %v", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	line := 0
	cases := 0
	for sc.Scan() {
		line++
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		idx := 0
		next := func() int {
			v, _ := strconv.Atoi(fields[idx])
			idx++
			return v
		}
		nextHex := func() uint64 {
			v, _ := strconv.ParseUint(fields[idx], 16, 64)
			idx++
			return v
		}
		w := next()
		h := next()
		mmsb := next()
		n := next()
		coeffs := make([]uint32, w*h)
		for i := 0; i < n; i++ {
			coeffs[i] = uint32(nextHex())
		}
		encLen := next()
		want := make([]byte, encLen)
		for i := 0; i < encLen; i++ {
			want[i] = byte(nextHex())
		}

		got := encodeHTCleanup(coeffs, mmsb, w, h, w)
		if len(got) != len(want) {
			t.Fatalf("line %d (%dx%d mmsb=%d): length %d != openjph %d\n got=%x\nwant=%x",
				line, w, h, mmsb, len(got), len(want), got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("line %d (%dx%d mmsb=%d): byte %d = %02x != openjph %02x\n got=%x\nwant=%x",
					line, w, h, mmsb, i, got[i], want[i], got, want)
			}
		}
		cases++
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	t.Logf("compared %d code-blocks byte-exact vs openjph", cases)
}
