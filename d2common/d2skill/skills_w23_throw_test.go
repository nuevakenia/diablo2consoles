package d2skill

import "testing"

func init() {
	rows = append(rows,
		row{"skill": "Double Throw", "Id": "712", "srvdofunc": "74", "range": "rng", "calc1": "skill('Double Swing'.blvl)*par8",
			"Param8": "8", "ToHit": "20", "LevToHit": "10", "HitShift": "8", "SrcDam": "128", "manashift": "8", "mana": "1"},
	)
}

// Double Throw throws two missiles along the same line (no fan) and fails without a throwing weapon.
func TestDoubleThrowStraightPair(t *testing.T) {
	cf := newClassFixture(map[string]int{"Double Throw": 5})

	if _, r := cf.cast("Double Throw", 20, 0); r.OK || r.Reason != ReasonNoWeapon {
		t.Errorf("unarmed: %+v", r)
	}

	cf.u.thrown = "firearrow"
	_, r := cf.cast("Double Throw", 20, 5)

	if !r.OK || len(r.Missiles) != 2 {
		t.Fatalf("throw: %+v", r)
	}

	if a, b := r.Missiles[0], r.Missiles[1]; a.DX != b.DX || a.DY != b.DY {
		t.Errorf("the two missiles diverge: (%v,%v) (%v,%v)", a.DX, a.DY, b.DX, b.DY)
	}
}
