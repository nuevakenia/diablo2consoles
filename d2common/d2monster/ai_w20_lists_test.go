package d2monster

import (
	"strings"
	"testing"
)

// The wave 20 list counts only handlers no earlier batch list counted.
func TestW20ListsAreNew(t *testing.T) {
	seen := map[string]bool{}

	for _, l := range [][]string{FaithfulA(), FaithfulB(), FaithfulBatch3(), FaithfulBatch4(), FaithfulBatch5(),
		FaithfulBatch7(), FaithfulBatch8(), FaithfulSpawners()} {
		for _, n := range l {
			seen[strings.ToLower(n)] = true
		}
	}

	for _, n := range FaithfulW20() {
		if seen[strings.ToLower(n)] {
			t.Errorf("%s already counted by an earlier batch", n)
		}

		if _, ok := AITargetMode(n); !ok {
			t.Errorf("%s is not an exe AI name", n)
		}
	}

	if len(FaithfulW20()) != 5 {
		t.Errorf("w20 counts %d", len(FaithfulW20()))
	}

	for _, n := range RefixedW20() {
		if !seen[strings.ToLower(n)] {
			t.Errorf("%s was not counted before", n)
		}
	}
}
