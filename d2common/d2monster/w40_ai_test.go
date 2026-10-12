package d2monster

import "testing"

// refusingImp is the Imp world whose offset cast is refused.
type refusingImp struct{ *f3World }

func (refusingImp) RandomOffsetCast(*Brain, int, int) bool { return false }

// 0x5e1ef0: the Imp think returns right after MONAI_QueueRandomOffsetSkillCast whatever it did; a refused cast
// does not fall through to the walk-away / wander tail (it sleeps instead).
func TestW40ImpOffsetCastAlwaysReturns(t *testing.T) {
	w := newF3(5)
	w.classAIP = map[[2]int]int{
		{impClassA, 1}: 50, {impClassA, 2}: 6, {impClassA, 3}: 0,
		{impClassB, 1}: 3,
		{impClassC, 1}: 0, {impClassC, 2}: 100, {impClassC, 3}: 0, {impClassC, 4}: 0,
		{impClassD, 3}: 0, {impClassD, 4}: 0,
	}
	w.inRange = true
	b := f3Brain("Imp", 1, []int{0}, 0)
	b.HPPercent = 30

	r := refusingImp{w}
	Tick(r, b)

	if len(w.log) != 0 || b.Wake != 10 {
		t.Fatalf("refused cast fell through: log=%v wake=%d", w.log, b.Wake)
	}
}

func TestW40Lists(t *testing.T) {
	for _, n := range append(FaithfulW40(), ReverifiedW40()...) {
		if _, ok := AITargetMode(n); !ok {
			t.Errorf("%s is not an exe AI name", n)
		}

		if _, ok := Lookup(n); !ok {
			t.Errorf("%s is not registered", n)
		}
	}
}

// 0x642b10: both axes lose sizeA/2 + sizeB/2 before max + min/2.
func TestW40UnitDistance(t *testing.T) {
	for _, tc := range []struct{ dx, dy, sa, sb, want int }{
		{10, 4, 1, 1, 12},  // no gap with size 1 (halves are 0)
		{10, 4, 4, 2, 7},   // gap 3: 7 and 1 -> 7 + 0
		{-10, -4, 4, 2, 7}, // signs ignored
		{2, 1, 8, 8, 0},    // clamped at zero
		{20, 20, 0, 0, 30}, // plain metric
		{0, 0, 0, 0, 0},
	} {
		if got := UnitDistance(tc.dx, tc.dy, tc.sa, tc.sb); got != tc.want {
			t.Errorf("UnitDistance(%d,%d,%d,%d) = %d, want %d", tc.dx, tc.dy, tc.sa, tc.sb, got, tc.want)
		}
	}
}
