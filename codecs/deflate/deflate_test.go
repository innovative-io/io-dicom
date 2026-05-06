package deflate

import (
	"testing"
)

func TestDeflateInflateFrameRoundTrip(t *testing.T) {
	in := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}

	deflated, err := DeflateFrame(in)
	if err != nil {
		t.Fatalf("DeflateFrame failed: %v", err)
	}

	out, err := InflateFrame(deflated, len(in))
	if err != nil {
		t.Fatalf("InflateFrame failed: %v", err)
	}

	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("out[%d]=%d want=%d", i, out[i], in[i])
		}
	}
}

func TestInflateFrameWrongSize(t *testing.T) {
	in := []byte{1, 2, 3, 4}
	deflated, err := DeflateFrame(in)
	if err != nil {
		t.Fatalf("DeflateFrame failed: %v", err)
	}

	if _, err := InflateFrame(deflated, len(in)+1); err == nil {
		t.Fatal("expected size mismatch error")
	}
}
