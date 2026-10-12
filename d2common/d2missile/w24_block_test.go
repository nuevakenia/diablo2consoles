package d2missile

import "testing"

// blockTarget is a fakeTarget with a shield block.
type blockTarget struct {
	*fakeTarget
	blocks bool
	rolls  int
	sawMis bool
}

func (b *blockTarget) RollBlock(missile bool) bool {
	b.rolls++
	b.sawMis = missile

	return b.blocks
}

// blockWorld hands the sim the blocking wrapper (the sim finds the Blocker through the Target it is given).
type blockWorld struct {
	*fakeWorld
	bt *blockTarget
}

func (w *blockWorld) Targets(x, y int) []Target {
	if w.bt.alive && w.bt.x == x && w.bt.y == y {
		return []Target{w.bt}
	}

	return nil
}

func (w *blockWorld) IsEnemy(Owner, Target) bool { return true }

func blockWorldWrap(bt *blockTarget) *blockWorld {
	return &blockWorld{fakeWorld: newWorld(), bt: bt}
}

func runBlock(w *blockWorld, frames int, owner Owner) []Event {
	return runBlockDmg(w, frames, owner, DamageDesc{PhysMin: 256, PhysMax: 512}, true)
}

// runBlockDmg runs the bolt with the given damage and ToHit flag.
func runBlockDmg(w *blockWorld, frames int, owner Owner, dd DamageDesc, toHit bool) []Event {
	sp := fireBolt()
	sp.ToHit = toHit
	s := NewSim(w, nil)
	_, _ = s.Create(CreateParams{Spec: sp, Level: 1, DestX: 30, Owner: owner, Damage: dd})

	var evs []Event

	s.OnEvent = func(e Event) { evs = append(evs, e) }

	for i := 0; i < frames; i++ {
		w.frame++
		s.Step()
	}

	return evs
}

// A blocked hit reports an EventMiss with Blocked and deals no damage; the block is rolled once, after the damage
// is built (MISSILE_BuildHitFlagsAndRollBlock, w28).
func TestMissileShieldBlock(t *testing.T) {
	bt := &blockTarget{fakeTarget: newTarget("m", 6, 0), blocks: true}
	evs := runBlock(blockWorldWrap(bt), 20, Owner{Level: 10, AttackRating: 5000, Roller: fakeRoller{0}})

	if k := kinds(evs); k[EventMiss] != 1 || k[EventHit] != 0 {
		t.Fatalf("events %v", k)
	}

	for _, e := range evs {
		if e.Kind == EventMiss && !e.Blocked {
			t.Fatal("the miss must be flagged Blocked")
		}
	}

	if bt.rolls != 1 || !bt.sawMis {
		t.Fatalf("rolls %d missile flag %v", bt.rolls, bt.sawMis)
	}
}

// A target whose block does not trigger takes the hit; a failed to-hit roll never reaches the block roll.
func TestMissileNoBlockAndMissSkipsBlock(t *testing.T) {
	bt := &blockTarget{fakeTarget: newTarget("m", 6, 0)}

	evs := runBlock(blockWorldWrap(bt), 20, Owner{Level: 10, AttackRating: 5000, Roller: fakeRoller{0}})
	if k := kinds(evs); k[EventHit] != 1 || bt.rolls != 1 {
		t.Fatalf("events %v rolls %d", k, bt.rolls)
	}

	bt2 := &blockTarget{fakeTarget: newTarget("m", 6, 0), blocks: true}
	bt2.defense = 100000

	evs = runBlock(blockWorldWrap(bt2), 60, Owner{Level: 1, AttackRating: 10, Roller: fakeRoller{99}})
	if k := kinds(evs); k[EventMiss] != 1 || bt2.rolls != 0 {
		t.Fatalf("events %v rolls %d (a missed to-hit must not roll the block)", k, bt2.rolls)
	}
}

// The block is only attempted when the damage struct holds physical damage, and it does not depend on the
// to-hit flag (MISSILE_BuildHitFlagsAndRollBlock passes "physical max != 0" to COMBAT_RollShieldBlock).
func TestMissileBlockNeedsPhysicalDamage(t *testing.T) {
	owner := Owner{Level: 10, AttackRating: 5000, Roller: fakeRoller{0}}

	bt := &blockTarget{fakeTarget: newTarget("m", 6, 0), blocks: true}
	evs := runBlockDmg(blockWorldWrap(bt), 20, owner, DamageDesc{Fire: Elem{Min: 256, Max: 512}}, true)

	if k := kinds(evs); k[EventHit] != 1 || bt.rolls != 0 {
		t.Fatalf("elemental-only missile: events %v rolls %d, want a hit and no block roll", k, bt.rolls)
	}

	bt = &blockTarget{fakeTarget: newTarget("m", 6, 0), blocks: true}
	evs = runBlockDmg(blockWorldWrap(bt), 20, owner, DamageDesc{PhysMin: 256, PhysMax: 512}, false)

	if k := kinds(evs); k[EventMiss] != 1 || k[EventHit] != 0 || bt.rolls != 1 {
		t.Fatalf("physical missile without the to-hit flag: events %v rolls %d, want a block", k, bt.rolls)
	}
}
