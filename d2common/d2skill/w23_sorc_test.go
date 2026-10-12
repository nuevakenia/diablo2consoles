package d2skill

import "testing"

// SRVDO_ChargedBolt (0x5c73a0, VERIFIED wave 23): every bolt goes through the
// per-missile hook 0x5c7340, so it walks path type 10 seeded with the
// destination x plus its index; there is no random angle fan.
func TestChargedBoltPathType10(t *testing.T) {
	f := newFixture(map[string]int{"Charged Bolt": 5})
	f.u.base = f.u.levels

	_, do := f.cast("Charged Bolt", 20, 7)
	if !do.OK || len(do.Missiles) != 7 {
		t.Fatalf("%+v", do)
	}

	for i, m := range do.Missiles {
		if !m.OnWalkPath() {
			t.Fatalf("bolt %d does not walk path type 10", i)
		}
	}
}

func TestStaticFieldResistComp(t *testing.T) {
	// non negative resist: untouched, the resist step does the work
	if StaticFieldResistComp(10<<8, 0) != 10<<8 || StaticFieldResistComp(10<<8, 50) != 10<<8 {
		t.Fatal("non negative resist must not change the damage")
	}

	// -100 resist: the later step multiplies by 2, so the damage is halved first
	if got := StaticFieldResistComp(10<<8, -100); got != 5<<8 {
		t.Fatalf("got %d", got)
	}

	d := 20 << 8
	if got := StaticFieldResistComp(d, -50) * 150 / 100; got < d-2 || got > d {
		t.Fatalf("round trip %d vs %d", got, d)
	}
}

// The exe skips the cast when the aim has a zero coordinate.
func TestChargedBoltZeroAimRefused(t *testing.T) {
	f := newFixture(map[string]int{"Charged Bolt": 5})
	f.u.base = f.u.levels

	if _, do := f.cast("Charged Bolt", 20, 0); do.OK {
		t.Fatalf("zero aim should create nothing: %+v", do)
	}
}

// The freeze length of Frozen Armor is calc1 evaluated at the wearer's level
// of the skill when the event fires (handler 0x5c8e20).
func TestPipelineCalc1FrozenArmorFreeze(t *testing.T) {
	f := newFixture(map[string]int{"Frozen Armor": 1})
	f.u.base = map[string]int{"Frozen Armor": 1, "Shiver Armor": 2, "Chilling Armor": 4}
	f.u.levels = f.u.base

	// 30 base frames, +3 per level, +5 percent per synergy level (2+4)
	if got := f.p.Calc1(f.u, f.id("Frozen Armor"), 1); got != 39 {
		t.Fatalf("level 1: %d", got)
	}

	if got := f.p.Calc1(f.u, f.id("Frozen Armor"), 3); got != (30+6)*130/100 {
		t.Fatalf("level 3: %d", got)
	}

	if f.p.Calc1(f.u, 9999, 1) != 0 {
		t.Fatal("unknown skill")
	}
}

func TestBlazeTrailOnlyWhileMoving(t *testing.T) {
	if BlazeTrailStep(5, 5, 5, 5) {
		t.Fatal("a standing player leaves no fire")
	}

	if !BlazeTrailStep(5, 5, 6, 5) || !BlazeTrailStep(5, 5, 5, 4) {
		t.Fatal("a step leaves fire")
	}
}
