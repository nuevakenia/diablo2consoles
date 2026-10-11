package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

func TestOwnerGoneFollowsTheSummoner(t *testing.T) {
	owner := petUnit(1, 0, 0, "BaalCrab", "")
	clone := petUnit(2, 1, 0, "BaalCrabClone", "")
	orphan := petUnit(3, 2, 0, "BaalTentacle", "")
	natural := petUnit(4, 3, 0, "BaalTentacle", "")

	clone.summoner, orphan.summoner = 1, 99

	d := testDirector(owner, clone, orphan, natural)

	if d.OwnerGone(clone.b) {
		t.Error("clone of a living Baal reported ownerless")
	}

	if !d.OwnerGone(orphan.b) {
		t.Error("unit whose summoner is gone reported owned")
	}

	if d.OwnerGone(natural.b) {
		t.Error("a unit with no recorded owner is treated as owned")
	}

	delete(d.units, owner.b.ID)

	if !d.OwnerGone(clone.b) {
		t.Error("clone of a dead Baal reported owned")
	}
}

func TestTargetModeAndStates(t *testing.T) {
	a := petUnit(1, 0, 0, "BaalTaunt", "")
	m := petUnit(2, 1, 0, "Zombie", "")
	m.b.Mode = d2monster.ModeWalk
	m.states = map[int]bool{7: true}

	d := testDirector(a, m)
	tg := d2monster.Target{ID: unitTargetBase + m.b.ID}

	if d.TargetMode(a.b, tg) != d2monster.ModeWalk {
		t.Error("monster target mode not the brain mode")
	}

	if !d.TargetHasState(a.b, tg, 7) || d.TargetHasState(a.b, tg, 8) {
		t.Error("target state test wrong")
	}
}

func TestRiteStateNames(t *testing.T) {
	d := testDirector()

	if d.AncientFightActive() || d.FightDone() || d.PortalCount() != 0 {
		t.Error("without a quest host the Ancients fight and the statues stay idle")
	}

	d.SetRiteState(&RiteState{})

	if !d.AncientFightActive() || d.FightDone() {
		t.Error("before the fight starts the Ancients wait (exe: byte +0x11 is zero)")
	}

	d.SetRiteState(&RiteState{Started: true, Portals: 1})

	if d.AncientFightActive() || !d.FightDone() || d.PortalCount() != 1 {
		t.Error("after the fight starts the Ancients attack and the statue state is readable")
	}
}
