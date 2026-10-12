package d2skill

import "testing"

func init() {
	rows = append(rows, row{"skill": "Jab W25", "Id": "340", "srvstfunc": "5", "srvdofunc": "7", "calc1": "0", "SrcDam": "128",
		"ToHit": "10", "HitShift": "8", "manashift": "8"})
}

func TestJabStrikesOncePerDoRowOfItsSequence(t *testing.T) {
	cf := newClassFixture(map[string]int{"Jab W25": 1})
	z := cf.addFoe("z", 2, 0)
	cf.u.roller = &seq{vals: make([]uint32, 400)}
	cf.u.ar = 100000
	cf.p.Opt.EventBursts = true

	seqRows, _ := Sequence(SeqJab, "1ht")
	cf.p.Clock = func(Unit, *Skill) *ActionClock { return NewActionClock(seqRows.Flags(), 0, 256, seqRows.Rows) }

	_, r := cf.castOn("Jab W25", z)
	if r.Melee == nil || r.Burst == nil || r.Burst.Left() != 2 {
		t.Fatalf("first thrust: melee=%v burst=%v", r.Melee != nil, r.Burst)
	}

	// do rows 3, 9, 15 of the sequence: two further thrusts, 6 game frames apart
	for i := 0; i < 2; i++ {
		d, fromClock := r.Burst.NextDelay()
		if !fromClock || d != 6 {
			t.Fatalf("thrust %d: delay %d clock=%v, want 6", i+2, d, fromClock)
		}

		var rr DoResult
		if !cf.p.BurstEvent(r.Burst, &rr) || len(rr.Melees) != 1 {
			t.Fatalf("thrust %d did not strike", i+2)
		}
	}

	if !r.Burst.Done() || r.Burst.Shots() != 3 {
		t.Errorf("shots %d done %v, want 3 and done", r.Burst.Shots(), r.Burst.Done())
	}
}

func TestJabWithoutClockIsOneStrike(t *testing.T) {
	cf := newClassFixture(map[string]int{"Jab W25": 1})
	z := cf.addFoe("z", 2, 0)
	cf.u.roller = &seq{vals: make([]uint32, 400)}
	cf.u.ar = 100000
	cf.p.Opt.EventBursts = true

	if _, r := cf.castOn("Jab W25", z); r.Melee == nil || r.Burst != nil {
		t.Errorf("melee=%v burst=%v, want one strike", r.Melee != nil, r.Burst)
	}
}

func TestStrafeBurstAlternatesAndKeepsItsCadence(t *testing.T) {
	cf := w23Fixture(map[string]int{"Strafe W23": 3})
	cf.u.ranged = "arrow"
	a := cf.addFoe("a", 8, 0)
	cf.addFoe("b", 10, 3)
	cf.addFoe("c", 9, -3)
	cf.p.Opt.EventBursts = true
	cf.p.Skills.ByName("Strafe W23").Params[6] = 50
	cf.p.Clock = func(Unit, *Skill) *ActionClock { return NewActionClock(flagsAt(12, 6, 1), 0, 256, 12) }

	_, r := cf.castOn("Strafe W23", a)
	if len(r.Missiles) != 1 || r.Burst == nil {
		t.Fatalf("first arrow: missiles=%d burst=%v", len(r.Missiles), r.Burst)
	}

	if r.Burst.Left() != 2 {
		t.Fatalf("arrows left %d, want 2 (calc1 6, calc3 2, 3 enemies found: 3 arrows)", r.Burst.Left())
	}

	// Param6 50: gap 6 - 6*50/100 = 3
	if d, ok := r.Burst.NextDelay(); !ok || d != 3 {
		t.Errorf("delay %d clock=%v, want 3", d, ok)
	}

	var rr DoResult

	cf.p.BurstEvent(r.Burst, &rr)
	cf.p.BurstEvent(r.Burst, &rr)

	if len(rr.Missiles) != 2 || !r.Burst.Done() {
		t.Errorf("missiles %d done %v", len(rr.Missiles), r.Burst.Done())
	}
}

func TestStrafeLoneEnemyGetsOneArrow(t *testing.T) {
	cf := w23Fixture(map[string]int{"Strafe W23": 3})
	cf.u.ranged = "arrow"
	a := cf.addFoe("a", 8, 0)
	cf.p.Opt.EventBursts = true

	_, r := cf.castOn("Strafe W23", a)
	if len(r.Missiles) != 1 || r.Burst != nil {
		t.Errorf("missiles=%d burst=%v, want one arrow and no burst", len(r.Missiles), r.Burst)
	}
}
