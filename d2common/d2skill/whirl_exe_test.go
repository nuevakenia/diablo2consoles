package d2skill

import "testing"

func TestWhirlSchedule(t *testing.T) {
	// 0x5d7de0: the first call hits once and stores frame+4, then one hit per period
	s := &WhirlSched{}

	var got []int

	for f := 100; f <= 120; f++ {
		got = append(got, s.Step(f, 6, false))
	}

	hitFrames := []int{}

	for i, n := range got {
		if n > 0 {
			hitFrames = append(hitFrames, 100+i)
		}
	}

	want := []int{100, 104, 110, 116}
	if len(hitFrames) != len(want) {
		t.Fatalf("hit frames %v, want %v", hitFrames, want)
	}

	for i := range want {
		if hitFrames[i] != want[i] {
			t.Fatalf("hit frames %v, want %v", hitFrames, want)
		}
	}

	// dual wielding hits twice per tick
	d := &WhirlSched{}
	d.Step(10, 6, true)

	if n := d.Step(14, 6, true); n != 2 {
		t.Errorf("dual tick hits = %d", n)
	}

	// a late tick restarts the period from the current frame
	l := &WhirlSched{}
	l.Step(10, 6, false)
	l.Step(14, 6, false)

	if n := l.Step(40, 6, false); n != 1 || l.Next != 46 {
		t.Errorf("late tick n=%d next=%d", n, l.Next)
	}
}

func TestWhirlNextRoundRobin(t *testing.T) {
	ids := []string{"c", "a", "b"}

	for _, tc := range []struct {
		last string
		want string
	}{{"", "a"}, {"a", "b"}, {"b", "c"}, {"c", "a"}, {"zz", "a"}} {
		i := WhirlNext(ids, tc.last)
		if ids[i] != tc.want {
			t.Errorf("after %q got %q, want %q", tc.last, ids[i], tc.want)
		}
	}

	// a lone monster is hit again and again
	if WhirlNext([]string{"x"}, "x") != 0 {
		t.Error("lone victim not repeated")
	}

	if WhirlNext(nil, "") != -1 {
		t.Error("empty must be -1")
	}
}
