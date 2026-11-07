package roundrobin_test

import (
	"fmt"
	"strings"
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

func nextValue[T comparable](rr *hqgoroundrobin.RoundRobin[T]) (value T) {
	value, _ = rr.Next()

	return
}

func TestNew_EmptyCycle(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New[string]()

	require.NotNil(t, rr, "Expected an empty call to build a round-robin instance")
	assert.Equal(t, 0, rr.Len(), "Expected an empty call to build an empty cycle")
	assert.Empty(t, rr.Items(), "Expected an empty cycle to list no items")
	assert.Empty(t, nextValue(rr), "Expected Next on an empty cycle to return the zero value")

	value, ok := rr.Next()

	assert.Empty(t, value, "Expected Next on an empty cycle to return the zero value")
	assert.False(t, ok, "Expected Next to report that no serve was possible")
}

func TestNew_CollapsesDuplicates(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b", "a", "c", "b")

	assert.Equal(t, 3, rr.Len(), "Expected duplicate values to be collapsed")
	assert.Equal(t, []string{"a", "b", "c"}, rr.Items(), "Expected the first occurrence order to be kept")
}

func TestNext_RotationOrder(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b", "c")

	served := make([]string, 0, 6)

	for range 6 {
		served = append(served, nextValue(rr))
	}

	assert.Equal(t, []string{"a", "b", "c", "a", "b", "c"}, served, "Expected strict rotation in insertion order")
}

func TestNext_ReportsServe(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b")

	value, ok := rr.Next()

	assert.Equal(t, "a", value, "Expected the first serve to be the first item")
	assert.True(t, ok, "Expected a serve on a non-empty cycle")

	value, ok = rr.Next()

	assert.Equal(t, "b", value, "Expected the second serve to be the second item")
	assert.True(t, ok, "Expected a serve on a non-empty cycle")
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

			rr := hqgoroundrobin.New("a", "b", "c")

			rr.SetRotateAmount(tc.rotateAmount)

			served := make([]string, 0, len(tc.expected))

			for range len(tc.expected) {
				served = append(served, nextValue(rr))
			}

			assert.Equal(t, tc.expected, served, "Expected each item to be served rotateAmount times before advancing")
		})
	}
}

func TestAddWithWeight_Distribution(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b", "c")
	rr.AddWithWeight("a", 2)

	counts := make(map[string]int)

	for range 8 {
		counts[nextValue(rr)]++
	}

	assert.Equal(t, 4, counts["a"], "Expected item %q to be served in proportion to its weight", "a")
	assert.Equal(t, 2, counts["b"], "Expected item %q to be served in proportion to its weight", "b")
	assert.Equal(t, 2, counts["c"], "Expected item %q to be served in proportion to its weight", "c")
}

func TestAddWithWeight_NonPositiveWeightsFallBackToDefault(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b", "c")
	rr.AddWithWeight("a", 0)
	rr.AddWithWeight("b", -3)

	counts := make(map[string]int)

	for range 6 {
		counts[nextValue(rr)]++
	}

	for _, value := range []string{"a", "b", "c"} {
		assert.Equal(t, 2, counts[value], "Expected item %q to be served an equal share under strict rotation", value)
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

			rr := hqgoroundrobin.New("a", "b", "c")

			for _, wv := range tc.weights {
				rr.AddWithWeight(wv.value, wv.weight)
			}

			counts := make(map[string]int)

			totalWeight := 0

			for _, wv := range tc.weights {
				totalWeight += wv.weight
			}

			for i, want := range tc.expected {
				got := nextValue(rr)

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

	rr := hqgoroundrobin.New("a", "b", "c")

	rr.SetRotateAmount(2)
	rr.AddWithWeight("a", 3)

	served := make([]string, 0, 10)

	for range 10 {
		served = append(served, nextValue(rr))
	}

	assert.Equal(t, []string{"a", "a", "b", "b", "a", "a", "c", "c", "a", "a"}, served, "Expected each weighted pick to be served rotateAmount times in a row")
}

func TestAddWithWeight_NewValue(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b")

	rr.AddWithWeight("c", 3)

	assert.Equal(t, 3, rr.Len(), "Expected AddWithWeight to append a new value")

	served := make([]string, 0, 10)

	for range 10 {
		served = append(served, nextValue(rr))
	}

	assert.Equal(t, []string{"c", "a", "c", "b", "c", "c", "a", "c", "b", "c"}, served, "Expected the new weighted value to shape a deterministic selection order")
}

func TestWeighted_RevertsToStrictRotation(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b", "c")

	rr.AddWithWeight("a", 3)
	rr.AddWithWeight("b", 2)

	weighted := make([]string, 0, 5)

	for range 5 {
		weighted = append(weighted, nextValue(rr))
	}

	assert.Equal(t, []string{"a", "b", "a", "c", "b"}, weighted, "Expected smooth weighted round-robin while non-default weights are set")

	rr.AddWithWeight("a", 1)
	rr.AddWithWeight("b", 1)

	served := make([]string, 0, 6)

	for range 6 {
		served = append(served, nextValue(rr))
	}

	assert.Equal(t, []string{"c", "a", "b", "c", "a", "b"}, served, "Expected strict rotation once every weight returns to the default")
}

func TestAddWithWeight_UpdatesExistingItem(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b")

	rr.AddWithWeight("a", 3)

	served := make([]string, 0, 4)

	for range 4 {
		served = append(served, nextValue(rr))
	}

	assert.Equal(t, []string{"a", "a", "b", "a"}, served, "Expected the updated weight to shape the selection")

	assert.Equal(t, 2, rr.Len(), "Expected re-adding an existing value to not duplicate it")
}

func TestAddWithWeight_NormalizesNonPositiveWeight(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b")

	rr.AddWithWeight("c", 0)
	rr.AddWithWeight("d", -3)

	served := make([]string, 0, 8)

	for range 8 {
		served = append(served, nextValue(rr))
	}

	assert.Equal(t, []string{"a", "b", "c", "d", "a", "b", "c", "d"}, served, "Expected non-positive weights to fall back to the default weight")
}

func TestAdd_DuringIteration(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b")

	assert.Equal(t, "a", nextValue(rr), "Expected the first serve to be the first item")

	rr.Add("c", "c", "a")

	assert.Equal(t, 3, rr.Len(), "Expected Add to append new values and skip duplicates")

	served := make([]string, 0, 3)

	for range 3 {
		served = append(served, nextValue(rr))
	}

	assert.Equal(t, []string{"b", "c", "a"}, served, "Expected the cycle to pick up the added item in order")
}

func TestRemove_DuringIteration(t *testing.T) {
	t.Parallel()

	t.Run("removing the served item starts its successor fresh", func(t *testing.T) {
		t.Parallel()

		rr := hqgoroundrobin.New("a", "b", "c")

		assert.Equal(t, "a", nextValue(rr), "Expected the first serve to be the first item")
		assert.True(t, rr.Remove("a"), "Expected removing a known value to report true")
		assert.False(t, rr.Remove("a"), "Expected removing an unknown value to report false")

		served := make([]string, 0, 4)

		for range 4 {
			served = append(served, nextValue(rr))
		}

		assert.Equal(t, []string{"b", "c", "b", "c"}, served, "Expected rotation to continue from the removed item's successor")
	})

	t.Run("removing ahead of the served item preserves the active run", func(t *testing.T) {
		t.Parallel()

		rr := hqgoroundrobin.New("a", "b", "c")

		rr.SetRotateAmount(2)

		assert.Equal(t, "a", nextValue(rr), "Expected the first serve to be the first item")
		assert.Equal(t, "a", nextValue(rr), "Expected the rotate amount to hold the item")
		assert.Equal(t, "b", nextValue(rr), "Expected the selection to advance after the rotate amount")
		assert.True(t, rr.Remove("a"), "Expected removing a known value to report true")

		served := make([]string, 0, 3)

		for range 3 {
			served = append(served, nextValue(rr))
		}

		assert.Equal(t, []string{"b", "c", "c"}, served, "Expected the active run of b to complete before advancing to c")
	})

	t.Run("removing every item empties the cycle", func(t *testing.T) {
		t.Parallel()

		rr := hqgoroundrobin.New("a")

		assert.True(t, rr.Remove("a"), "Expected removing a known value to report true")
		assert.Equal(t, 0, rr.Len(), "Expected the cycle to be empty")
		assert.Empty(t, nextValue(rr), "Expected Next on an empty cycle to return the zero value")
		assert.Empty(t, rr.Items(), "Expected Items on an empty cycle to be empty")
	})
}

func TestRemove_WeightedMode(t *testing.T) {
	t.Parallel()

	t.Run("removing an item keeps weighted selection proportional", func(t *testing.T) {
		t.Parallel()

		rr := hqgoroundrobin.New("a", "b", "c")

		rr.AddWithWeight("a", 2)

		assert.Equal(t, "a", nextValue(rr), "Expected the weighted first pick")

		assert.True(t, rr.Remove("c"), "Expected removing a known value to report true")

		served := make([]string, 0, 6)

		for range 6 {
			served = append(served, nextValue(rr))
		}

		assert.Equal(t, []string{"b", "a", "a", "b", "a", "a"}, served, "Expected weighted selection to stay proportional after the removal")
	})

	t.Run("removing the last non-default weight restores strict rotation", func(t *testing.T) {
		t.Parallel()

		rr := hqgoroundrobin.New("a", "b", "c")

		rr.AddWithWeight("a", 3)

		assert.Equal(t, "a", nextValue(rr), "Expected the weighted first pick")
		assert.Equal(t, "b", nextValue(rr), "Expected the weighted second pick")

		assert.True(t, rr.Remove("a"), "Expected removing a known value to report true")

		served := make([]string, 0, 6)

		for range 6 {
			served = append(served, nextValue(rr))
		}

		assert.Equal(t, []string{"c", "b", "c", "b", "c", "b"}, served, "Expected strict rotation once the last non-default weight is removed")
	})
}

func TestItems_CopyIsolation(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b", "c")

	items := rr.Items()

	items[0] = "mutated"

	items = append(items, "appended")

	assert.Len(t, items, 4, "Expected the returned slice to accept appends")
	assert.Equal(t, []string{"a", "b", "c"}, rr.Items(), "Expected mutating the returned slice to leave the cycle untouched")
	assert.Equal(t, 3, rr.Len(), "Expected appending to the returned slice to leave the cycle untouched")
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	t.Run("mixed operations under race detector", func(t *testing.T) {
		t.Parallel()

		rr := hqgoroundrobin.New("server-0", "server-1", "server-2", "server-3")

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
						_ = rr.Items()
						_ = rr.Len()
					default:
						_ = nextValue(rr)
					}
				}
			}(g)
		}

		wg.Wait()

		assert.GreaterOrEqual(t, rr.Len(), 0, "Expected the cycle to stay consistent after mixed concurrent access")
	})
}

// ExampleNew demonstrates strict rotation over a fixed set of items.
func ExampleNew() {
	rr := hqgoroundrobin.New("server-1", "server-2", "server-3")

	for range 6 {
		fmt.Println(nextValue(rr))
	}
	// Output:
	// server-1
	// server-2
	// server-3
	// server-1
	// server-2
	// server-3
}

// ExampleRoundRobin_AddWithWeight demonstrates smooth weighted round-robin selection.
func ExampleRoundRobin_AddWithWeight() {
	rr := hqgoroundrobin.New("fast", "slow")

	rr.AddWithWeight("fast", 3)

	for range 8 {
		fmt.Println(nextValue(rr))
	}
	// Output:
	// fast
	// fast
	// slow
	// fast
	// fast
	// fast
	// slow
	// fast
}

// ExampleRoundRobin_Next demonstrates explicit handling of an empty cycle.
func ExampleRoundRobin_Next() {
	rr := hqgoroundrobin.New[string]()

	if value, ok := rr.Next(); ok {
		fmt.Println(value)

		return
	}

	fmt.Println("cycle is empty")
	// Output: cycle is empty
}

// FuzzAddRemoveNext exercises construction, serving, and removal over arbitrary
// value lists: the cycle must never panic, duplicate values must collapse, and
// removing every value must always empty the cycle.
func FuzzAddRemoveNext(f *testing.F) {
	f.Add("a b c")
	f.Add("a")
	f.Add("")

	f.Fuzz(func(t *testing.T, items string) {
		values := strings.Fields(items)

		rr := hqgoroundrobin.New(values...)

		for range rr.Len() * 2 {
			_, _ = rr.Next()
		}

		for _, value := range values {
			_ = rr.Remove(value)
		}

		if rr.Len() != 0 {
			t.Fatalf("Expected removing every value to empty the cycle, got %d items", rr.Len())
		}

		if _, ok := rr.Next(); ok {
			t.Fatal("Expected Next to report an empty cycle")
		}
	})
}
