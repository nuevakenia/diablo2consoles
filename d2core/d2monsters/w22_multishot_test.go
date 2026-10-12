package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

type recLauncher struct{ shots []Shot }

func (r *recLauncher) Launch(s Shot) bool { r.shots = append(r.shots, s); return true }
func (r *recLauncher) Step()              {}
func (r *recLauncher) InFlight() int      { return len(r.shots) }

// Multishot: a unique with modifier 29 sends two more shots beside the aim point; others do not.
func TestMultishotFiresTwoMore(t *testing.T) {
	d := testDirector()
	rl := &recLauncher{}
	d.launcher = rl
	d.targets = map[uint32]*d2mapentity.Player{}

	m := fxTestMonster()
	m.Modifiers = []int{29}
	u := &unit{m: m, b: d2monster.NewBrain(1, 1, d2monster.Normal, &d2monster.Profile{}, 3)}
	atk := d2mapentity.MonsterAttack{Min: 1, Max: 2}

	// the monster stands at (0,0); the aim point (10,0) lies east: unit step (-1,0)
	d.multishot(u, d2monster.ModeAttack1, atk, 10, 0)

	if len(rl.shots) != 2 || d.Counters.Multishots != 2 {
		t.Fatalf("%d shots, counter %d", len(rl.shots), d.Counters.Multishots)
	}

	if rl.shots[0].To.X != 10 || rl.shots[0].To.Y != -1 || rl.shots[1].To.X != 10 || rl.shots[1].To.Y != 1 {
		t.Errorf("aim points %v %v, want (10,-1) and (10,1)", rl.shots[0].To, rl.shots[1].To)
	}

	// not unique, or no modifier: nothing
	m.TypeFlags = 0
	d.multishot(u, d2monster.ModeAttack1, atk, 10, 0)

	m.TypeFlags = d2mapentity.MonTypeUnique
	m.Modifiers = []int{9}
	d.multishot(u, d2monster.ModeAttack1, atk, 10, 0)

	if len(rl.shots) != 2 {
		t.Errorf("%d shots, want 2", len(rl.shots))
	}
}
