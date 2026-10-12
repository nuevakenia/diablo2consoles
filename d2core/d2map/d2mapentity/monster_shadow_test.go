package d2mapentity

import "testing"

func TestUseHeroSpeeds(t *testing.T) {
	m := &Monster{}
	if m.walkSpeed != 0 || m.runSpeed != 0 {
		t.Fatal("a bare monster starts without speed")
	}

	m.UseHeroSpeeds()

	if m.walkSpeed != baseWalkSpeed || m.runSpeed != baseRunSpeed {
		t.Errorf("speeds %v %v, want the hero's %v %v", m.walkSpeed, m.runSpeed, baseWalkSpeed, baseRunSpeed)
	}
}
