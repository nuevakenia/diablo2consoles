package d2monsters

import "testing"

// TestRolledModCount pins the unique modifier count of 0x59e330: 1 + difficulty.
func TestRolledModCount(t *testing.T) {
	for diff, want := range []int{1, 2, 3} {
		if got := rolledModCount(diff, false); got != want {
			t.Fatalf("difficulty %d: got %d want %d", diff, got, want)
		}
	}

	if got := rolledModCount(2, true); got != 1 {
		t.Fatalf("champion: got %d", got)
	}
}
