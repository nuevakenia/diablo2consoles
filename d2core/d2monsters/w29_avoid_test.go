package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

// COMBAT_RollShieldBlock 0x57bfa0 falls through to COMBAT_RollDodgeAvoidEvade 0x57bd60 when the block chance is
// below 1 or the block roll fails: a summon target avoids missiles, dodges melee and evades while walking.
func TestMonsterTargetDodgeAvoidEvade(t *testing.T) {
	u := blockUnit(1, 0, 5)
	u.ally = &allyState{opt: MinionOptions{Dodge: 100}}
	d := blockDirector(u)

	if !d.MonsterBlock(u.m, false) {
		t.Fatal("dodge 100 must dodge a melee hit")
	}

	if d.MonsterBlock(u.m, true) {
		t.Fatal("dodge does not apply to missiles")
	}

	u.ally.opt = MinionOptions{Avoid: 100}

	if !d.MonsterBlock(u.m, true) || d.MonsterBlock(u.m, false) {
		t.Fatal("avoid 100 must avoid missiles only")
	}

	// evade only counts while walking, and then nothing else is rolled
	u.ally.opt = MinionOptions{Evade: 100, Dodge: 100}
	if !d.MonsterBlock(u.m, false) { // idle: dodge
		t.Fatal("idle monster dodges")
	}

	u.ally.opt = MinionOptions{Evade: 100}
	if d.MonsterBlock(u.m, false) {
		t.Fatal("evade must not work while standing")
	}

	u.m.SetMode(d2monster.ModeWalk)
	if u.m.Mode() == d2monster.ModeWalk && !d.MonsterBlock(u.m, false) {
		t.Fatal("evade 100 must work while walking")
	}

	// a plain monster without passive skills never avoids and takes no seed step
	p := blockUnit(1, 0, 5)
	d = blockDirector(p)

	if d.MonsterBlock(p.m, true) {
		t.Fatal("plain monster must not avoid")
	}
}
