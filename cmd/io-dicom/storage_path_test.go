package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestStoragePathContainsTraversal is the path-traversal regression.
//
// PatientID, StudyInstanceUID, SeriesInstanceUID and SOPInstanceUID all arrive
// in the C-STORE payload from the remote peer, and were passed straight to
// filepath.Join. Join cleans a path but does not contain it, so a peer sending
// PatientID "../../.." wrote a .dcm file outside the datastore — with no
// authentication, against a binary the Makefile installs.
func TestStoragePathContainsTraversal(t *testing.T) {
	root := t.TempDir()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}

	hostile := []struct {
		name                        string
		patient, study, series, sop string
	}{
		{"traversal in PatientID", "../../../../etc", "1.2.3", "1.2.4", "1.2.5"},
		{"traversal in StudyUID", "PID", "../../..", "1.2.4", "1.2.5"},
		{"traversal in SeriesUID", "PID", "1.2.3", "../..", "1.2.5"},
		{"traversal in SOPUID", "PID", "1.2.3", "1.2.4", "../../../evil"},
		{"absolute PatientID", "/etc/cron.d", "1.2.3", "1.2.4", "1.2.5"},
		{"separator in PatientID", "a/b/../../../c", "1.2.3", "1.2.4", "1.2.5"},
		{"dot-dot only", "..", "1.2.3", "1.2.4", "1.2.5"},
		{"backslash separator", `..\..\windows`, "1.2.3", "1.2.4", "1.2.5"},
		{"newline injection", "pid\n../../x", "1.2.3", "1.2.4", "1.2.5"},
		{"nul byte", "pid\x00/../..", "1.2.3", "1.2.4", "1.2.5"},
	}

	for _, tc := range hostile {
		t.Run(tc.name, func(t *testing.T) {
			path, err := storagePath(root, tc.patient, tc.study, tc.series, tc.sop)
			if err != nil {
				return // rejected outright, which is fine
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				t.Fatal(err)
			}
			rel, err := filepath.Rel(absRoot, abs)
			if err != nil {
				t.Fatalf("Rel(%q, %q): %v", absRoot, abs, err)
			}
			if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Fatalf("a peer escaped the datastore: %q resolved to %q (%q outside %q)",
					tc.patient, path, rel, absRoot)
			}
		})
	}
}

// TestStoragePathRejectsEmptyComponents pins that a missing Type-1 attribute
// cannot collapse the hierarchy — without it, an absent SeriesInstanceUID would
// silently write the instance one directory up.
func TestStoragePathRejectsEmptyComponents(t *testing.T) {
	root := t.TempDir()
	cases := []struct{ patient, study, series, sop string }{
		{"", "1.2.3", "1.2.4", "1.2.5"},
		{"PID", "", "1.2.4", "1.2.5"},
		{"PID", "1.2.3", "", "1.2.5"},
		{"PID", "1.2.3", "1.2.4", ""},
		{"   ", "1.2.3", "1.2.4", "1.2.5"},
		{".", "1.2.3", "1.2.4", "1.2.5"},
	}
	for _, tc := range cases {
		if p, err := storagePath(root, tc.patient, tc.study, tc.series, tc.sop); err == nil {
			t.Errorf("storagePath(%q,%q,%q,%q) = %q, want an error",
				tc.patient, tc.study, tc.series, tc.sop, p)
		}
	}
}

// TestStoragePathKeepsOrdinaryValues guards the sanitiser from mangling the
// normal case: real UIDs are dotted digits and must survive untouched.
func TestStoragePathKeepsOrdinaryValues(t *testing.T) {
	root := t.TempDir()
	path, err := storagePath(root, "PAT-001", "1.2.840.113619.2.55.3", "1.2.840.113619.2.55.4", "1.2.840.113619.2.55.5")
	if err != nil {
		t.Fatalf("storagePath on ordinary values: %v", err)
	}
	want := filepath.Join(root, "PAT-001", "1.2.840.113619.2.55.3",
		"1.2.840.113619.2.55.4", "1.2.840.113619.2.55.5.dcm")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

// TestSafeComponentReplacesSeparators pins the element-level guarantee the
// containment check is layered on top of.
func TestSafeComponentReplacesSeparators(t *testing.T) {
	for _, in := range []string{"a/b", `a\b`, "a:b", "a b", "a\tb", "a\x00b"} {
		got := safeComponent(in)
		if strings.ContainsAny(got, `/\`) {
			t.Errorf("safeComponent(%q) = %q, still contains a separator", in, got)
		}
	}
	if got := safeComponent(strings.Repeat("x", 200)); len(got) > 64 {
		t.Errorf("safeComponent did not bound length: got %d", len(got))
	}
}
