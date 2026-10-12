package d2gamescreen

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// parallelFor runs fn(0..n-1) on up to workers goroutines (at most GOMAXPROCS) and waits. fn must only
// read shared state and write its own result slot; the caller applies the results in index order, so the
// outcome does not depend on scheduling.
func parallelFor(n, workers int, fn func(i int)) {
	if m := runtime.GOMAXPROCS(0); workers > m {
		workers = m
	}

	if workers > n {
		workers = n
	}

	if workers <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}

		return
	}

	next := int64(-1)

	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				i := int(atomic.AddInt64(&next, 1))
				if i >= n {
					return
				}

				fn(i)
			}
		}()
	}

	wg.Wait()
}
