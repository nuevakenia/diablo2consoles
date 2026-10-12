package d2asset

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

func TestLayerValues(t *testing.T) {
	nu := d2enum.MonsterAnimationModeNeutral
	wl := d2enum.MonsterAnimationModeWalk

	tests := []struct {
		name    string
		options []string
		mode    d2enum.MonsterAnimationMode
		want    []string
	}{
		{"no options, any mode", nil, wl, []string{"lit"}},
		{"no options, neutral", nil, nu, []string{"lit"}},
		{"options, neutral also loads lit first", []string{"hvy", "lgt"}, nu, []string{"lit", "hvy", "lgt"}},
		{"options, other mode", []string{"hvy", "lgt"}, wl, []string{"hvy", "lgt"}},
		{"empty option means lit, no duplicates", []string{"", "lit", "hvy"}, wl, []string{"lit", "hvy"}},
	}

	for _, tc := range tests {
		if got := layerValues(tc.options, tc.mode); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestNewPrefetchTypesSkipsRepeats(t *testing.T) {
	prefetchMu.Lock()
	prefetchSeen = map[string]bool{}
	prefetchMu.Unlock()

	a := MonsterPrefetch{Token: "ZZ", WeaponClass: "HTH", Palette: "p"}
	b := MonsterPrefetch{Token: "YY", WeaponClass: "HTH", Palette: "p"}

	if got := newPrefetchTypes([]MonsterPrefetch{a, a, b, {}}); len(got) != 2 {
		t.Fatalf("first call: %d types, want 2", len(got))
	}

	if got := newPrefetchTypes([]MonsterPrefetch{a, b}); len(got) != 0 {
		t.Fatalf("second call: %d types, want 0", len(got))
	}
}

func TestRunParallelRunsEachIndexOnce(t *testing.T) {
	const n = 50

	var hits [n]int32

	runParallel(n, 4, func(i int) { atomic.AddInt32(&hits[i], 1) })

	for i := range hits {
		if hits[i] != 1 {
			t.Fatalf("index %d ran %d times", i, hits[i])
		}
	}

	runParallel(0, 4, func(int) { t.Fatal("nothing to run") })
}

func TestKeyLocksSerialiseSameKey(t *testing.T) {
	var k keyLocks

	var inside, overlap int32

	var wg sync.WaitGroup

	for i := 0; i < 16; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			unlock := k.lock("a")
			if atomic.AddInt32(&inside, 1) > 1 {
				atomic.AddInt32(&overlap, 1)
			}

			atomic.AddInt32(&inside, -1)
			unlock()
		}()
	}

	wg.Wait()

	if overlap != 0 {
		t.Fatalf("%d goroutines held the same key at once", overlap)
	}

	if len(k.m) != 0 {
		t.Fatalf("lock table not cleaned: %d entries", len(k.m))
	}

	// different keys do not block each other
	u1 := k.lock("x")
	u2 := k.lock("y")
	u2()
	u1()
}
