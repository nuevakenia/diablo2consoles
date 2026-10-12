package d2missile

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
)

// BoulderKnockChance (SrvDmgFunc 14, 0x5ab120): the molten boulder row has dParam1 1 and dParam2 50.
func TestBoulderKnockChance(t *testing.T) {
	for _, c := range []struct {
		name                 string
		mode, base           int
		player, small, large bool
		want                 int
	}{
		{"no chance column", 1, 0, false, true, false, 0},
		{"player, mode 0", 0, 50, true, false, false, 0},
		{"player, mode 1", 1, 50, true, false, false, 50},
		{"mode 0 small only", 0, 50, false, true, false, 50},
		{"mode 0 plain monster", 0, 50, false, false, false, 0},
		{"mode 1 small doubled", 1, 50, false, true, false, 100},
		{"mode 1 large immune", 1, 50, false, false, true, 0},
		{"mode 1 plain", 1, 50, false, false, false, 50},
		{"mode 2 small tripled", 2, 50, false, true, false, 150},
		{"mode 2 large base", 2, 50, false, false, true, 50},
		{"mode 2 plain doubled", 2, 50, false, false, false, 100},
	} {
		if got := BoulderKnockChance(c.mode, c.base, c.player, c.small, c.large); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}

type classTarget struct {
	*fakeTarget
	small, large bool
}

func (c classTarget) ClassBit(bit int) bool {
	return (bit == ClassBitSmall && c.small) || (bit == ClassBitLarge && c.large)
}

// The boulder's hits carry result bit 8 for a small monster (chance 100 percent at dParam2 50, mode 1) and never for a
// large one; a plain monster is pushed in roughly half of the hits.
func TestDmgFunc14Knockback(t *testing.T) {
	sp := &Spec{ID: 452, Name: "moltenboulder", SrvDmgFunc: 14, DParam: [2]int{1, 50}}

	push := func(tg Target, n int) int {
		s := NewSim(newWorld(), specs{})
		m := &Missile{Spec: sp}
		m.seed.Init(7)

		pushed := 0

		for i := 0; i < n; i++ {
			d := d2combat.Damage{}
			s.dmgFunc(m, tg, &d)

			if d.Result&ResultKnockback != 0 {
				pushed++
			}
		}

		return pushed
	}

	if n := push(classTarget{newTarget("big", 0, 0), false, true}, 200); n != 0 {
		t.Errorf("large monster pushed %d times", n)
	}

	if n := push(classTarget{newTarget("tiny", 0, 0), true, false}, 200); n != 200 {
		t.Errorf("small monster pushed %d of 200 (chance 100)", n)
	}

	if n := push(newTarget("plain", 0, 0), 2000); n < 850 || n > 1150 {
		t.Errorf("plain monster pushed %d of 2000, want about half", n)
	}

	if n := push(nil, 10); n != 0 {
		t.Errorf("no target pushed %d", n)
	}
}

// A positive dParam1 replaces the computed stun of function 7.
func TestDmgFunc7DParamOverride(t *testing.T) {
	m := &Missile{Spec: &Spec{SrvDmgFunc: 7, DParam: [2]int{12, 0}}, Level: 3, Skill: SkillInfo{Param: [7]int{0, 40, 15}}}
	if got := stunFor(m); got != 12 {
		t.Errorf("override %d", got)
	}

	m.Spec.DParam[0] = 0
	if got := stunFor(m); got != 70 {
		t.Errorf("computed %d", got)
	}
}
