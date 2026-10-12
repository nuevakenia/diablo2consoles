package d2skill

import "testing"

// The Cursed modifier casts skill 66 (Amplify Damage) at monster level / 5 + 1: radius 3 + (lvl-1), 200 + 75 * (lvl-1)
// frames and the stat pair damageresist -100 (the Param1..Param5 of the stock row).
func TestMonsterCurseOfRealRow(t *testing.T) {
	reg := loadRealRegistry(t)

	sk := reg.ByID(MonsterCurseSkill)
	if sk == nil || sk.Name != "Amplify Damage" {
		t.Fatalf("skill %d = %v, want Amplify Damage", MonsterCurseSkill, sk)
	}

	for _, c := range []struct{ lvl, radius, frames int }{{1, 3, 200}, {5, 7, 500}, {9, 11, 800}} {
		got := MonsterCurseOf(reg, sk, c.lvl, 40)
		if got == nil {
			t.Fatalf("lvl %d: nil", c.lvl)
		}

		if got.Radius != c.radius || got.Frames != c.frames || got.State != "amplifydamage" {
			t.Errorf("lvl %d: radius %d frames %d state %q, want %d %d amplifydamage", c.lvl, got.Radius, got.Frames,
				got.State, c.radius, c.frames)
		}

		if len(got.Stats) != 1 || got.Stats[0].Stat != "damageresist" || got.Stats[0].Value != -100 {
			t.Errorf("lvl %d: stats %v, want damageresist -100", c.lvl, got.Stats)
		}
	}

	if MonsterCurseOf(reg, nil, 3, 10) != nil || MonsterCurseOf(reg, sk, 0, 10) != nil {
		t.Error("nil skill or level 0 must give nil")
	}
}
