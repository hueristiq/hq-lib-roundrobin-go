package roundrobin

import (
	"errors"
	"slices"
	"sync"
)

// DefaultRotateAmount is the default number of consecutive serves of one item
// before the selection advances to the next item.
//
// It applies when [WithRotateAmount] is not supplied, and when a supplied value
// less than or equal to 0 is normalized back to the default.
const DefaultRotateAmount int32 = 1

// defaultWeight is the selection weight of items added through [RoundRobin.Add],
// and the value non-positive weights passed to [RoundRobin.AddWithWeight] are
// normalized to.
const defaultWeight = 1

// options holds the configurable knobs of a [RoundRobin], populated through
// [OptionFunc] values passed to [New].
//
// Fields:
//   - rotateAmount (int32): the number of consecutive serves of one item before
//     the selection advances to the next item.
type options struct {
	rotateAmount int32
}

// OptionFunc configures a [RoundRobin] through the functional-options idiom.
//
// Parameters:
//   - opts (*options): the options bag to mutate.
type OptionFunc func(opts *options)

// WithRotateAmount configures how many times in a row one item is served before
// the selection advances to the next item.
//
// Parameters:
//   - rotateAmount (int32): the number of consecutive serves per item. Values
//     less than or equal to 0 fall back to [DefaultRotateAmount].
//
// Returns:
//   - of (OptionFunc): the option applying rotateAmount.
func WithRotateAmount(rotateAmount int32) (of OptionFunc) {
	of = func(opts *options) {
		opts.rotateAmount = rotateAmount
	}

	return
}

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
// smooth weighted round-robin: over every window of sum(weights) selections each
// item is picked exactly weight times, spread across the window. Removing items
// perturbs the order of the very next picks, but the long-run distribution stays
// proportional to the weights.
//
// The zero value is not usable; construct through [New].
//
// Type Parameters:
//   - T: the type of the items in the cycle; must be comparable.
//
// Fields:
//   - mu (sync.Mutex): guards every field below.
//   - items ([]*item[T]): the items in the cycle, in insertion order.
//   - index (map[T]*item[T]): the same items keyed by value, for deduplication,
//     removal, and statistics lookup.
//   - current (int): the index into items of the item being served; -1 before
//     the first serve and right after the served item was removed.
//   - serves (int32): how many times the current item has been served in its
//     active run; a new item is selected when it reaches rotateAmount.
//   - rotateAmount (int32): the number of consecutive serves of one item before
//     the selection advances; always at least 1.
//   - totalWeight (int64): the sum of the weights of all items in the cycle.
//   - weighted (bool): whether any item ever carried a non-default weight,
//     switching selection from strict rotation to smooth weighted round-robin.
type RoundRobin[T comparable] struct {
	mu           sync.Mutex
	items        []*item[T]
	index        map[T]*item[T]
	current      int
	serves       int32
	rotateAmount int32
	totalWeight  int64
	weighted     bool
}

// New creates a [RoundRobin] over the given items, configured by ofs.
//
// Duplicate values are collapsed, keeping the first occurrence. At least one
// item must be provided. Options are applied and then normalized: a non-positive
// rotate amount falls back to [DefaultRotateAmount].
//
// Type Parameters:
//   - T: the type of the items in the cycle; must be comparable.
//
// Parameters:
//   - items ([]T): the initial values of the cycle.
//   - ofs (...OptionFunc): functional options configuring the cycle.
//
// Returns:
//   - rr (*RoundRobin[T]): the new round-robin instance.
//   - err (error): non-nil when items is empty.
func New[T comparable](items []T, ofs ...OptionFunc) (rr *RoundRobin[T], err error) {
	if len(items) == 0 {
		err = errors.New("roundrobin.New: no items")

		return
	}

	opts := &options{
		rotateAmount: DefaultRotateAmount,
	}

	for _, of := range ofs {
		of(opts)
	}

	if opts.rotateAmount <= 0 {
		opts.rotateAmount = DefaultRotateAmount
	}

	rr = &RoundRobin[T]{
		items:        make([]*item[T], 0, len(items)),
		index:        make(map[T]*item[T], len(items)),
		current:      -1,
		rotateAmount: opts.rotateAmount,
	}

	rr.Add(items...)

	return
}

// Add appends items to the cycle, each with the default weight. Values already
// in the cycle are skipped: their weight and statistics are preserved.
//
// Parameters:
//   - items (...T): the values to add.
func (r *RoundRobin[T]) Add(items ...T) {
	r.mu.Lock()

	defer r.mu.Unlock()

	for _, value := range items {
		if _, exists := r.index[value]; exists {
			continue
		}

		it := &item[T]{
			value:  value,
			weight: defaultWeight,
		}

		r.items = append(r.items, it)
		r.index[value] = it
		r.totalWeight += defaultWeight
	}
}

// AddWithWeight adds a single item with the given selection weight, switching
// selection to smooth weighted round-robin: over every window of sum(weights)
// selections the item is picked weight times, its picks spread across the
// window.
//
// Re-adding a value already in the cycle updates its weight and preserves its
// statistics. A non-positive weight falls back to the default weight of 1.
//
// Parameters:
//   - value (T): the value to add.
//   - weight (int): the relative selection weight of the item.
func (r *RoundRobin[T]) AddWithWeight(value T, weight int) {
	if weight <= 0 {
		weight = defaultWeight
	}

	r.mu.Lock()

	defer r.mu.Unlock()

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

	if weight != defaultWeight {
		r.weighted = true
	}
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
		return
	}

	delete(r.index, value)

	r.totalWeight -= int64(it.weight)

	i := slices.Index(r.items, it)

	r.items = slices.Delete(r.items, i, i+1)

	// Keep current pointing at the element it served before the shift; when the
	// removed item was being served, step back and start a fresh run so the next
	// advance lands on the removed item's successor (-1 is valid and wraps to
	// the head of the cycle).
	switch {
	case i < r.current:
		r.current--
	case i == r.current:
		r.current--
		r.serves = 0
	}

	removed = true

	return
}

// Next serves the next item in the cycle: the current item is served until it
// has been served rotateAmount times in a row, then the selection advances —
// stepping cyclically while all weights are equal, or running one smooth
// weighted round-robin step otherwise. The served item's serve count is
// incremented.
//
// Calling Next on an empty cycle — possible after every item was removed with
// [RoundRobin.Remove] — returns the zero value of T.
//
// Returns:
//   - value (T): the value of the served item, or the zero value of T when the
//     cycle is empty.
func (r *RoundRobin[T]) Next() (value T) {
	r.mu.Lock()

	defer r.mu.Unlock()

	if len(r.items) == 0 {
		return
	}

	if r.serves == 0 || r.serves >= r.rotateAmount {
		r.advance()

		r.serves = 0
	}

	r.serves++

	it := r.items[r.current]
	it.statistics.incrementServesCount()

	value = it.value

	return
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

	return
}

// Len returns the number of items in the cycle.
//
// Returns:
//   - n (int): the number of items.
func (r *RoundRobin[T]) Len() (n int) {
	r.mu.Lock()

	defer r.mu.Unlock()

	n = len(r.items)

	return
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
	if found {
		statistics = &it.statistics
	}

	return
}

// advance moves the selection to the next item. While no item carries a
// non-default weight it steps cyclically through the cycle; otherwise it runs
// one step of smooth weighted round-robin — add each item's weight to its
// current weight, pick the heaviest, and subtract the total weight from the
// pick — which selects each item exactly weight times per window of
// totalWeight selections, spread evenly across the window.
func (r *RoundRobin[T]) advance() {
	if !r.weighted {
		r.current = (r.current + 1) % len(r.items)

		return
	}

	best := 0

	for i, it := range r.items {
		it.currentWeight += int64(it.weight)

		if it.currentWeight > r.items[best].currentWeight {
			best = i
		}
	}

	r.items[best].currentWeight -= r.totalWeight

	r.current = best
}
