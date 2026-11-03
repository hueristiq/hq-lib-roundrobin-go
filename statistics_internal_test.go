package roundrobin

import (
	"sync"
	"testing"
)

// TestStatistics covers the atomic counter: increment, read, reset, and
// concurrent access.
func TestStatistics(t *testing.T) {
	t.Parallel()

	var stats Statistics

	stats.incrementServesCount()
	stats.incrementServesCount()

	if got := stats.ServesCount(); got != 2 {
		t.Errorf("ServesCount() = %d; want 2", got)
	}

	stats.ResetServesCount()

	if got := stats.ServesCount(); got != 0 {
		t.Errorf("ServesCount() after reset = %d; want 0", got)
	}
}

// TestStatisticsConcurrent increments the counter from many goroutines.
func TestStatisticsConcurrent(t *testing.T) {
	t.Parallel()

	var stats Statistics

	var wg sync.WaitGroup

	for range 64 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			stats.incrementServesCount()
		}()
	}

	wg.Wait()

	if got := stats.ServesCount(); got != 64 {
		t.Errorf("ServesCount() = %d; want 64", got)
	}
}
