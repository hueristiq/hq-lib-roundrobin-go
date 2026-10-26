package roundrobin_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

// weightedValue pairs a value with its selection weight for weighted test cases,
// giving them a deterministic order instead of map iteration.
type weightedValue struct {
	value  string
	weight int
}

func TestNew_ZeroItems(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New[string](nil)

	require.EqualError(t, err, "no items", "Expected a plain sentinel message")
	require.ErrorIs(t, err, hqgoroundrobin.ErrNoItems, "Expected the zero-items error to match the ErrNoItems sentinel")
	assert.Nil(t, rr, "Expected no round-robin instance on failure")
}

func TestNew_CollapsesDuplicates(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b", "a", "c", "b"})

	require.NoError(t, err, "Expected creating a round-robin with duplicates to succeed")
	assert.Equal(t, 3, rr.Len(), "Expected duplicate values to be collapsed")
	assert.Equal(t, []string{"a", "b", "c"}, rr.Items(), "Expected the first occurrence order to be kept")
}

func TestNext_RotationOrder(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b", "c"})
	require.NoError(t, err, "Expected creating a round-robin to succeed")

	served := make([]string, 0, 6)

	for range 6 {
		served = append(served, rr.Next())
	}

	assert.Equal(t, []string{"a", "b", "c", "a", "b", "c"}, served, "Expected strict rotation in insertion order")
}

func TestNext_RotateAmount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		rotateAmount int
		expected     []string
	}{
		{name: "default", rotateAmount: hqgoroundrobin.DefaultRotateAmount, expected: []string{"a", "b", "c", "a", "b", "c"}},
		{name: "two serves per item", rotateAmount: 2, expected: []string{"a", "a", "b", "b", "c", "c"}},
		{name: "zero normalizes to default", rotateAmount: 0, expected: []string{"a", "b", "c", "a", "b", "c"}},
		{name: "negative normalizes to default", rotateAmount: -5, expected: []string{"a", "b", "c", "a", "b", "c"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rr, err := hqgoroundrobin.New([]string{"a", "b", "c"}, hqgoroundrobin.WithRotateAmount(tc.rotateAmount))
			require.NoError(t, err, "Expected creating a round-robin to succeed")

			served := make([]string, 0, len(tc.expected))

			for range len(tc.expected) {
				served = append(served, rr.Next())
			}

			assert.Equal(t, tc.expected, served, "Expected each item to be served rotateAmount times before advancing")
		})
	}
}

func TestNext_WeightedDistribution(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		weights  []weightedValue
		expected []string
	}{
		{
			name:     "two to one to one",
			weights:  []weightedValue{{value: "a", weight: 2}, {value: "b", weight: 1}, {value: "c", weight: 1}},
			expected: []string{"a", "b", "c", "a", "a", "b", "c", "a"},
		},
		{
			name:     "two to one to three",
			weights:  []weightedValue{{value: "a", weight: 2}, {value: "b", weight: 1}, {value: "c", weight: 3}},
			expected: []string{"c", "a", "b", "c", "a", "c"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rr, err := hqgoroundrobin.New([]string{"a", "b", "c"})
			require.NoError(t, err, "Expected creating a round-robin to succeed")

			for _, wv := range tc.weights {
				rr.AddWithWeight(wv.value, wv.weight)
			}

			counts := make(map[string]int)

			totalWeight := 0

			for _, wv := range tc.weights {
				totalWeight += wv.weight
			}

			for i, want := range tc.expected {
				got := rr.Next()

				assert.Equal(t, want, got, "Expected smooth weighted round-robin order at pick %d", i)

				counts[got]++
			}

			for _, wv := range tc.weights {
				assert.Equal(t, wv.weight*len(tc.expected)/totalWeight, counts[wv.value], "Expected item %q to be served in proportion to its weight", wv.value)
			}
		})
	}
}

func TestNext_WeightedWithRotateAmount(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b", "c"}, hqgoroundrobin.WithRotateAmount(2))
	require.NoError(t, err, "Expected creating a round-robin to succeed")

	rr.AddWithWeight("a", 3)

	served := make([]string, 0, 10)

	for range 10 {
		served = append(served, rr.Next())
	}

	assert.Equal(t, []string{"a", "a", "b", "b", "a", "a", "c", "c", "a", "a"}, served, "Expected each weighted pick to be served rotateAmount times in a row")
}

func TestAddWithWeight_NewValue(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b"})
	require.NoError(t, err, "Expected creating a round-robin to succeed")

	rr.AddWithWeight("c", 3)

	assert.Equal(t, 3, rr.Len(), "Expected AddWithWeight to append a new value")

	served := make([]string, 0, 10)

	for range 10 {
		served = append(served, rr.Next())
	}

	assert.Equal(t, []string{"c", "a", "c", "b", "c", "c", "a", "c", "b", "c"}, served, "Expected the new weighted value to shape a deterministic selection order")
}

func TestWeighted_RevertsToStrictRotation(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b", "c"})
	require.NoError(t, err, "Expected creating a round-robin to succeed")

	rr.AddWithWeight("a", 3)
	rr.AddWithWeight("b", 2)

	weighted := make([]string, 0, 5)

	for range 5 {
		weighted = append(weighted, rr.Next())
	}

	assert.Equal(t, []string{"a", "b", "a", "c", "b"}, weighted, "Expected smooth weighted round-robin while non-default weights are set")

	rr.AddWithWeight("a", 1)
	rr.AddWithWeight("b", 1)

	served := make([]string, 0, 6)

	for range 6 {
		served = append(served, rr.Next())
	}

	assert.Equal(t, []string{"c", "a", "b", "c", "a", "b"}, served, "Expected strict rotation once every weight returns to the default")
}

func TestAddWithWeight_UpdatesExistingItem(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b"})
	require.NoError(t, err, "Expected creating a round-robin to succeed")

	rr.AddWithWeight("a", 3)

	served := make([]string, 0, 4)

	for range 4 {
		served = append(served, rr.Next())
	}

	assert.Equal(t, []string{"a", "a", "b", "a"}, served, "Expected the updated weight to shape the selection")

	assert.Equal(t, 2, rr.Len(), "Expected re-adding an existing value to not duplicate it")
}

func TestAddWithWeight_NormalizesNonPositiveWeight(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b"})
	require.NoError(t, err, "Expected creating a round-robin to succeed")

	rr.AddWithWeight("c", 0)
	rr.AddWithWeight("d", -3)

	served := make([]string, 0, 8)

	for range 8 {
		served = append(served, rr.Next())
	}

	assert.Equal(t, []string{"a", "b", "c", "d", "a", "b", "c", "d"}, served, "Expected non-positive weights to fall back to the default weight")
}

func TestAdd_DuringIteration(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b"})
	require.NoError(t, err, "Expected creating a round-robin to succeed")

	assert.Equal(t, "a", rr.Next(), "Expected the first serve to be the first item")

	rr.Add("c", "c", "a")

	assert.Equal(t, 3, rr.Len(), "Expected Add to append new values and skip duplicates")

	served := make([]string, 0, 3)

	for range 3 {
		served = append(served, rr.Next())
	}

	assert.Equal(t, []string{"b", "c", "a"}, served, "Expected the cycle to pick up the added item in order")
}

func TestRemove_DuringIteration(t *testing.T) {
	t.Parallel()

	t.Run("removing the served item starts its successor fresh", func(t *testing.T) {
		t.Parallel()

		rr, err := hqgoroundrobin.New([]string{"a", "b", "c"})
		require.NoError(t, err, "Expected creating a round-robin to succeed")

		assert.Equal(t, "a", rr.Next(), "Expected the first serve to be the first item")
		assert.True(t, rr.Remove("a"), "Expected removing a known value to report true")
		assert.False(t, rr.Remove("a"), "Expected removing an unknown value to report false")

		served := make([]string, 0, 4)

		for range 4 {
			served = append(served, rr.Next())
		}

		assert.Equal(t, []string{"b", "c", "b", "c"}, served, "Expected rotation to continue from the removed item's successor")
	})

	t.Run("removing ahead of the served item preserves the active run", func(t *testing.T) {
		t.Parallel()

		rr, err := hqgoroundrobin.New([]string{"a", "b", "c"}, hqgoroundrobin.WithRotateAmount(2))
		require.NoError(t, err, "Expected creating a round-robin to succeed")

		assert.Equal(t, "a", rr.Next(), "Expected the first serve to be the first item")
		assert.Equal(t, "a", rr.Next(), "Expected the rotate amount to hold the item")
		assert.Equal(t, "b", rr.Next(), "Expected the selection to advance after the rotate amount")
		assert.True(t, rr.Remove("a"), "Expected removing a known value to report true")

		served := make([]string, 0, 3)

		for range 3 {
			served = append(served, rr.Next())
		}

		assert.Equal(t, []string{"b", "c", "c"}, served, "Expected the active run of b to complete before advancing to c")
	})

	t.Run("removing every item empties the cycle", func(t *testing.T) {
		t.Parallel()

		rr, err := hqgoroundrobin.New([]string{"a"})
		require.NoError(t, err, "Expected creating a round-robin to succeed")

		assert.True(t, rr.Remove("a"), "Expected removing a known value to report true")
		assert.Equal(t, 0, rr.Len(), "Expected the cycle to be empty")
		assert.Empty(t, rr.Next(), "Expected Next on an empty cycle to return the zero value")
		assert.Empty(t, rr.Items(), "Expected Items on an empty cycle to be empty")
	})
}

func TestRemove_WeightedMode(t *testing.T) {
	t.Parallel()

	t.Run("removing an item keeps weighted selection proportional", func(t *testing.T) {
		t.Parallel()

		rr, err := hqgoroundrobin.New([]string{"a", "b", "c"})
		require.NoError(t, err, "Expected creating a round-robin to succeed")

		rr.AddWithWeight("a", 2)

		assert.Equal(t, "a", rr.Next(), "Expected the weighted first pick")

		assert.True(t, rr.Remove("c"), "Expected removing a known value to report true")

		served := make([]string, 0, 6)

		for range 6 {
			served = append(served, rr.Next())
		}

		assert.Equal(t, []string{"b", "a", "a", "b", "a", "a"}, served, "Expected weighted selection to stay proportional after the removal")
	})

	t.Run("removing the last non-default weight restores strict rotation", func(t *testing.T) {
		t.Parallel()

		rr, err := hqgoroundrobin.New([]string{"a", "b", "c"})
		require.NoError(t, err, "Expected creating a round-robin to succeed")

		rr.AddWithWeight("a", 3)

		assert.Equal(t, "a", rr.Next(), "Expected the weighted first pick")
		assert.Equal(t, "b", rr.Next(), "Expected the weighted second pick")

		assert.True(t, rr.Remove("a"), "Expected removing a known value to report true")

		served := make([]string, 0, 6)

		for range 6 {
			served = append(served, rr.Next())
		}

		assert.Equal(t, []string{"c", "b", "c", "b", "c", "b"}, served, "Expected strict rotation once the last non-default weight is removed")
	})
}

func TestItems_CopyIsolation(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b", "c"})
	require.NoError(t, err, "Expected creating a round-robin to succeed")

	items := rr.Items()

	items[0] = "mutated"

	items = append(items, "appended")

	assert.Len(t, items, 4, "Expected the returned slice to accept appends")
	assert.Equal(t, []string{"a", "b", "c"}, rr.Items(), "Expected mutating the returned slice to leave the cycle untouched")
	assert.Equal(t, 3, rr.Len(), "Expected appending to the returned slice to leave the cycle untouched")
}

func TestStatistics(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b", "c"})
	require.NoError(t, err, "Expected creating a round-robin to succeed")

	for range 7 {
		rr.Next()
	}

	stats, found := rr.StatisticsOf("a")

	require.True(t, found, "Expected statistics for an item in the cycle")
	require.NotNil(t, stats, "Expected statistics for an item in the cycle")
	assert.Equal(t, int64(3), stats.ServesCount(), "Expected one serve per cycle pass")

	_, found = rr.StatisticsOf("zzz")
	assert.False(t, found, "Expected no statistics for an unknown value")

	stats.ResetServesCount()

	assert.Equal(t, int64(0), stats.ServesCount(), "Expected the serve count to reset to zero")

	assert.True(t, rr.Remove("a"), "Expected removing a known value to report true")

	rr.Next()
	rr.Next()

	assert.Equal(t, int64(0), stats.ServesCount(), "Expected the statistics of a removed item to stay readable and unchanged")

	statsB, found := rr.StatisticsOf("b")

	require.True(t, found, "Expected statistics for an item in the cycle")
	assert.Equal(t, int64(3), statsB.ServesCount(), "Expected serves to keep accumulating for live items")
}

func TestStatistics_ReaddedValueGetsFreshStatistics(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New([]string{"a", "b"})
	require.NoError(t, err, "Expected creating a round-robin to succeed")

	assert.Equal(t, "a", rr.Next(), "Expected the first serve to be the first item")

	old, found := rr.StatisticsOf("a")

	require.True(t, found, "Expected statistics for an item in the cycle")
	require.NotNil(t, old, "Expected statistics for an item in the cycle")
	assert.Equal(t, int64(1), old.ServesCount(), "Expected the serve count of the served item")

	assert.True(t, rr.Remove("a"), "Expected removing a known value to report true")

	rr.Add("a")

	assert.Equal(t, "b", rr.Next(), "Expected rotation to continue with the remaining item")
	assert.Equal(t, "a", rr.Next(), "Expected the re-added value to rejoin the cycle")

	fresh, found := rr.StatisticsOf("a")

	require.True(t, found, "Expected statistics for an item in the cycle")
	require.NotSame(t, old, fresh, "Expected the re-added value to get fresh statistics")
	assert.Equal(t, int64(1), fresh.ServesCount(), "Expected the fresh statistics to start counting from zero")
	assert.Equal(t, int64(1), old.ServesCount(), "Expected the old statistics of the removed item to stay frozen")
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	t.Run("deterministic equal distribution", func(t *testing.T) {
		t.Parallel()

		rr, err := hqgoroundrobin.New([]string{"a", "b", "c", "d"})
		require.NoError(t, err, "Expected creating a round-robin to succeed")

		var wg sync.WaitGroup

		for range 100 {
			wg.Add(1)

			go func() {
				defer wg.Done()

				for range 3 {
					rr.Next()
				}
			}()
		}

		wg.Wait()

		total := int64(0)

		for _, value := range rr.Items() {
			stats, found := rr.StatisticsOf(value)

			require.True(t, found, "Expected statistics for every item in the cycle")
			assert.Equal(t, int64(75), stats.ServesCount(), "Expected item %q to be served an equal share of the 300 serves", value)

			total += stats.ServesCount()
		}

		assert.Equal(t, int64(300), total, "Expected every Next to be accounted in the statistics")
	})

	t.Run("mixed operations under race detector", func(t *testing.T) {
		t.Parallel()

		rr, err := hqgoroundrobin.New([]string{"server-0", "server-1", "server-2", "server-3"})
		require.NoError(t, err, "Expected creating a round-robin to succeed")

		var wg sync.WaitGroup

		for g := range 8 {
			wg.Add(1)

			go func(g int) {
				defer wg.Done()

				for i := range 500 {
					value := fmt.Sprintf("server-%d", (g*500+i)%16)

					switch i % 5 {
					case 0:
						rr.Add(value)
					case 1:
						rr.AddWithWeight(value, 1+i%4)
					case 2:
						rr.Remove(value)
					case 3:
						_, _ = rr.StatisticsOf(value)

						_ = rr.Items()
						_ = rr.Len()
					default:
						_ = rr.Next()
					}
				}
			}(g)
		}

		wg.Wait()

		assert.GreaterOrEqual(t, rr.Len(), 0, "Expected the cycle to stay consistent after mixed concurrent access")
	})
}
