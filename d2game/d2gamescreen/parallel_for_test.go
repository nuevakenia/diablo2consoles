package d2gamescreen

import (
	"sync/atomic"
	"testing"
)

func TestParallelForVisitsEachIndexOnce(t *testing.T) {
	for _, workers := range []int{0, 1, 3, 64} {
		for _, n := range []int{0, 1, 7, 500} {
			hits := make([]int32, n)

			parallelFor(n, workers, func(i int) { atomic.AddInt32(&hits[i], 1) })

			for i, h := range hits {
				if h != 1 {
					t.Fatalf("workers=%d n=%d: index %d ran %d times", workers, n, i, h)
				}
			}
		}
	}
}
