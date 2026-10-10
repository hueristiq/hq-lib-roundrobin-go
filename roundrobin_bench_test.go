package roundrobin_test

import (
	"strconv"
	"testing"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

// nextSink keeps benchmark results alive so the compiler cannot optimise the
// measured calls away.
var nextSink string

// newStringRoundRobin builds a round-robin of n distinct values named after
// their index, failing the benchmark when construction fails.
//
// Parameters:
//   - b (*testing.B): the benchmark to fail on error.
//   - n (int): the number of items in the cycle.
//
// Returns:
//   - rr (*hqgoroundrobin.RoundRobin[string]): the constructed cycle.
func newStringRoundRobin(b *testing.B, n int) (rr *hqgoroundrobin.RoundRobin[string]) {
	b.Helper()

	items := make([]string, 0, n)

	for i := range n {
		items = append(items, "server-"+strconv.Itoa(i))
	}

	rr, err := hqgoroundrobin.New(items)
	if err != nil {
		b.Fatalf("creating round-robin: %v", err)
	}

	return
}

// BenchmarkRoundRobinNext measures the cost of serving the next item from a
// moderately sized cycle, comparing strict rotation against a weighted cycle
// where selection runs a smooth weighted round-robin step.
func BenchmarkRoundRobinNext(b *testing.B) {
	cases := []struct {
		name    string
		prepare func(rr *hqgoroundrobin.RoundRobin[string])
	}{
		{name: "unweighted", prepare: func(_ *hqgoroundrobin.RoundRobin[string]) {}},
		{name: "weighted", prepare: func(rr *hqgoroundrobin.RoundRobin[string]) {
			rr.AddWithWeight("server-0", 3)
			rr.AddWithWeight("server-1", 2)
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			rr := newStringRoundRobin(b, 100)

			tc.prepare(rr)

			b.ReportAllocs()

			var next string

			for b.Loop() {
				next = rr.Next()
			}

			nextSink = next
		})
	}
}

// BenchmarkRoundRobinNextParallel measures the throughput of serving the next
// item under contention, with many goroutines hammering a shared cycle.
func BenchmarkRoundRobinNextParallel(b *testing.B) {
	rr := newStringRoundRobin(b, 100)

	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rr.Next()
		}
	})
}
