package roundrobin_test

import (
	"strconv"
	"testing"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

var nextSink string

func newStringRoundRobin(b *testing.B, n int) (rr *hqgoroundrobin.RoundRobin[string]) {
	b.Helper()

	items := make([]string, 0, n)

	for i := range n {
		items = append(items, "server-"+strconv.Itoa(i))
	}

	rr = hqgoroundrobin.New(items...)

	return
}

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

			for range b.N {
				next, _ = rr.Next()
			}

			nextSink = next
		})
	}
}

func BenchmarkRoundRobinNextParallel(b *testing.B) {
	rr := newStringRoundRobin(b, 100)

	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = rr.Next()
		}
	})
}
