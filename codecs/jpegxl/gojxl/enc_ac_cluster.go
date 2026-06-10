package gojxl

import "math"

// maxACClusters bounds the number of distinct ANS histograms the AC contexts are
// clustered into. More clusters model the coefficient distribution better but
// cost more histogram-header bits; this is a reasonable trade for photographic
// content and stays well under kMaxClusters (256).
const maxACClusters = 64

type symCount struct {
	sym   uint32
	count int64
}

// clusterContexts groups the used raw contexts (by their token-frequency tables)
// into at most maxClusters clusters using a cross-entropy (compression-cost)
// nearest-centroid assignment with a few Lloyd refinement passes. It returns the
// full context->cluster map (length numContexts; unused contexts map to cluster
// 0), the per-cluster normalized histograms, and the cluster count.
func clusterContexts(used []int, ctxSparse map[int][]symCount, ctxTotal map[int]int64, numContexts, alphabet, maxClusters int) ([]uint8, [][]int32, int) {
	ctxToCluster := make([]uint8, numContexts)
	if len(used) == 0 {
		return ctxToCluster, [][]int32{createFlatHistogram(1, ansTabSize)}, 1
	}

	// Seed clusters with the highest-count contexts (descending total).
	order := append([]int(nil), used...)
	insertionSortByTotalDesc(order, ctxTotal)
	k := maxClusters
	if k > len(order) {
		k = len(order)
	}
	centroid := make([][]int64, k)
	for c := 0; c < k; c++ {
		centroid[c] = make([]int64, alphabet)
		for _, sc := range ctxSparse[order[c]] {
			centroid[c][sc.sym] += sc.count
		}
	}

	assign := make([]int, len(order))
	for iter := 0; iter < 4; iter++ {
		// Precompute log2 probabilities (Laplace-smoothed) per centroid.
		logp := make([][]float64, len(centroid))
		for c := range centroid {
			var total int64
			for _, v := range centroid[c] {
				total += v
			}
			denom := math.Log2(float64(total + int64(alphabet)))
			lp := make([]float64, alphabet)
			for s := 0; s < alphabet; s++ {
				lp[s] = math.Log2(float64(centroid[c][s]+1)) - denom
			}
			logp[c] = lp
		}
		// Assign each context to the centroid that codes it cheapest.
		for i, ctx := range order {
			best, bestCost := 0, math.Inf(1)
			sp := ctxSparse[ctx]
			for c := range centroid {
				cost := 0.0
				lp := logp[c]
				for _, sc := range sp {
					cost -= float64(sc.count) * lp[sc.sym]
				}
				if cost < bestCost {
					bestCost, best = cost, c
				}
			}
			assign[i] = best
		}
		// Recompute centroids from the assignment.
		for c := range centroid {
			for s := range centroid[c] {
				centroid[c][s] = 0
			}
		}
		for i, ctx := range order {
			cc := centroid[assign[i]]
			for _, sc := range ctxSparse[ctx] {
				cc[sc.sym] += sc.count
			}
		}
	}

	// Compact non-empty clusters to a dense id range and build histograms.
	remap := make([]int, len(centroid))
	for i := range remap {
		remap[i] = -1
	}
	var counts [][]int32
	for c := range centroid {
		var total int64
		for _, v := range centroid[c] {
			total += v
		}
		if total == 0 {
			continue
		}
		remap[c] = len(counts)
		freq := make([]int64, alphabet)
		copy(freq, centroid[c])
		counts = append(counts, normalizeHistogram(freq))
	}
	if len(counts) == 0 { // defensive: keep at least one histogram
		counts = append(counts, createFlatHistogram(1, ansTabSize))
		for i := range remap {
			remap[i] = 0
		}
	}
	for i, ctx := range order {
		ctxToCluster[ctx] = uint8(remap[assign[i]])
	}
	return ctxToCluster, counts, len(counts)
}

func insertionSortByTotalDesc(order []int, total map[int]int64) {
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && total[order[j]] > total[order[j-1]]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
}

// writeACEntropyHeader builds clustered AC histograms from the per-group
// tokenized coefficients, writes the entropy-code header (lz77 off, an
// entropy-coded context map, then one uint config + one histogram per cluster),
// and returns the context->cluster map and per-cluster alias tables for encoding
// the rANS data.
func writeACEntropyHeader(w *bitWriter, perGroupAC [][]encTokenized, alphabet, numContexts int) (ctxToCluster []uint8, clusterRev [][][]uint16, clusterFreqs []([]uint16)) {
	// Gather per-context sparse frequency tables across all groups.
	ctxSparseMap := map[int]map[uint32]int64{}
	ctxTotal := map[int]int64{}
	for _, g := range perGroupAC {
		for _, t := range g {
			m := ctxSparseMap[t.ctx]
			if m == nil {
				m = map[uint32]int64{}
				ctxSparseMap[t.ctx] = m
			}
			m[t.token]++
			ctxTotal[t.ctx]++
		}
	}
	used := make([]int, 0, len(ctxSparseMap))
	ctxSparse := make(map[int][]symCount, len(ctxSparseMap))
	var totalTokens int64
	for ctx, m := range ctxSparseMap {
		used = append(used, ctx)
		sp := make([]symCount, 0, len(m))
		for sym, cnt := range m {
			sp = append(sp, symCount{sym, cnt})
		}
		ctxSparse[ctx] = sp
		totalTokens += ctxTotal[ctx]
	}

	// Each extra cluster costs a histogram header; only worthwhile when there are
	// enough tokens to amortize it. Scale the cluster cap with the token volume so
	// small images don't pay header overhead that exceeds the coding savings.
	cap := int(totalTokens / 3000)
	if cap < 1 {
		cap = 1
	}
	if cap > maxACClusters {
		cap = maxACClusters
	}
	ctxToCluster, clusterCounts, numClusters := clusterContexts(used, ctxSparse, ctxTotal, numContexts, alphabet, cap)

	clusterRev = make([][][]uint16, numClusters)
	clusterFreqs = make([][]uint16, numClusters)
	for c := 0; c < numClusters; c++ {
		_, rev, fr := reverseAliasMap(clusterCounts[c], encLogAlpha)
		clusterRev[c], clusterFreqs[c] = rev, fr
	}

	w.WriteBool(false) // lz77 disabled
	writeContextMap(w, ctxToCluster, numClusters)
	w.WriteBits(0, 1)                     // use_prefix_code = 0 (ANS)
	w.WriteBits(uint64(encLogAlpha-5), 2) // log_alpha_size
	for c := 0; c < numClusters; c++ {
		writeUintConfig(w, encLogAlpha, encUintConfig)
	}
	for c := 0; c < numClusters; c++ {
		writeANSHistogram(w, clusterCounts[c])
	}
	return ctxToCluster, clusterRev, clusterFreqs
}

// encodeACDataMulti rANS-codes the AC tokens of one group (backward), selecting
// each token's histogram via its context's cluster, and writes the 32-bit final
// state followed by the bit chunks in decode order. It is the multi-histogram
// counterpart of encodeANSData.
func encodeACDataMulti(w *bitWriter, tokens []encTokenized, ctxToCluster []uint8, clusterRev [][][]uint16, clusterFreqs [][]uint16) {
	type chunk struct {
		bits  uint64
		nbits int
	}
	var out []chunk
	state := uint32(ansSignature << 16)
	for i := len(tokens) - 1; i >= 0; i-- {
		t := tokens[i]
		cl := ctxToCluster[t.ctx]
		out = append(out, chunk{uint64(t.extra), t.nbits})
		freq := uint32(clusterFreqs[cl][t.token])
		var ansBits uint64
		ansN := 0
		if (state >> (32 - ansLogTabSize)) >= freq {
			ansBits = uint64(state & 0xFFFF)
			state >>= 16
			ansN = 16
		}
		res := clusterRev[cl][t.token][state%freq]
		state = ((state / freq) << ansLogTabSize) | uint32(res)
		out = append(out, chunk{ansBits, ansN})
	}
	w.WriteBits(uint64(state), 32)
	for i := len(out) - 1; i >= 0; i-- {
		w.WriteBits(out[i].bits, out[i].nbits)
	}
}
