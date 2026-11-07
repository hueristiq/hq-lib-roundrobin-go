package roundrobin

// item is a single element of a [RoundRobin] cycle, pairing a value with its
// selection weight and smooth weighted selection state. Items are handled
// through pointers so slice moves do not copy mutable selection state.
//
// Fields:
//   - value (T): value handed back by [RoundRobin.Next].
//   - weight (int): relative selection weight; always at least 1.
//   - currentWeight (int64): smooth weighted round-robin state, mutated on
//     every weighted selection while holding [RoundRobin.mu].
type item[T comparable] struct {
	value         T
	weight        int
	currentWeight int64
}
