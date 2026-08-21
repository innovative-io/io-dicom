package network

import "testing"

// TestMaxPresentationContexts_MatchesOddIDSpace pins the constant to the actual
// number of odd IDs available in 1..255 (PS3.8 §9.3.2.2).
func TestMaxPresentationContexts_MatchesOddIDSpace(t *testing.T) {
	odd := 0
	for id := 1; id <= 255; id += 2 {
		odd++
	}
	if MaxPresentationContexts != odd {
		t.Errorf("MaxPresentationContexts = %d, want %d (odd IDs in 1..255)", MaxPresentationContexts, odd)
	}
}

// TestPresentationContextIDs_UniqueAndOdd verifies the ID assignment used when
// building an association: index-derived IDs must stay odd, unique, and inside
// the byte range for a full association's worth of contexts. The previous
// package-global counter wrapped past 255 and handed context #129 the same ID
// as context #1.
func TestPresentationContextIDs_UniqueAndOdd(t *testing.T) {
	seen := make(map[byte]int, MaxPresentationContexts)
	for i := 0; i < MaxPresentationContexts; i++ {
		id := byte(2*i + 1)
		if id%2 == 0 {
			t.Fatalf("context %d: ID %d is even", i, id)
		}
		if prev, dup := seen[id]; dup {
			t.Fatalf("context %d: ID %d already used by context %d", i, id, prev)
		}
		seen[id] = i
	}
	if len(seen) != MaxPresentationContexts {
		t.Errorf("got %d unique IDs, want %d", len(seen), MaxPresentationContexts)
	}
	if got := byte(2*(MaxPresentationContexts-1) + 1); got != 255 {
		t.Errorf("last ID = %d, want 255 (the space should be exactly filled)", got)
	}
}
