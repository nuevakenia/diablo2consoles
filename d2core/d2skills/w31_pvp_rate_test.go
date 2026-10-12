package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Wave 31: the DamageRate of the missile that hit travels with the PvP hit so the defender can scale its flat
// damage reductions (0x579ef0).
func TestHurtPlayerRateTravels(t *testing.T) {
	e := newTestEngine()
	e.Logger = d2util.NewLogger()

	src, dst := &d2mapentity.Player{}, &d2mapentity.Player{}
	rate := -1

	e.OnPvPHit = func(h PvPHit) { rate = h.Rate }

	d := d2combat.Damage{Fire: 2450 << 8}
	e.hurtPlayerRate(dst, src, &d, "Fire Wall", 41)

	if rate != 41 {
		t.Errorf("rate %d, want 41", rate)
	}

	d = d2combat.Damage{Fire: 2450 << 8}
	e.hurtPlayer(dst, src, &d, "Fire Wall")

	if rate != 0 {
		t.Errorf("plain hit rate %d, want 0", rate)
	}
}
