package gojxl

import (
	"os"
	"path/filepath"
	"testing"
)

// parseToFrame runs signature → SizeHeader → ImageMetadata → FrameHeader → TOC.
func parseToFrame(t *testing.T, data []byte) (Header, FrameHeader, []uint32, *bitReader) {
	t.Helper()
	cs, err := codestream(data)
	if err != nil {
		t.Fatalf("codestream: %v", err)
	}
	b := newBitReader(cs[2:])
	sh, err := readSizeHeader(b)
	if err != nil {
		t.Fatalf("size: %v", err)
	}
	meta, err := readImageMetadata(b)
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	if meta.Color.WantICC {
		t.Skip("want_icc files need ICC-blob decoding (not yet implemented)")
	}
	readTransformData(b, meta.XYBEncoded)
	if err := b.JumpToByteBoundary(); err != nil {
		t.Fatalf("byte boundary: %v", err)
	}
	fh, err := readFrameHeader(b, &meta)
	if err != nil {
		t.Fatalf("frame header: %v", err)
	}
	fd := computeFrameDimensions(sh.Xsize, sh.Ysize, fh.GroupSizeShift, fh.Upsampling)
	toc, _, err := readTOC(b, numTocEntries(fd.numGroups, fd.numDCGroups, fh.NumPasses))
	if err != nil {
		t.Fatalf("toc: %v", err)
	}
	return Header{Size: sh, Meta: meta}, fh, toc, b
}

func TestFrameHeaderFixtures(t *testing.T) {
	files := []string{
		"gray8_8x4_lossless.jxl",
		"rgb8_8x4_lossless.jxl",
		"rgba8_8x4_lossless.jxl",
		"gray8_300x200_lossless.jxl",
	}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", f))
			if err != nil {
				t.Skipf("fixture unavailable: %v", err)
			}
			_, fh, toc, b := parseToFrame(t, data)
			if fh.Type != frameRegular {
				t.Errorf("frame type = %d, want regular(0)", fh.Type)
			}
			if fh.Encoding != frameModular {
				t.Errorf("encoding = %d, want modular(1)", fh.Encoding)
			}
			if !fh.IsLast {
				t.Errorf("is_last = false, want true for single-frame file")
			}
			if len(toc) == 0 {
				t.Fatalf("empty TOC")
			}
			var total uint32
			for _, s := range toc {
				total += s
			}
			// After the TOC the reader is byte-aligned; the single TOC entry must
			// equal exactly the remaining frame bytes (last/only frame).
			consumedBytes := b.bitsConsumed() / 8
			remaining := uint32(len(b.data) - consumedBytes)
			if total != remaining {
				t.Errorf("toc total %d != remaining frame bytes %d", total, remaining)
			}
			t.Logf("%s: %d toc entries, frame data = %d bytes (exact)", f, len(toc), total)
		})
	}
}
