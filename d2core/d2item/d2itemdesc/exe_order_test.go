package d2itemdesc

import (
	"math/rand"
	"sort"
	"testing"
)

func TestExeQsortSortsByPriority(t *testing.T) {
	rng := rand.New(rand.NewSource(7))

	for n := 0; n < 300; n++ {
		a := make([]prioPair, n)
		for i := range a {
			a[i] = prioPair{i, rng.Intn(40)}
		}

		exeQsort(a)

		if !sort.SliceIsSorted(a, func(i, j int) bool { return a[i].prio < a[j].prio }) {
			t.Fatalf("n=%d not sorted", n)
		}

		seen := map[int]bool{}
		for _, p := range a {
			seen[p.id] = true
		}

		if len(seen) != n {
			t.Fatalf("n=%d lost elements", n)
		}
	}
}

// Small inputs go through shortsort only: the first maximum moves to the end.
func TestExeQsortShortTies(t *testing.T) {
	a := []prioPair{{0, 5}, {1, 5}, {2, 1}, {3, 5}}
	exeQsort(a)

	want := []int{2, 1, 3, 0}
	for i, p := range a {
		if p.id != want[i] {
			t.Fatalf("got %v want ids %v", a, want)
		}
	}
}

// With the real ItemStatCost the tie groups come out as the exe's qsort
// leaves them (checked against a step by step run of the decompiled routine).
func TestExeStatOrderReal(t *testing.T) {
	tb := realTables(t)
	rank := tb.displayRank()

	byRank := map[int]int{}
	for id, r := range rank {
		byRank[r] = id
	}

	last := 1 << 30

	for r := 0; r < len(rank); r++ {
		p := tb.Stats[byRank[r]].Priority
		if p > last {
			t.Fatalf("rank %d priority %d after %d", r, p, last)
		}

		last = p
	}

	// priority 81: array order 117,107,108,97 so shown 97,108,107,117
	order := []int{97, 108, 107, 117}
	for i := 1; i < len(order); i++ {
		if rank[order[i-1]] > rank[order[i]] {
			t.Fatalf("group 81 order wrong")
		}
	}

	// priority 88: array order has 335 before 331, shown 331 first
	if rank[331] > rank[335] {
		t.Fatal("cold mastery should list before cold pierce")
	}
}
