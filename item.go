package roundrobin

// item is a single element of a [RoundRobin] cycle, pairing a value with its
// selection weight, its smooth weighted selection state, and its usage
// statistics. Items are always handled through pointers so an item's statistics
// stay reachable through [RoundRobin.StatisticsOf] regardless of slice moves or
// removal.
//
// Fields:
//   - value (T): value handed back by [RoundRobin.Next].
//   - weight (int): relative selection weight; always at least 1.
//   - currentWeight (int64): smooth weighted round-robin state, mutated on
//     every weighted selection while holding [RoundRobin.mu].
//   - statistics (Statistics): per-item usage metrics, updated atomically.
type item[T comparable] struct {
	value         T
	weight        int
	currentWeight int64
	statistics    Statistics
}
