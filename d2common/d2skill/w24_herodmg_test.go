package d2skill

import "testing"

// The test unit reports the wave 24 weapon numbers from three stat keys: "_thrmin" / "_thrmax" (a throwing
// weapon's thrown range, 0x9f/0xa0), "_wbonus" (the weapon's strength / dexterity percent) and "_barepct" (the
// strength percent of a bare handed blow).
func (u *testUnit) MissileWeaponDamage() (int, int) {
	if u.stats["_thrmax"] > 0 {
		return u.stats["_thrmin"], u.stats["_thrmax"]
	}

	return u.WeaponDamage()
}
func (u *testUnit) WeaponBonusPct() int { return u.stats["_wbonus"] }
func (u *testUnit) BareHandPct() int    { return u.stats["_barepct"] }

// MISSILE_BuildDamageDescriptor (0x64ca60): a throwing weapon takes stats 0x9f/0xa0, every other weapon 0x15/0x16;
// the floor is min 1 and max min + 1.
func TestMissileWeaponRange(t *testing.T) {
	cases := []struct {
		name         string
		wmin, wmax   int
		thrMin, thrM int
		wantMin      int
		wantMax      int
	}{
		{"bow", 10, 25, 0, 0, 10, 25},
		{"javelin uses the thrown range", 3, 8, 14, 32, 14, 32},
		{"bare hands floor", 0, 0, 0, 0, 1, 2},
		{"max not above min", 5, 5, 0, 0, 5, 6},
	}

	for _, tc := range cases {
		cf := w23Fixture(map[string]int{"Strafe W23": 1})
		cf.u.wmin, cf.u.wmax = tc.wmin, tc.wmax
		cf.u.stats["_thrmin"], cf.u.stats["_thrmax"] = tc.thrMin, tc.thrM

		if mn, mx := missileWeapon(cf.u); mn != tc.wantMin || mx != tc.wantMax {
			t.Errorf("%s: %d-%d, want %d-%d", tc.name, mn, mx, tc.wantMin, tc.wantMax)
		}
	}
}

// The weapon's strength / dexterity bonus joins the descriptor's percent pool for skills with a weapon part only;
// melee blows add it (or a bare hand's strength) to the weapon range.
func TestWeaponBonusPools(t *testing.T) {
	cf := w23Fixture(map[string]int{"Strafe W23": 1, "Poison Javelin W23": 1})
	strafe := cf.reg.ByName("Strafe W23")
	cf.u.stats["_wbonus"] = 40
	cf.u.stats["damagepercent"] = 10

	if got := descriptorPct(cf.u, strafe, 5); got != 10+40+5 {
		t.Errorf("pool %d, want 55", got)
	}

	noWeapon := *strafe
	noWeapon.DamageSpec.SrcDam = 0

	if got := descriptorPct(cf.u, &noWeapon, 0); got != 10 {
		t.Errorf("skill without a weapon part: pool %d, want 10", got)
	}

	cf.u.stats["_barepct"] = 25

	if got := meleeBonusPct(cf.u); got != 65 {
		t.Errorf("melee bonus %d, want 65", got)
	}
}

// A missile skill cast with a throwing weapon builds its physical range from the thrown stats, scaled by SrcDam
// (128 / 128 for the test Poison Javelin, whole points), so enhanced damage and added damage of the gear reach the arrow.
func TestMissileCastUsesGearRange(t *testing.T) {
	cases := []struct {
		name       string
		wmin, wmax int
		thrM       int
		thrMin     int
		wantMax    int32
	}{
		{"bow range", 10, 40, 0, 0, 40},
		{"thrown range wins", 3, 8, 60, 20, 60},
	}

	for _, tc := range cases {
		cf := w23Fixture(map[string]int{"Poison Javelin W23": 5})
		cf.u.wmin, cf.u.wmax = tc.wmin, tc.wmax
		cf.u.stats["_thrmin"], cf.u.stats["_thrmax"] = tc.thrMin, tc.thrM
		cf.u.thrown = "javelin"

		_, r := cf.cast("Poison Javelin W23", 15, 0)
		if !r.OK || len(r.Missiles) != 1 {
			t.Fatalf("%s: cast: %s", tc.name, r.Reason)
		}

		if got := r.Missiles[0].Damage.PhysMax; got != tc.wantMax {
			t.Errorf("%s: PhysMax %d, want %d", tc.name, got, tc.wantMax)
		}
	}
}
