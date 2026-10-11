package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

func TestSetAuraBoostScalesLife(t *testing.T) {
	pet := petUnit(1, 0, 0, "NecroPet", "minion")
	pet.m.Vitals.MaxHP, pet.m.Vitals.HP = 100, 50

	d := testDirector(pet)
	d.byEntity[pet.m.ID()] = pet

	if !d.SetAuraBoost(pet.m, AuraBoost{MaxHPPct: 50, DamagePct: 20}) {
		t.Fatal("minion refused")
	}

	if v := pet.m.Vitals; v.MaxHP != 150 || v.HP != 75 {
		t.Errorf("boosted life %d/%d, want 75/150", v.HP, v.MaxHP)
	}

	// the same boost again changes nothing
	d.SetAuraBoost(pet.m, AuraBoost{MaxHPPct: 50, DamagePct: 20})

	if v := pet.m.Vitals; v.MaxHP != 150 || v.HP != 75 {
		t.Errorf("repeat changed life %d/%d", v.HP, v.MaxHP)
	}

	if got := d.AuraBoostOf(pet.m); got.DamagePct != 20 {
		t.Errorf("boost %+v", got)
	}

	// leaving the aura restores the base maximum, life keeps its ratio
	d.SetAuraBoost(pet.m, AuraBoost{})

	if v := pet.m.Vitals; v.MaxHP != 100 || v.HP != 50 {
		t.Errorf("cleared life %d/%d, want 50/100", v.HP, v.MaxHP)
	}
}

func TestSetAuraBoostNeedsAMinion(t *testing.T) {
	foe := petUnit(2, 0, 0, "Melee", "")
	d := testDirector(foe)
	d.byEntity[foe.m.ID()] = foe

	if d.SetAuraBoost(foe.m, AuraBoost{DamagePct: 5}) {
		t.Error("a hostile monster took a boost")
	}
}

func TestPetCasterFlag(t *testing.T) {
	mk := func(ai string, name string, mode d2monster.Mode) *d2monster.Profile {
		p := &d2monster.Profile{AI: ai}
		p.Skills[0] = d2monster.SkillSlot{Name: name, Mode: mode}

		return p
	}

	minion := &allyState{kind: "minion"}

	tests := []struct {
		name string
		a    *allyState
		p    *d2monster.Profile
		want int
	}{
		{"skeleton mage", minion, mk("NecroPet", "NecromageMissile", d2monster.ModeAttack1), 1},
		{"plain skeleton", minion, mk("NecroPet", "", 0), 0},
		{"blood golem passive skill", minion, mk("NecroPet", "BloodGolem", d2monster.ModeNeutral), 0},
		{"hostile monster", nil, mk("NecroPet", "NecromageMissile", d2monster.ModeAttack1), 0},
		{"other AI", minion, mk("Skeleton", "Firebolt", d2monster.ModeAttack1), 0},
		{"no profile", minion, nil, 0},
	}

	for _, tt := range tests {
		if got := petCasterFlag(tt.a, tt.p); got != tt.want {
			t.Errorf("%s: %d, want %d", tt.name, got, tt.want)
		}
	}
}
