package roundrobin_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

func TestNew_ZeroItems(t *testing.T) {
	t.Parallel()

	rr, err := hqgoroundrobin.New[string](nil)

	require.EqualError(t, err, "roundrobin.New: no items", "Expected the zero-items error to name its producing function")
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
		rotateAmount int32
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
		weights  map[string]int
		expected []string
	}{
		{
			name:     "two to one to one",
			weights:  map[string]int{"a": 2, "b": 1, "c": 1},
			expected: []string{"a", "b", "c", "a", "a", "b", "c", "a"},
		},
		{
			name:     "three to one to two",
			weights:  map[string]int{"a": 2, "b": 1, "c": 3},
			expected: []string{"c", "a", "b", "c", "a", "c"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rr, err := hqgoroundrobin.New([]string{"a", "b", "c"})
			require.NoError(t, err, "Expected creating a round-robin to succeed")

			for value, weight := range tc.weights {
				rr.AddWithWeight(value, weight)
			}

			counts := make(map[string]int)

			totalWeight := 0

			for _, weight := range tc.weights {
				totalWeight += weight
			}

			for i, want := range tc.expected {
				got := rr.Next()

				assert.Equal(t, want, got, "Expected smooth weighted round-robin order at pick %d", i)

				counts[got]++
			}

			for value, weight := range tc.weights {
				assert.Equal(t, weight*len(tc.expected)/totalWeight, counts[value], "Expected item %q to be served in proportion to its weight", value)
			}
		})
	}
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
