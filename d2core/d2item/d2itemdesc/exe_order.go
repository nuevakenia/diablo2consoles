package d2itemdesc

import "sort"

// Tooltip line order, as Game.exe 1.14b builds it (w43 audit).
//
// DATATBL_LoadItemStatCostTable (0x6387e0) collects every ItemStatCost row
// whose descfunc byte is non zero as a 4 byte pair {id, descpriority} in row
// order and sorts the pairs with the CRT qsort (0x6897e0, the Visual Studio
// 2005 version: median of three, partitions of at most 8 elements by
// shortsort) and a comparator that orders by ascending descpriority only
// (0x6387b0: -1 when a < b, else a != b). qsort is not stable, so rows with
// the same priority land in a scrambled but fixed order. The item tooltip
// builder (UI_BuildItemPropertyTooltipText 0x4e3110) walks that array from the
// start and, per stat, the stat list's parameter entries from the lowest key.
//
// On screen the lines run from the highest priority down, so the text is
// shown bottom up: the display order is the array reversed (VERIFIED for
// priorities by the well known item listings; the reversal as the cause is
// UNVERIFIED, but it also explains the listing of several skills with the
// same stat id, e.g. Call to Arms: Battle Command (155), Battle Orders (149),
// Battle Cry (146), so parameters run from high to low).

type prioPair struct {
	id, prio int
}

func prioCmp(a, b prioPair) int {
	switch {
	case a.prio < b.prio:
		return -1
	case a.prio != b.prio:
		return 1
	}

	return 0
}

// exeQsort is the VS2005 CRT qsort (Game.exe 0x6897e0 with shortsort 0x689750)
// over prioPair elements, step for step, so equal keys end up where the exe
// puts them.
func exeQsort(a []prioPair) {
	if len(a) < 2 {
		return
	}

	type span struct{ lo, hi int }

	var stack []span

	lo, hi := 0, len(a)-1

	swap := func(i, j int) { a[i], a[j] = a[j], a[i] }

	for {
		size := hi - lo + 1

		if size <= 8 { // shortsort: move the (first) maximum to the end, repeat
			for h := hi; h > lo; h-- {
				m := lo

				for p := lo + 1; p <= h; p++ {
					if prioCmp(a[p], a[m]) > 0 {
						m = p
					}
				}

				if m != h {
					swap(m, h)
				}
			}
		} else {
			mid := lo + size/2

			if prioCmp(a[lo], a[mid]) > 0 {
				swap(lo, mid)
			}

			if prioCmp(a[lo], a[hi]) > 0 {
				swap(lo, hi)
			}

			if prioCmp(a[mid], a[hi]) > 0 {
				swap(mid, hi)
			}

			lg, hg := lo, hi

			for {
				if lg < mid {
					for {
						lg++
						if lg >= mid || prioCmp(a[lg], a[mid]) > 0 {
							break
						}
					}
				}

				if lg >= mid {
					for {
						lg++
						if lg > hi || prioCmp(a[lg], a[mid]) > 0 {
							break
						}
					}
				}

				for {
					hg--
					if hg <= mid || prioCmp(a[hg], a[mid]) <= 0 {
						break
					}
				}

				if lg > hg {
					break
				}

				if lg != hg {
					swap(lg, hg)
				}

				if mid == hg {
					mid = lg
				}
			}

			hg++

			if mid < hg {
				for {
					hg--
					if hg <= mid || prioCmp(a[hg], a[mid]) != 0 {
						break
					}
				}
			}

			if mid >= hg {
				for {
					hg--
					if hg <= lo || prioCmp(a[hg], a[mid]) != 0 {
						break
					}
				}
			}

			if hg-lo < hi-lg {
				if lg < hi {
					stack = append(stack, span{lg, hi})
				}

				hi = hg
				if hi > lo {
					continue
				}
			} else {
				if lo < hg {
					stack = append(stack, span{lo, hg})
				}

				lo = lg
				if lo < hi {
					continue
				}
			}
		}

		if len(stack) == 0 {
			return
		}

		lo, hi = stack[len(stack)-1].lo, stack[len(stack)-1].hi
		stack = stack[:len(stack)-1]
	}
}

// displayRank returns, for every stat with a description, its position on the
// tooltip (0 = first line) among stats of any priority, as the exe's sorted
// array gives it. Stats without descfunc are absent from the map.
func (t *Tables) displayRank() map[int]int {
	t.rankOnce.Do(func() {
		ids := make([]int, 0, len(t.Stats))

		for id, d := range t.Stats {
			if d != nil && d.Func != 0 {
				ids = append(ids, id)
			}
		}

		sort.Ints(ids) // table row order

		arr := make([]prioPair, len(ids))
		for i, id := range ids {
			arr[i] = prioPair{id, t.Stats[id].Priority}
		}

		exeQsort(arr)

		t.rank = make(map[int]int, len(arr))
		for i, p := range arr {
			t.rank[p.id] = len(arr) - 1 - i
		}
	})

	return t.rank
}
