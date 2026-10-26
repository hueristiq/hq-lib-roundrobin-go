// Package roundrobin provides a generic, concurrency-safe round-robin selector:
// it cycles through a set of items, handing out one per call, optionally serving
// some items more often than others through weights.
//
// It suits work that spreads load across a fixed or slowly changing set of
// candidates — upstream servers in a load balancer, worker queues, API endpoints,
// shards — where each request should go to the next item in line, or to items in
// proportion to their capacity.
//
// # Entry points
//
// [New] builds a [RoundRobin] over an initial set of items; at least one item is
// required, or [New] fails with [ErrNoItems]. [RoundRobin.Next] hands out the
// next item, following the selection policy described below. Items can be added
// with [RoundRobin.Add] and [RoundRobin.AddWithWeight], removed with
// [RoundRobin.Remove], and inspected with [RoundRobin.Items], [RoundRobin.Len],
// and [RoundRobin.StatisticsOf].
//
// Selection order depends on the weights: while every item carries the default
// weight, the cycle is strict rotation — items are served one after another in
// insertion order, wrapping around. As soon as an item carries a different weight,
// selection switches to smooth weighted round-robin (the nginx algorithm): in the
// steady state each item is picked in proportion to its weight, with its picks
// spread across the cycle instead of bunched into a run. Once every item carries
// the default weight again, selection returns to strict rotation.
//
// # Configuration
//
// Behavior is set through functional options passed to [New]: [WithRotateAmount]
// controls how many times in a row one item is served before the selection
// advances to the next item. Unset, or set to a non-positive value, it falls back
// to [DefaultRotateAmount].
//
// # Concurrency
//
// Every method is safe for concurrent use: all mutable state is guarded by a
// single mutex, and per-item statistics counters are updated and read atomically.
//
// The implementation relies on the Go standard library only.
package roundrobin
