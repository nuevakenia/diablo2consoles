package d2records

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2summon"
)

// w30 (wiki cross-check): the druid pets that get their damage from the monstats SkillDamage column.
// MONSTER_FillSkillDamageInfo 0x56bdb0 (VERIFIED): physical min/max (>> 8) and the to-hit bonus of the owner's skill
// at its level, added by MONSTER_SetupLevelScaledStats 0x5a2960 to the level scaled A1 numbers (stats 0x15, 0x16,
// 0x13). monstats gives the hawk, both wolves and the bear no A1 damage at all, so without this they hit for the
// 1-3 fallback of allyStrike.

func realTemplates(t *testing.T) *d2summon.Templates {
	t.Helper()

	root := os.Getenv("D2_TABLES")
	if root == "" {
		t.Skip("D2_TABLES not set")
	}

	buf, err := os.ReadFile(filepath.Join(root, "monsters", "patch_d2", "monstats.txt"))
	if err != nil {
		t.Skip(err)
	}

	tp, err := d2summon.LoadTemplates(buf)
	if err != nil {
		t.Fatal(err)
	}

	return tp
}

// summonStats casts a summon skill at a level and computes the minion the way Summoner.Cast does.
func summonStats(t *testing.T, tp *d2summon.Templates, skill string, lvl int) d2summon.Stats {
	t.Helper()

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

	tpl, ok := tp.ByID(o.Key)
	if !ok {
		t.Fatalf("%s: no template %q", skill, o.Key)
	}

	return d2summon.Compute(tpl, d2summon.Normal, d2summon.FromOrder(o, r.p.SummonPassives(r.u, sk.ID)), nil)
}

func TestW30DruidPetDamageFromSkill(t *testing.T) {
	tp := realTemplates(t)

	for _, tc := range []struct {
		skill      string
		lvl        int
		min, max   int
		minAR, key string
	}{
		// Spirit Wolf (1.14b table: MinDam 2 +1/2/4/5/8 per tier, HitShift 8; passive damagepercent is the Grizzly
		// synergy, 0 here). The wiki's 15-19 and 49-52 at levels 10 and 20 are the patch 2.4 numbers.
		{"Summon Spirit Wolf", 1, 2, 6, "", "spiritwolf"},
		{"Summon Spirit Wolf", 10, 13, 17, "", "spiritwolf"},
		{"Summon Spirit Wolf", 20, 41, 45, "", "spiritwolf"},
		// Dire Wolf matches the wiki exactly: 7-12, 27-32, 69-74
		{"Summon Fenris", 1, 7, 12, "", "fenris"},
		{"Summon Fenris", 10, 27, 32, "", "fenris"},
		{"Summon Fenris", 20, 69, 74, "", "fenris"},
		// Grizzly: the wiki lists the damage with its own passive percent (25 / 115 / 215) applied:
		// 30-60 * 1.25 = 37-75, 130-160 * 2.15 = 279-344, 300-330 * 3.15 = 945-1039
		{"Summon Grizzly", 1, 37, 75, "", "druidbear"},
		{"Summon Grizzly", 10, 279, 344, "", "druidbear"},
		{"Summon Grizzly", 20, 945, 1039, "", "druidbear"},
		// Raven: MinDam 2, MaxDam 4, +1 per level (HitShift 8): 2-4, 11-13, 21-23
		{"Raven", 1, 2, 4, "", "druidhawk"},
		{"Raven", 10, 11, 13, "", "druidhawk"},
		{"Raven", 20, 21, 23, "", "druidhawk"},
	} {
		s := summonStats(t, tp, tc.skill, tc.lvl)
		if s.DmgMin != tc.min || s.DmgMax != tc.max {
			t.Errorf("%s L%d (%s): damage %d-%d, want %d-%d", tc.skill, tc.lvl, tc.key, s.DmgMin, s.DmgMax, tc.min, tc.max)
		}
	}
}

// The to-hit bonus of the skill is part of the minion's attack rating: Fenris ToHit 150 + 20 per level.
func TestW30DruidPetAttackRating(t *testing.T) {
	tp := realTemplates(t)
	base := func(id string) int { b, _ := tp.ByID(id); return b.Diff[d2summon.Normal].A1TH }

	// Dire Wolf passive tohit is the Spirit Wolf synergy (0 here)
	for _, lvl := range []int{1, 10, 20} {
		s := summonStats(t, tp, "Summon Fenris", lvl)
		if want := base("fenris") + 150 + 20*(lvl-1); s.AR != want {
			t.Errorf("Fenris L%d AR %d, want %d", lvl, s.AR, want)
		}
	}
}

// Life: the wiki lists the average of the monstats minimum and maximum (71, 114, 650); Compute without a roller
// takes the maximum, the roll covers min..max.
func TestW30DruidPetLife(t *testing.T) {
	tp := realTemplates(t)

	for id, want := range map[string]int{"spiritwolf": 71, "fenris": 114, "druidbear": 650} {
		if got, _ := tp.AvgLife(id, d2summon.Normal); got != want {
			t.Errorf("%s average life %d, want %d", id, got, want)
		}
	}
}
