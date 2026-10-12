package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

func deathSentryRig(t *testing.T) (*Director, *unit, *unit, *unit, *fakeFirer) {
	t.Helper()

	sentry := petUnit(1, 10, 10, "DeathSentry", "trap")
	sentry.b.Profile = &d2monster.Profile{AI: "DeathSentry"}
	foe := petUnit(2, 20, 10, "Melee", "")
	near := petUnit(3, 22, 10, "Melee", "")
	far := petUnit(4, 40, 10, "Melee", "")

	for _, c := range []*unit{near, far} {
		c.m.MarkCorpse(2)
	}

	d := testDirector(sentry, foe, near, far)
	d.byEntity[sentry.m.ID()] = sentry
	f := &fakeFirer{ok: true}
	d.SetSkillFirer(f)
	sentry.ally.owner = &d2mapentity.Player{}

	return d, sentry, foe, near, f
}

// FindReviveCorpse searches 10 subtiles around the TARGET (0x56c140), not around the sentry.
func TestDeathSentryCorpseSearch(t *testing.T) {
	d, sentry, foe, near, _ := deathSentryRig(t)

	tgt := d2monster.Target{ID: foe.b.ID, X: 20, Y: 10, Size: 1}

	c, ok := d.FindReviveCorpse(sentry.b, 0, 1, tgt)
	if !ok || c.ID != corpseTargetBase+near.b.ID {
		t.Fatalf("got %+v ok=%v", c, ok)
	}

	if _, ok := d.FindReviveCorpse(sentry.b, 0, 1, d2monster.Target{X: 100, Y: 100}); ok {
		t.Error("corpse beyond the search radius")
	}

	near.raising = true

	if _, ok := d.FindReviveCorpse(sentry.b, 0, 1, tgt); ok {
		t.Error("reserved corpse found")
	}
}

// Skill1 on a corpse goes to the firer with the corpse; Skill2 on a monster as a normal shot.
func TestDeathSentryRouting(t *testing.T) {
	d, sentry, foe, near, f := deathSentryRig(t)
	sentry.ally.trap = &TrapSpec{SkillID: 276, Death: true, ExplodeSkillID: 312, Level: 5, Life: 9, ReviveRadius: 14, Missile2: "bolt"}

	if handled, ok := d.fireTrap(sentry, 0, d2monster.Target{ID: corpseTargetBase + near.b.ID}); !handled || !ok {
		t.Fatalf("corpse cast handled=%v ok=%v", handled, ok)
	}

	if s := f.shots[0]; s.Corpse != near.m || s.Slot != 0 || s.Target != nil {
		t.Errorf("shot %+v", s)
	}

	if handled, ok := d.fireTrap(sentry, 1, d2monster.Target{ID: foe.b.ID}); !handled || !ok || f.shots[1].Target != foe.m || f.shots[1].Slot != 1 {
		t.Errorf("monster cast %+v", f.shots)
	}

	sentry.ally.trap.Death = false

	if _, ok := d.fireTrap(sentry, 0, d2monster.Target{ID: corpseTargetBase + near.b.ID}); ok {
		t.Error("corpse fired by another sentry")
	}

	sentry.ally.trap.Death = true

	if _, ok := d.fireTrap(sentry, 0, d2monster.Target{ID: corpseTargetBase + foe.b.ID}); ok {
		t.Error("living unit exploded")
	}
}

// The Death Sentry is AI driven only with a shot budget; its radius comes from the spec.
func TestDeathSentryAIDriven(t *testing.T) {
	d, sentry, _, _, _ := deathSentryRig(t)

	sentry.ally.trap = &TrapSpec{Death: true, Level: 5, Life: 0}
	if d.TrapAIDriven(sentry.m) {
		t.Error("no budget")
	}

	sentry.ally.trap.Life = 9
	if !d.TrapAIDriven(sentry.m) {
		t.Error("budget set")
	}

	sentry.ally.trap.ReviveRadius = 14
	if d.ReviveRadius(sentry.b, 0, 5) != 14 {
		t.Error("radius")
	}
}
