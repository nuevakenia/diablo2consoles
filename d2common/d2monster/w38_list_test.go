package d2monster

import (
	"strings"
	"testing"
)

func TestW38ListIsNew(t *testing.T) {
	seen := map[string]bool{}

	for _, l := range [][]string{FaithfulA(), FaithfulB(), FaithfulBatch3(), FaithfulBatch4(), FaithfulBatch5(),
		FaithfulBatch7(), FaithfulBatch8(), FaithfulSpawners(), FaithfulW20(), ReverifiedBatch5(), ReverifiedBatch7(),
		ReverifiedBatch8(), ReverifiedW20()} {
		for _, n := range l {
			seen[strings.ToLower(n)] = true
		}
	}

	for _, n := range FaithfulW38() {
		if seen[strings.ToLower(n)] {
			t.Errorf("%s already listed", n)
		}

		if _, ok := AITargetMode(n); !ok {
			t.Errorf("%s is not an exe AI name", n)
		}

		if _, ok := Lookup(n); !ok {
			t.Errorf("%s is not registered", n)
		}
	}
}
