package d2records

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2summon"
)

// wikiUnit is a caster whose skill levels come from a name map (base levels = effective levels).
type wikiUnit struct {
	reg    *d2skill.Registry
	levels map[string]int
	clvl   int
	rng    *d2rand.Seed
	cd     map[int]int
}

func (u *wikiUnit) ID() string     { return "hero" }
func (u *wikiUnit) IsPlayer() bool { return true }
func (u *wikiUnit) Level() int     { return u.clvl }
func (u *wikiUnit) SkillLevel(id int) int {
	if sk := u.reg.ByID(id); sk != nil {
		return u.levels[sk.Name]
	}

	return 0
}
func (u *wikiUnit) BaseSkillLevel(id int) int   { return u.SkillLevel(id) }
func (u *wikiUnit) Stat(string) int             { return 0 }
func (u *wikiUnit) Mana() int                   { return 5000 << 8 }
func (u *wikiUnit) SetMana(int)                 {}
func (u *wikiUnit) Pos() (int, int)             { return 50, 50 }
func (u *wikiUnit) InTown() bool                { return false }
func (u *wikiUnit) Roller() d2combat.Roller     { return u.rng }
func (u *wikiUnit) AttackRating() int           { return 500 }
func (u *wikiUnit) WeaponDamage() (int, int)    { return 10, 20 }
func (u *wikiUnit) RangedWeaponMissile() string { return "" }
func (u *wikiUnit) ThrownMissile() string       { return "" }
func (u *wikiUnit) HasAmmo() bool               { return true }
func (u *wikiUnit) ConsumeAmmo() bool           { return true }
func (u *wikiUnit) Cooldown(id int) int         { return u.cd[id] }
func (u *wikiUnit) SetCooldown(id, until int)   { u.cd[id] = until }

// necroSummon casts a summon skill with the caster's levels and returns the order, its minion stats on normal
// difficulty (maximum life) and the skill's pet limit.
func necroSummon(t *testing.T, tp *d2summon.Templates, skill string, levels map[string]int, clvl int) (*d2skill.SummonOrder, d2summon.Stats) {
	t.Helper()

	rm := loadRealRecords(t)
	reg, mt := rm.SkillTable(), rm.MissileTable()
	sk := reg.ByName(skill)

	w := &auditWorld{grid: d2path.NewCellGrid(-50, -50, 300, 200), foe: &auditFoe{x: 55, y: 50}}
	sim := d2missile.NewSim(w, mt)
	p := &d2skill.Pipeline{Skills: reg, Missiles: mt, Sim: sim, Grid: w.grid, Frame: func() int { return 0 },
		Opt: d2skill.Options{IgnoreTown: true}}
	p.ApplyState = func(d2skill.Unit, d2missile.Target, string, int) {}
	p.After = func(int, func()) {}

	u := &wikiUnit{reg: reg, levels: levels, clvl: clvl, rng: d2rand.New(1), cd: map[int]int{}}
	tg := d2skill.Target{X: 55, Y: 50, Corpse: true, CX: 55, CY: 50, CorpseID: "c1", CorpseHP: 200, CorpseKey: "zombie", CorpseLevel: 5}

	if st := p.Start(u, sk.ID, tg); !st.OK {
		t.Fatalf("%s: start refused %q", skill, st.Reason)
	}

	res := p.Do(u, sk.ID, tg)

	for _, e := range res.Effects {
		if e.Kind != "summon" || e.Summon == nil {
			continue
		}

		o := e.Summon
		mods := d2summon.FromOrder(o, p.SummonPassives(u, sk.ID))

		if o.Level > 0 {
			if ac, th, ok := tp.LevelBonus(o.Level, d2summon.Normal); ok {
				mods.LevelAC, mods.LevelAR = ac, th
			}
		}

		tpl, ok := tp.ByID(o.Key)
		if !ok {
			t.Fatalf("no monstats row %q", o.Key)
		}

		return o, d2summon.Compute(tpl, d2summon.Normal, mods, nil)
	}

	t.Fatalf("%s: no summon effect", skill)

	return nil, d2summon.Stats{}
}

func wikiTemplates(t *testing.T) *d2summon.Templates {
	t.Helper()

	root := os.Getenv("D2_TABLES")
	if root == "" {
		t.Skip("D2_TABLES not set")
	}

	ms, err := os.ReadFile(filepath.Join(root, "monsters", "patch_d2", "monstats.txt"))
	if err != nil {
		t.Skip(err)
	}

	tp, err := d2summon.LoadTemplates(ms)
	if err != nil {
		t.Fatal(err)
	}

	if ml, err := os.ReadFile(filepath.Join(root, "monsters", "patch_d2", "monlvl.txt")); err == nil {
		if err = tp.LoadMonLvl(ml); err != nil {
			t.Fatal(err)
		}
	}

	return tp
}

// Summon stats on normal difficulty (maximum life, damage, defense) for the Necromancer skills, driven through the
// real skills.txt, monstats.txt and monlvl.txt. The wiki numbers asserted here are the ones the 1.14b data agrees
// with; the differences are in ~/git/d2-re-notes/w30-wiki-skills-necro.md.
func TestWikiNecroSummons(t *testing.T) {
	tp := wikiTemplates(t)

	type want struct {
		skill          string
		lvl            int
		life, lo, hi   int
		count          int
		defBonus       int // armorclass of the order's stats (wiki "defense")
		wikiDmgDiffers string
	}

	for _, w := range []want{
		// wiki: 21 31 94 126 199 life, damage 1-2 1-2 (6-7 12-14 on the wiki) 37-39, 1 3 5 6 8 skeletons
		{"Raise Skeleton", 1, 21, 1, 2, 1, 15, ""}, {"Raise Skeleton", 4, 31, 1, 2, 3, 60, ""},
		{"Raise Skeleton", 10, 94, 4, 5, 5, 150, "wiki 6-7"}, {"Raise Skeleton", 13, 126, 10, 11, 6, 195, "wiki 12-14"},
		{"Raise Skeleton", 20, 199, 37, 39, 8, 300, ""},
		// mages: defense +10 per level, count as the skeletons; life 61 +7 percent per level above 3 (10 on the 2.4 wiki)
		{"Raise Skeletal Mage", 1, 61, 1, 2, 1, 10, ""}, {"Raise Skeletal Mage", 10, 90, 1, 2, 5, 100, ""},
		{"Raise Skeletal Mage", 20, 133, 1, 2, 8, 200, ""},
		// clay golem: wiki life 100 205 415 520 765, damage 2-5 4-10 8-20 10-26 15-38
		{"Clay Golem", 1, 100, 2, 5, 1, 0, ""}, {"Clay Golem", 4, 205, 4, 10, 1, 0, ""}, {"Clay Golem", 10, 415, 8, 20, 1, 0, ""},
		{"Clay Golem", 13, 520, 10, 26, 1, 0, ""}, {"Clay Golem", 20, 765, 15, 38, 1, 0, ""},
		// blood golem: wiki damage 6-16 12-32 24-66 31-83 45-122, life 201
		{"BloodGolem", 1, 201, 6, 16, 1, 0, ""}, {"BloodGolem", 4, 201, 12, 32, 1, 0, ""}, {"BloodGolem", 10, 201, 24, 66, 1, 0, ""},
		{"BloodGolem", 13, 201, 31, 83, 1, 0, ""}, {"BloodGolem", 20, 201, 45, 122, 1, 0, ""},
	} {
		o, s := necroSummon(t, tp, w.skill, map[string]int{w.skill: w.lvl}, 30)

		if s.MaxHP != w.life || s.DmgMin != w.lo || s.DmgMax != w.hi {
			t.Errorf("%s L%d: life %d dmg %d-%d, want %d %d-%d %s", w.skill, w.lvl, s.MaxHP, s.DmgMin, s.DmgMax, w.life, w.lo, w.hi, w.wikiDmgDiffers)
		}

		if o.Max != w.count {
			t.Errorf("%s L%d: limit %d, want %d", w.skill, w.lvl, o.Max, w.count)
		}

		if w.defBonus != 0 {
			if v, _ := statValue(o.Stats, "armorclass"); v != w.defBonus {
				t.Errorf("%s L%d: armorclass bonus %d, want %d", w.skill, w.lvl, v, w.defBonus)
			}
		}
	}

	// the wiki "attack rating / defense" of a skeleton (20 65 155 200 305) is the template value (5) plus the skill
	// bonus of 15 per level: the stat the order carries is 15 * level
	for _, c := range []struct{ lvl, th int }{{1, 15}, {4, 60}, {10, 150}, {20, 300}} {
		o, _ := necroSummon(t, tp, "Raise Skeleton", map[string]int{"Raise Skeleton": c.lvl}, 30)
		if v, _ := statValue(o.Stats, "tohit"); v != c.th {
			t.Errorf("skeleton L%d tohit bonus %d, want %d", c.lvl, v, c.th)
		}
	}

	// masteries: Skeleton Mastery 10 on Raise Skeleton 10: (21 + 80 life) * 4.5 = 454, damage (1 + 20 + 2) * 1.49;
	// Golem Mastery 10 on Clay Golem 10: life 415 * (1 + 200 percent) = 1245 (wiki +200 percent), Golem Mastery 20 on
	// Blood Golem 20: 201 * 5 = 1005 (wiki +400 percent)
	if _, s := necroSummon(t, tp, "Raise Skeleton", map[string]int{"Raise Skeleton": 10, "Skeleton Mastery": 10}, 30); s.MaxHP != 454 || s.DmgMin != 34 || s.DmgMax != 35 {
		t.Errorf("skeleton + mastery 10: life %d dmg %d-%d", s.MaxHP, s.DmgMin, s.DmgMax)
	}

	if _, s := necroSummon(t, tp, "Clay Golem", map[string]int{"Clay Golem": 10, "Golem Mastery": 10}, 30); s.MaxHP != 1245 {
		t.Errorf("clay golem + mastery 10: life %d", s.MaxHP)
	}

	if _, s := necroSummon(t, tp, "BloodGolem", map[string]int{"BloodGolem": 20, "Golem Mastery": 20}, 30); s.MaxHP != 1005 {
		t.Errorf("blood golem + mastery 20: life %d", s.MaxHP)
	}

	// Fire Golem: the fire damage added to every hit is the skill's firemindam/firemaxdam (wiki 44-90 187-242 411-476 are
	// about twice the 1.14b values 10-27 93-119 197-233: wiki differs, UNVERIFIED which patch)
	for _, c := range []struct{ lvl, lo, hi int }{{1, 10, 27}, {10, 93, 119}, {20, 197, 233}} {
		_, s := necroSummon(t, tp, "FireGolem", map[string]int{"FireGolem": c.lvl}, 30)
		if s.FireMin != c.lo || s.FireMax != c.hi || s.MaxHP != 313 {
			t.Errorf("fire golem L%d: fire %d-%d life %d", c.lvl, s.FireMin, s.FireMax, s.MaxHP)
		}
	}

	// Bone Wall: life 19 * (1 + 25 percent per level above 1) (wiki 20 / 63 / 111, 1 to 2 points more: UNVERIFIED)
	for _, c := range []struct{ lvl, life int }{{1, 19}, {10, 61}, {20, 109}} {
		for _, sk := range []string{"Bone Wall", "Bone Prison"} {
			if _, s := necroSummon(t, tp, sk, map[string]int{sk: c.lvl}, 30); s.MaxHP != c.life {
				t.Errorf("%s L%d life %d, want %d", sk, c.lvl, s.MaxHP, c.life)
			}
		}
	}
}

// Projectile counts: Poison Nova 64, Teeth = level + 1 up to 24 (calc1 = min(ln12, 24)).
func TestWikiNecroProjectiles(t *testing.T) {
	for _, c := range []struct {
		skill, missile string
		lvl, n         int
	}{{"Poison Nova", "poisonnova", 1, 64}, {"Poison Nova", "poisonnova", 20, 64}, {"Teeth", "teeth", 1, 2},
		{"Teeth", "teeth", 10, 11}, {"Teeth", "teeth", 20, 21}, {"Bone Spirit", "bonespirit", 10, 1}, {"Bone Spear", "bonespear", 10, 1}} {
		r := realCast(t, c.skill, c.lvl)
		n := 0

		for _, m := range r.res.Missiles {
			if m.Spec.Name == c.missile {
				n++
			}
		}

		if n != c.n {
			t.Errorf("%s L%d: %d %s, wiki %d", c.skill, c.lvl, n, c.missile, c.n)
		}
	}
}

func statValue(stats []d2skill.StatMod, name string) (int, bool) {
	for _, s := range stats {
		if s.Stat == name {
			return s.Value, true
		}
	}

	return 0, false
}
