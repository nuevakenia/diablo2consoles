package d2gamescreen

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2skills"
)

func TestRefusalNeedsHero(t *testing.T) {
	for reason, want := range map[string]bool{d2skill.ReasonLOS: true, d2skill.ReasonMana: false, d2skill.ReasonCooldown: false, "": false} {
		if got := refusalNeedsHero(reason); got != want {
			t.Errorf("refusalNeedsHero(%q) = %v, want %v", reason, got, want)
		}
	}
}

func TestParseCastPart(t *testing.T) {
	for _, c := range []struct {
		in    string
		skill string
		count int
		must  bool
	}{
		{"Bash", "Bash", castTestDefaultCount, false},
		{" Bash,1 ", "Bash", 1, false},
		{"Bash,1!", "Bash", 1, true},
		{"Chain Lightning,2!", "Chain Lightning", 2, true},
		{"Chain Lightning !", "Chain Lightning", castTestDefaultCount, true},
		{"Fire Ball,x", "Fire Ball,x", castTestDefaultCount, false},
	} {
		skill, count, must := parseCastPart(c.in)
		if skill != c.skill || count != c.count || must != c.must {
			t.Errorf("%q: got (%q,%d,%v), want (%q,%d,%v)", c.in, skill, count, must, c.skill, c.count, c.must)
		}
	}
}

func TestCastSpreadEnv(t *testing.T) {
	for _, c := range []struct {
		in    string
		state string
		n     int
	}{
		{"", "", 0},
		{"rabies", "", 0},
		{"rabies,2", "rabies", 2},
		{" rabies , 3 ", "rabies", 3},
		{"rabies,0", "", 0},
		{",2", "", 0},
	} {
		t.Setenv("OD2_AUTOCAST_SPREAD", c.in)

		if state, n := castSpreadEnv(); state != c.state || n != c.n {
			t.Errorf("%q: got (%q,%d), want (%q,%d)", c.in, state, n, c.state, c.n)
		}
	}
}

func TestCastCrowdEnv(t *testing.T) {
	for _, c := range []struct {
		in    string
		n     int
		state string
	}{
		{"", 0, ""},
		{"3", 3, ""},
		{" 3 , rabies ", 3, "rabies"},
		{"0,rabies", 0, ""},
		{"x", 0, ""},
	} {
		t.Setenv("OD2_AUTOCAST_CROWD", c.in)

		if n, state := castCrowdEnv(); n != c.n || state != c.state {
			t.Errorf("%q: got (%d,%q), want (%d,%q)", c.in, n, state, c.n, c.state)
		}
	}
}

// A mustHit skill whose cast hit nobody (the hero's minions killed the target
// in the cast animation, or the swing missed) is cast again, a bounded number
// of times; other skills never are.
func TestMissedHit(t *testing.T) {
	start := d2skills.Counters{Hits: 7, Melee: 3}
	it := &castItem{mustHit: true, c0: start}

	if !it.missedHit(d2skills.Counters{Hits: 7, Melee: 4, Missiles: 2, Misses: 1}) {
		t.Error("a swing that missed or a missile that hit nothing must be repeated")
	}

	if it.missedHit(d2skills.Counters{Hits: 8, Melee: 4}) {
		t.Error("a skill that hit once is done")
	}

	if it.missedHit(d2skills.Counters{Hits: 7, Melee: 3, AreaHits: 1}) {
		t.Error("an aura pulse (Holy Fire) that hurt someone is a hit")
	}

	it.hitRetries = castItemMaxHitRetries
	if it.missedHit(start) {
		t.Error("retries are bounded")
	}

	if (&castItem{c0: start}).missedHit(start) {
		t.Error("a skill without the mustHit flag is never repeated")
	}
}

// "!!" (ownHit) ignores area pulses: other skills' fields must not hide a miss.
func TestOwnHitIgnoresAreaPulses(t *testing.T) {
	start := d2skills.Counters{Hits: 1}
	it := &castItem{mustHit: true, ownHit: true, c0: start}

	if !it.missedHit(d2skills.Counters{Hits: 1, AreaHits: 3, Melee: 1}) {
		t.Error("area hits of other skills do not count as the strike hitting")
	}

	if it.missedHit(d2skills.Counters{Hits: 2, Melee: 1}) {
		t.Error("a strike hit is a hit")
	}

	// the swing missed but the bolt (or the blast of a missed arrow) reached the target: the direct hit is still missing
	start2 := d2skills.Counters{Hits: 1}
	it2 := &castItem{mustHit: true, ownHit: true, c0: start2, landed: func() int { return 1 }, landed0: 0}

	if !it2.missedHit(d2skills.Counters{Hits: 1, Misses: 1, Melee: 1}) {
		t.Error("a to-hit miss with no direct hit is cast again even when something else of the cast landed")
	}

	if it2.missedHit(d2skills.Counters{Hits: 2, Misses: 1, Melee: 2}) {
		t.Error("a miss followed by a hit of the same cast is a hit")
	}

	if s, n, must := parseCastPart("Bash,1!!"); s != "Bash" || n != 1 || !must {
		t.Errorf("parse: %q %d %v", s, n, must)
	}
}

// w29: "!!" counts what the skill ITSELF landed (Engine.Landed) plus missile states, not the global Hits: a late hit
// of the previous skill's missile must not hide a miss.
func TestOwnHitUsesTheSkillsOwnLandedCount(t *testing.T) {
	landed := 4
	start := d2skills.Counters{Hits: 1, States: 2}
	it := &castItem{mustHit: true, ownHit: true, c0: start, landed: func() int { return landed }, landed0: 4, states0: 2}

	if !it.missedHit(d2skills.Counters{Hits: 2, AreaHits: 1, States: 2}) {
		t.Error("a hit of another skill's missile is no hit of this skill")
	}

	landed = 5
	if it.missedHit(d2skills.Counters{Hits: 3, States: 2}) {
		t.Error("the skill's own landing is a hit")
	}

	landed = 4
	if it.missedHit(d2skills.Counters{Hits: 1, States: 3}) {
		t.Error("a state a missile ring put on a monster counts (Battle Cry)")
	}

	it.landedKey = "#explode"
	if !it.missedHit(d2skills.Counters{Hits: 1, States: 3}) {
		t.Error("a keyed item (corpse explosion) ignores states")
	}
}

func TestSplitEntrySuffixes(t *testing.T) {
	part, key := splitLandedKey("Death Sentry,1!!~explode")
	if part != "Death Sentry,1!!" || key != "#explode" {
		t.Errorf("landed key: %q %q", part, key)
	}

	part, eng := splitEngage("Battle Cry,1!!@3")
	if part != "Battle Cry,1!!" || eng != 3 {
		t.Errorf("engage: %q %d", part, eng)
	}

	if part, eng = splitEngage("Bash,1!"); part != "Bash,1!" || eng != 0 {
		t.Errorf("no suffix: %q %d", part, eng)
	}
}
