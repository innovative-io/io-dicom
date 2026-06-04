// Package codecctx carries per-call codec hints on a context.Context.
//
// The main hint today is the intra-frame thread count. When a caller decodes
// or encodes many independent frames concurrently (one goroutine per frame),
// running each codec call single-threaded gives better total throughput than
// letting every call spin up all cores and oversubscribe. Such a caller sets
// the thread count to 1 here; the native backends read it and size their
// thread pool accordingly (and skip pool creation entirely when it is 1).
package codecctx

import "context"

type threadsKey struct{}

// WithThreads returns a context that requests the native codec backends use n
// intra-frame worker threads. n <= 0 means "auto" (use all available cores),
// which is the default when the hint is absent. n == 1 means single-threaded.
func WithThreads(ctx context.Context, n int) context.Context {
	if n < 0 {
		n = 0
	}
	return context.WithValue(ctx, threadsKey{}, n)
}

// Threads returns the requested intra-frame thread count, or 0 ("auto") when
// the hint is absent.
func Threads(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	if n, ok := ctx.Value(threadsKey{}).(int); ok {
		return n
	}
	return 0
}
