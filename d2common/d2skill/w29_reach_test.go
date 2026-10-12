package d2skill

import "testing"

func TestAttackInReach(t *testing.T) {
	// player: table distance <= meleeRange + 2 (extra 1 from 0x57cc10 plus the +1 of 0x622e40)
	if !AttackInReach(false, 0, 3, 0) || AttackInReach(false, 0, 4, 0) || AttackInReach(false, 0, 3, 2) {
		t.Error("player reach with bare hands wrong")
	}

	// a polearm (rangeadder 2) adds two
	if !AttackInReach(false, 2, 4, 0) || AttackInReach(false, 2, 5, 0) {
		t.Error("player reach with rangeadder 2 wrong")
	}

	// monster attackers get extra 3: table distance <= meleeRange + 4
	if !AttackInReach(true, 0, 4, 0) || AttackInReach(true, 0, 5, 0) {
		t.Error("monster reach wrong")
	}

	// touching units are always in reach
	if !AttackInReach(false, 0, 0, 0) {
		t.Error("same subtile must be in reach")
	}
}

func TestMeleeGateDropsFarTargets(t *testing.T) {
	f := newFixture(map[string]int{"Attack": 1})
	f.u.base = f.u.levels
	target := &testTarget{id: "zombie", alive: true, level: 1, defense: 10}
	id := f.id("Attack")

	run := func(ux int) DoResult {
		tg := Target{Unit: target, UX: ux}
		f.p.Start(f.u, id, tg)

		return f.p.Do(f.u, id, tg)
	}

	// gate off (default): the host decides the reach, a distant target is still struck
	if do := run(9); do.Melee == nil || !do.Melee.Hit {
		t.Fatalf("gate off: %+v", do.Melee)
	}

	f.p.Opt.MeleeGate = true

	if do := run(9); do.Melee == nil || do.Melee.Hit {
		t.Fatalf("gate on, far target must not be hit: %+v", do.Melee)
	}

	if do := run(2); do.Melee == nil || !do.Melee.Hit {
		t.Fatalf("gate on, near target must be hit: %+v", do.Melee)
	}
}
