package d2combat

import "testing"

// Wiki cross-check (wave 30). Wiki numbers come from the community formulas
// (Arreat Summit / Maxroll style pages, 1.14 era; recalled, WIKI-ONLY
// confidence) and are compared against the exe-ported engine functions.

func TestWikiChanceToHit(t *testing.T) {
	// wiki: chance = 200 * AR/(AR+DR) * alvl/(alvl+dlvl) percent, clamp 5..95.
	for _, tc := range []struct {
		name                   string
		ar, def, alvl, dlvl, w int
	}{
		{"equal ar def and levels", 1000, 1000, 50, 50, 50},
		{"ar 3000 vs def 1000 same level", 3000, 1000, 80, 80, 75},
		{"huge ar clamps to 95", 100000, 10, 80, 80, 95},
		{"zero ar clamps to 5", 0, 5000, 80, 80, 5},
		{"defender much higher level", 1000, 1000, 30, 90, 25},
		{"attacker above defender level", 1000, 1000, 90, 30, 75},
		{"negative defense treated as 0", 500, -500, 50, 50, 95},
	} {
		got := ToHitChance(ToHitInput{AttackRating: tc.ar, Defense: tc.def, AttackerLevel: tc.alvl, DefenderLevel: tc.dlvl})
		if got != tc.w {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.w)
		}
	}
}

func TestWikiPlayerAttackRatingAndDefense(t *testing.T) {
	// wiki: AR = (dex-7)*5 + class tohit factor (+gear); defense = dex/4 + gear.
	if got := PlayerAttackRating(0, 25, 5); got != 95 {
		t.Errorf("amazon AR %d", got)
	}

	if got := PlayerAttackRating(0, 25, -15); got != 75 {
		t.Errorf("sorc AR %d", got)
	}

	if got := Defense(0, 25, 0); got != 6 {
		t.Errorf("defense dex 25: %d", got)
	}
	// +100% enhanced defense doubles (armor + dex/4)
	if got := Defense(100, 40, 100); got != 220 {
		t.Errorf("ed defense %d", got)
	}
}

func TestWikiBlock(t *testing.T) {
	// wiki: block = (shield% + class bonus) * (dex-15) / (clvl*2), cap 75.
	in := PlayerBlockInput{HasShield: true, ToBlock: 75, ClassBlockFactor: 30, Dex: 100, Level: 80, IncludeDex: true}
	if got := PlayerBlockChance(in); got != 55 {
		t.Errorf("block %d want 55", got)
	}

	in.Dex, in.Level = 500, 30
	if got := PlayerBlockChance(in); got != 75 {
		t.Errorf("cap %d want 75", got)
	}

	in.HasShield = false
	if got := PlayerBlockChance(in); got != 0 {
		t.Errorf("no shield %d", got)
	}
}

type seqRoller struct {
	vals []int
	i    int
}

func (s *seqRoller) Roll(n int32) uint32 {
	v := s.vals[s.i%len(s.vals)]
	s.i++

	return uint32(v)
}

func TestWikiRunningBlockPenalty(t *testing.T) {
	// wiki: block chance is divided by 3 while moving.
	if !RollShieldBlock(&seqRoller{vals: []int{24}}, 75, true) {
		t.Errorf("roll 24 must block at 75/3=25")
	}

	if RollShieldBlock(&seqRoller{vals: []int{25}}, 75, true) {
		t.Errorf("roll 25 must not block at 25")
	}
}

func TestWikiResistCapsAndPenalties(t *testing.T) {
	// wiki: cap 75 (+max resist, hard 95), floor -100, difficulty penalty
	// -40 nightmare / -100 hell, physical resist cap 50.
	for _, tc := range []struct {
		name string
		in   ResistInput
		want int
	}{
		{"cap 75", ResistInput{Resist: 120}, 75},
		{"max resist +20 -> 95", ResistInput{Resist: 120, HasMaxResist: true, MaxResistBonus: 20}, 95},
		{"max resist +30 still 95", ResistInput{Resist: 120, HasMaxResist: true, MaxResistBonus: 30}, 95},
		{"nightmare penalty", ResistInput{Resist: 75, DifficultyPenalty: -40}, 35},
		{"hell penalty", ResistInput{Resist: 75, DifficultyPenalty: -100}, -25},
		{"floor -100", ResistInput{Resist: 10, DifficultyPenalty: -100, HasPierce: true, Pierce: 50}, -100},
		{"physical cap 50", ResistInput{Resist: 80, IsPhysical: true, NoDifficultyPenalty: true, DifficultyPenalty: -100}, 50},
		{"pierce reduces", ResistInput{Resist: 75, HasPierce: true, Pierce: 20}, 55},
	} {
		if got := EffectiveResist(tc.in); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}

	if got := ApplyResist(1000, 100); got != 0 {
		t.Errorf("immune %d", got)
	}

	if got := ApplyResist(1000, -50); got != 1500 {
		t.Errorf("neg resist %d", got)
	}
}

func TestWikiCrushingBlow(t *testing.T) {
	// wiki: 1/4 of current life for normal monsters, 1/8 bosses, 1/10 players;
	// missiles halve the effect (divisor doubles).
	for _, tc := range []struct {
		name  string
		kind  int
		boss  bool
		event int
		want  int32
	}{
		{"monster", 1, false, 0, 4},
		{"boss", 1, true, 0, 8},
		{"player", 0, false, 0, 10},
		{"monster by missile", 1, false, EventMissile, 8},
	} {
		if got := CrushingDivisor(tc.kind, false, tc.boss, false, 1, tc.event); got != tc.want {
			t.Errorf("%s: divisor %d want %d", tc.name, got, tc.want)
		}
	}
	// multiplayer: +50% divisor per extra player (exe helper 0x571720).
	if got := CrushingDivisor(1, false, false, false, 3, 0); got != 8 {
		t.Errorf("players 3: %d want 8", got)
	}
}

func TestWikiDeadlyCriticalCombine(t *testing.T) {
	// wiki: critical strike and deadly strike are independent chances (combined
	// 1-(1-a)(1-b)); damage doubles once, never x4.
	if RollStrike(&seqRoller{vals: []int{40, 40}}, StrikeInput{CriticalChance: 30, DeadlyChance: 20}) {
		t.Errorf("no hit expected")
	}

	if !RollStrike(&seqRoller{vals: []int{40, 10}}, StrikeInput{CriticalChance: 30, DeadlyChance: 20}) {
		t.Errorf("deadly roll 10 < 20 must hit")
	}

	d := Damage{Physical: 100}
	d.ApplyStrike(&seqRoller{vals: []int{0}}, StrikeInput{CriticalChance: 100, DeadlyChance: 100})

	if d.Physical != 200 {
		t.Errorf("doubled once, got %d", d.Physical)
	}
}

func TestWikiOpenWounds(t *testing.T) {
	r := &seqRoller{vals: []int{0}}

	o := RollOpenWoundsExe(r, OpenWoundsInput{Chance: 100, AttackerLevel: 1, DefenderKind: 1})
	if !o.Hit || o.Value != 40 {
		t.Errorf("level 1 monster: %+v", o)
	}

	o = RollOpenWoundsExe(r, OpenWoundsInput{Chance: 100, AttackerLevel: 1, DefenderKind: 0})
	if o.Value != 10 {
		t.Errorf("vs player %+v", o)
	}
}
