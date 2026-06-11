package gojxl

import (
	"encoding/hex"
	"testing"
)

// TestDecodeJPEGData parses a real `jbrd` box and checks that the recovered
// JPEG reconstruction metadata matches the source baseline JPEG. It exercises
// the bit-coded field decoder and the embedded Brotli stream (APP marker data).
func TestDecodeJPEGData(t *testing.T) {
	box, err := hex.DecodeString(jbrdVectorHex)
	if err != nil {
		t.Fatalf("bad jbrd vector: %v", err)
	}
	jd, err := decodeJPEGData(box)
	if err != nil {
		t.Fatalf("decodeJPEGData: %v", err)
	}

	wantMarkers := []uint8{0xe0, 0xdb, 0xdb, 0xc0, 0xc4, 0xc4, 0xc4, 0xc4, 0xda, 0xd9}
	if len(jd.markerOrder) != len(wantMarkers) {
		t.Fatalf("markerOrder = % x, want % x", jd.markerOrder, wantMarkers)
	}
	for i, m := range wantMarkers {
		if jd.markerOrder[i] != m {
			t.Fatalf("markerOrder[%d] = %#x, want %#x", i, jd.markerOrder[i], m)
		}
	}

	if len(jd.components) != 3 || len(jd.quant) != 2 || len(jd.huffmanCode) != 4 || len(jd.scanInfo) != 1 {
		t.Fatalf("structure mismatch: comps=%d quant=%d huff=%d scans=%d",
			len(jd.components), len(jd.quant), len(jd.huffmanCode), len(jd.scanInfo))
	}

	// YCbCr component layout: ids 1,2,3 with quant tables 0,1,1.
	wantQuant := []uint32{0, 1, 1}
	for i, c := range jd.components {
		if c.id != uint32(i+1) || c.quantIdx != wantQuant[i] {
			t.Errorf("comp[%d] id=%d quant=%d, want id=%d quant=%d", i, c.id, c.quantIdx, i+1, wantQuant[i])
		}
	}

	// Standard DC/AC luma/chroma Huffman slot ids.
	wantSlots := []int{0x00, 0x10, 0x01, 0x11}
	for i, h := range jd.huffmanCode {
		if h.slotID != wantSlots[i] {
			t.Errorf("huff[%d] slotID=%#x, want %#x", i, h.slotID, wantSlots[i])
		}
		if !h.isLast {
			t.Errorf("huff[%d] isLast=false", i)
		}
	}

	sc := jd.scanInfo[0]
	if sc.numComponents != 3 || sc.Ss != 0 || sc.Se != 63 || sc.Ah != 0 || sc.Al != 0 {
		t.Errorf("scan = %+v, want baseline 3-component Ss=0 Se=63", sc)
	}

	// The single APP marker is the JFIF APP0 segment, recovered from the Brotli
	// stream: 0xE0, length 0x0010, "JFIF\0".
	if len(jd.appData) != 1 || jd.appMarkerType[0] != appMarkerUnknown {
		t.Fatalf("appData=%d type=%v", len(jd.appData), jd.appMarkerType)
	}
	app := jd.appData[0]
	if len(app) != 17 || app[0] != 0xE0 || app[1] != 0x00 || app[2] != 0x10 ||
		string(app[3:8]) != "JFIF\x00" {
		t.Errorf("APP0 = % x, want JFIF segment", app)
	}
}
