package gojxl

import (
	"math/rand"
	"testing"
)

// TestMoveToFrontRoundTrip checks the forward MTF transform is the exact inverse
// of inverseMoveToFront (used to (de)serialize context maps).
func TestMoveToFrontRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for trial := 0; trial < 2000; trial++ {
		n := 1 + rng.Intn(300)
		orig := make([]uint8, n)
		for i := range orig {
			orig[i] = uint8(rng.Intn(20))
		}
		enc := append([]uint8(nil), orig...)
		moveToFront(enc)
		inverseMoveToFront(enc)
		for i := range orig {
			if enc[i] != orig[i] {
				t.Fatalf("trial %d: MTF round-trip differs at %d: %d != %d", trial, i, enc[i], orig[i])
			}
		}
	}
}

// TestContextMapRoundTrip checks writeContextMap is the inverse of
// decodeContextMap for both the trivial single-cluster form and entropy-coded
// multi-cluster maps over a range of cluster counts and sizes.
func TestContextMapRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for trial := 0; trial < 3000; trial++ {
		numClusters := 1 + rng.Intn(40)
		n := numClusters + rng.Intn(400)
		ctxMap := make([]uint8, n)
		// Ensure every cluster id in [0,numClusters) appears (decodeContextMap
		// requires a complete map) and the rest are random.
		for i := 0; i < numClusters; i++ {
			ctxMap[i] = uint8(i)
		}
		for i := numClusters; i < n; i++ {
			ctxMap[i] = uint8(rng.Intn(numClusters))
		}
		rng.Shuffle(n, func(i, j int) { ctxMap[i], ctxMap[j] = ctxMap[j], ctxMap[i] })

		w := newBitWriter()
		writeContextMap(w, ctxMap, numClusters)
		w.ZeroPadToByte()

		got := make([]uint8, n)
		nh, err := decodeContextMap(got, newBitReader(w.Bytes()))
		if err != nil {
			t.Fatalf("trial %d (clusters=%d, n=%d): decodeContextMap: %v", trial, numClusters, n, err)
		}
		if nh != numClusters {
			t.Fatalf("trial %d: numHistograms %d, want %d", trial, nh, numClusters)
		}
		for i := range ctxMap {
			if got[i] != ctxMap[i] {
				t.Fatalf("trial %d: context map differs at %d: %d != %d", trial, i, got[i], ctxMap[i])
			}
		}
	}
}
