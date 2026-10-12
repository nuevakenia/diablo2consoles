package d2equip

import "testing"

func TestActiveMeleeRangeAdder(t *testing.T) {
	spear := HandWeapon{Present: true, Weapon: true, RangeAdder: 3}
	claw := HandWeapon{Present: true, Weapon: true, RangeAdder: 1}
	shield := HandWeapon{Present: true, Weapon: false, RangeAdder: 0}
	brokenPole := HandWeapon{Present: true, Weapon: true, Broken: true, RangeAdder: 4}

	tests := []struct {
		name        string
		right, left HandWeapon
		want        int
	}{
		{"bare hands", HandWeapon{}, HandWeapon{}, 0},
		{"spear right", spear, HandWeapon{}, 3},
		{"right wins over left", claw, spear, 1},
		{"shield on the right is skipped", shield, claw, 1},
		{"broken right falls to left", brokenPole, claw, 1},
		{"broken only weapon", brokenPole, shield, 0},
		{"left only", HandWeapon{}, spear, 3},
		{"negative column", HandWeapon{Present: true, Weapon: true, RangeAdder: -1}, HandWeapon{}, 0},
	}

	for _, tc := range tests {
		if got := ActiveMeleeRangeAdder(tc.right, tc.left); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}
