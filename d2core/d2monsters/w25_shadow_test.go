package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Skill ids of the test table (the real ids of the Assassin tree).
const (
	skClawMastery = 252
	skTiger       = 254
	skSentry      = 261
	skQuickness   = 258
	skShadowWar   = 268
	skShadowMast  = 279
	skFireTrauma  = 251
	skSwordOther  = 10 // another class
)

func shadowRecords() *d2records.RecordManager {
	rec := &d2records.RecordManager{}
	rec.Skill.Details = d2records.SkillDetails{
		0:             {ID: 0, Skill: "Attack", Anim: d2enum.PlayerAnimationModeAttack1},
		skFireTrauma:  {ID: skFireTrauma, Skill: "Fire Trauma", Charclass: "ass", Aitype: 5, EType: "fire", Anim: d2enum.PlayerAnimationModeSkill2, Reqlevel: 1},
		skClawMastery: {ID: skClawMastery, Skill: "Claw Mastery", Charclass: "ass", Aitype: 10},
		skTiger: {ID: skTiger, Skill: "Tiger Strike", Charclass: "ass", Aitype: 4, Progressive: true, Aurastate: "progressive_damage",
			Anim: d2enum.PlayerAnimationModeAttack1, Mana: 1, Manashift: 8},
		skQuickness: {ID: skQuickness, Skill: "Burst of Speed", Charclass: "ass", Aitype: 1, Aurastate: "quickness",
			Anim: d2enum.PlayerAnimationModeCast, Mana: 10, Manashift: 8},
		skSentry: {ID: skSentry, Skill: "Charged Bolt Sentry", Charclass: "ass", Aitype: 3, Summon: "chargeboltsentry",
			Pettype: "assassintrap", Anim: d2enum.PlayerAnimationModeSkill2, Mana: 13, Manashift: 8},
		skShadowWar: {ID: skShadowWar, Skill: "Shadow Warrior", Charclass: "ass", Aitype: 3, Summon: "shadowwarrior",
			Pettype: "shadowwarrior", Mana: 54, Lvlmana: 1, Manashift: 7},
		skShadowMast: {ID: skShadowMast, Skill: "Shadow Master", Charclass: "ass", Aitype: 3, Summon: "shadowmaster",
			Pettype: "shadowwarrior", Mana: 70, Lvlmana: 1, Manashift: 7},
		skSwordOther: {ID: skSwordOther, Skill: "Bash", Charclass: "bar", Aitype: 4},
	}

	return rec
}

type shadowCall struct {
	skill string
	level int
	hasT  bool
}

// shadowRig is a shadow of an Assassin owner in a Director with a fake skill engine.
type shadowRig struct {
	d      *Director
	u      *unit
	owner  *d2mapentity.Player
	casts  []shadowCall
	played []d2monster.Mode
	states map[string]bool
}

func newShadowRig(t *testing.T, ai string, key string) *shadowRig {
	t.Helper()

	owner := &d2mapentity.Player{Class: d2enum.HeroAssassin, Skills: map[int]*d2hero.HeroSkill{}}
	for id, pts := range map[int]int{skClawMastery: 6, skTiger: 10, skQuickness: 5, skShadowWar: 12, skShadowMast: 20, skSentry: 3} {
		owner.Skills[id] = &d2hero.HeroSkill{SkillPoints: pts}
	}

	u := petUnit(1, 100, 100, ai, "minion")
	u.m.Stat = &d2records.MonStatRecord{Key: key}
	u.ally.owner = owner
	u.ally.opt.Tag = "shadowwarrior"
	u.b.Profile = &d2monster.Profile{}

	if ai == "ShadowWarrior" {
		u.b.Profile.AIPRaw[8] = [3]int{skShadowWar, 5, 64}
	} else {
		u.b.Profile.AIPRaw[8] = [3]int{skShadowMast, 0, 0}
	}

	u.b.Seed = d2rand.New(1234)

	d := testDirector(u)
	d.asset = &d2asset.AssetManager{Records: shadowRecords()}

	r := &shadowRig{d: d, u: u, owner: owner, states: map[string]bool{}}
	d.shadowAttack = func(_ *d2monster.Brain, m d2monster.Mode, _ d2monster.Target) bool {
		r.played = append(r.played, m)

		return true
	}
	d.mercHooks = MercSkillHooks{
		Supported: func(name string) bool { return name != "Fire Trauma" }, // one skill the engine cannot run
		Cast: func(c MercCast) bool {
			r.casts = append(r.casts, shadowCall{c.Skill, c.SkillLevel, c.Target != nil})

			return true
		},
		State: func(_ *d2mapentity.Player, _ uint32, _ *d2mapentity.Monster, st string) MercStateView {
			return MercStateView{Active: r.states[st]}
		},
		Delay: func(MercCast) int { return 30 },
	}

	return r
}

func TestShadowMasterInitCopiesClassSkills(t *testing.T) {
	r := newShadowRig(t, "ShadowMaster", "shadowmaster")
	b := r.u.b
	b.Scratch = [3]int{7, 7, 7}

	r.d.ShadowInit(b, d2monster.ShadowKindMaster)

	// Scratch: 0, 0 and the owner's level of the Shadow Master (aip8 names skill 279: 20 points)
	if b.Scratch != [3]int{0, 0, 20} {
		t.Errorf("scratch %v, want [0 0 20]", b.Scratch)
	}

	_, sb := r.d.shadowOf(b)

	var ids []int
	levels := map[int]int{}

	for _, e := range sb.entries {
		ids = append(ids, e.view.ID)
		levels[e.view.ID] = e.view.Level
	}

	// the plain attack first, then the class skills in id order; Fire Trauma (engine cannot run it), the two shadow
	// skills (same pet type as the shadow / summons its own class) and the other class's skill are out; the sentry
	// (pet type assassintrap) stays
	want := []int{0, skClawMastery, skTiger, skQuickness, skSentry}
	if len(ids) != len(want) {
		t.Fatalf("skills %v, want %v", ids, want)
	}

	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("skills %v, want %v", ids, want)
		}
	}

	// level = owner points / 2 + 20 / 2 (1 point counted for a skill the owner does not have), 1..24
	for id, want := range map[int]int{0: 1, skClawMastery: 3 + 10, skTiger: 5 + 10, skQuickness: 2 + 10, skSentry: 1 + 10} {
		if levels[id] != want {
			t.Errorf("level of %d = %d, want %d", id, levels[id], want)
		}
	}

	if !r.d.HasSkills(b) {
		t.Error("a Shadow Master with skills must not idle")
	}
}

func TestShadowMasterNoInitKeepsScratch(t *testing.T) {
	r := newShadowRig(t, "ShadowMasterNoInit", "shadowmaster")
	b := r.u.b
	b.Scratch = [3]int{4, 5, 0}

	r.d.ShadowInit(b, d2monster.ShadowKindMasterNoInit)

	if b.Scratch[0] != 4 || b.Scratch[1] != 5 || b.Scratch[2] != 20 {
		t.Errorf("scratch %v: only [2] is the owner's level", b.Scratch)
	}

	if _, sb := r.d.shadowOf(b); len(sb.entries) != 1 {
		t.Errorf("NoInit copies no class skills, has %d", len(sb.entries))
	}
}

func TestShadowWarriorInitLevel(t *testing.T) {
	r := newShadowRig(t, "ShadowWarrior", "shadowwarrior")
	r.d.ShadowInit(r.u.b, d2monster.ShadowKindWarrior)

	if r.u.b.Scratch[2] != 12 {
		t.Errorf("level %d, want the owner's 12 points of skill 268", r.u.b.Scratch[2])
	}

	// no owner skill: level 1
	delete(r.owner.Skills, skShadowWar)
	r.u.b.Scratch = [3]int{}
	r.d.ShadowInit(r.u.b, d2monster.ShadowKindWarrior)

	if r.u.b.Scratch[2] != 1 {
		t.Errorf("level %d, want 1", r.u.b.Scratch[2])
	}
}

func TestShadowWarriorCopiesOwnerSkills(t *testing.T) {
	r := newShadowRig(t, "ShadowWarrior", "shadowwarrior")
	r.d.ShadowInit(r.u.b, d2monster.ShadowKindWarrior)

	// the owner's right skill is Tiger Strike (10 points), the left Burst of Speed (5 points)
	r.owner.RightSkill = &d2hero.HeroSkill{SkillRecord: &d2records.SkillRecord{ID: skTiger}, SkillPoints: 10}
	r.owner.LeftSkill = &d2hero.HeroSkill{SkillRecord: &d2records.SkillRecord{ID: skQuickness}, SkillPoints: 5}

	tg := d2monster.Target{ID: 9, X: 101, Y: 100, Size: 1}
	victim := petUnit(9, 101, 100, "Melee", "")
	victim.m.Stat = &d2records.MonStatRecord{Key: "victim"}
	r.d.units[9] = victim
	r.d.nextID = 9

	// with the dice never allowing the fatigue gate to pass the copy fails and nothing is queued
	queued := 0

	for i := 0; i < 80; i++ {
		r.u.b.Scratch[0] = 0
		if calc, ok := r.d.Copy(r.u.b, d2monster.Target{}, tg, false, false, true); ok {
			queued++

			if calc != 30 && calc != 0 {
				t.Fatalf("calc %d: the delay hook says 30, the plain attack 0", calc)
			}
		}
	}

	if queued == 0 {
		t.Fatal("nothing was ever queued")
	}

	// the shadow now knows both skills, at a third of its own level plus half the owner's total
	_, sb := r.d.shadowOf(r.u.b)
	if e := sb.byID[skTiger]; e == nil || e.view.Level != 12/3+10/2 {
		t.Errorf("tiger strike on the shadow: %+v", e)
	}

	if e := sb.byID[skQuickness]; e == nil || e.view.Level != 12/3+5/2 {
		t.Errorf("burst of speed on the shadow: %+v", e)
	}

	// melee kind in reach is cast through the engine at the hit frame; the buff is refused in reach (aitype 1 is for
	// out of reach), so every cast that fired is Tiger Strike
	for i := 0; i < 80 && sb.pending == nil; i++ {
		r.u.b.Scratch[0], r.u.b.Scratch[1] = 0, 0
		r.d.Copy(r.u.b, d2monster.Target{}, tg, false, false, true)
	}

	if sb.pending == nil {
		t.Fatal("a copied skill must be pending until its hit frame")
	}

	if sb.pending.e.view.ID != skTiger {
		t.Errorf("pending skill %d, in reach only the melee kind is allowed", sb.pending.e.view.ID)
	}
}

func TestShadowWarriorNoSelectedSkillsStrikes(t *testing.T) {
	r := newShadowRig(t, "ShadowWarrior", "shadowwarrior")
	r.d.ShadowInit(r.u.b, d2monster.ShadowKindWarrior)
	r.d.units[9] = petUnit(9, 101, 100, "Melee", "")
	r.d.nextID = 9

	// a hero without selected skills: both sides read as the plain attack, which strikes in reach
	calc, ok := r.d.Copy(r.u.b, d2monster.Target{}, d2monster.Target{ID: 9, X: 101, Y: 100, Size: 1}, true, false, true)
	if !ok || calc != 0 || len(r.played) != 1 || r.played[0] != d2monster.ModeAttack1 {
		t.Errorf("calc %d ok %v played %v", calc, ok, r.played)
	}
}

func TestShadowFireCasts(t *testing.T) {
	r := newShadowRig(t, "ShadowWarrior", "shadowwarrior")
	r.d.ShadowInit(r.u.b, d2monster.ShadowKindWarrior)

	// nothing pending: the strike is the plain one
	if r.d.shadowFire(r.u) {
		t.Fatal("no pending skill")
	}

	_, sb := r.d.shadowOf(r.u.b)
	e := r.d.shadowLearn(r.u, sb, skQuickness, 7)
	sb.pending = &shadowPending{e: e, self: true}

	if !r.d.shadowFire(r.u) || len(r.casts) != 1 {
		t.Fatalf("pending buff not cast: %v", r.casts)
	}

	if c := r.casts[0]; c.skill != "Burst of Speed" || c.level != 7 || c.hasT {
		t.Errorf("cast %+v", c)
	}

	if sb.pending != nil {
		t.Error("the pending cast stays after firing")
	}

	// a refused cast falls back to the plain strike
	r.d.mercHooks.Cast = func(MercCast) bool { return false }
	sb.pending = &shadowPending{e: e, self: true}

	if r.d.shadowFire(r.u) {
		t.Error("a refused cast must leave the strike to the plain attack")
	}
}

func TestShadowUpkeepCastsBuffOnce(t *testing.T) {
	r := newShadowRig(t, "ShadowMaster", "shadowmaster")
	r.d.ShadowInit(r.u.b, d2monster.ShadowKindMaster)

	cast := 0

	for i := 0; i < 60 && cast == 0; i++ {
		if r.d.Upkeep(r.u.b, nil, true) {
			cast++
		}
	}

	if cast == 0 {
		t.Fatal("the buff was never cast")
	}

	_, sb := r.d.shadowOf(r.u.b)
	if sb.pending == nil || sb.pending.e.view.ID != skQuickness || !sb.pending.self {
		t.Fatalf("pending %+v: Burst of Speed on the shadow itself", sb.pending)
	}

	// with the state on, nothing is queued
	r.states["quickness"] = true
	sb.pending = nil

	for i := 0; i < 200; i++ {
		if r.d.Upkeep(r.u.b, nil, true) {
			t.Fatal("the buff is on: no recast")
		}
	}
}

func TestShadowScan(t *testing.T) {
	r := newShadowRig(t, "ShadowMaster", "shadowmaster")
	mk := func(id uint32, x, y int, boss bool) *unit {
		u := petUnit(id, x, y, "Melee", "")
		u.m.Position.Set(float64(x), float64(y))
		u.m.Stat = &d2records.MonStatRecord{Key: "hostile", IsKillable: true, IsSpecialBoss: boss}
		r.d.units[id] = u

		return u
	}

	r.d.nextID = 5

	near := mk(2, 103, 100, false) // dist2 9
	mk(3, 130, 100, true)          // dist2 900 (inside 0x400), a boss: valid target
	mk(4, 200, 100, false)         // far
	_ = near

	owner := &d2monster.Target{ID: 77, X: 110, Y: 100}
	sc := r.d.ShadowScan(r.u.b, owner)

	if sc.WithinCount != 2 || sc.CloseCount != 1 {
		t.Errorf("within %d close %d, want 2 and 1", sc.WithinCount, sc.CloseCount)
	}

	if sc.Nearest == nil || sc.Nearest.ID != 2 || sc.NearestDist2 != 9 {
		t.Errorf("nearest %+v dist %d", sc.Nearest, sc.NearestDist2)
	}

	if sc.ValidTarget == nil || sc.ValidTarget.ID != 3 {
		t.Errorf("valid target %+v, want the boss", sc.ValidTarget)
	}

	// nearest to the owner at (110,100): the unit at 103 is 49 away, the boss at 130 is 400, the far one 8100
	if sc.NearOwner == nil || sc.NearOwner.ID != 2 || sc.NearOwnerDist2 != 49 {
		t.Errorf("near owner %+v dist %d", sc.NearOwner, sc.NearOwnerDist2)
	}

	// friendly special classes are counted, not scored
	sp := petUnit(5, 101, 100, "Melee", "minion")
	sp.b.Class = 0x19a
	sp.m.Stat = &d2records.MonStatRecord{}
	r.d.units[5] = sp

	if sc2 := r.d.ShadowScan(r.u.b, owner); sc2.SpecialCount != 1 || sc2.WithinCount != 2 {
		t.Errorf("special %d within %d", sc2.SpecialCount, sc2.WithinCount)
	}
}

func TestShadowEnvQueries(t *testing.T) {
	r := newShadowRig(t, "ShadowMaster", "shadowmaster")
	r.d.ShadowInit(r.u.b, d2monster.ShadowKindMaster)
	b := r.u.b

	if r.d.LeftSkillSet(b) {
		t.Error("no left skill yet")
	}

	r.d.SelectActiveSkill(b, skTiger)

	if !r.d.LeftSkillSet(b) {
		t.Error("the left skill is set")
	}

	_, sb := r.d.shadowOf(b)
	q := sb.byID[skQuickness].view

	if r.d.SelfHasState(b, q.AuraState) {
		t.Error("state off")
	}

	r.states["quickness"] = true

	if !r.d.SelfHasState(b, q.AuraState) {
		t.Error("state on")
	}

	// the resist stat of a hostile monster
	h := petUnit(2, 105, 100, "Melee", "")
	h.m.Stat = &d2records.MonStatRecord{Key: "x"}
	h.m.Vitals.Level = 30
	r.d.units[2] = h

	if is, lvl := r.d.TargetMonsterThreat(d2monster.Target{ID: 2}, d2monster.Normal); !is || lvl != 30 {
		t.Errorf("threat %v %d", is, lvl)
	}

	if is, _ := r.d.TargetMonsterThreat(d2monster.Target{ID: 99}, d2monster.Normal); is {
		t.Error("an unknown target is no monster")
	}
}

func TestShadowEType(t *testing.T) {
	for code, want := range map[string]int{"": 0, "fire": 1, "ltng": 2, "mag": 3, "cold": 4, "pois": 5, "stun": 9, "frze": 12, "FIRE": 1} {
		if got := shadowEType(code); got != want {
			t.Errorf("etype %q = %d, want %d", code, got, want)
		}
	}

	for code, want := range map[string]int{"ama": 0, "sor": 1, "nec": 2, "pal": 3, "bar": 4, "dru": 5, "ass": 6, "": 7, "xyz": 7} {
		if got := shadowClassIndex(code); got != want {
			t.Errorf("class %q = %d, want %d", code, got, want)
		}
	}
}
