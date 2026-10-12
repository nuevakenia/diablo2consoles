package d2skill

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2calc"
)

// Rows with the patch_d2 values of the Berserk skill (wave 23).
func init() {
	rows = append(rows,
		row{"skill": "Berserk", "Id": "711", "srvstfunc": "39", "srvdofunc": "2", "aurastate": "berserk",
			"auralencalc": "par4-min(((110*lvl)/(lvl+6)*(par4-par3)/100),(par4-par3))", "aurastat1": "damageresist",
			"aurastatcalc1": "par5", "aurastat2": "armor_override_percent", "aurastatcalc2": "-100", "range": "h2h",
			"calc1": "ln12", "calc2": "par4-min(((110*lvl)/(lvl+6)*(par4-par3)/100),(par4-par3))", "calc4": "100",
			"Param1": "150", "Param2": "15", "Param3": "25", "Param4": "75", "Param5": "0", "ToHit": "100", "LevToHit": "15",
			"HitShift": "8", "SrcDam": "128", "EType": "mag", "manashift": "8", "mana": "4"},
	)
}

func TestElemConvClass(t *testing.T) {
	for name, want := range map[string]byte{"fire": 1, "ltng": 2, "mag": 3, "cold": 4, "pois": 5, "rand": 10, "burn": 11,
		"frze": 12, "stun": 0, "": 0} {
		if got := ElemConvClass(name); got != want {
			t.Errorf("ElemConvClass(%q) = %d, want %d", name, got, want)
		}
	}
}

// Berserk (0x5d82a0): every point of physical damage becomes magic because calc4 is 100; the state lasts calc2
// frames and sets the defense to zero (armor_override_percent -100).
func TestBerserkConvertsAndGivesState(t *testing.T) {
	cf := newClassFixture(map[string]int{"Berserk": 10})
	cf.u.ar = 100000
	cf.u.wmin, cf.u.wmax = 10, 10
	cf.u.roller = &seq{vals: make([]uint32, 100)}
	z := cf.addFoe("z", 2, 0)

	_, r := cf.castOn("Berserk", z)
	if !r.OK || r.Melee == nil || !r.Melee.Hit {
		t.Fatalf("berserk: %+v", r)
	}

	if d := r.Melee.Damage; d.Physical != 0 || d.Magic <= 0 {
		t.Errorf("damage not converted: phys %d magic %d", d.Physical, d.Magic)
	}

	e := effectOf(t, r, "self_state")
	// calc2 = 75 - min(110*10/16*50/100, 50) = 75 - min(34, 50) = 41 (integer steps as in the exe)
	want := cf.env("Berserk", 10).eval(cf.reg.ByName("Berserk").Calc[2])

	if e.State != "berserk" || e.Frames != want || want <= 0 {
		t.Errorf("berserk state %+v, want frames %d", e, want)
	}

	if v, ok := statOf(e.Stats, "armor_override_percent"); !ok || v != -100 {
		t.Errorf("berserk stats %+v", e.Stats)
	}

	// the state comes even when the blow misses
	cf.u.ar = 0
	cf.u.cooldowns = map[int]int{}
	_, r = cf.castOn("Berserk", z)
	effectOf(t, r, "self_state")
}

// A conversion that is only part of the damage (Concentrate / Frenzy: calc4 percent) keeps the rest physical.
func TestFrenzyConvertsCalc4Percent(t *testing.T) {
	cf := newClassFixture(map[string]int{"Frenzy": 2})
	cf.reg.ByName("Frenzy").EType = "mag"
	cf.u.ar = 100000
	cf.u.wmin, cf.u.wmax = 100, 100
	cf.u.roller = &seq{vals: make([]uint32, 100)}
	z := cf.addFoe("z", 2, 0)

	_, r := cf.castOn("Frenzy", z)
	m := r.Melees[0]

	// calc4 of the test row is 3: three percent of the blow becomes magic
	if m.Damage.Magic <= 0 || m.Damage.Physical <= 0 || m.Damage.Magic*100 > (m.Damage.Magic+m.Damage.Physical)*4 {
		t.Errorf("frenzy conversion: phys %d magic %d", m.Damage.Physical, m.Damage.Magic)
	}
}

// Frenzy swings twice. The first event never has the state (the previous blow is unknown); the next ones get it
// only after a blow that hit; a miss breaks the chain. The count is capped by the skill level and the stats are
// those of the count.
func TestFrenzyChainsOnHits(t *testing.T) {
	cf := newClassFixture(map[string]int{"Frenzy": 3})
	cf.u.ar = 100000
	cf.u.roller = &seq{vals: make([]uint32, 400)}
	z := cf.addFoe("z", 2, 0)

	count := func(r DoResult) int {
		n := 0

		for _, e := range r.Effects {
			if e.Kind == "self_state" {
				n++
			}
		}

		return n
	}

	_, r := cf.castOn("Frenzy", z)
	if len(r.Melees) != 2 || count(r) != 1 {
		t.Fatalf("first swing: %d blows %d states", len(r.Melees), count(r))
	}

	// the second swing starts after a hit: a state before each of its two events
	_, r = cf.castOn("Frenzy", z)
	if len(r.Melees) != 2 || count(r) != 2 {
		t.Fatalf("second swing: %d blows %d states", len(r.Melees), count(r))
	}

	e := effectOf(t, r, "self_state")
	if e.Stack != 3 || len(e.CountStats) != 3 {
		t.Errorf("stack %d count stats %d, want 3", e.Stack, len(e.CountStats))
	}

	// stats of count n use n as the level: dm34 climbs with the count
	a, _ := statOf(e.CountStats[1], "velocitypercent")
	b, _ := statOf(e.CountStats[3], "velocitypercent")

	if a >= b || b != d2calc.DM(20, 200, 3) || a != d2calc.DM(20, 200, 1) {
		t.Errorf("velocity by count: %d, %d", a, b)
	}

	// a miss ends the chain: no state before the next swing's first event
	miss := make([]uint32, 100)
	for i := range miss {
		miss[i] = 99
	}

	cf.u.roller = &seq{vals: miss}
	cf.u.ar = 1
	_, r = cf.castOn("Frenzy", z)

	if len(r.Melees) != 2 || r.Melees[0].Hit || r.Melees[1].Hit {
		t.Fatalf("the swing was meant to miss: %+v", r.Melees)
	}

	cf.u.roller = &seq{vals: make([]uint32, 100)}
	cf.u.ar = 100000
	_, r = cf.castOn("Frenzy", z)

	if count(r) != 1 {
		t.Errorf("after a missed swing: %d states, want 1", count(r))
	}
}

// Both events of Double Swing / Frenzy: the second one goes to the next unit id in reach, wrapping around; a
// lone enemy is struck twice and nobody outside melee range + 4 is touched.
func TestSwingSecondVictimRoundRobin(t *testing.T) {
	cf := newClassFixture(map[string]int{"Double Swing": 5, "Bash": 4})
	a := cf.addFoe("a", 2, 0)
	cf.addFoe("b", 3, 1)
	cf.addFoe("far", 30, 0)
	cf.u.ar = 100000
	cf.u.roller = &seq{vals: make([]uint32, 100)}

	_, r := cf.castOn("Double Swing", a)
	if len(r.Melees) != 2 || r.Melees[0].Target.ID() != "a" || r.Melees[1].Target.ID() != "b" {
		t.Fatalf("a then b expected: %+v", ids(r))
	}

	// the first target is the largest id: the second blow wraps to the smallest
	cf.u.cooldowns = map[int]int{}
	b := cf.foes[1]
	_, r = cf.castOn("Double Swing", b)

	if got := ids(r); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Errorf("wrap: %v", got)
	}

	// a lone enemy is struck twice
	cf.foes[1].alive = false
	cf.u.cooldowns = map[int]int{}
	_, r = cf.castOn("Double Swing", a)

	if got := ids(r); len(got) != 2 || got[0] != "a" || got[1] != "a" {
		t.Errorf("lone enemy: %v", got)
	}
}

func ids(r DoResult) []string {
	var out []string
	for _, m := range r.Melees {
		out = append(out, m.Target.ID())
	}

	return out
}

func TestOctDistAndLeapClamp(t *testing.T) {
	for _, c := range []struct{ dx, dy, want int }{{10, 0, 10}, {0, -10, 10}, {6, 8, 11}, {-8, 6, 11}, {0, 0, 0}, {1, 1, 1}} {
		if got := octDist(c.dx, c.dy); got != c.want {
			t.Errorf("octDist(%d,%d) = %d, want %d", c.dx, c.dy, got, c.want)
		}
	}

	for _, c := range []struct{ hx, hy, x, y, r, wx, wy int }{
		{0, 0, 30, 0, 10, 10, 0},  // pulled back to the radius
		{0, 0, 20, 10, 10, 8, 4},  // d = 25, scaled by 10/25
		{0, 0, 5, 5, 10, 5, 5},    // inside
		{5, 5, -20, 5, 10, -5, 5}, // negative direction truncates toward zero as in C
		{0, 0, 1, 0, 0, 0, 0},     // distance below 2 counts as 1, radius 0 keeps the caster
	} {
		if x, y := LeapClamp(c.hx, c.hy, c.x, c.y, c.r); x != c.wx || y != c.wy {
			t.Errorf("LeapClamp%v = (%d,%d), want (%d,%d)", c, x, y, c.wx, c.wy)
		}
	}
}

func TestLeapAttackLandingAndFrames(t *testing.T) {
	for _, c := range []struct {
		hx, hy, tx, ty, wx, wy int
		ok                     bool
	}{
		{0, 0, 10, 0, 12, 0, true}, // two subtiles beyond the target
		{0, 0, 0, -10, 0, -12, true},
		{0, 0, 6, 8, 7, 9, true}, // d = 11: 12/11 = 1 on each axis
		{3, 3, 3, 3, 0, 0, false},
	} {
		x, y, ok := LeapAttackLanding(c.hx, c.hy, c.tx, c.ty)
		if ok != c.ok || (ok && (x != c.wx || y != c.wy)) {
			t.Errorf("LeapAttackLanding%v = (%d,%d,%v)", c, x, y, ok)
		}
	}

	if got := LeapFrames(0, 0, 3, 4); got != 5 {
		t.Errorf("LeapFrames 3,4 = %d, want 5 (one subtile per frame)", got)
	}

	if got := LeapFrames(0, 0, 0, 0); got != 1 {
		t.Errorf("LeapFrames on the spot = %d", got)
	}
}

// Leap: range from aurarangecalc (dm12 = 4..30), the knockback waits for the landing.
func TestLeapRangeAndTiming(t *testing.T) {
	cf := b3Fixture(map[string]int{"Leap": 3})
	r := cf.env("Leap", 3).eval(cf.reg.ByName("Leap").AuraRangeCalc)

	_, res := cf.cast("Leap", 100, 0)
	m := effectOf(t, res, "move")

	if m.X != r || m.Y != 0 || m.Frames != r {
		t.Errorf("move %+v, want x=%d in %d frames", m, r, r)
	}

	k := effectOf(t, res, "knock_area")
	if k.Delay != r || k.X != r {
		t.Errorf("knock area %+v", k)
	}
}

// Leap Attack lands beyond the target and everything comes after the jump; a target already in reach is struck
// without a jump.
func TestLeapAttackBeyondTargetAndDelay(t *testing.T) {
	cf := b3Fixture(map[string]int{"Leap Attack": 5})
	z := cf.addFoe("z", 10, 0)

	_, res := cf.castOn("Leap Attack", z)
	m := effectOf(t, res, "move")

	if m.X != 12 || m.Frames != 12 {
		t.Errorf("landing %+v, want (12,0) after 12 frames", m)
	}

	if len(res.Melees) != 1 || res.Melees[0].Delay != 12 {
		t.Fatalf("blow %+v", res.Melees)
	}

	for _, kind := range []string{"knockback", "overlay", "unit_clear_state"} {
		if e := effectOf(t, res, kind); e.Delay != 12 {
			t.Errorf("%s delay %d, want 12", kind, e.Delay)
		}
	}

	// in reach: no jump, no delay
	cf2 := b3Fixture(map[string]int{"Leap Attack": 5})
	near := cf2.addFoe("near", 2, 0)
	_, res = cf2.castOn("Leap Attack", near)

	for _, e := range res.Effects {
		if e.Kind == "move" {
			t.Errorf("jump toward a target in reach: %+v", e)
		}
	}

	if len(res.Melees) != 1 || res.Melees[0].Delay != 0 {
		t.Errorf("close blow %+v", res.Melees)
	}
}
