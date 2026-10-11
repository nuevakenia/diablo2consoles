package d2skill

import "testing"

func init() {
	rows = append(rows,
		row{"skill": "BloodGolem", "Id": "725", "charclass": "nec", "srvdofunc": "56", "summon": "bloodgolem",
			"pettype": "golem", "petmax": "1", "summode": "S1", "calc1": "10", "manashift": "8",
			"Param5": "10", "Param6": "25"},
		row{"skill": "Armageddon9", "Id": "726", "charclass": "dru", "srvdofunc": "124", "aurastate": "armageddon",
			"srvmissilea": "armageddoncontrol",
			"auralencalc": "250", "aurarangecalc": "par3", "Param3": "8", "Param4": "8", "manashift": "8"},
	)
}

func seqRoller(vals ...int) Roller {
	i := 0

	return func(n int) int {
		v := vals[i%len(vals)] % n
		i++

		return v
	}
}

func TestArmageddonPointIsASquareAroundTheCaster(t *testing.T) {
	// radius 8: two rolls below 17 per try; 0 maps to -8, 16 to +8
	x, y, ok := ArmageddonPoint(seqRoller(0, 16), 8, 100, 200, nil)
	if !ok || x != 92 || y != 208 {
		t.Fatalf("corner point: %d,%d ok=%v", x, y, ok)
	}

	// a corner is farther than the radius: a square, not a circle
	if dx, dy := x-100, y-200; dx*dx+dy*dy <= 8*8 {
		t.Fatalf("the corner should lie outside the circle: %d,%d", dx, dy)
	}
}

func TestArmageddonPointRetriesFiveTimes(t *testing.T) {
	tries := 0
	blocked := func(x, y int) bool {
		tries++

		return false
	}

	if _, _, ok := ArmageddonPoint(seqRoller(3), 6, 0, 0, blocked); ok || tries != ArmageddonTries {
		t.Fatalf("all blocked: ok=%v tries=%d", ok, tries)
	}

	// the third try is clear: the first two rolls pairs are rejected, the point of the third is returned
	n := 0
	third := func(x, y int) bool {
		n++

		return n == 3
	}

	x, y, ok := ArmageddonPoint(seqRoller(0, 0, 1, 1, 2, 2), 6, 10, 10, third)
	if !ok || x != 6 || y != 6 { // roll pair (2,2) -> 2-6+10
		t.Fatalf("third try: %d,%d ok=%v", x, y, ok)
	}
}

func TestArmageddonPointNeedsARadius(t *testing.T) {
	if _, _, ok := ArmageddonPoint(seqRoller(0), 0, 0, 0, nil); ok {
		t.Fatal("radius 0 drops no rock")
	}
}

func TestStormIntervalIsParam4(t *testing.T) {
	for in, want := range map[int]int{8: 8, 1: 1, 0: 1, -3: 1, 20: 20} {
		if got := StormInterval(in); got != want {
			t.Errorf("StormInterval(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestArmageddonEffectUsesParam4AndRadius(t *testing.T) {
	cf := newClassFixture(map[string]int{"Armageddon9": 5})
	_, r := cf.cast("Armageddon9", 5, 5)

	e := effectOf(t, r, "storm")
	if e.Interval != 8 || e.Radius != 8 || e.Mode != "scatter" {
		t.Fatalf("storm effect: interval=%d radius=%d mode=%q", e.Interval, e.Radius, e.Mode)
	}
}

func TestBloodGolemDamageShare(t *testing.T) {
	tests := []struct {
		name                              string
		dmg, pct, life, wantLife, wantDmg int
	}{
		{"ten percent", 100, 10, 500, 490, 90},
		{"rounds toward zero", 19, 10, 500, 499, 18},
		{"owner never below one point", 100, 10, 5, 1, 90},
		{"dead owner shares nothing", 100, 10, 0, 0, 100},
		{"no damage", 0, 10, 500, 500, 0},
		{"no share configured", 100, 0, 500, 500, 100},
		{"all of it", 40, 100, 500, 460, 0},
	}

	for _, tt := range tests {
		life, dmg := BloodGolemDamageShare(tt.dmg, tt.pct, tt.life)
		if life != tt.wantLife || dmg != tt.wantDmg {
			t.Errorf("%s: life %d dmg %d, want %d %d", tt.name, life, dmg, tt.wantLife, tt.wantDmg)
		}
	}
}

func TestBloodGolemHealShare(t *testing.T) {
	tests := []struct {
		name                                 string
		heal, pct, golemLife, golemMax, want int
	}{
		{"quarter of the heal", 200, 25, 100, 1000, 50},
		{"limited by the missing life", 200, 25, 990, 1000, 10},
		{"golem at full life", 200, 25, 1000, 1000, 0},
		{"no heal", 0, 25, 10, 100, 0},
		{"no share", 200, 0, 10, 100, 0},
	}

	for _, tt := range tests {
		if got := BloodGolemHealShare(tt.heal, tt.pct, tt.golemLife, tt.golemMax); got != tt.want {
			t.Errorf("%s: %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestBloodGolemOrderCarriesTheShares(t *testing.T) {
	cf := newClassFixture(map[string]int{"BloodGolem": 3})
	cf.u.level = 20

	_, r := cf.cast("BloodGolem", 5, 5)

	e := effectOf(t, r, "summon")
	if e.Summon.OwnerDamagePct != 10 || e.Summon.HealSharePct != 25 {
		t.Fatalf("shares %d %d, want 10 25 (Param5, Param6)", e.Summon.OwnerDamagePct, e.Summon.HealSharePct)
	}

	// other golems carry none
	cf2 := newClassFixture(map[string]int{"Clay Golem5": 3})
	_, r2 := cf2.cast("Clay Golem5", 5, 5)

	if e2 := effectOf(t, r2, "summon"); e2.Summon.OwnerDamagePct != 0 || e2.Summon.HealSharePct != 0 {
		t.Fatalf("clay golem shares: %+v", e2.Summon)
	}
}
