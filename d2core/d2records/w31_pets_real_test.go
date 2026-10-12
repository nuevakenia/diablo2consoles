package d2records

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2summon"
)

// w31 pets follow-up on the real tables (D2_TABLES; skipped without). The auditUnit owner is character level 30.

// petStats casts a summon like Summoner.Cast does, including the monlvl armor class and attack rating of the unit's level.
func petStats(t *testing.T, skill string, lvl int) (d2summon.Stats, *d2skill.SummonOrder) {
	t.Helper()

	tp := realTemplates(t)

	buf, err := os.ReadFile(filepath.Join(os.Getenv("D2_TABLES"), "monsters", "patch_d2", "monlvl.txt"))
	if err != nil {
		t.Skip(err)
	}

	if err = tp.LoadMonLvl(buf); err != nil {
		t.Fatal(err)
	}

	r := realCast(t, skill, lvl)
	sk := r.p.Skills.ByName(skill)

	var o *d2skill.SummonOrder

	for _, e := range r.res.Effects {
		if e.Kind == "summon" {
			o = e.Summon
		}
	}

	if o == nil {
		t.Fatalf("%s: no summon effect", skill)
	}

	tpl, _ := tp.ByID(o.Key)
	m := d2summon.FromOrder(o, r.p.SummonPassives(r.u, sk.ID))

	if o.Level > 0 {
		m.LevelAC, m.LevelAR, _ = tp.LevelBonus(o.Level, d2summon.Normal)
	}

	return d2summon.Compute(tpl, d2summon.Normal, m, nil), o
}

// Valkyrie (SRVDO_016 0x5daf10, SKILL_SpawnSummonedMinion 0x5c22c0, COMBAT_RollPhysicalDamage 0x579120, VERIFIED):
// monstats A1 9-24 TH 250 AC 141 (normal) scaled by strength 25 per level as percent damage; the unit level is the
// SummonLevel of owner level 30 and skill level (not calc2, which is only the item level): 22 + slvl, never above the
// owner. Armor class: item_armor_percent par3 * (lvl - 1) after the monlvl row of the unit's level is added.
func TestW31Valkyrie(t *testing.T) {
	for _, tc := range []struct {
		lvl, unitLvl, min, max, ac, ar int
	}{
		{1, 23, 11, 30, 279, 522},  // 25 percent; row 23: AC 138, TH 232
		{10, 30, 31, 84, 609, 959}, // 250 percent; row 30: AC 180, TH 309; armor (141+180) * 1.9
		{20, 30, 54, 144, 930, 1359},
		{30, 30, 76, 204, 1251, 1759}, // the wiki's cap of 99 does not exist for the unit level; the owner's level does
	} {
		s, o := petStats(t, "Valkyrie", tc.lvl)

		if o.Level != tc.unitLvl {
			t.Errorf("L%d unit level %d, want %d", tc.lvl, o.Level, tc.unitLvl)
		}

		if s.DmgMin != tc.min || s.DmgMax != tc.max || s.Str != 25*tc.lvl {
			t.Errorf("L%d damage %d-%d str %d, want %d-%d str %d", tc.lvl, s.DmgMin, s.DmgMax, s.Str, tc.min, tc.max, 25*tc.lvl)
		}

		if s.Defense != tc.ac || s.AR != tc.ar {
			t.Errorf("L%d AC %d AR %d, want %d / %d", tc.lvl, s.Defense, s.AR, tc.ac, tc.ar)
		}
	}
}

// The decoy takes the same SummonLevel (SRVDO_015 0x5dad10 calls SKILL_ComputeSummonLevel with no fixed level after it
// copied the owner's level): its AC and AR include the monlvl row.
func TestW31DopplezonLevel(t *testing.T) {
	_, o := petStats(t, "Dopplezon", 1)
	if o.Level != 23 {
		t.Errorf("unit level %d, want 23", o.Level)
	}
}

// Pet armor class and attack rating scale with the level of the unit, which is fixed when it is summoned
// (SKILL_ComputeSummonLevel 0x5c2850 sets the level stat and adds monlvl L-AC / L-TH as stats 0x1f / 0x13 once). The
// druid summons take calc2 as that level (ulvl for the wolves and bear, ulvl + par1 + lvl for the raven), the vines
// ulvl * 3 / 4 + lvl, the necromancer's minions SummonLevel; row index capped at the last monlvl row.
func TestW31PetLevelScaling(t *testing.T) {
	for _, tc := range []struct {
		skill   string
		lvl     int
		unitLvl int
	}{
		{"Summon Spirit Wolf", 1, 30}, {"Summon Spirit Wolf", 20, 30},
		{"Summon Grizzly", 5, 30},
		{"Plague Poppy", 1, 23}, {"Plague Poppy", 10, 32}, {"Plague Poppy", 20, 42},
		{"Raise Skeletal Mage", 1, 23}, {"Raise Skeletal Mage", 10, 30},
	} {
		_, o := petStats(t, tc.skill, tc.lvl)
		if o.Level != tc.unitLvl {
			t.Errorf("%s L%d unit level %d, want %d", tc.skill, tc.lvl, o.Level, tc.unitLvl)
		}
	}

	// the row for a level is read from the table (rows are capped at the last one)
	tp := realTemplates(t)

	buf, _ := os.ReadFile(filepath.Join(os.Getenv("D2_TABLES"), "monsters", "patch_d2", "monlvl.txt"))
	if err := tp.LoadMonLvl(buf); err != nil {
		t.Fatal(err)
	}

	for lvl, want := range map[int][2]int{23: {138, 232}, 30: {180, 309}, 42: {252, 441}} {
		if ac, th, _ := tp.LevelBonus(lvl, d2summon.Normal); ac != want[0] || th != want[1] {
			t.Errorf("level %d row %d / %d, want %v", lvl, ac, th, want)
		}
	}

	lo, lt, _ := tp.LevelBonus(110, d2summon.Normal)
	hi, ht, _ := tp.LevelBonus(250, d2summon.Normal)

	if lo != hi || lt != ht {
		t.Errorf("a level beyond the table must use the last row: %d/%d vs %d/%d", lo, lt, hi, ht)
	}
}

// Skeletal Mage (SRVDO_149 0x5cc400, SKILL_SpawnSummonedMinion): the monster skill NecromageMissile gets the level
// sumsk1calc = Skeleton Mastery + (lvl < 4 ? 0 : (lvl - 2) / 2); at 0 the exe adds no skill and the mage only melees
// (monstats A1 1-2). The shot is necromage1 (poison) .. 4 (lightning) with the missile row's own EMin/Emax/ELev at that
// level. Values here are 8.8 per component (poison per frame, whole units), skill level S, no mastery.
func TestW31SkeletalMageDamageByLevel(t *testing.T) {
	type dmg struct{ pMin, pLen, coldMin, coldMax, fireMin, fireMax, ltngMin, ltngMax int32 }

	want := map[int]struct {
		trap int
		d    dmg
	}{
		4:  {1, dmg{6, 500, 256, 1024, 512, 1536, 256, 1792}},
		10: {4, dmg{9, 1250, 1024, 1792, 2048, 3072, 1024, 4096}},
		20: {9, dmg{15, 2500, 2560, 3328, 4864, 5888, 2304, 8448}},
		30: {14, dmg{25, 3750, 5120, 5888, 8704, 9728, 3584, 14848}},
	}

	for s, w := range want {
		r := realCast(t, "Raise Skeletal Mage", s)
		sk := r.p.Skills.ByName("Raise Skeletal Mage")

		var o *d2skill.SummonOrder

		for _, e := range r.res.Effects {
			if e.Kind == "summon" {
				o = e.Summon
			}
		}

		if o.TrapLevel != w.trap {
			t.Errorf("S%d monster skill level %d, want %d", s, o.TrapLevel, w.trap)
			continue
		}

		tg := d2skill.Target{X: 55, Y: 50, Unit: r.w.foe, UX: 55, UY: 50}
		cast := func(name string) (pMin, pLen, cMin, cMax, fMin, fMax, lMin, lMax int32) {
			m := r.p.CastTrapOwn(r.u, sk.ID, name, o.TrapLevel, 50, 50, tg)
			if m == nil {
				t.Fatalf("no %s", name)
			}

			d := m.Damage

			return d.Poison.Min, d.Poison.Len, d.Cold.Min, d.Cold.Max, d.Fire.Min, d.Fire.Max, d.Lightning.Min, d.Lightning.Max
		}

		g := dmg{}
		g.pMin, g.pLen, _, _, _, _, _, _ = cast("necromage1")
		_, _, g.coldMin, g.coldMax, _, _, _, _ = cast("necromage2")
		_, _, _, _, g.fireMin, g.fireMax, _, _ = cast("necromage3")
		_, _, _, _, _, _, g.ltngMin, g.ltngMax = cast("necromage4")

		if g != w.d {
			t.Errorf("S%d mage damage %+v, want %+v", s, g, w.d)
		}
	}

	// below skill level 4 and without Skeleton Mastery the monster skill has no level: no shots
	for s := 1; s <= 3; s++ {
		r := realCast(t, "Raise Skeletal Mage", s)

		for _, e := range r.res.Effects {
			if e.Kind == "summon" && e.Summon.TrapLevel > 0 {
				t.Errorf("S%d: monster skill level %d, want none", s, e.Summon.TrapLevel)
			}
		}
	}
}

// Plague Poppy ("Poison Creeper" on the wiki): the vine missile carries the skill's poison, a damage over time of
// the per-frame 8.8 value over ELen 100 frames (4 s), EMin 12 + 7 per level up to 8, + 12 up to 16, + 15 (skills.txt
// EMinLev1..). The engine totals (value * length / 256) are 4.7-6.3 hp at level 1, 33-35 at 10 and 85-86 at 20. The
// patch 2.4 pages (6-9 / 62-67 / 150-165) are rescaled and do not apply. The skill path is a real damage over time
// (Engine.dot); only a monster's own elemental attack still deals its poison at once (vitals.go).
func TestW31PlaguePoppyPoison(t *testing.T) {
	for _, tc := range []struct {
		lvl      int
		min, max int32
	}{{1, 12, 16}, {2, 19, 23}, {8, 61, 65}, {9, 73, 77}, {10, 85, 89}, {16, 157, 161}, {17, 172, 176}, {20, 217, 221}} {
		r := realCast(t, "Plague Poppy", tc.lvl)
		sk := r.p.Skills.ByName("Plague Poppy")
		tg := d2skill.Target{X: 55, Y: 50, Unit: r.w.foe, UX: 55, UY: 50}

		m := r.p.CastTrap(r.u, sk.ID, "Vine Beast Attack", 50, 50, tg)
		if m == nil {
			t.Fatalf("L%d: no vine", tc.lvl)
		}

		if p := m.Damage.Poison; p.Min != tc.min || p.Max != tc.max || p.Len != 100 {
			t.Errorf("L%d poison %+v, want %d-%d over 100", tc.lvl, p, tc.min, tc.max)
		}
	}
}

// Rabies: the bite is a melee strike with the skill's poison (EMin 6 / EMax 14 at HitShift 3, ELen 100 frames), the
// infection state lasts 100 + 10 per level frames. The strike's rolled poison grows with the table tiers.
func TestW31RabiesPoison(t *testing.T) {
	prev := int32(0)

	for _, lvl := range []int{1, 2, 8, 9, 10, 20} {
		r := realCast(t, "Rabies", lvl)
		if r.res.Melee == nil || !r.res.Melee.Hit {
			t.Fatalf("L%d: no hit", lvl)
		}

		p := r.res.Melee.Damage.Poison
		if p <= prev {
			t.Errorf("L%d poison %d does not grow past %d", lvl, p, prev)
		}

		prev = p

		if want := "foe:rabies:" + itoa(100+10*(lvl-1)); len(r.states) == 0 || r.states[0] != want {
			t.Errorf("L%d states %v, want %s", lvl, r.states, want)
		}
	}
}

func itoa(n int) string {
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}

	return s
}
