package codecctx

import (
	"context"
	"testing"
)

func TestThreadsDefaultsToAuto(t *testing.T) {
	if got := Threads(context.Background()); got != 0 {
		t.Fatalf("Threads on a bare context = %d, want 0 (auto)", got)
	}
	//nolint:staticcheck // intentionally passing a nil context to test the guard
	if got := Threads(nil); got != 0 {
		t.Fatalf("Threads(nil) = %d, want 0", got)
	}
}

func TestWithThreadsRoundTrip(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{in: 1, want: 1},
		{in: 4, want: 4},
		{in: 0, want: 0},
		{in: -3, want: 0}, // negative is clamped to auto
	}
	for _, tc := range cases {
		ctx := WithThreads(context.Background(), tc.in)
		if got := Threads(ctx); got != tc.want {
			t.Fatalf("Threads(WithThreads(ctx, %d)) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
