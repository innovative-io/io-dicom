package jpeg2000

import (
	"math/rand"
	"testing"
)

// TestMQRoundTrip encodes random context-coded bits with the MQ encoder and
// verifies the MQ decoder recovers them exactly (shared Qe table / context model).
func TestMQRoundTrip(t *testing.T) {
	for seed := int64(0); seed < 50; seed++ {
		r := rand.New(rand.NewSource(seed))
		n := 1 + r.Intn(4000)
		const numCtx = 19
		type ev struct{ ctx, bit int }
		evs := make([]ev, n)
		for i := range evs {
			evs[i] = ev{ctx: r.Intn(numCtx), bit: r.Intn(2)}
		}
		// encode
		enc := newMQEncoder()
		ectx := make([]mqContext, numCtx)
		for i := range evs {
			enc.encode(&ectx[evs[i].ctx], evs[i].bit)
		}
		data := enc.flush()
		// decode
		dec := newMQDecoder(data, 0, len(data))
		dctx := make([]mqContext, numCtx)
		for i := range evs {
			got := dec.decode(&dctx[evs[i].ctx])
			if got != evs[i].bit {
				t.Fatalf("seed=%d ev=%d ctx=%d: got %d want %d (n=%d, %d bytes)",
					seed, i, evs[i].ctx, got, evs[i].bit, n, len(data))
			}
		}
	}
}
