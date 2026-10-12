package d2skill

import "testing"

func TestW38MeleeReachFull(t *testing.T) {
	blocked := func() bool { return true }
	clear := func() bool { return false }

	// touching: in reach without the trace
	if !MeleeReachFull(0, 0, 2, 1, 1, 1, -1, blocked) {
		t.Fatal("touching must not trace")
	}
	// within range, line blocked: refused; clear: allowed; nil trace: allowed
	if MeleeReachFull(3, 0, 2, 1, 1, 1, -1, blocked) || !MeleeReachFull(3, 0, 2, 1, 1, 1, -1, clear) || !MeleeReachFull(3, 0, 2, 1, 1, 1, -1, nil) {
		t.Fatal("trace gate")
	}
	// out of range: no
	if MeleeReachFull(20, 0, 2, 1, 1, 1, -1, clear) {
		t.Fatal("far")
	}
	// water watcher parts: range +8, trace ignored
	if !MeleeReachFull(6, 0, 2, 1, 1, 1, 0x102, blocked) || !MeleeReachFull(6, 0, 2, 1, 1, 1, 0x105, blocked) {
		t.Fatal("special classes")
	}
}
