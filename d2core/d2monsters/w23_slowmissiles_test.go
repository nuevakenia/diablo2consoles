package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Slow Missiles (state 0x57, stat 0xa1): MISSILE_CreateServerMissile (0x59d5d0) multiplies the velocity of a
// missile that can be slowed by the owner's stat percent; other missiles and unslowed owners are untouched.
func TestSlowedSpeed(t *testing.T) {
	if got := slowedSpeed(1.5, 33, true); got != 1.5*33/100 {
		t.Errorf("slowed %v", got)
	}

	if got := slowedSpeed(1.5, 33, false); got != 1.5 {
		t.Errorf("a missile that cannot be slowed: %v", got)
	}

	if got := slowedSpeed(1.5, 0, true); got != 1.5 {
		t.Errorf("no state: %v", got)
	}
}

func TestSetMissileSlowStoresThePercent(t *testing.T) {
	m := fxTestMonster()
	u := &unit{m: m, b: d2monster.NewBrain(1, 1, d2monster.Normal, &d2monster.Profile{}, 3)}
	d := testDirector(u)
	d.byEntity[m.ID()] = u

	d.SetMissileSlow(m, 33)

	if u.missileSlow != 33 {
		t.Errorf("stored %d", u.missileSlow)
	}

	d.SetMissileSlow(m, 0)

	if u.missileSlow != 0 {
		t.Errorf("cleared %d", u.missileSlow)
	}

	// an unknown monster is ignored
	d.SetMissileSlow(&d2mapentity.Monster{}, 10)
}
