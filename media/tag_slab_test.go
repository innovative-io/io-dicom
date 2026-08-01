package media

import "testing"

// TestTagSlabCrossesChunkBoundary is the core safety property: a full slab is
// never grown in place (that would relocate earlier tags and dangle every
// pointer already handed out). Allocating more than one chunk's worth of tags
// and then reading them all back must return the exact values written.
func TestTagSlabCrossesChunkBoundary(t *testing.T) {
	const n = tagSlabChunk*3 + 7 // spans four chunks, partially filling the last
	buf := &DICOMBuffer{}
	got := make([]*DICOMTag, n)
	for i := 0; i < n; i++ {
		tag := buf.newTag()
		tag.Group = uint16(0x7000 + i)
		tag.Element = uint16(i)
		got[i] = tag
	}
	// Every earlier pointer must still hold its own value after later chunks
	// were allocated — proving no in-place grow relocated them.
	for i := 0; i < n; i++ {
		if got[i].Group != uint16(0x7000+i) || got[i].Element != uint16(i) {
			t.Fatalf("tag %d corrupted: got (%#04x,%#04x), want (%#04x,%#04x) — a "+
				"slab grow must have relocated earlier tags",
				i, got[i].Group, got[i].Element, uint16(0x7000+i), uint16(i))
		}
	}
	// Distinct pointers throughout.
	for i := 1; i < n; i++ {
		if got[i] == got[i-1] {
			t.Fatalf("tags %d and %d share a pointer", i-1, i)
		}
	}
}

// TestTagSlabReleaseReuseIsClean pins that releasing the most-recent tag rewinds
// the slab and that the reused slot comes back zeroed — otherwise a failed read
// would leak its stale fields into the next tag handed out.
func TestTagSlabReleaseReuseIsClean(t *testing.T) {
	buf := &DICOMBuffer{}
	first := buf.newTag()
	first.Group = 0x0010
	first.Element = 0x0020
	first.VR = "PN"
	first.Length = 42
	first.Name = "PatientName"
	first.Data = []byte("stale")

	buf.releaseTag(first)

	reused := buf.newTag()
	if reused != first {
		t.Fatalf("release did not rewind: expected the same slot back")
	}
	if reused.Group != 0 || reused.Element != 0 || reused.VR != "" ||
		reused.Length != 0 || reused.Name != "" || reused.Data != nil {
		t.Fatalf("reused slot carried stale fields: %+v", *reused)
	}
}

// TestTagSlabReleaseNonLatestIsNoop guards the rewind guard: releasing anything
// other than the last-handed-out tag must not corrupt the index or reclaim a
// live tag's slot.
func TestTagSlabReleaseNonLatestIsNoop(t *testing.T) {
	buf := &DICOMBuffer{}
	a := buf.newTag()
	a.Group = 0x1111
	b := buf.newTag()
	b.Group = 0x2222

	buf.releaseTag(a) // not the latest — must be ignored

	c := buf.newTag()
	if c == a || c == b {
		t.Fatalf("releasing a non-latest tag wrongly reclaimed a live slot")
	}
	if a.Group != 0x1111 || b.Group != 0x2222 {
		t.Fatalf("live tags were mutated by a no-op release")
	}
}
