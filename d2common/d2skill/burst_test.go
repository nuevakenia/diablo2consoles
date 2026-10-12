package d2skill

import (
	"math"
	"testing"
)

func burstFixture() (*classFixture, *testTarget) {
	cf := newClassFixture(map[string]int{"Zeal": 7, "Fend": 3})
	z := cf.addFoe("z", 3, 0)
	cf.addFoe("z2", 4, 0)   // inside melee range 0 + 4 (UNIT_GetMeleeRange is 0 for most weapons)
	cf.addFoe("far", 20, 0) // never reached
	cf.u.roller = &seq{vals: make([]uint32, 400)}
	cf.u.ar = 100000

	return cf, z
}

func TestBurstEventPerAnimationEvent(t *testing.T) {
	cf, z := burstFixture()
	cf.p.Opt.EventBursts = true

	// Zeal keeps calc1 hits whatever is around (Fend keeps min(calc1, enemies in reach), TestFendCountsEnemies)
	_, r := cf.castOn("Zeal", z)
	if len(r.Melees) != 1 || r.Burst == nil || r.Burst.Left() != 4 {
		t.Fatalf("first event: melees=%d burst=%v", len(r.Melees), r.Burst)
	}

	want := []string{"z", "z2", "z", "z2"}
	got := []string{r.Melees[0].Target.ID()}

	for i := 0; i < 3; i++ {
		var rr DoResult
		if !cf.p.BurstEvent(r.Burst, &rr) {
			t.Fatalf("event %d did not hit", i)
		}

		got = append(got, rr.Melees[0].Target.ID())
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("victims %v, want %v", got, want)
		}
	}

	if r.Burst.Left() != 1 || r.Burst.Last() != "z2" {
		t.Errorf("left=%d last=%s", r.Burst.Left(), r.Burst.Last())
	}
}

func TestBurstEndsWhenNobodyInReach(t *testing.T) {
	cf, z := burstFixture()
	cf.p.Opt.EventBursts = true

	_, r := cf.castOn("Zeal", z)
	if r.Burst == nil {
		t.Fatal("no burst")
	}

	z.x = 30 // walked away: not in melee range and nobody within range+4
	cf.foes[1].x = 31

	var rr DoResult
	if cf.p.BurstEvent(r.Burst, &rr) || !r.Burst.Done() {
		t.Error("burst should end without a hit")
	}
}

func TestBurstSingleFoeKeepsHitting(t *testing.T) {
	cf := newClassFixture(map[string]int{"Zeal": 7})
	z := cf.addFoe("z", 2, 0)
	cf.u.roller = &seq{vals: make([]uint32, 100)}
	cf.u.ar = 100000

	_, r := cf.castOn("Zeal", z)
	if len(r.Melees) != 5 {
		t.Errorf("zeal strikes on a lone foe = %d, want 5", len(r.Melees))
	}
}

func TestChargedBoltAim(t *testing.T) {
	if ChargedBoltSeed(10, 3) != 13 {
		t.Error("seed is destX + index")
	}

	if ChargedBoltAim(0, 0, 5, 0, 0) != ChargedBoltAim(0, 0, 5, 0, 0) {
		t.Error("aim must be repeatable")
	}

	distinct := map[float64]bool{}

	for i := 0; i < 6; i++ {
		a := ChargedBoltAim(0, 0, 5, 0, i)
		if math.Abs(a) > math.Pi/2+1e-9 {
			t.Errorf("bolt %d leaves the forward half plane: %f", i, a)
		}

		distinct[a] = true
	}

	if len(distinct) < 3 {
		t.Errorf("bolts barely differ: %v", distinct)
	}
}

// UNIT_GetMeleeRange (0x622a30) is the weapon's rangeadder, 0 for most weapons; the scans use that + 4.
func TestMeleeReachIsRangeAdderPlus4(t *testing.T) {
	cf, _ := burstFixture()
	cf.addFoe("edge", 5, 0)

	if cf.p.meleeRange() != 0 || cf.p.swingReach() != 4 {
		t.Fatalf("meleeRange=%d swingReach=%d", cf.p.meleeRange(), cf.p.swingReach())
	}

	cf.p.Opt.MeleeRange = 2 // polearm-like rangeadder

	if cf.p.swingReach() != 6 || FendReach(cf.p.meleeRange()) != 6 {
		t.Fatalf("swingReach=%d", cf.p.swingReach())
	}

	cf.p.Opt.MeleeRange = 0
	_, r := cf.castOn("Fend", cf.foes[0])
	// z (3) and z2 (4) are in reach 4, edge (5) and far (20) are not: min(calc1, 2) strikes
	if len(r.Melees) != 2 {
		t.Errorf("fend strikes = %d, want 2", len(r.Melees))
	}
}
