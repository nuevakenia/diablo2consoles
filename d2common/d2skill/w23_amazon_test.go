package d2skill

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
)

// Rows with the real patch_d2 values of the Amazon skills the wave 23 audit re-read from the exe.
func init() {
	rows = append(rows,
		row{"skill": "Guided Arrow W23", "Id": "330", "srvstfunc": "4", "srvdofunc": "10", "srvmissilea": "guidedarrow",
			"srvmissileb": "guidedarrow", "calc1": "ln34", "Param3": "0", "Param4": "5", "SrcDam": "128", "HitShift": "8",
			"manashift": "8"},
		row{"skill": "Strafe W23", "Id": "331", "srvstfunc": "8", "srvdofunc": "12", "srvmissilea": "strafearrow",
			"srvmissileb": "strafebolt", "aurarangecalc": "par5", "calc1": "min(par3 + lvl - 1, par4)", "calc2": "ln12",
			"calc3": "2+lvl/4", "Param1": "5", "Param2": "5", "Param3": "4", "Param4": "10", "Param5": "35", "SrcDam": "96",
			"HitShift": "8", "manashift": "8"},
		row{"skill": "Multiple Shot W23", "Id": "332", "srvstfunc": "4", "srvdofunc": "8", "srvmissilea": "multipleshotarrow",
			"srvmissileb": "multipleshotbolt", "calc1": "min(24,ln12)", "calc2": "par3", "calc3": "2", "Param1": "2",
			"Param2": "1", "Param3": "1", "SrcDam": "96", "HitShift": "8", "manashift": "8"},
		row{"skill": "Poison Javelin W23", "Id": "333", "srvstfunc": "4", "srvmissile": "poisonjav", "EType": "pois",
			"EMin": "32", "EMax": "48", "ELen": "200", "SrcDam": "128", "HitShift": "0", "manashift": "8"},
	)
}

func w23Fixture(levels map[string]int) *classFixture {
	cf := newClassFixture(levels)
	tab := classMissiles()
	tab["strafearrow"] = &d2missile.Spec{ID: 682, Name: "strafearrow", SrvDoFunc: 1, Vel: 24, MaxVel: 24, Range: 50,
		CollideType: 3, CollideKill: true, ToHit: true, SrcDam: 96}
	tab["strafebolt"] = &d2missile.Spec{ID: 683, Name: "strafebolt", SrvDoFunc: 1, Vel: 24, MaxVel: 24, Range: 50,
		CollideType: 3, CollideKill: true, ToHit: true, SrcDam: 96}
	tab["multipleshotarrow"].SrcDam = 96
	tab["poisonjav"] = &d2missile.Spec{ID: 38, Name: "poisonjav", SrvDoFunc: 2, Vel: 24, MaxVel: 24, Range: 25,
		CollideType: 3, CollideKill: true, ToHit: true, SubMissile: [3]string{"poisonjavcloud"}}
	tab["poisonjavcloud"] = &d2missile.Spec{ID: 39, Name: "poisonjavcloud", SrvDoFunc: 3, Range: 60, CollideType: 3,
		SkillName: "Poison Javelin W23", SrcDam: -1}
	cf.p.Missiles = tab
	cf.sim = d2missile.NewSim(cf.w, tab)
	cf.sim.OnEvent = func(e d2missile.Event) { cf.evs = append(cf.evs, e) }
	cf.p.Sim = cf.sim

	return cf
}

// Guided Arrow (SRVDO_010, 0x5da2f0): a non-zero calc1 reaches the arrow as damage percent through the create
// callback 0x5da2c0 (ln34 = 0 + 5 * (level - 1)).
func TestGuidedArrowCarriesCalc1AsDamagePercent(t *testing.T) {
	cf := w23Fixture(map[string]int{"Guided Arrow W23": 4})
	cf.u.stats["damagepercent"] = 10
	cf.u.ranged = "arrow"

	foe := cf.addFoe("f", 12, 0)
	_, r := cf.castOn("Guided Arrow W23", foe)

	if !r.OK || len(r.Missiles) != 1 {
		t.Fatalf("cast: ok=%v missiles=%d %s", r.OK, len(r.Missiles), r.Reason)
	}

	if got := r.Missiles[0].Damage.DamagePct; got != 10+15 {
		t.Errorf("damage percent %d, want 25", got)
	}
}

// Strafe (SRVDO_012, 0x5da6c0): calc2 (ln12 = 5 + 5 * (level - 1)) is the damage percent of every arrow.
func TestStrafeArrowsCarryCalc2AsDamagePercent(t *testing.T) {
	cf := w23Fixture(map[string]int{"Strafe W23": 3})
	cf.u.ranged = "arrow"
	cf.addFoe("a", 8, 0)
	cf.addFoe("b", 10, 3)
	foe := cf.foes[0]

	_, r := cf.castOn("Strafe W23", foe)
	if !r.OK || len(r.Missiles) == 0 {
		t.Fatalf("cast: ok=%v missiles=%d %s", r.OK, len(r.Missiles), r.Reason)
	}

	for _, m := range r.Missiles {
		if m.Damage.DamagePct != 15 {
			t.Errorf("arrow damage percent %d, want 15", m.Damage.DamagePct)
		}
	}

	// a crossbow shoots the bolt record
	cf.u.ranged = "crossbowbolt"
	_, r = cf.castOn("Strafe W23", foe)

	for _, m := range r.Missiles {
		if m.Spec.Name != "strafebolt" {
			t.Errorf("crossbow arrow %q", m.Spec.Name)
		}
	}
}

// Multiple Shot (SRVDO_008, 0x5d9fe0): the create flag 0x800 hands calc2 (par3 = 1) over as the Activate delay.
func TestMultipleShotActivationFrame(t *testing.T) {
	cf := w23Fixture(map[string]int{"Multiple Shot W23": 5})
	cf.u.ranged = "arrow"

	_, r := cf.cast("Multiple Shot W23", 20, 0)
	if !r.OK || len(r.Missiles) < 2 {
		t.Fatalf("cast: ok=%v missiles=%d %s", r.OK, len(r.Missiles), r.Reason)
	}

	for _, m := range r.Missiles {
		if m.CollideFrom != m.Life-1 {
			t.Errorf("collide from %d, life %d", m.CollideFrom, m.Life)
		}
	}
}

// The percent pool of MISSILE_BuildDamageDescriptor (0x64ca60): owner stat 0x19, the missile's own percent and the
// larger of item_maxdamage_percent / item_mindamage_percent (weapon skills), never below -90.
func TestDescriptorPercentPool(t *testing.T) {
	cf := w23Fixture(map[string]int{"Strafe W23": 1, "Guided Arrow W23": 1})
	sk := cf.reg.ByName("Strafe W23")

	cf.u.stats = map[string]int{"damagepercent": 10, "item_maxdamage_percent": 80, "item_mindamage_percent": 30}

	if got := descriptorPct(cf.u, sk, 15); got != 10+80+15 {
		t.Errorf("pool %d", got)
	}

	cf.u.stats["item_maxdamage_percent"] = 20 // the min percent is the larger one now

	if got := descriptorPct(cf.u, sk, 0); got != 10+30 {
		t.Errorf("pool %d", got)
	}

	cf.u.stats = map[string]int{"damagepercent": -300}

	if got := descriptorPct(cf.u, sk, 0); got != -90 {
		t.Errorf("floor %d", got)
	}
}

// COMBAT_RollToHit (0x57b9c0): the player's stat 0x77 (Penetrate, Blessed Aim, gear) adds to the to-hit percent of a
// missile hit too.
func TestMissileToHitIncludesItemToHitPercent(t *testing.T) {
	cf := w23Fixture(map[string]int{"Strafe W23": 1})
	cf.u.ranged = "arrow"
	cf.u.stats["item_tohit_percent"] = 45
	foe := cf.addFoe("a", 8, 0)

	_, r := cf.castOn("Strafe W23", foe)
	if !r.OK || len(r.Missiles) == 0 {
		t.Fatalf("cast: %s", r.Reason)
	}

	if got := r.Missiles[0].Damage.ToHit; got != 45 {
		t.Errorf("to-hit percent %d, want 45", got)
	}
}

// A sub missile whose SrcDamage is -1 builds its descriptor without the weapon part (0x64ca60): the poison cloud
// of Poison Javelin does poison only, the javelin itself keeps the weapon blow.
func TestPoisonCloudHasNoWeaponDamage(t *testing.T) {
	cf := w23Fixture(map[string]int{"Poison Javelin W23": 5})
	cf.u.wmin, cf.u.wmax = 20, 30
	cf.u.stats["damagepercent"] = 50
	cf.u.thrown = "javelin"

	sk := cf.reg.ByName("Poison Javelin W23")
	env := NewEnv(sk, 5, cf.u, cf.reg)

	own := cf.p.ownDamages(cf.u, sk, 5, env, cf.p.Missiles.ByName("poisonjav"))
	cloud, ok := own["poisonjavcloud"]

	if !ok {
		t.Fatal("no own damage for the cloud")
	}

	if cloud.PhysMax != 0 || cloud.PhysMin != 0 || cloud.DamagePct != 0 || cloud.Crit {
		t.Errorf("cloud carries weapon damage: %+v", cloud)
	}

	if cloud.Poison.Max <= 0 || cloud.Poison.Len <= 0 {
		t.Errorf("cloud poison %+v", cloud.Poison)
	}

	_, r := cf.cast("Poison Javelin W23", 15, 0)
	if !r.OK || len(r.Missiles) != 1 {
		t.Fatalf("cast: %s", r.Reason)
	}

	if r.Missiles[0].Damage.PhysMax <= 0 {
		t.Errorf("the javelin lost its weapon blow: %+v", r.Missiles[0].Damage)
	}
}
