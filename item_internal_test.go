package roundrobin

import "testing"

func TestItemSelectionStateIsIndependent(t *testing.T) {
	t.Parallel()

	first := &item[string]{value: "first", weight: 2}
	second := &item[string]{value: "second", weight: 1}

	first.currentWeight += int64(first.weight)
	if first.value != "first" || first.weight != 2 || first.currentWeight != 2 {
		t.Fatalf("unexpected first item: %#v", first)
	}

	if second.currentWeight != 0 {
		t.Fatal("selection state was shared between items")
	}
}
