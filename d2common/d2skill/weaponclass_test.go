package d2skill

import "testing"

type classUnit struct {
	*testUnit
	code string
}

func (u classUnit) AttackWeaponClass() string { return u.code }

// SRVDO_008 / 010 / 012: srvmissileb replaces srvmissilea whenever the attack weapon class is not 1 (bow).
func TestUsesMissileBByWeaponClass(t *testing.T) {
	sk := &Skill{SrvMissileA: "a", SrvMissileB: "b"}
	base := &testUnit{}

	for code, want := range map[string]bool{"bow": false, "BOW": false, "xbw": true, "hth": true, "1hs": true, "2ht": true} {
		if got := UsesMissileB(classUnit{base, code}, sk); got != want {
			t.Errorf("class %q: %v, want %v", code, got, want)
		}
	}

	if UsesMissileB(classUnit{base, "xbw"}, &Skill{SrvMissileA: "a"}) {
		t.Error("no srvmissileb, nothing to pick")
	}

	// A unit without a class keeps the old rule (ranged missile name with "bolt").
	base.ranged = "crossbowbolt"
	if !UsesMissileB(base, sk) || !UsesMissileB(classUnit{base, ""}, sk) {
		t.Error("old rule lost")
	}

	base.ranged = "arrow"
	if UsesMissileB(base, sk) {
		t.Error("arrow picked missile b")
	}
}
