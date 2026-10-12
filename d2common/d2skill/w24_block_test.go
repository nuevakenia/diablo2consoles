package d2skill

import "testing"

// blockingTarget is a testTarget with a shield block.
type blockingTarget struct {
	*testTarget
	blocks bool
	rolls  int
	melee  bool
}

func (b *blockingTarget) RollBlock(missile bool) bool {
	b.rolls++
	b.melee = !missile

	return b.blocks
}

func meleeAgainst(t *testing.T, target *blockingTarget, rolls ...uint32) *MeleeResult {
	t.Helper()

	f := newFixture(map[string]int{"Attack": 1})
	f.u.base = f.u.levels

	if len(rolls) > 0 {
		f.u.roller = &seq{vals: rolls}
	}

	id := f.id("Attack")
	tg := Target{Unit: target}
	f.p.Start(f.u, id, tg)

	return f.p.Do(f.u, id, tg).Melee
}

// A melee skill strike rolls the target's shield block after the to-hit roll succeeded (0x57cc10); a blocked
// blow is no hit and carries no damage.
func TestStrikeShieldBlock(t *testing.T) {
	b := &blockingTarget{testTarget: &testTarget{id: "z", alive: true, level: 1, defense: 10}, blocks: true}

	mr := meleeAgainst(t, b)
	if mr == nil || mr.Hit || !mr.Blocked || b.rolls != 1 || !b.melee {
		t.Fatalf("blocked strike: %+v rolls=%d melee=%v", mr, b.rolls, b.melee)
	}

	if mr.Damage.Physical != 0 || mr.Total != 0 {
		t.Fatalf("a blocked strike has no damage: %+v", mr)
	}

	free := &blockingTarget{testTarget: &testTarget{id: "z", alive: true, level: 1, defense: 10}}

	mr = meleeAgainst(t, free)
	if mr == nil || !mr.Hit || mr.Blocked || free.rolls != 1 {
		t.Fatalf("unblocked strike: %+v rolls=%d", mr, free.rolls)
	}
}

// A strike that misses the to-hit roll never rolls the block (no seed step).
func TestStrikeMissSkipsBlock(t *testing.T) {
	b := &blockingTarget{testTarget: &testTarget{id: "z", alive: true, level: 1, defense: 1 << 24}, blocks: true}

	mr := meleeAgainst(t, b, 99)
	if mr == nil || mr.Hit || mr.Blocked || b.rolls != 0 {
		t.Fatalf("missed strike: %+v rolls=%d", mr, b.rolls)
	}
}
