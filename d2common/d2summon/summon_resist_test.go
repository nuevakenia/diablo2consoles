package d2summon

import "testing"

// SKILL_AttachSummonMasteryStatList (0x5c1f20): the owner's Summon Resist becomes resistances of the minion.
func TestSummonResistReachesMinion(t *testing.T) {
	sk := must(t, loadFixture(t), "necroskeleton")
	base := Compute(sk, Normal, Mods{}, nil)
	s := Compute(sk, Normal, Mods{SummonResist: 30}, nil)

	for i, r := range []int{ResFire, ResLightning, ResCold, ResPoison} {
		if s.Res[r] != base.Res[r]+30 {
			t.Errorf("resist %d: %d, base %d", i, s.Res[r], base.Res[r])
		}
	}

	if s.Res[ResPhysical] != base.Res[ResPhysical] || s.Res[ResMagic] != base.Res[ResMagic] {
		t.Error("physical and magic resistance are not touched")
	}

	// the cap of Compute still holds
	if c := Compute(sk, Normal, Mods{SummonResist: 200}, nil); c.Res[ResFire] != ResistCap {
		t.Errorf("cap: %d", c.Res[ResFire])
	}
}
