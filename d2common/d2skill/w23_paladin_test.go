package d2skill

import "testing"

// Wave 23 Paladin audit: each test pins one rule read again in Game.exe 1.14b (addresses in the comments).

// stepRoller returns 0 for the small to-hit style rolls (n <= 100) and half the range minus nothing for the wide
// damage rolls, so a blow always lands and a damage roll is the low end plus 512 (2 points in 8.8).
type stepRoller struct{}

func (stepRoller) Roll(n int32) uint32 {
	if n <= 100 {
		return 0
	}

	if n > 512 {
		return 512
	}

	return uint32(n) - 1
}

type lifeTarget struct {
	*testTarget
	hp int
}

func (l lifeTarget) LifePoints() int { return l.hp }

func vengeanceFixture() *classFixture {
	cf := newClassFixture(map[string]int{"Vengeance": 5})
	cf.u.ar = 100000
	cf.u.roller = stepRoller{}
	cf.reg.Add(skillFromRow(row{"skill": "Vengeance", "Id": "420", "srvstfunc": "35", "srvdofunc": "2", "calc1": "50",
		"calc2": "40", "calc3": "30", "EType": "cold", "ELen": "30", "HitShift": "8", "SrcDam": "128", "manashift": "8"}))

	return cf
}

// SRVST_035 0x5ce340: the three elements are percents of ONE bare weapon roll (stats 0x15 / 0x16, rolled again
// after the physical part), each calc raised by its mastery stat (0x149 fire, 0x14b cold, 0x14a lightning); the
// skill's ELen is added as cold length; the physical damage is untouched.
func TestVengeanceElementsOfBareWeaponRoll(t *testing.T) {
	cf := vengeanceFixture()
	cf.u.stats["passive_fire_mastery"] = 50
	z := cf.addFoe("z", 1, 0)

	_, r := cf.castOn("Vengeance", z)
	if !r.OK || r.Melee == nil || !r.Melee.Hit {
		t.Fatalf("vengeance %+v", r)
	}

	w := int32(5<<8 + 512) // wmin 5 .. wmax 9, the roller adds 512
	d := r.Melee.Damage

	if want := w * (50 + 25) / 100; d.Fire != want {
		t.Errorf("fire %d, want %d (calc1 50 raised by the 50 percent mastery)", d.Fire, want)
	}

	if want := w * 40 / 100; d.Cold != want {
		t.Errorf("cold %d, want %d", d.Cold, want)
	}

	if want := w * 30 / 100; d.Lightning != want {
		t.Errorf("lightning %d, want %d", d.Lightning, want)
	}

	if d.ColdLen != 30 {
		t.Errorf("cold length %d, want ELen 30", d.ColdLen)
	}

	if d.Physical == 0 {
		t.Error("the physical part must stay")
	}
}

func TestVengeanceBaseFloors(t *testing.T) {
	r := &seq{}
	// a minimum below one point becomes one, a maximum not above the minimum becomes minimum + 1 (0x5ce260)
	if got := vengeanceBase(r, 0, 0); got != 1<<8 {
		t.Errorf("0..0 -> %d, want %d", got, 1<<8)
	}

	if got := vengeanceBase(r, 7, 3); got != 7<<8 {
		t.Errorf("7..3 -> %d, want the minimum (the range is min..min+1 point and the zero roll takes the low end)", got)
	}
}

// SRVDO_064 0x5cccb0: the self cost is calc2 percent (par3 = 8) of the physical damage the blow rolled, capped by
// the life the target had; it is not a share of the caster's maximum life.
func TestSacrificeCostIsShareOfDamageDealt(t *testing.T) {
	cf := newClassFixture(map[string]int{"Sacrifice": 3})
	cf.u.ar = 100000
	cf.u.roller = stepRoller{}
	id := cf.id("Sacrifice")

	big := lifeTarget{cf.addFoe("big", 2, 0), 5000}
	tg := Target{X: 2, Y: 0, Unit: big, UX: 2, UY: 0}
	cf.p.Start(cf.u, id, tg)
	r := cf.p.Do(cf.u, id, tg)

	if !r.OK || r.Melee == nil || !r.Melee.Hit {
		t.Fatalf("sacrifice %+v", r)
	}

	e := effectOf(t, r, "self_damage")
	if want := int(r.Melee.Damage.Physical) * 8 / 100; e.SelfDamageFixed != want || want == 0 {
		t.Errorf("self damage %d, want %d (8 percent of %d)", e.SelfDamageFixed, want, r.Melee.Damage.Physical)
	}

	// the target had one point of life: the cost is 8 percent of that point
	weak := lifeTarget{cf.addFoe("weak", 2, 1), 1}
	tg = Target{X: 2, Y: 1, Unit: weak, UX: 2, UY: 1}
	cf.p.Start(cf.u, id, tg)
	r = cf.p.Do(cf.u, id, tg)

	if e := effectOf(t, r, "self_damage"); e.SelfDamageFixed != 256*8/100 {
		t.Errorf("capped self damage %d, want %d", e.SelfDamageFixed, 256*8/100)
	}
}

// SRVST_031 0x5cdbb0 + SRVDO_067: the rush takes time at the Charge velocity, the blow and the overlay land when
// it ends.
func TestChargeTakesTimeAtTheChargeVelocity(t *testing.T) {
	// 14.0625 subtiles per second at velocity 9 * (150 + 100) / 100 -> 1.40625 per frame: 9 subtiles = 7 frames
	if got := ChargeFrames(9, 9, 150, 100); got != 7 {
		t.Errorf("ChargeFrames(9) = %d, want 7", got)
	}

	// the velocity percent is floored at 50: 9 * 200 / 100 -> 1.125 per frame, 9 subtiles = 8 frames
	if got := ChargeFrames(9, 9, 150, 0); got != 8 {
		t.Errorf("ChargeFrames floor = %d, want 8", got)
	}

	if got := ChargeFrames(0, 9, 150, 100); got != 1 {
		t.Errorf("no distance = %d, want one frame", got)
	}

	cf := nextFixture(map[string]int{"Charge": 5})
	cf.u.stats["velocitypercent"] = 100
	cf.u.roller = &seq{vals: make([]uint32, 20)}
	z := cf.addFoe("z", 10, 0)
	id := cf.id("Charge")
	tg := Target{X: 10, Y: 0, Unit: z, UX: 10, UY: 0}
	cf.p.Start(cf.u, id, tg)
	r := cf.p.Do(cf.u, id, tg)

	m := effectOf(t, r, "move")
	if !r.OK || m.X != 9 || m.Frames != 7 {
		t.Errorf("move %+v, want to x=9 over 7 frames", m)
	}

	if r.Melee == nil || r.Melee.Delay != 7 {
		t.Errorf("blow %+v, want a delay of 7 frames", r.Melee)
	}

	if o := effectOf(t, r, "overlay"); o.Delay != 7 {
		t.Errorf("overlay delay %d, want 7", o.Delay)
	}
}

// SRVDO_065 0x5cd4b0: stats are gathered at mana >= upkeep, the passive stats need mana > upkeep.
func TestAuraUpkeepThresholds(t *testing.T) {
	cases := []struct {
		mana, upkeep    int
		stats, passives bool
	}{
		{0, 0, true, false},     // no upkeep, no mana: the aura works but Fanaticism's damage does not
		{1, 0, true, true},      // one 8.8 point of mana is enough for the passives
		{256, 256, true, false}, // exactly the upkeep: stats yes, passives no
		{257, 256, true, true},
		{255, 256, false, false},
	}

	for _, c := range cases {
		if AuraStatsCovered(c.mana, c.upkeep) != c.stats || AuraPassiveCovered(c.mana, c.upkeep) != c.passives {
			t.Errorf("mana %d upkeep %d: stats %v passives %v, want %v %v", c.mana, c.upkeep,
				AuraStatsCovered(c.mana, c.upkeep), AuraPassiveCovered(c.mana, c.upkeep), c.stats, c.passives)
		}
	}
}

// SKILL_GetAuraParamAtLevel 0x645ed0: (lvlmana*(lvl-1)+mana)<<manashift in 24.8. Prayer is the only friendly aura
// of the shipped rows with an upkeep; Sanctuary (mana 1) is the damage aura that pays.
func TestAuraUpkeepRealRows(t *testing.T) {
	reg := loadRealRegistry(t)

	cases := []struct {
		name string
		lvl  int
		want int
	}{
		{"Prayer", 1, 16 << 4},
		{"Prayer", 5, (3*4 + 16) << 4},
		{"Sanctuary", 1, 1 << 8},
		{"Might", 10, 0},
		{"Fanaticism", 10, 0},
		{"Redemption", 10, 0},
	}

	for _, c := range cases {
		if got := int(reg.ByName(c.name).calcMana(c.lvl, false)); got != c.want {
			t.Errorf("%s level %d upkeep %d, want %d", c.name, c.lvl, got, c.want)
		}
	}
}

// SRVST_036 0x5ce6f0: Holy Shield starts only with a shield in a hand.
func TestHolyShieldNeedsAShield(t *testing.T) {
	cf := next2Fixture(map[string]int{"Holy Shield": 5})
	cf.reg.ByName("Holy Shield").SrvStFunc = 36
	id := cf.id("Holy Shield")
	tg := Target{X: 5, Y: 5}

	if st := cf.p.Start(shieldUnit{testUnit: cf.u}, id, tg); st.OK || st.Reason != ReasonNoShield {
		t.Errorf("without a shield: %+v", st)
	}

	if st := cf.p.Start(shieldUnit{testUnit: cf.u, lo: 1, hi: 2, has: true}, id, tg); !st.OK {
		t.Errorf("with a shield: %+v", st)
	}
}
