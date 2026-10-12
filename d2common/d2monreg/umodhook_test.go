package d2monreg

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

func TestCursedRoll(t *testing.T) {
	s := d2rand.New(12345)

	// a non unique carrier never rolls and never steps the seed
	before := *s

	if cast, _ := CursedRoll(s, false, 50); cast || *s != before {
		t.Fatalf("non unique: cast %v seed moved %v", cast, *s != before)
	}

	// an independent replay of the exe's test: step, skip when the low two bits are zero
	ref := d2rand.New(12345)
	casts := 0

	for i := 0; i < 400; i++ {
		want := ref.Step()&3 != 0
		got, lvl := CursedRoll(s, true, 37)

		if got != want {
			t.Fatalf("roll %d: cast %v want %v", i, got, want)
		}

		if got {
			casts++

			if lvl != 8 {
				t.Fatalf("level %d want 8 (37/5+1)", lvl)
			}
		}
	}

	if casts < 250 || casts > 350 { // three in four
		t.Errorf("%d casts in 400, want about 300", casts)
	}
}

func TestCursedLevelAndRadius(t *testing.T) {
	for in, want := range map[int]int{-3: 1, 0: 1, 4: 1, 5: 2, 9: 2, 10: 3, 85: 18} {
		if got := CursedLevel(in); got != want {
			t.Errorf("CursedLevel(%d)=%d want %d", in, got, want)
		}
	}

	for in, want := range map[int]int{-5: 1, 0: 1, 1: 1, 2: 2, 39: 39, 40: 40, 99: 40} {
		if got := CurseRadius(in); got != want {
			t.Errorf("CurseRadius(%d)=%d want %d", in, got, want)
		}
	}
}

func TestMultishot(t *testing.T) {
	if !MultishotEligible(true, false, false) || MultishotEligible(false, false, false) ||
		MultishotEligible(true, true, false) || MultishotEligible(true, false, true) {
		t.Fatal("eligibility")
	}

	// monster at (10,10), aim at (20,10): step (-1, 0): points (20-0, 10-1) and (20+0, 10+1)
	got := MultishotTargets(10, 10, 20, 10, 5)
	if got != [2][2]int{{20, 9}, {20, 11}} {
		t.Errorf("east aim: %v", got)
	}

	// aim at (4,14): dx=+1, dy=-1 -> (tx-dy, ty+dx) = (5,15) and (tx+dy, ty-dx) = (3,13)
	got = MultishotTargets(10, 10, 4, 14, 5)
	if got != [2][2]int{{5, 15}, {3, 13}} {
		t.Errorf("diagonal aim: %v", got)
	}

	// the rows 0x3f..0x42 aim both extra missiles at the aim point
	for _, row := range []int{0x3f, 0x40, 0x41, 0x42} {
		if got := MultishotTargets(10, 10, 20, 3, row); got != [2][2]int{{20, 3}, {20, 3}} {
			t.Errorf("row %#x: %v", row, got)
		}
	}

	if got := MultishotTargets(10, 10, 20, 3, 0x43); got == [2][2]int{{20, 3}, {20, 3}} {
		t.Error("row 0x43 must not be degenerate")
	}
}

func TestTimerWakes(t *testing.T) {
	for _, c := range []struct {
		mode, class int
		alive, wake bool
		name        string
	}{
		{0, 5, false, false, "death mode"}, {0xc, 5, false, false, "dead mode"},
		{1, 5, true, true, "neutral"}, {2, 5, true, false, "walking other class"},
		{2, 0x6e, true, true, "walking special class"}, {5, 0x76, true, true, "attack special class"},
		{2, 0x88, true, true, "0x88"}, {2, 0xf7, true, true, "0xf7"}, {2, 0x6f, true, false, "0x6f"},
	} {
		alive, wake := TimerWakes(c.mode, c.class)
		if alive != c.alive || wake != c.wake {
			t.Errorf("%s: alive %v wake %v want %v %v", c.name, alive, wake, c.alive, c.wake)
		}
	}
}

func TestRollRareName(t *testing.T) {
	// replay with the generator by hand: seed (name seed, 0x29a); suffix, prefix, then 100, then (long form)
	// appellation, suffix, prefix
	for seed := 0; seed < 400; seed++ {
		const np, ns, na = 213, 190, 64

		s := d2rand.New(uint32(seed))
		wantSuf := int(s.Roll(ns))
		wantPre := int(s.Roll(np))
		long := s.Roll(100) < 50

		got := RollRareName(uint16(seed), np, ns, na)
		if got.Suffix1 != wantSuf || got.Prefix1 != wantPre || got.Long != long {
			t.Fatalf("seed %d: %+v want suffix %d prefix %d long %v", seed, got, wantSuf, wantPre, long)
		}

		if !long {
			if got.Prefix != wantPre || got.Suffix != wantSuf {
				t.Fatalf("seed %d short: %+v", seed, got)
			}

			continue
		}

		app := int(s.Roll(na))
		suf := int(s.Roll(ns))
		pre := int(s.Roll(np))

		if got.Appellation != app || got.Suffix != suf || got.Prefix != pre {
			t.Fatalf("seed %d long: %+v want app %d suffix %d prefix %d", seed, got, app, suf, pre)
		}
	}

	// an empty table neither rolls nor panics
	if got := RollRareName(7, 0, 0, 0); got.Prefix != 0 || got.Suffix != 0 {
		t.Errorf("empty tables: %+v", got)
	}
}

func TestFixedNameString(t *testing.T) {
	for class, want := range map[int]int{0x9c: 0xbcd, 0xd3: 0xbee, 0x2c4: 0xbee, 0xf3: 0xbf4, 0x14d: 0xbf4, 0x2c3: 0x2b92} {
		if got, ok := FixedNameString(class); !ok || got != want {
			t.Errorf("class %#x: %#x %v want %#x", class, got, ok, want)
		}
	}

	if _, ok := FixedNameString(0x10); ok {
		t.Error("class 0x10 has a random name")
	}
}
