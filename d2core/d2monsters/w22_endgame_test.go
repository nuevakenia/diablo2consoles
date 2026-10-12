package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// endgameHero is a hero with a stat list of the given (stat id, value) pairs.
func endgameHero(x, y float64, hp int, stats ...int) *d2mapentity.Player {
	l := d2statlist.NewList()
	for i := 0; i+1 < len(stats); i += 2 {
		l.Add(stats[i], 0, int64(stats[i+1]))
	}

	p := &d2mapentity.Player{Stats: &d2hero.HeroStatsState{Health: hp, MaxHealth: 200,
		Totals: &d2statlist.Totals{Stats: l}}}
	p.Position.Set(x, y)

	return p
}

// The Diablo candidates carry the resist group, the damage group and state 0xb now.
func TestDiabloCandidatesCarryResistAndDamageGroups(t *testing.T) {
	hero := endgameHero(5, 5, 100,
		d2statlist.StatDamageResist, 20, d2statlist.StatMagicResist, 15, d2statlist.StatMaxDamage, 30,
		d2statlist.StatFireMax, 40, d2statlist.StatLightMax, 50, statMagicMaxDam, 12, d2statlist.StatColdMax, 60,
		d2statlist.StatPoisonMax, 300, statPoisonLen, 100)

	d := testDirector()
	d.opt.IgnoreTown = true
	d.targets = map[uint32]*d2mapentity.Player{1: hero}
	d.HeroHasState = func(_ *d2mapentity.Player, state int) bool { return state == 0xb }

	got := d.DiabloCandidates(&d2monster.Brain{ID: 9})
	if len(got) != 1 {
		t.Fatalf("%d candidates", len(got))
	}

	st := got[0].Stats
	if st.S24 != 20 || st.S25 != 15 || st.S16 != 30 || st.S31 != 40 || st.S33 != 50 || st.S35 != 12 || st.S37 != 60 ||
		st.S3A != 3 || !st.HasState0xb {
		t.Errorf("stats %+v", st)
	}

	// the score reads them: the same hero without the groups scores lower
	bare := d2monster.DiaStats{LifePct: 50}
	if full, none := d2monster.DiabloScore(st, false, true, false), d2monster.DiabloScore(bare, false, true, false); full <= none {
		t.Errorf("score with groups %d, without %d", full, none)
	}
}

// Baal's per-target inputs: life percent, state 0xb and the last town portal in the Worldstone Chamber.
func TestBaalSituationPerTargetInputs(t *testing.T) {
	hero := endgameHero(100, 100, 40) // 40 of 200 life

	b := d2monster.NewBrain(1, 1, d2monster.Hell, &d2monster.Profile{}, 3)
	b.X, b.Y = 100, 100
	b.LevelID = 0x84
	b.AppendCommand(d2monster.Command{Type: d2monster.CmdAnchor, X: 90, Y: 90})

	u := &unit{m: fxTestMonster(), b: b}
	d := testDirector(u)
	d.opt.IgnoreTown = true
	d.targets = map[uint32]*d2mapentity.Player{1: hero}
	d.HeroHasState = func(_ *d2mapentity.Player, state int) bool { return state == 0xb }
	d.HeroPortal = func(*d2mapentity.Player) (int, int, int, bool) { return 0x84, 120, 100, true }

	s := d2monster.BaalSituation{TargetHP: 100, Target: &d2monster.Target{ID: 1, IsPlayer: true}}
	d.BaalSituation(b, &s)

	if s.TargetHP != 20 {
		t.Errorf("target life percent %d, want 20", s.TargetHP)
	}

	if !s.TargetBlocked {
		t.Error("state 0xb on the target must set TargetBlocked")
	}

	if !s.Worldstone {
		t.Error("a portal in level 0x84 within 0x4b of the anchor must set Worldstone")
	}

	// a portal in another level, or far from the anchor, does not
	d.HeroPortal = func(*d2mapentity.Player) (int, int, int, bool) { return 0x6c, 120, 100, true }
	s = d2monster.BaalSituation{Target: &d2monster.Target{ID: 1, IsPlayer: true}}
	d.BaalSituation(b, &s)

	if s.Worldstone {
		t.Error("a portal in the Chaos Sanctuary is not the Worldstone")
	}

	d.HeroPortal = func(*d2mapentity.Player) (int, int, int, bool) { return 0x84, 400, 400, true }
	s = d2monster.BaalSituation{Target: &d2monster.Target{ID: 1, IsPlayer: true}}
	d.BaalSituation(b, &s)

	if s.Worldstone {
		t.Error("a portal 300 subtiles from the anchor is not near")
	}

	// no portal at all
	d.HeroPortal = func(*d2mapentity.Player) (int, int, int, bool) { return 0, 0, 0, false }
	s = d2monster.BaalSituation{Target: &d2monster.Target{ID: 1, IsPlayer: true}}
	d.BaalSituation(b, &s)

	if s.Worldstone {
		t.Error("no portal, no Worldstone")
	}
}

func TestWorldstoneInputBounds(t *testing.T) {
	// below 0x4b counts, 0x4b itself does not; without an anchor any portal in the level counts
	if !baalWorldstoneInput(0x84, 74, 0, 0, 0, true) || baalWorldstoneInput(0x84, 75, 0, 0, 0, true) {
		t.Error("distance bound 0x4b")
	}

	if !baalWorldstoneInput(0x84, 999, 999, 0, 0, false) {
		t.Error("no anchor: any distance")
	}

	if baalWorldstoneInput(0x83, 0, 0, 0, 0, false) {
		t.Error("level 0x83 is the Throne")
	}
}

func TestLastPortalHost(t *testing.T) {
	hero := endgameHero(1, 1, 100)
	d := testDirector()
	d.targets = map[uint32]*d2mapentity.Player{1: hero}

	tg := d2monster.Target{ID: 1, IsPlayer: true}

	if _, _, _, ok := d.LastPortal(nil, tg); ok {
		t.Fatal("no host, no portal")
	}

	d.HeroPortal = func(*d2mapentity.Player) (int, int, int, bool) { return 108, 30, 40, true }

	_, p, lvl, ok := d.LastPortal(nil, tg)
	if !ok || lvl != 108 || p.X != 30 || p.Y != 40 {
		t.Errorf("portal %v %d %v", p, lvl, ok)
	}

	if _, _, _, ok = d.LastPortal(nil, d2monster.Target{ID: 1}); ok {
		t.Error("a non player target has no town portal")
	}
}

// The Ancients wait until the introduction was heard (byte +0x11 of the quest node).
func TestRiteAncientsWaitForTheIntroduction(t *testing.T) {
	d := testDirector()

	if d.AncientFightActive() {
		t.Fatal("without a quest host the Ancients fight at once")
	}

	d.noteSpawnForRite(100) // a normal monster changes nothing

	if d.rite != nil {
		t.Fatal("only the Ancients start the wait")
	}

	d.noteSpawnForRite(541)

	if !d.AncientFightActive() || d.FightDone() {
		t.Fatal("an Ancient exists, the fight has not started: they stand idle")
	}

	if d.RiteIntroHeard(false) || d.FightDone() {
		t.Fatal("without the three statues the exe does nothing")
	}

	d.rite.Portals = 1
	if d.RiteIntroHeard(true) || d.FightDone() {
		t.Fatal("a counted portal blocks the start")
	}

	d.rite.Portals = 0

	if !d.RiteIntroHeard(true) {
		t.Fatal("the introduction starts the fight")
	}

	if d.AncientFightActive() || !d.FightDone() || d.rite.Statues != [3]int{3, 3, 3} {
		t.Errorf("after the start: active %v done %v statues %v", d.AncientFightActive(), d.FightDone(), d.rite.Statues)
	}

	// a second hearing does nothing (the statues are awake)
	if d.RiteIntroHeard(true) {
		t.Error("the fight starts once")
	}
}

// Mephisto's scan walks every unit in the rooms: allied units and mercenaries count with the heroes.
func TestMephScanTakesAlliesAndMercenaries(t *testing.T) {
	hero := endgameHero(10, 0, 500)

	pet := fxTestMonster()
	pet.Vitals.HP = 30
	pb := d2monster.NewBrain(2, 1, d2monster.Normal, &d2monster.Profile{}, 1)
	pb.Allied = true
	petUnit := &unit{m: pet, b: pb}

	mercM := fxTestMonster()
	mercM.Vitals.HP = 10
	mb := d2monster.NewBrain(3, 1, d2monster.Normal, &d2monster.Profile{}, 1)
	mercUnitU := &unit{m: mercM, b: mb, merc: &mercUnit{owner: hero}}

	far := fxTestMonster()
	far.Vitals.HP = 1
	far.Position.Set(100, 100) // beyond the 32 subtiles of the scan
	fb := d2monster.NewBrain(4, 1, d2monster.Normal, &d2monster.Profile{}, 1)
	fb.Allied = true
	farUnit := &unit{m: far, b: fb}

	d := testDirector(petUnit, mercUnitU, farUnit)
	d.opt.IgnoreTown = true
	d.targets = map[uint32]*d2mapentity.Player{1: hero}

	b := &d2monster.Brain{ID: 9}

	// all three at the origin except the far one (set below): the merc has the least life
	w, _ := d.MephScan(b)
	if w == nil || w.ID != mercTargetBase+3 || w.IsPlayer {
		t.Fatalf("weakest %+v, want the merc", w)
	}

	// without the merc the pet beats the hero
	delete(d.units, 3)

	w, _ = d.MephScan(b)
	if w == nil || w.ID != unitTargetBase+2 {
		t.Fatalf("weakest %+v, want the pet", w)
	}

	// a hostile unit that is not on the heroes' side is not a target
	pb.Allied = false
	w, _ = d.MephScan(b)

	if w == nil || w.ID != 1 || !w.IsPlayer {
		t.Fatalf("weakest %+v, want the hero", w)
	}
}
