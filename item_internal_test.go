package roundrobin

import "testing"

// TestItemDefaults covers the internal item record: the weight floor and the
// zero-value statistics.
func TestItemDefaults(t *testing.T) {
	t.Parallel()

	it := &item[string]{value: "a", weight: 1}

	if it.value != "a" || it.weight != 1 || it.currentWeight != 0 {
		t.Errorf("item = %+v; want value a, weight 1, currentWeight 0", it)
	}

	if got := it.statistics.ServesCount(); got != 0 {
		t.Errorf("ServesCount() = %d; want 0", got)
	}
}

// TestItemStatisticsPointerStability confirms the statistics pointer stays
// valid regardless of the item's position, as documented on item.
func TestItemStatisticsPointerStability(t *testing.T) {
	t.Parallel()

	it := &item[int]{value: 1, weight: 1}
	stats := &it.statistics

	stats.incrementServesCount()

	if got := it.statistics.ServesCount(); got != 1 {
		t.Errorf("ServesCount() after increment = %d; want 1", got)
	}
}
