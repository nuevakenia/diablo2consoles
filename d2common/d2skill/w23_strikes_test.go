package d2skill

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
)

func init() {
	rows = append(rows,
		row{"skill": "Charged Strike W23", "Id": "334", "srvstfunc": "6", "srvdofunc": "11", "srvmissilea": "chargedstrikebolt",
			"calc1": "par1+lvl/par2", "Param1": "3", "Param2": "5", "EType": "ltng", "EMin": "1", "EMax": "30",
			"HitShift": "8", "manashift": "8"},
		row{"skill": "Lightning Strike W23", "Id": "335", "srvstfunc": "10", "srvdofunc": "14", "srvmissilea": "lightningstrike",
			"aurarangecalc": "par1", "calc1": "20", "calc2": "ln34", "Param1": "20", "Param3": "2", "Param4": "1",
			"EType": "ltng", "EMin": "1", "EMax": "25", "HitShift": "8", "manashift": "8"},
	)
}

func missileSpecStd(id int, name string) *d2missile.Spec {
	return &d2missile.Spec{ID: id, Name: name, SrvDoFunc: 1, Vel: 24, MaxVel: 24, Range: 40, CollideType: 3, CollideKill: true}
}

func w23StrikeFixture() *classFixture {
	cf := w23Fixture(map[string]int{"Charged Strike W23": 5, "Lightning Strike W23": 3})
	cf.p.Missiles.(missileTable)["chargedstrikebolt"] = missileSpecStd(210, "chargedstrikebolt")
	cf.p.Missiles.(missileTable)["lightningstrike"] = missileSpecStd(211, "lightningstrike")
	cf.u.roller = &seq{vals: make([]uint32, 400)}
	cf.u.ar = 100000

	return cf
}

// Charged Strike and Lightning Strike hit with SRVST_006 / SRVST_010: the skill's lightning is part of the melee
// blow (SKILL_RollElementalSkillDamage), and calc1 is its damage percent.
func TestChargedAndLightningStrikeBlowsCarryLightning(t *testing.T) {
	cf := w23StrikeFixture()
	z := cf.addFoe("z", 3, 0)

	for _, name := range []string{"Charged Strike W23", "Lightning Strike W23"} {
		_, r := cf.castOn(name, z)
		if !r.OK || r.Melee == nil || !r.Melee.Hit {
			t.Fatalf("%s: ok=%v %s", name, r.OK, r.Reason)
		}

		if r.Melee.Damage.Lightning <= 0 {
			t.Errorf("%s: the blow has no lightning: %+v", name, r.Melee.Damage)
		}
	}
}

// Lightning Strike (SRVDO_014, 0x5dab30): the chain bolt goes to the enemy with the smallest unit id above the struck
// unit's (else the smallest of all), not to the nearest.
func TestLightningStrikeScanPicksNextUnitID(t *testing.T) {
	cf := w23StrikeFixture()
	z := cf.addFoe("z", 3, 0)       // serial 1, struck
	near := cf.addFoe("near", 4, 0) // serial 2
	far := cf.addFoe("far", 9, 3)   // serial 3
	cf.foes[1].serial, cf.foes[2].serial = 7, 5
	z.serial = 4

	_, r := cf.castOn("Lightning Strike W23", z)
	if !r.OK || len(r.Missiles) != 1 {
		t.Fatalf("ok=%v missiles=%d %s", r.OK, len(r.Missiles), r.Reason)
	}

	// serial 5 (far) is the smallest id above the struck 4; the nearest (serial 7) is not chosen
	if dx, dy := r.Missiles[0].Dest(); int(dx) != far.x || int(dy) != far.y {
		t.Errorf("bolt goes to (%v,%v), want the unit (%d,%d); near is (%d,%d)", dx, dy, far.x, far.y, near.x, near.y)
	}
}
