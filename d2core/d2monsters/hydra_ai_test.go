package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

// A Hydra head armed with Hydra:true is driven by MONAI_Think_Hydra (no shot budget needed), and only
// while its AI is the Hydra one.
func TestTrapAIDrivenHydra(t *testing.T) {
	s := petUnit(1, 0, 0, "Hydra", "trap")
	s.b.Profile = &d2monster.Profile{AI: "Hydra"}
	d := testDirector(s)
	d.byEntity[s.m.ID()] = s
	d.SetSkillFirer(&fakeFirer{})
	d.ArmTrap(s.m, TrapSpec{Level: 5, Hydra: true})

	if !d.TrapAIDriven(s.m) {
		t.Error("an armed hydra head runs the ported think")
	}

	if d.SkillLevel(s.b, 0) != 5 {
		t.Errorf("level %d", d.SkillLevel(s.b, 0))
	}

	s.b.Profile.AI = "AssassinSentry"

	if d.TrapAIDriven(s.m) {
		t.Error("the hydra flag does not drive other AIs")
	}
}
