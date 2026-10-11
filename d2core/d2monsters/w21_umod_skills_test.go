package d2monsters

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Teleportation (monumod 26) grants skill 0xb8 and opens the wounded teleport of the unit's brain (the AI flag
// 0x20 that 0x5aedc0 tests).
func TestTeleportationOpensWoundedTeleport(t *testing.T) {
	m := fxTestMonster()
	m.Brain = d2monster.NewBrain(1, 1, d2monster.Normal, &d2monster.Profile{}, 1)
	m.Modifiers = []int{26}
	u := (&Director{}).newFxUnit(m)

	fxTestCtx().ApplyInit(u, m.Modifiers, nil)
	u.finish()

	if !m.Brain.CanTeleport || !m.UMod.AIFlag20 {
		t.Errorf("CanTeleport %v flag %v", m.Brain.CanTeleport, m.UMod.AIFlag20)
	}

	if len(m.UMod.Skills) != 1 || m.UMod.Skills[0].Skill != 0xb8 {
		t.Errorf("skills %+v", m.UMod.Skills)
	}
}

// Aura Enchanted: the aura skill's stats land on the monster. Needs $D2_TABLES (skills.txt).
func TestAuraEnchantedStatsApplied(t *testing.T) {
	root := os.Getenv("D2_TABLES")
	if root == "" {
		t.Skip("D2_TABLES not set")
	}

	rm, err := d2records.NewRecordManager(d2util.LogLevelNone)
	if err != nil {
		t.Fatal(err)
	}

	buf, err := os.ReadFile(filepath.Join(root, "monsters", "patch_d2", "skills.txt"))
	if err != nil {
		t.Skip(err)
	}

	if err = rm.Load(d2resource.Skills, d2txt.LoadDataDictionary(buf)); err != nil {
		t.Fatal(err)
	}

	dir := &Director{asset: &d2asset.AssetManager{Records: rm}}

	cases := []struct {
		skill string
		id    int
		lvl   int
		check func(t *testing.T, bonus, elems int)
	}{
		{"Holy Fire", 102, 8, nil},
		{"Holy Shock", 118, 8, nil},
		{"Might", 98, 8, nil},
		{"Fanaticism", 122, 8, nil},
	}

	for _, c := range cases {
		m := fxTestMonster()
		u := dir.newFxUnit(m)
		u.GrantSkill(c.id, c.lvl, true)
		u.finish()

		sk := rm.SkillTable().ByID(c.id)
		if sk == nil || sk.Name != c.skill {
			t.Fatalf("skill %d is %v", c.id, sk)
		}

		switch c.skill {
		case "Holy Fire", "Holy Shock":
			if len(m.UMod.Elems) == 0 || m.UMod.Elems[0].Max <= 0 {
				t.Errorf("%s: no elemental damage %+v", c.skill, m.UMod.Elems)
			}
		default:
			if m.Vitals.A1.Max <= 20 {
				t.Errorf("%s: damage not raised, A1 %+v", c.skill, m.Vitals.A1)
			}
		}

		t.Logf("%s lvl %d: elems %+v A1 %+v", c.skill, c.lvl, m.UMod.Elems, m.Vitals.A1)
	}
}
