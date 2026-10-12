package d2records

import (
	"math"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
)

// w29: SKILL_SpawnShockFieldMissiles (0x5d4540) creates its caltrops with kind 0x420 (lob): each one ends at its
// throw point. Before the fix they flew their whole range (50 frames at 12) past the aim, so the fields lay ~12 tiles
// beyond the zombies. The hero stands at (50, 50), the aim is (55, 50).
func TestRealShockFieldCaltropsLandAtTheirPoints(t *testing.T) {
	r := realCast(t, "Shock Field", 20)

	if len(r.res.Missiles) < 2 {
		t.Fatalf("only %d caltrops", len(r.res.Missiles))
	}

	r.step(120)

	n := 0

	for _, e := range r.evs {
		if e.Kind != d2missile.EventExpire || e.Missile.Spec.Name != "shock field in air" {
			continue
		}

		n++

		if d := math.Hypot(e.Missile.X-55.5, e.Missile.Y-50.5); d > 12 {
			t.Errorf("caltrop ended at (%.1f,%.1f), %.1f from the aim: it flew past its point", e.Missile.X, e.Missile.Y, d)
		}
	}

	if n == 0 {
		t.Fatal("no caltrop ended")
	}
}
