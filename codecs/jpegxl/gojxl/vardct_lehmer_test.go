package gojxl

import "testing"

// TestLehmerRoundTrip verifies decodeLehmerCode inverts computeLehmerCode for a
// range of permutations and sizes.
func TestLehmerRoundTrip(t *testing.T) {
	perms := [][]uint32{
		{0},
		{0, 1, 2, 3},          // identity
		{3, 2, 1, 0},          // reverse
		{1, 0, 3, 2},          // swaps
		{2, 0, 3, 1, 4},       // size 5 (non power of two)
		{5, 3, 0, 6, 1, 4, 2}, // size 7
	}
	for _, p := range perms {
		n := len(p)
		code := computeLehmerCode(p, n)
		got := decodeLehmerCode(code, n)
		for i := range p {
			if got[i] != p[i] {
				t.Fatalf("perm %v: decode gave %v", p, got)
			}
		}
	}
}

// TestLehmerIdentity: an all-zero Lehmer code decodes to the identity.
func TestLehmerIdentity(t *testing.T) {
	n := 16
	code := make([]uint32, n)
	got := decodeLehmerCode(code, n)
	for i := 0; i < n; i++ {
		if got[i] != uint32(i) {
			t.Fatalf("identity: pos %d = %d", i, got[i])
		}
	}
}

// TestLehmerLargerPermutation exercises a 64-element permutation (8x8 block).
func TestLehmerLargerPermutation(t *testing.T) {
	n := 64
	p := make([]uint32, n)
	for i := range p {
		p[i] = uint32((i*37 + 11) % n) // a bijection since gcd(37,64)=1
	}
	// confirm bijection
	seen := make([]bool, n)
	for _, v := range p {
		if seen[v] {
			t.Fatal("test perm not a bijection")
		}
		seen[v] = true
	}
	code := computeLehmerCode(p, n)
	got := decodeLehmerCode(code, n)
	for i := range p {
		if got[i] != p[i] {
			t.Fatalf("pos %d: got %d want %d", i, got[i], p[i])
		}
	}
}

func TestCoeffOrderContext(t *testing.T) {
	// token = (val==0)?0 : 1+floor(log2(val)); clamped to 7.
	cases := []struct{ val, want uint32 }{
		{0, 0},
		{1, 1},       // 1+0
		{2, 2},       // 1+1
		{3, 2},       // 1+1
		{4, 3},       // 1+2
		{255, 8 - 1}, // 1+7=8 -> clamp 7
		{100000, 7},  // clamp
	}
	for _, c := range cases {
		if got := coeffOrderContext(c.val); got != c.want {
			t.Errorf("coeffOrderContext(%d)=%d, want %d", c.val, got, c.want)
		}
	}
}

func TestStrategyOrderRange(t *testing.T) {
	for s := 0; s < acNumValidStrategies; s++ {
		if kStrategyOrder[s] >= kNumOrders {
			t.Errorf("strategy %d order %d >= kNumOrders", s, kStrategyOrder[s])
		}
	}
}
