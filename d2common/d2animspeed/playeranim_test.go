package d2animspeed

import "testing"

// Wave 31: per weapon class AnimData records of the heroes and the Druid forms (VERIFIED against the expansion
// animdata.d2), the attack length rule and the start frames.

func TestPlayerRecordAnchors(t *testing.T) {
	for _, tc := range []struct {
		tok, mode, wc string
		want          Mode
	}{
		{"PA", "BL", "1HS", Mode{3, 256}}, // the shield never changes the record
		{"PA", "BL", "HTH", Mode{3, 256}},
		{"PA", "BL", "2HS", Mode{3, 168}},
		{"AM", "BL", "1HS", Mode{3, 88}},
		{"DZ", "GH", "1HS", Mode{7, 248}},
		{"DZ", "GH", "HTH", Mode{7, 256}},
		{"PA", "GH", "2HT", Mode{5, 192}},
		{"PA", "GH", "STF", Mode{5, 192}},
		{TokenWerewolf, "GH", "HTH", Mode{4, 256}},
		{TokenWerebear, "GH", "HTH", Mode{5, 184}},
		{TokenWerewolf, "BL", "HTH", Mode{5, 256}},
		{TokenWerebear, "BL", "HTH", Mode{5, 200}},
		{"SO", "A1", "STF", Mode{18, 256}},
		{"PA", "A1", "1HS", Mode{15, 256}},
		{"BA", "A1", "2HS", Mode{18, 256}},
		{"AI", "A1", "HT1", Mode{11, 208}},
		{"ai", "a1", "ht1", Mode{11, 208}}, // case is ignored
	} {
		got, ok := PlayerRecord(tc.tok, tc.mode, tc.wc)
		if !ok || got != tc.want {
			t.Errorf("%s%s%s = %v %v, want %v", tc.tok, tc.mode, tc.wc, got, ok, tc.want)
		}
	}

	if _, ok := PlayerRecord("AM", "BL", "2HS"); ok {
		t.Error("an Amazon has no block record for a two handed sword")
	}
}

func TestAttackStartFrame(t *testing.T) {
	for _, tc := range []struct {
		class, wc string
		want      int
	}{
		{"Amazon", "HTH", 1}, {"Sorceress", "hth", 1},
		{"Amazon", "1HS", 2}, {"Sorceress", "STF", 2}, {"Amazon", "2HT", 2},
		{"Amazon", "BOW", 0}, {"Sorceress", "XBW", 0},
		{"Paladin", "1HS", 0}, {"Barbarian", "2HS", 0}, {"Assassin", "HT1", 0}, {"Druid", "STF", 0},
	} {
		if got := AttackStartFrame(tc.class, tc.wc); got != tc.want {
			t.Errorf("start frame %s %s = %d, want %d", tc.class, tc.wc, got, tc.want)
		}
	}
}

// attackRef is an independent statement of the rule: the percent is stat 0x44 plus the diminished IAS clamped to
// 15..175, the rate is speed*percent/100, the animation runs from the start frame to frames and ends one event later.
func attackRef(frames, speed, start, stat44, ias int) int {
	eff := ias
	if ias > 0 {
		eff = 120 * ias / (120 + ias)
	}

	pct := stat44 + eff
	if pct < 15 {
		pct = 15
	}

	if pct > 175 {
		pct = 175
	}

	rate := speed * pct / 100
	n := ((frames-start)*256 + rate - 1) / rate

	if n < 2 {
		n = 2
	}

	return n
}

func TestAttackTicksMatchesRule(t *testing.T) {
	for _, m := range []Mode{{16, 256}, {15, 256}, {11, 208}, {20, 256}, {13, 256}, {14, 224}} {
		for _, start := range []int{0, 1, 2} {
			for _, stat44 := range []int{80, 90, 100, 110, 130} {
				for ias := 0; ias <= 300; ias += 7 {
					if got, want := AttackTicks(m, start, stat44, ias), attackRef(m.Frames, m.Speed, start, stat44, ias); got != want {
						t.Fatalf("%v start %d stat44 %d ias %d: %d want %d", m, start, stat44, ias, got, want)
					}
				}
			}
		}
	}
}

func TestAttackAnchors(t *testing.T) {
	// Paladin one handed swing: 15 frames, no start frame; the first breakpoint is the IAS that lifts the rate
	// to 15/14 of normal: 108 percent, i.e. a diminished 8, i.e. 9 IAS (120*9/129 = 8).
	pal := Mode{15, 256}
	if got := AttackTicks(pal, 0, AttackStat44(0, 0), 0); got != 15 {
		t.Errorf("paladin 1hs at 0 ias: %d", got)
	}

	bp := AttackBreakpoints(pal, 0, AttackStat44(0, 0))
	if len(bp) < 3 || bp[0] != 0 || bp[1] != 9 {
		t.Errorf("paladin 1hs breakpoints %v", bp)
	}

	// Sorceress staff: 18 frames from frame 2 = 16 frames at normal speed.
	if got := AttackTicks(Mode{18, 256}, AttackStartFrame("Sorceress", "STF"), AttackStat44(0, 0), 0); got != 16 {
		t.Errorf("sorceress staff: %d", got)
	}

	// A slow weapon (speed +10) lengthens the swing, a fast one (-20) shortens it.
	slow := AttackTicks(pal, 0, AttackStat44(10, 0), 0)
	fast := AttackTicks(pal, 0, AttackStat44(-20, 0), 0)

	if !(fast < 15 && 15 < slow) {
		t.Errorf("wsm: fast %d normal 15 slow %d", fast, slow)
	}

	// the clamp: no amount of IAS goes beyond 175 percent
	if a, b := AttackTicks(pal, 0, 100, 500), AttackTicks(pal, 0, 100, 5000); a != b {
		t.Errorf("clamp: %d %d", a, b)
	}
}
