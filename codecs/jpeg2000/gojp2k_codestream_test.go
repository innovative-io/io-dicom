package jpeg2000

import (
	"encoding/binary"
	"os"
	"testing"
)

// extractFirstJ2KFrame pulls the first encapsulated pixel-data fragment from a
// DICOM file and returns its JPEG 2000 codestream (skipping any leading JP2
// signature box, though DICOM uses the raw codestream).
func extractFirstJ2KFrame(t *testing.T, dcm []byte) []byte {
	t.Helper()
	pix := []byte{0xE0, 0x7F, 0x10, 0x00}
	idx := -1
	for i := 0; i+12 < len(dcm); i++ {
		if dcm[i] == pix[0] && dcm[i+1] == pix[1] && dcm[i+2] == pix[2] && dcm[i+3] == pix[3] {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("pixel data tag not found")
	}
	pos := idx + 12 // OB + reserved + length(0xFFFFFFFF)
	read := func() []byte {
		if pos+8 > len(dcm) {
			return nil
		}
		l := binary.LittleEndian.Uint32(dcm[pos+4 : pos+8])
		pos += 8
		if l == 0xFFFFFFFF || pos+int(l) > len(dcm) {
			return nil
		}
		b := dcm[pos : pos+int(l)]
		pos += int(l)
		return b
	}
	read() // basic offset table
	frame := read()
	if len(frame) == 0 {
		t.Fatal("could not read frame item")
	}
	// Skip a JP2 signature box if present.
	if len(frame) >= 2 && binary.BigEndian.Uint16(frame) != mSOC {
		if i := indexSOC(frame); i >= 0 {
			frame = frame[i:]
		}
	}
	return frame
}

func indexSOC(b []byte) int {
	for i := 0; i+1 < len(b); i++ {
		if b[i] == 0xFF && b[i+1] == 0x4F {
			return i
		}
	}
	return -1
}

func TestGoJ2KParseTestCodestream(t *testing.T) {
	data, err := os.ReadFile("../../testdata/test.j2k")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	cs, err := parseCodestream(data)
	if err != nil {
		t.Fatalf("parseCodestream: %v", err)
	}
	if cs.xsiz != 1576 || cs.ysiz != 1134 {
		t.Errorf("size = %dx%d, want 1576x1134", cs.xsiz, cs.ysiz)
	}
	if cs.numComps() != 3 {
		t.Errorf("comps = %d, want 3", cs.numComps())
	}
	for i, c := range cs.comps {
		if c.precision != 8 || c.signed {
			t.Errorf("comp %d precision=%d signed=%v, want 8/false", i, c.precision, c.signed)
		}
	}
	if cs.cod.transform != 1 {
		t.Errorf("transform = %d, want 1 (5/3 reversible)", cs.cod.transform)
	}
	if cs.cod.decompLevels != 5 {
		t.Errorf("decompLevels = %d, want 5", cs.cod.decompLevels)
	}
	if cs.cod.cbW != 64 || cs.cod.cbH != 64 {
		t.Errorf("code-block = %dx%d, want 64x64", cs.cod.cbW, cs.cod.cbH)
	}
	if cs.numTilesX() != 1 || cs.numTilesY() != 1 {
		t.Errorf("tiles = %dx%d, want 1x1", cs.numTilesX(), cs.numTilesY())
	}
	if len(cs.tileParts) != 1 {
		t.Errorf("tileParts = %d, want 1", len(cs.tileParts))
	}
	t.Logf("test.j2k: %dx%d %dc p%d tiles=%dx%d levels=%d cb=%dx%d transform=%d layers=%d prog=%d mct=%d tilePart[0]=[%d,%d)",
		cs.xsiz, cs.ysiz, cs.numComps(), cs.comps[0].precision,
		cs.numTilesX(), cs.numTilesY(), cs.cod.decompLevels, cs.cod.cbW, cs.cod.cbH,
		cs.cod.transform, cs.cod.numLayers, cs.cod.progression, cs.cod.mct,
		cs.tileParts[0].dataStart, cs.tileParts[0].dataEnd)
}

func TestGoJ2KParseDICOMFixtures(t *testing.T) {
	for _, path := range []string{
		"../../testdata/cornerstone-CTImage-jpeg2000-lossless.dcm",
		"../../testdata/cornerstone-CTImage-jpeg2000.dcm",
		"../../testdata/pydicom-JPEG2000.dcm",
	} {
		dcm, err := os.ReadFile(path)
		if err != nil {
			t.Logf("%s: unavailable", path)
			continue
		}
		frame := extractFirstJ2KFrame(t, dcm)
		cs, err := parseCodestream(frame)
		if err != nil {
			t.Errorf("%s: parseCodestream: %v", path, err)
			continue
		}
		t.Logf("%-52s %dx%d %dc p%d signed=%v levels=%d cb=%dx%d transform=%d layers=%d mct=%d tiles=%dx%d parts=%d",
			path[13:], cs.xsiz, cs.ysiz, cs.numComps(), cs.comps[0].precision, cs.comps[0].signed,
			cs.cod.decompLevels, cs.cod.cbW, cs.cod.cbH, cs.cod.transform, cs.cod.numLayers, cs.cod.mct,
			cs.numTilesX(), cs.numTilesY(), len(cs.tileParts))
	}
}
