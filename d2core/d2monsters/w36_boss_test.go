package d2monsters

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Wave 36 boss fidelity tests (notes: w36-bosses.md). The real-table ones need D2_TABLES.

func TestW36NovaPoints(t *testing.T) {
	inner, outer := novaPoints(2)

	wantInner := [][2]int{{0, 2}, {2, 2}, {2, 0}, {2, -2}, {0, -2}, {-2, -2}, {-2, 0}, {-2, 2}}
	if len(inner) != 8 {
		t.Fatalf("inner ring: %v", inner)
	}

	for i, w := range wantInner {
		if inner[i] != w {
			t.Errorf("inner[%d] = %v, want %v", i, inner[i], w)
		}
	}

	wantOuter := [][2]int{{1, 2}, {2, 1}, {2, -1}, {1, -2}, {-1, -2}, {-2, -1}, {-2, 1}, {-1, 2}}
	if len(outer) != 8 {
		t.Fatalf("outer ring (calc2 2): %v", outer)
	}

	for i, w := range wantOuter {
		if outer[i] != w {
			t.Errorf("outer[%d] = %v, want %v", i, outer[i], w)
		}
	}

	// the exe makes the second ring only when calc2 is above 1
	if _, o := novaPoints(1); len(o) != 0 {
		t.Errorf("calc2 1 makes no second ring, got %v", o)
	}

	// calc2 3: byte offsets 0, 12, 24, 36, 48 -> entries 1, 4, 7, 10, 13
	if _, o := novaPoints(3); len(o) != 5 || o[1] != [2]int{novaRingX[4], novaRingY[4]} {
		t.Errorf("calc2 3: %v", o)
	}
}

func TestW36FanAngle(t *testing.T) {
	got := []float64{fanAngle(0, 0.4), fanAngle(1, 0.4), fanAngle(2, 0.4), fanAngle(3, 0.4), fanAngle(4, 0.4)}
	want := []float64{0, 0.4, -0.4, 0.8, -0.8}

	for i := range want {
		if d := got[i] - want[i]; d > 1e-9 || d < -1e-9 {
			t.Errorf("fanAngle(%d) = %v, want %v", i, got[i], want[i])
		}
	}
}

// loadW36 is loadReal with skills.txt and missiles.txt on top.
func loadW36(t *testing.T) *d2records.RecordManager {
	t.Helper()

	root, rm := loadReal(t)

	for _, f := range []struct{ path, file string }{
		{d2resource.Skills, "skills.txt"},
		{d2resource.Missiles, "missiles.txt"},
	} {
		buf, err := os.ReadFile(filepath.Join(root, "skills", "patch_d2", f.file))
		if err != nil {
			t.Skip(err)
		}

		if err = rm.Load(f.path, d2txt.LoadDataDictionary(buf)); err != nil {
			t.Fatalf("%s: %v", f.file, err)
		}
	}

	return rm
}

// bossUnit is a real boss of the table on a difficulty, with its computed vitals.
func bossUnit(t *testing.T, rm *d2records.RecordManager, key string, diff d2monster.Difficulty) (*Director, *unit) {
	t.Helper()

	st := rm.Monster.Stats[key]
	if st == nil {
		t.Fatalf("no monster %s", key)
	}

	d := realDirector(rm, diff, 0, false)
	d.asset = &d2asset.AssetManager{Records: rm}

	u := petUnit(1, 0, 0, "Melee", "")
	u.b = d2monster.NewBrain(1, st.ID, diff, &d2monster.Profile{}, 1)
	u.m.Stat = st
	u.m.Vitals = d.computeVitals(st, u.b)

	return d, u
}

// Every missile a boss makes must carry damage: the missile row's own columns or the skill its Skill column names.
// Before wave 36 these skills dealt nothing (srvmissilea was not read, the do functions were not ported).
func TestW36BossMissilesDeal(t *testing.T) {
	rm := loadW36(t)

	cases := []struct {
		boss, missile string
		lvl           int
		elem          string
	}{
		{"andariel", "andarielspray", 1, "pois"},
		{"diablo", "diablight", 3, "ltng"},
		{"diablo", "diabwall", 3, "fire"},
		{"mephisto", "chargedstrikebolt", 6, ""},
		{"mephisto", "primepoisoncloud", 6, "pois"},
		{"mephisto", "monsterlight", 6, "ltng"},
	}

	for _, diff := range []d2monster.Difficulty{d2monster.Normal, d2monster.Hell} {
		for _, c := range cases {
			d, u := bossUnit(t, rm, c.boss, diff)

			desc := d.missileDesc(u, c.missile, c.lvl, d2monster.ModeSkill1)
			if desc.Empty() {
				t.Errorf("%s %s diff %d: no damage", c.boss, c.missile, diff)

				continue
			}

			got := ""

			switch {
			case desc.Poison.Max > 0:
				got = "pois"
			case desc.Lightning.Max > 0:
				got = "ltng"
			case desc.Fire.Max > 0:
				got = "fire"
			case desc.Cold.Max > 0:
				got = "cold"
			}

			if c.elem != "" && got != c.elem {
				t.Errorf("%s %s: element %q, want %q", c.boss, c.missile, got, c.elem)
			}

			t.Logf("%s %s diff %d lvl %d: phys %d-%d (8.8) ltng %d-%d fire %d-%d pois %d-%d x%d", c.boss, c.missile, diff, c.lvl,
				desc.PhysMin, desc.PhysMax, desc.Lightning.Min, desc.Lightning.Max, desc.Fire.Min, desc.Fire.Max, desc.Poison.Min,
				desc.Poison.Max, desc.Poison.Len)
		}
	}
}

// The curse numbers of the boss skills at their monstats levels.
func TestW36BossCurses(t *testing.T) {
	rm := loadW36(t)
	reg := rm.SkillTable()

	cases := []struct {
		skill string
		lvl   int
		stat  string
		sign  int // expected sign of the value
	}{
		{"Weaken", 4, "damagepercent", -1},
		{"Defense Curse", 3, "skill_armor_percent", -1},
	}

	for _, c := range cases {
		sk := reg.ByName(c.skill)
		if sk == nil {
			t.Fatalf("no skill %s", c.skill)
		}

		mc := d2skill.MonsterCurseOf(reg, sk, c.lvl, 80)
		if mc == nil || mc.Radius < 1 || mc.Frames < 1 {
			t.Fatalf("%s: %+v", c.skill, mc)
		}

		found := false

		for _, s := range mc.Stats {
			if s.Stat == c.stat && s.Value*c.sign > 0 {
				found = true
			}
		}

		if !found {
			t.Errorf("%s lvl %d: stats %v lack a %s of sign %d", c.skill, c.lvl, mc.Stats, c.stat, c.sign)
		}

		t.Logf("%s lvl %d: radius %d frames %d stats %v", c.skill, c.lvl, mc.Radius, mc.Frames, mc.Stats)
	}

	// Blood Mana has no aurastat pair (UNVERIFIED effect, documented)
	if mc := d2skill.MonsterCurseOf(reg, reg.ByName("Blood Mana"), 3, 80); mc == nil || len(mc.Stats) != 0 {
		t.Errorf("Blood Mana should carry no stat pair: %+v", mc)
	}
}

// Duriel's Holy Freeze: the enemy stats slow a hero, clamped at the -50 a non-monster has as cold effect.
func TestW36HolyFreezeAura(t *testing.T) {
	rm := loadW36(t)
	reg := rm.SkillTable()
	sk := reg.ByName("Holy Freeze")

	for _, lvl := range []int{1, 5, 20} {
		mc := d2skill.MonsterCurseOf(reg, sk, lvl, 88)
		if mc == nil || mc.Radius < 1 {
			t.Fatalf("lvl %d: %+v", lvl, mc)
		}

		var mods []d2state.StatMod
		for _, s := range mc.Stats {
			mods = append(mods, d2state.StatMod{Stat: s.Stat, Value: s.Value})
		}

		for _, m := range d2state.ClampAuraSlow(mods, -50) {
			if m.Stat == "velocitypercent" && m.Value < -50 {
				t.Errorf("lvl %d: velocity %d below the clamp", lvl, m.Value)
			}
		}

		t.Logf("Holy Freeze lvl %d: radius %d stats %v", lvl, mc.Radius, mc.Stats)
	}

	// the passive pairs put cold damage on the caster
	got := map[string]int{}
	for _, s := range d2skill.MonsterAuraStats(reg, sk, 1, 88) {
		got[s.Stat] = s.Value
	}

	if got["coldmaxdam"] <= 0 {
		t.Errorf("Holy Freeze passive pairs give no cold damage: %v", got)
	}
}

// AuraStarter / RightSkillChecker: the Director hosts Duriel's aura and answers the think function's test.
func TestW36DirectorHostsAura(t *testing.T) {
	var _ d2monster.AuraStarter = (*Director)(nil)

	var _ d2monster.RightSkillChecker = (*Director)(nil)

	d := &Director{}
	b := &d2monster.Brain{ID: 7}

	if d.HasRightSkill(b) {
		t.Fatal("no aura before StartAura")
	}

	d.fx.auras = map[uint32]*monAura{7: {}}

	if !d.HasRightSkill(b) {
		t.Fatal("aura running")
	}
}

// A standing fire hits every frame for a few 1/256 of a life point: the fractions add up instead of rounding each hit.
func TestW36HitFractionsCarry(t *testing.T) {
	d := &Director{}
	u := petUnit(1, 0, 0, "Melee", "")
	u.b = &d2monster.Brain{ID: 1}
	hero := hooksHero(0, 0)

	total := 0

	for i := 0; i < 100; i++ {
		atk := d.damageToAttack(u, hero, d2combat.Damage{Fire: 30}) // 30/256 per hit
		total += atk.ElemMin

		if atk.ElemType != "" && atk.ElemType != "fire" {
			t.Fatalf("element %q", atk.ElemType)
		}
	}

	if want := 100 * 30 / 256; total != want {
		t.Errorf("100 hits of 30/256 gave %d points, want %d", total, want)
	}

	// poison is the rate times its length, dealt at once
	atk := d.damageToAttack(u, hero, d2combat.Damage{Poison: 64, PoisonLen: 400})
	if atk.ElemType != "pois" || atk.ElemMin != 64*400>>8 {
		t.Errorf("poison attack %+v", atk)
	}
}

// A boss curse lands on the heroes within its range of the aim point, with the stats of the skill at the monster's
// skill level, for the length of the aura.
func TestW36LandCurse(t *testing.T) {
	d := skillsDirector(t)
	near, far := hooksHero(100, 100), hooksHero(160, 100)
	d.targets[1], d.targets[2] = near, far
	d.frame = 500

	rec := d.asset.Records.GetSkillByName("Weaken")
	if rec == nil {
		t.Fatal("no Weaken")
	}

	u := petUnit(9, 90, 100, "Summoner", "")
	u.b = &d2monster.Brain{ID: 9}
	u.m.Vitals.Level = 80
	u.aimX, u.aimY = 100, 100

	if !d.landCurse(u, rec, 4) {
		t.Fatal("a curse is handled")
	}

	got := d.cursedStats(near)
	if got["damagepercent"] != -33 {
		t.Fatalf("near hero: %v, want damagepercent -33", got)
	}

	if d.cursedStats(far) != nil {
		t.Errorf("a hero 60 subtiles away is out of range: %v", d.cursedStats(far))
	}

	if until := d.curses[near]["damagepercent"].until; until != 500+530 {
		t.Errorf("the curse ends at frame %d, want %d (ln34 at level 4)", until, 500+530)
	}

	// it passes into the hero's totals through the timed stats
	if d.TimedHeroStats(near)["damagepercent"] != -33 {
		t.Errorf("timed stats %v", d.TimedHeroStats(near))
	}
}

// Duriel's aura: StartAura gives him cold damage and makes the heroes in range slow; the aura ends with its owner.
func TestW36AuraSlowsHeroes(t *testing.T) {
	d := skillsDirector(t)
	near, far := hooksHero(100, 100), hooksHero(200, 100)
	d.targets[1], d.targets[2] = near, far
	d.frame = 100

	u := petUnit(3, 100, 100, "Duriel", "")
	u.b = &d2monster.Brain{ID: 3, Profile: &d2monster.Profile{}}
	u.b.Profile.Skills[3] = d2monster.SkillSlot{Name: "Holy Freeze", Level: 5}
	u.m.Vitals.Level, u.m.Vitals.HP, u.m.Vitals.MaxHP = 88, 100, 100
	d.units[3] = u

	d.StartAura(u.b, 3, 1)

	if !d.HasRightSkill(u.b) {
		t.Fatal("the aura is running")
	}

	if len(u.m.UMod.Elems) == 0 || u.m.UMod.Elems[0].Type != "cold" || u.m.UMod.Elems[0].Max <= 0 {
		t.Errorf("the caster takes cold damage: %+v", u.m.UMod.Elems)
	}

	d.stepAuras()

	if v := d.cursedStats(near)["velocitypercent"]; v >= 0 || v < -50 {
		t.Errorf("near hero velocity %d, want a slow between -50 and 0", v)
	}

	if d.cursedStats(far) != nil {
		t.Errorf("a hero 100 subtiles away is not slowed: %v", d.cursedStats(far))
	}

	u.m.Vitals.HP = 0
	d.stepAuras()

	if d.HasRightSkill(u.b) {
		t.Error("a dead owner ends the aura")
	}
}
