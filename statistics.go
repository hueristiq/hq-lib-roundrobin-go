package roundrobin

import "sync/atomic"

// Statistics holds the usage metrics of a single item in a [RoundRobin] cycle.
//
// The counters are read and written atomically, so a *Statistics obtained through
// [RoundRobin.StatisticsOf] stays valid for the lifetime of the round-robin
// instance — even after the item is removed from the cycle, though it then no
// longer changes. A Statistics value must not be copied after first use; use the
// pointer returned by [RoundRobin.StatisticsOf].
//
// Fields:
//   - servesCount (atomic.Int64): number of times the item has been served.
type Statistics struct {
	servesCount atomic.Int64
}

// ServesCount returns how many times the item has been served.
//
// Returns:
//   - n (int64): the current serve count.
func (s *Statistics) ServesCount() (n int64) {
	n = s.servesCount.Load()

	return
}

// ResetServesCount atomically resets the serve count to zero.
func (s *Statistics) ResetServesCount() {
	s.servesCount.Store(0)
}

// incrementServesCount atomically increments the serve count by one. It is called
// by [RoundRobin.Next] for the served item.
func (s *Statistics) incrementServesCount() {
	s.servesCount.Add(1)
}
