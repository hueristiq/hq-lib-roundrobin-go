package roundrobin

import (
	"slices"
	"sync"
)

// RoundRobin is a concurrency-safe round-robin selector over a set of items of
// type T: every call to [RoundRobin.Next] serves one item, cycling through the
// whole set, optionally in proportion to per-item weights.
//
// T must be comparable: values double as map keys, which is what deduplication
// on add, removal by value, and statistics lookup by value are built on.
//
// Selection order depends on the weights. While every item carries the default
// weight, the cycle is strict rotation in insertion order. Once an item carries
// a different weight (see [RoundRobin.AddWithWeight]), selection switches to
// smooth weighted round-robin: in the steady state each item is picked in
// proportion to its weight, with its picks spread across the cycle. Once every
// item carries the default weight again, selection returns to strict rotation.
// Removing items perturbs the order of the very next picks, but the long-run
// distribution stays proportional to the weights.
//
// The zero value is not usable; construct through [New] or [NewWeighted]. A
// RoundRobin must not be copied after first use.
//
// Type Parameters:
//   - T: the type of the items in the cycle; must be comparable.
//
// Fields:
//   - mu (sync.Mutex): guards every field below.
//   - items ([]*item[T]): items in the cycle, in insertion order.
//   - index (map[T]*item[T]): items keyed by value, for deduplication, removal,
//     and statistics lookup.
//   - current (int): index into items of the item being served; -1 before the
//     first serve.
//   - serves (int): consecutive serves of the current item; advances at
//     rotateAmount.
//   - rotateAmount (int): consecutive serves of one item before advancing; at
//     least 1.
//   - totalWeight (int64): sum of the weights of all items in the cycle.
//   - weighted (bool): whether any item currently carries a non-default weight.
type RoundRobin[T comparable] struct {
	mu           sync.Mutex
	items        []*item[T]
	index        map[T]*item[T]
	current      int
	serves       int
	rotateAmount int
	totalWeight  int64
	weighted     bool
}

// Add appends items to the cycle, each with [DefaultWeight]. Values already
// in the cycle are skipped: their weight and statistics are preserved.
//
// Parameters:
//   - items (...T): the values to add.
func (r *RoundRobin[T]) Add(items ...T) {
	r.mu.Lock()

	defer r.mu.Unlock()

	r.addLocked(items...)
}

// addLocked appends items to the cycle with [DefaultWeight], skipping values
// already in the cycle. The caller must hold [RoundRobin.mu].
//
// Parameters:
//   - items (...T): the values to add.
func (r *RoundRobin[T]) addLocked(items ...T) {
	for _, value := range items {
		if _, exists := r.index[value]; exists {
			continue
		}

		it := &item[T]{
			value:  value,
			weight: DefaultWeight,
		}

		r.items = append(r.items, it)
		r.index[value] = it
		r.totalWeight += DefaultWeight
	}
}

// AddWithWeight adds a single item with the given selection weight. While any
// item carries a non-default weight, selection runs smooth weighted
// round-robin: in the steady state the item is picked in proportion to its
// weight, with its picks spread across the cycle. Once every item carries the
// default weight again, selection returns to strict rotation.
//
// Re-adding a value already in the cycle updates its weight and preserves its
// statistics. A non-positive weight falls back to [DefaultWeight].
//
// Parameters:
//   - value (T): the value to add.
//   - weight (int): the relative selection weight of the item.
func (r *RoundRobin[T]) AddWithWeight(value T, weight int) {
	r.mu.Lock()

	defer r.mu.Unlock()

	r.addWithWeightLocked(value, weight)
}

// addWithWeightLocked adds value with weight to the cycle, updating the weight
// of a value already in the cycle and recomputing the weighted flag. The caller
// must hold [RoundRobin.mu].
//
// Parameters:
//   - value (T): the value to add.
//   - weight (int): the relative selection weight of the item.
func (r *RoundRobin[T]) addWithWeightLocked(value T, weight int) {
	if weight <= 0 {
		weight = DefaultWeight
	}

	if it, exists := r.index[value]; exists {
		r.totalWeight += int64(weight - it.weight)

		it.weight = weight
	} else {
		it := &item[T]{
			value:  value,
			weight: weight,
		}

		r.items = append(r.items, it)
		r.index[value] = it
		r.totalWeight += int64(weight)
	}

	r.updateWeighted()
}

// SetRotateAmount sets how many times in a row one item is served before the
// selection advances to the next item. A non-positive value falls back to
// [DefaultRotateAmount].
//
// Parameters:
//   - rotateAmount (int): the number of consecutive serves per item.
func (r *RoundRobin[T]) SetRotateAmount(rotateAmount int) {
	if rotateAmount <= 0 {
		rotateAmount = DefaultRotateAmount
	}

	r.mu.Lock()

	defer r.mu.Unlock()

	r.rotateAmount = rotateAmount
}

// updateWeighted recomputes the weighted flag from the current item weights, so
// selection falls back to strict rotation once every item carries the default
// weight again. The caller must hold [RoundRobin.mu].
func (r *RoundRobin[T]) updateWeighted() {
	weighted := false

	for _, it := range r.items {
		if it.weight != DefaultWeight {
			weighted = true

			break
		}
	}

	r.weighted = weighted
}

// Remove drops value from the cycle; removing a value that is not in the cycle
// is a no-op. When the removed item was the one being served, the next
// [RoundRobin.Next] starts a fresh run on the item that follows it in the cycle.
// The statistics of a removed item stay readable through a *Statistics obtained
// earlier, but no longer change.
//
// Parameters:
//   - value (T): the value to remove.
//
// Returns:
//   - removed (bool): true when the value was in the cycle.
func (r *RoundRobin[T]) Remove(value T) (removed bool) {
	r.mu.Lock()

	defer r.mu.Unlock()

	it, exists := r.index[value]
	if !exists {
		return false
	}

	delete(r.index, value)

	r.totalWeight -= int64(it.weight)

	i := slices.Index(r.items, it)

	r.items = slices.Delete(r.items, i, i+1)

	// Keep current pointing at the element it served before the shift; when the
	// removed item was being served, step back and start a fresh run so the next
	// advance lands on the removed item's successor (-1 is valid and wraps to
	// the head of the cycle).
	if i <= r.current {
		if i == r.current {
			r.serves = 0
		}

		r.current--
	}

	r.updateWeighted()

	return true
}

// Next serves the next item in the cycle: the current item is served until it
// has been served rotateAmount times in a row, then the selection advances —
// stepping cyclically while all weights are equal, or running one smooth
// weighted round-robin step otherwise. The served item's serve count is
// incremented.
//
// Calling Next on an empty cycle — possible after every item was removed with
// [RoundRobin.Remove], or when the cycle was created empty — returns the zero
// value of T. Use [RoundRobin.NextOK] to distinguish an empty cycle from a
// served zero value.
//
// Returns:
//   - value (T): the value of the served item, or the zero value of T when the
//     cycle is empty.
func (r *RoundRobin[T]) Next() (value T) {
	value, _ = r.NextOK()

	return value
}

// NextOK serves the next item in the cycle like [RoundRobin.Next], additionally
// reporting whether a serve was possible.
//
// Returns:
//   - value (T): the value of the served item, or the zero value of T when the
//     cycle is empty.
//   - ok (bool): true when an item was served.
func (r *RoundRobin[T]) NextOK() (value T, ok bool) {
	r.mu.Lock()

	defer r.mu.Unlock()

	if len(r.items) == 0 {
		return value, false
	}

	if r.serves == 0 || r.serves >= r.rotateAmount {
		r.advance()

		r.serves = 0
	}

	r.serves++

	it := r.items[r.current]
	it.statistics.incrementServesCount()

	return it.value, true
}

// Items returns a snapshot copy of the values currently in the cycle, in cycle
// order. Mutating the returned slice never affects the round-robin state.
//
// Returns:
//   - items ([]T): the values in the cycle.
func (r *RoundRobin[T]) Items() (items []T) {
	r.mu.Lock()

	defer r.mu.Unlock()

	items = make([]T, 0, len(r.items))

	for _, it := range r.items {
		items = append(items, it.value)
	}

	return items
}

// Len returns the number of items in the cycle.
//
// Returns:
//   - n (int): the number of items.
func (r *RoundRobin[T]) Len() (n int) {
	r.mu.Lock()

	defer r.mu.Unlock()

	return len(r.items)
}

// StatisticsOf returns a live view of the serve statistics of the item carrying
// the given value. The returned pointer stays valid for the lifetime of the
// round-robin instance, even after the item is removed from the cycle.
//
// Parameters:
//   - value (T): the value whose statistics to look up.
//
// Returns:
//   - statistics (*Statistics): the item's statistics, or nil when the value is
//     not in the cycle.
//   - found (bool): true when the value is in the cycle.
func (r *RoundRobin[T]) StatisticsOf(value T) (statistics *Statistics, found bool) {
	r.mu.Lock()

	defer r.mu.Unlock()

	it, found := r.index[value]
	if !found {
		return nil, false
	}

	return &it.statistics, true
}

// advance moves the selection to the next item. While no item carries a
// non-default weight it steps cyclically through the cycle; otherwise it runs
// one step of smooth weighted round-robin — add each item's weight to its
// current weight, pick the heaviest, and subtract the total weight from the
// pick. In the steady state this selects each item in proportion to its
// weight, spreading its picks across the window.
func (r *RoundRobin[T]) advance() {
	if !r.weighted {
		r.current = (r.current + 1) % len(r.items)

		return
	}

	best := 0
	bestWeight := r.items[0].currentWeight

	for i, it := range r.items {
		it.currentWeight += int64(it.weight)

		if it.currentWeight > bestWeight {
			best = i
			bestWeight = it.currentWeight
		}
	}

	r.items[best].currentWeight -= r.totalWeight

	r.current = best
}

// DefaultRotateAmount is the default number of consecutive serves of one item
// before the selection advances to the next item.
//
// It applies when [RoundRobin.SetRotateAmount] is not called, and when a value
// less than or equal to 0 is normalized back to the default.
const DefaultRotateAmount int = 1

// DefaultWeight is the selection weight of items added through [RoundRobin.Add]
// and [New], and the value non-positive weights passed to
// [RoundRobin.AddWithWeight] and [NewWeighted] are normalized to.
const DefaultWeight = 1

// New creates a [RoundRobin] cycling through the given items in order.
//
// Duplicate values are collapsed, keeping the first occurrence. An empty call
// creates an empty cycle: items can be added later with [RoundRobin.Add], and
// [RoundRobin.NextOK] reports whether a serve was possible.
//
// Type Parameters:
//   - T: the type of the items in the cycle; must be comparable.
//
// Parameters:
//   - items (...T): the initial values of the cycle, in cycle order.
//
// Returns:
//   - rr (*RoundRobin[T]): the new round-robin instance.
func New[T comparable](items ...T) (rr *RoundRobin[T]) {
	rr = &RoundRobin[T]{
		items:        make([]*item[T], 0, len(items)),
		index:        make(map[T]*item[T], len(items)),
		current:      -1,
		rotateAmount: DefaultRotateAmount,
	}

	rr.addLocked(items...)

	return rr
}

// NewWeighted creates a [RoundRobin] with per-item selection weights drawn from
// weights. While any item carries a non-default weight, selection runs smooth
// weighted round-robin (see [RoundRobin]). A non-positive weight falls back to
// [DefaultWeight]. An empty map creates an empty cycle.
//
// Map iteration order is undefined; call [RoundRobin.Add] first when a
// deterministic initial cycle order matters.
//
// Type Parameters:
//   - T: the type of the items in the cycle; must be comparable.
//
// Parameters:
//   - weights (map[T]int): the initial values and their selection weights.
//
// Returns:
//   - rr (*RoundRobin[T]): the new round-robin instance.
func NewWeighted[T comparable](weights map[T]int) (rr *RoundRobin[T]) {
	rr = &RoundRobin[T]{
		items:        make([]*item[T], 0, len(weights)),
		index:        make(map[T]*item[T], len(weights)),
		current:      -1,
		rotateAmount: DefaultRotateAmount,
	}

	for value, weight := range weights {
		rr.addWithWeightLocked(value, weight)
	}

	return rr
}
