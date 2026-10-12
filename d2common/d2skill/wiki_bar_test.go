package d2skill

import "testing"

// Barbarian cross check against the community wiki numbers for 1.14 LoD (wave 30). The expected values are
// the numbers of the wiki formulas evaluated here (numbers only); the engine side is the real patch_d2
// skills.txt through the calc evaluator (D2_TABLES; skipped when unset). Frames are 25 per second, yards
// are 3/2 of a subtile unit.
//
// Where the wiki page of another patch (D2R / 2.4: War Cry stun and mana, Whirlwind 30/50, Fire Arrow mana flat 3)
// differs, the table is the truth for 1.14b and the test follows the table.

type barCase struct {
	skill string
	eff   int            // effective level (hard points + bonus)
	hard  int            // hard points of the skill itself (0 = eff)
	syn   map[string]int // hard points of other skills
	what  string
	got   func(sk *Skill, e *Env) int
	want  int
}

func (c barCase) run(t *testing.T, reg *Registry) {
	t.Helper()

	base := map[string]int{}
	for k, v := range c.syn {
		base[k] = v
	}

	hard := c.hard
	if hard == 0 {
		hard = c.eff
	}

	base[c.skill] = hard

	sk, e := wikiEnv(reg, c.skill, c.eff, base)
	if sk == nil {
		t.Fatalf("%s missing", c.skill)
	}

	if got := c.got(sk, e); got != c.want {
		t.Errorf("%s L%d (hard %d) %s: engine %d, wiki %d", c.skill, c.eff, hard, c.what, got, c.want)
	}
}

func fToHit(sk *Skill, e *Env) int { return sk.ToHitBonus(e, e.Level) }
func fCalc(n int) func(*Skill, *Env) int {
	return func(sk *Skill, e *Env) int { return e.eval(sk.Calc[n]) }
}
func fAuraLen(sk *Skill, e *Env) int   { return e.eval(sk.AuraLenCalc) }
func fAuraRange(sk *Skill, e *Env) int { return e.eval(sk.AuraRangeCalc) }
func fAuraStat(i int) func(*Skill, *Env) int {
	return func(sk *Skill, e *Env) int { return e.eval(sk.AuraStatCalc[i]) }
}
func fPassive(i int) func(*Skill, *Env) int {
	return func(sk *Skill, e *Env) int { return e.eval(sk.PassiveCalc[i]) }
}
func fMana(sk *Skill, e *Env) int    { return sk.ManaCost(e.Level) }
func fELen(sk *Skill, e *Env) int    { return sk.ElemLen(e, e.Level) }
func fPhysMin(sk *Skill, e *Env) int { return int(sk.PhysMin(e, e.Level, 0, false) >> 8) }
func fPhysMax(sk *Skill, e *Env) int { return int(sk.PhysMax(e, e.Level, 0, false) >> 8) }

func min2(a, b int) int {
	if a < b {
		return a
	}

	return b
}

// mulDivF is the wiki's "[110*slvl/(slvl+6)]" diminishing returns term (integer division as in the exe's dm12).
func dmWiki(base, slvl int) int { return base * (110 * slvl / (slvl + 6)) / 100 }

func TestWikiBarbarianActive(t *testing.T) {
	reg := loadRealRegistry(t)

	var cs []barCase

	add := func(c barCase) { cs = append(cs, c) }

	for _, l := range []int{1, 10, 20, 23} {
		// Bash: +AR 15+5*slvl, +damage 45+5*slvl, flat damage slvl (calc2); synergy Stun 5, Concentrate 5
		add(barCase{skill: "Bash", eff: l, what: "AR", got: fToHit, want: 15 + 5*l})
		add(barCase{skill: "Bash", eff: l, what: "dmg%", got: fCalc(1), want: 45 + 5*l})
		add(barCase{skill: "Bash", eff: l, what: "flat", got: fCalc(2), want: l})
		add(barCase{skill: "Bash", eff: l, syn: map[string]int{"Stun": 20, "Concentrate": 20}, what: "AR+syn", got: fToHit, want: 15 + 5*l + 5*20})
		add(barCase{skill: "Bash", eff: l, syn: map[string]int{"Stun": 20, "Concentrate": 20}, what: "dmg+syn", got: fCalc(1), want: 45 + 5*l + 5*20})
		add(barCase{skill: "Bash", eff: l, what: "mana", got: fMana, want: 2 << 8})

		// Stun: AR 10+5*slvl, length frames 25*(1.2.. ) from the table, Bash synergy 8, War Cry stun synergy 5
		add(barCase{skill: "Stun", eff: l, what: "AR", got: fToHit, want: 10 + 5*l})
		add(barCase{skill: "Stun", eff: l, syn: map[string]int{"Bash": 20}, what: "dmg+syn", got: fCalc(1), want: 8 * 20})

		// Double Swing: AR 10+5*slvl, damage 10*Bash, mana max(1.125-0.125*slvl,0)
		add(barCase{skill: "Double Swing", eff: l, what: "AR", got: fToHit, want: 10 + 5*l})
		add(barCase{skill: "Double Swing", eff: l, syn: map[string]int{"Bash": 20}, what: "dmg+syn", got: fCalc(1), want: 200})

		// Double Throw: AR 10+10*slvl, synergy Double Swing 8
		add(barCase{skill: "Double Throw", eff: l, what: "AR", got: fToHit, want: 10 + 10*l})
		add(barCase{skill: "Double Throw", eff: l, syn: map[string]int{"Double Swing": 20}, what: "dmg+syn", got: fCalc(1), want: 160})

		// Concentrate: defense 90+10*slvl, damage 65+5*slvl, AR 50+10*slvl, synergy Bash 5, Battle Orders 10, Berserk magic 1
		add(barCase{skill: "Concentrate", eff: l, what: "AR", got: fToHit, want: 50 + 10*l})
		add(barCase{skill: "Concentrate", eff: l, what: "dmg%", got: fCalc(1), want: 65 + 5*l})
		add(barCase{skill: "Concentrate", eff: l, what: "def", got: fAuraStat(1), want: 90 + 10*l})
		add(barCase{skill: "Concentrate", eff: l, syn: map[string]int{"Bash": 20, "Battle Orders": 20}, what: "dmg+syn", got: fCalc(1), want: 65 + 5*l + 100 + 200})
		add(barCase{skill: "Concentrate", eff: l, syn: map[string]int{"Berserk": 20}, what: "magic", got: fCalc(4), want: 20})

		// Frenzy: damage 85+5*slvl, AR 93+7*slvl, duration 6 s, synergy Double Swing 8 + Taunt 8, Berserk magic 1
		add(barCase{skill: "Frenzy", eff: l, what: "dmg%", got: fCalc(1), want: 85 + 5*l})
		add(barCase{skill: "Frenzy", eff: l, what: "AR", got: fToHit, want: 93 + 7*l})
		add(barCase{skill: "Frenzy", eff: l, what: "dur", got: fAuraLen, want: 150})
		add(barCase{skill: "Frenzy", eff: l, syn: map[string]int{"Double Swing": 20, "Taunt": 20}, what: "dmg+syn", got: fCalc(1), want: 85 + 5*l + 8*40})

		// Berserk: damage 135+15*slvl (magic), AR 85+15*slvl, synergy Howl 10 + Shout 10; debuff 3 s - ..., table-driven frames
		add(barCase{skill: "Berserk", eff: l, what: "dmg%", got: fCalc(1), want: 135 + 15*l})
		add(barCase{skill: "Berserk", eff: l, what: "AR", got: fToHit, want: 85 + 15*l})
		add(barCase{skill: "Berserk", eff: l, syn: map[string]int{"Howl": 20, "Shout": 20}, what: "dmg+syn", got: fCalc(1), want: 135 + 15*l + 400})

		// Leap Attack: AR 35+15*slvl, damage 70+30*slvl (+10 per Leap level)
		add(barCase{skill: "Leap Attack", eff: l, what: "AR", got: fToHit, want: 35 + 15*l})
		add(barCase{skill: "Leap Attack", eff: l, what: "dmg%", got: fCalc(1), want: 70 + 30*l})
		add(barCase{skill: "Leap Attack", eff: l, syn: map[string]int{"Leap": 20}, what: "dmg+syn", got: fCalc(1), want: 70 + 30*l + 200})

		// Leap: knockback radius (3+slvl)*2/3 yards = 3+slvl subtile units (calc1)
		add(barCase{skill: "Leap", eff: l, what: "radius", got: fCalc(1), want: 3 + l})

		// Whirlwind (LoD): AR 5*slvl-5, damage 8*slvl-58, mana 12+0.5*slvl
		add(barCase{skill: "Whirlwind", eff: l, what: "AR", got: fToHit, want: 5*l - 5})
		add(barCase{skill: "Whirlwind", eff: l, what: "dmg%", got: fCalc(1), want: 8*l - 58})
		add(barCase{skill: "Whirlwind", eff: l, what: "mana", got: fMana, want: (24 + l) * 128})

		// War Cry (LoD): mana 9+slvl, damage tiers, synergy 6 per Howl/Taunt/Battle Cry level
		add(barCase{skill: "War Cry", eff: l, what: "mana", got: fMana, want: (9 + l) << 8})

		// Howl: flee duration 2+slvl s (Param5+Param6*(lvl-1) frames), distance (19+5*slvl)*2/3 yards
		add(barCase{skill: "Howl", eff: l, what: "flee frames", got: func(sk *Skill, e *Env) int { return sk.Params[5] + sk.Params[6]*(e.Level-1) }, want: 25 * (2 + l)})
		add(barCase{skill: "Howl", eff: l, what: "flee dist", got: func(sk *Skill, e *Env) int { return sk.Params[3] + sk.Params[4]*(e.Level-1) }, want: (19 + 5*l) * 2 / 2 * 3 / 3})

		// Taunt: -(3+2*slvl) damage and AR
		add(barCase{skill: "Taunt", eff: l, what: "dmg", got: fAuraStat(2), want: -(3 + 2*l)})

		// Shout: defense 90+10*slvl, duration 10+10*slvl s; synergy 5 s per Battle Orders / Battle Command level
		add(barCase{skill: "Shout", eff: l, what: "def", got: fAuraStat(1), want: 90 + 10*l})
		add(barCase{skill: "Shout", eff: l, what: "dur", got: fAuraLen, want: 25 * (10 + 10*l)})
		add(barCase{skill: "Shout", eff: l, syn: map[string]int{"Battle Orders": 20, "Battle Command": 20}, what: "dur+syn", got: fAuraLen, want: 25 * (10 + 10*l + 200)})

		// Battle Orders: 32+3*slvl percent life/mana/stamina, duration 20+10*slvl s
		add(barCase{skill: "Battle Orders", eff: l, what: "pct", got: fAuraStat(1), want: 32 + 3*l})
		add(barCase{skill: "Battle Orders", eff: l, what: "dur", got: fAuraLen, want: 25 * (20 + 10*l)})
		add(barCase{skill: "Battle Orders", eff: l, syn: map[string]int{"Shout": 20, "Battle Command": 20}, what: "dur+syn", got: fAuraLen, want: 25 * (20 + 10*l + 200)})

		// Battle Command: duration 10*slvl-5 s, +1 all skills
		add(barCase{skill: "Battle Command", eff: l, what: "dur", got: fAuraLen, want: 25 * (10*l - 5)})
		add(barCase{skill: "Battle Command", eff: l, what: "allskills", got: fAuraStat(1), want: 1})

		// Battle Cry: defense -(48+2*slvl), damage -(24+slvl), duration 9.6+2.4*slvl s
		add(barCase{skill: "Battle Cry", eff: l, what: "def", got: fAuraStat(1), want: -(48 + 2*l)})
		add(barCase{skill: "Battle Cry", eff: l, what: "dmg", got: fAuraStat(2), want: -(24 + l)})
		add(barCase{skill: "Battle Cry", eff: l, what: "dur", got: fAuraLen, want: 240 + 60*l})

		// Grim Ward: radius (2+slvl)*2/3 yards = 2+slvl subtile units, duration 40 s
		add(barCase{skill: "Grim Ward", eff: l, what: "radius", got: fAuraRange, want: 2 + l})
		add(barCase{skill: "Grim Ward", eff: l, what: "dur", got: fCalc(1), want: 1000})

		// Find Potion / Find Item: min(110*slvl/(slvl+6),100) and min(5+55*(..)/100,60)
		add(barCase{skill: "Find Potion", eff: l, what: "chance", got: fCalc(1), want: min2(110*l/(l+6), 100)})
		add(barCase{skill: "Find Item", eff: l, what: "chance", got: fCalc(1), want: min2(5+dmWiki(55, l), 60)})
	}

	for _, c := range cs {
		c.run(t, reg)
	}
}

func TestWikiBarbarianPassive(t *testing.T) {
	reg := loadRealRegistry(t)

	var cs []barCase

	add := func(c barCase) { cs = append(cs, c) }

	for _, l := range []int{1, 10, 20, 23, 30} {
		add(barCase{skill: "Iron Skin", eff: l, what: "def%", got: fPassive(1), want: 20 + 10*l})
		add(barCase{skill: "Increased Stamina", eff: l, what: "stam%", got: fPassive(1), want: 15 + 15*l})
		add(barCase{skill: "Increased Speed", eff: l, what: "move%", got: fPassive(1), want: min2(7+dmWiki(43, l), 50)})
		add(barCase{skill: "Natural Resistance", eff: l, what: "res", got: fPassive(1), want: min2(dmWiki(80, l), 80)})

		// weapon masteries: AR, damage, critical (crit differs by one from the wiki formula at 10 and 30; the table values
		// listed by the wiki, 23 and 31, are the engine's)
		add(barCase{skill: "Sword Mastery", eff: l, what: "AR", got: fPassive(1), want: 20 + 8*l})
		add(barCase{skill: "Sword Mastery", eff: l, what: "dmg", got: fPassive(2), want: 23 + 5*l})
		add(barCase{skill: "Axe Mastery", eff: l, what: "AR", got: fPassive(1), want: 20 + 8*l})
		add(barCase{skill: "Mace Mastery", eff: l, what: "dmg", got: fPassive(2), want: 23 + 5*l})
		add(barCase{skill: "Pole Arm Mastery", eff: l, what: "AR", got: fPassive(1), want: 22 + 8*l})
		add(barCase{skill: "Spear Mastery", eff: l, what: "dmg", got: fPassive(2), want: 23 + 5*l})
		add(barCase{skill: "Throwing Mastery", eff: l, what: "AR", got: fPassive(1), want: 22 + 8*l})
		add(barCase{skill: "Throwing Mastery", eff: l, what: "dmg", got: fPassive(2), want: 23 + 5*l})
	}

	// critical strike chance as printed on the wiki pages (the formula there is off by one at 10 and 30)
	for l, w := range map[int]int{1: 5, 10: 23, 20: 29, 30: 31} {
		add(barCase{skill: "Sword Mastery", eff: l, what: "crit", got: fPassive(3), want: w})
		add(barCase{skill: "Throwing Mastery", eff: l, what: "crit", got: fPassive(3), want: w})
	}

	for _, c := range cs {
		c.run(t, reg)
	}
}

// Wiki values taken as printed (frames = seconds * 25): stun length of Stun, Berserk debuff time, Frenzy speed.
func TestWikiBarbarianPrinted(t *testing.T) {
	reg := loadRealRegistry(t)

	var cs []barCase

	add := func(c barCase) { cs = append(cs, c) }

	// Stun length 1.2 / 3.0 / 4.52 s
	add(barCase{skill: "Stun", eff: 1, what: "len", got: fELen, want: 30})
	add(barCase{skill: "Stun", eff: 10, what: "len", got: fELen, want: 75})
	add(barCase{skill: "Stun", eff: 20, what: "len", got: fELen, want: 113})
	// War Cry synergy on the stun of Stun: +5 percent per level at 20
	add(barCase{skill: "Stun", eff: 1, syn: map[string]int{"War Cry": 20}, what: "len+syn", got: fELen, want: 60})

	// Berserk debuff time 2.72 / 1.64 / 1.32 s
	add(barCase{skill: "Berserk", eff: 1, what: "debuff", got: fAuraLen, want: 68})
	add(barCase{skill: "Berserk", eff: 10, what: "debuff", got: fAuraLen, want: 41})
	add(barCase{skill: "Berserk", eff: 20, what: "debuff", got: fAuraLen, want: 33})

	// Frenzy walk and attack speed
	add(barCase{skill: "Frenzy", eff: 1, what: "walk", got: fAuraStat(1), want: 47})
	add(barCase{skill: "Frenzy", eff: 10, what: "walk", got: fAuraStat(1), want: 142})
	add(barCase{skill: "Frenzy", eff: 20, what: "walk", got: fAuraStat(1), want: 171})
	add(barCase{skill: "Frenzy", eff: 1, what: "attack", got: fAuraStat(2), want: 7})
	add(barCase{skill: "Frenzy", eff: 10, what: "attack", got: fAuraStat(2), want: 34})
	add(barCase{skill: "Frenzy", eff: 20, what: "attack", got: fAuraStat(2), want: 42})

	// Leap range (Leap aura range) 4 2/3, 14, 16 2/3 yards = 7, 21, 25 units
	add(barCase{skill: "Leap", eff: 1, what: "range", got: fAuraRange, want: 7})
	add(barCase{skill: "Leap", eff: 10, what: "range", got: fAuraRange, want: 21})
	add(barCase{skill: "Leap", eff: 20, what: "range", got: fAuraRange, want: 25})

	for _, c := range cs {
		c.run(t, reg)
	}
}

// War Cry physical damage (LoD wiki 20-30, 76-86, 150-160, 240-250 at 1, 10, 20, 30) and the synergy percent.
func TestWikiWarCryDamage(t *testing.T) {
	reg := loadRealRegistry(t)

	for _, tc := range []struct{ lvl, min, max int }{{1, 20, 30}, {10, 76, 86}, {20, 150, 160}, {30, 240, 250}} {
		c := barCase{skill: "War Cry", eff: tc.lvl, what: "min", got: fPhysMin, want: tc.min}
		c.run(t, reg)

		c = barCase{skill: "War Cry", eff: tc.lvl, what: "max", got: fPhysMax, want: tc.max}
		c.run(t, reg)

		// 6 percent per Howl, Taunt and Battle Cry level: all three at 20 = +360 percent
		c = barCase{skill: "War Cry", eff: tc.lvl, syn: map[string]int{"Howl": 20, "Taunt": 20, "Battle Cry": 20},
			what: "min+syn", got: fPhysMin, want: tc.min * 460 / 100}
		c.run(t, reg)
	}
}

// Double Swing mana: the wiki gives max(1.125-0.125*slvl, 0), the table has 8 - 1/level in 1/32 units with no floor.
func TestWikiDoubleSwingMana(t *testing.T) {
	reg := loadRealRegistry(t)

	for l := 1; l <= 12; l++ {
		w := 8 - (l - 1)
		if w < 0 {
			w = 0
		}

		c := barCase{skill: "Double Swing", eff: l, what: "mana", got: fMana, want: w << 5}
		c.run(t, reg)
	}
}
