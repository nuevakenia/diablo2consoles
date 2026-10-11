package d2skill

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
)

func init() {
	// patch_d2 skills.txt: Shock Wave (243), srvdofunc 8 with the stun in Param1 / Param2.
	rows = append(rows,
		row{"skill": "Shock Wave9", "Id": "744", "charclass": "dru", "srvdofunc": "8", "srvmissilea": "shockwave", "calc1": "5",
			"Param1": "40", "Param2": "15", "MinDam": "10", "MaxDam": "20", "HitShift": "8", "manashift": "8"},
	)
}

// The shockwave missile (pSrvDmgFunc 7) carries the casting skill's Param1 / Param2 so the hit can set the stun.
func TestShockWaveMissileCarriesStunParams(t *testing.T) {
	cf := vineFixture(map[string]int{"Shock Wave9": 3})
	cf.p.Missiles.(missileTable)["shockwave"] = &d2missile.Spec{ID: 511, Name: "shockwave", SrvDoFunc: 1, SrvDmgFunc: 7, Vel: 20,
		MaxVel: 20, Range: 14, CollideType: 3, CollideKill: true, Size: 1}

	_, r := cf.cast("Shock Wave9", 50, 20)
	if len(r.Missiles) != 5 {
		t.Fatalf("%d missiles, want calc1 = 5", len(r.Missiles))
	}

	for _, m := range r.Missiles {
		if m.Skill.Param[1] != 40 || m.Skill.Param[2] != 15 {
			t.Fatalf("skill params %v", m.Skill.Param)
		}

		if got := d2missile.StunLength(m.Skill.Param[1], m.Skill.Param[2], m.Level); got != 40+2*15 {
			t.Fatalf("stun %d at level %d", got, m.Level)
		}
	}
}
