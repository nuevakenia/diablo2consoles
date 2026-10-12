package d2monsters

import "testing"

// Table: which units a hero may hurt (UNIT_AreUnitsHostile model), and that the hero-sourced entry points leave a
// friendly unit's life alone.
func TestFriendlyGuard(t *testing.T) {
	owner := golemOwner(500)

	pet := golemUnit(1, owner, 0, 0, 300, 300)
	merc := petUnit(2, 0, 0, "Merc", "")
	merc.merc = &mercUnit{}
	conv := petUnit(3, 0, 0, "Skeleton", "")
	conv.b.Allied = true
	foe := petUnit(4, 5, 5, "Zombie", "")
	ownerless := petUnit(5, 0, 0, "raven", "minion")

	tests := []struct {
		name    string
		u       *unit
		hostile bool
	}{
		{"summon of the hero", pet, false},
		{"mercenary", merc, false},
		{"converted monster", conv, false},
		{"summon without owner record", ownerless, false},
		{"ordinary monster", foe, true},
	}

	for _, tc := range tests {
		tc.u.m.Vitals.HP, tc.u.m.Vitals.MaxHP = 100, 100
		d := testDirector(tc.u)
		d.byEntity[tc.u.m.ID()] = tc.u

		if got := d.HostileToHeroes(tc.u.m); got != tc.hostile {
			t.Errorf("%s: HostileToHeroes=%v want %v", tc.name, got, tc.hostile)
		}

		if tc.hostile {
			continue
		}

		d.Damage(tc.u.m, 40, owner)
		d.DamageOverTime(tc.u.m, 40, owner)

		if tc.u.m.Vitals.HP != 100 {
			t.Errorf("%s: a hero hurt its own unit, hp=%d", tc.name, tc.u.m.Vitals.HP)
		}

		if d.HeroStrike(owner, tc.u.m) {
			t.Errorf("%s: HeroStrike hit its own unit", tc.name)
		}

		if d.Counters.FriendlyRefused != 3 {
			t.Errorf("%s: refused=%d want 3", tc.name, d.Counters.FriendlyRefused)
		}
	}
}

// The corpse of a converted monster is not a Corpse Explosion / Raise target of the hero.
func TestCorpsesSkipFriendlyDead(t *testing.T) {
	conv := petUnit(1, 0, 0, "Skeleton", "")
	conv.b.Allied = true
	conv.m.Vitals.HP = 0

	if got := testDirector(conv).Corpses(); len(got) != 0 {
		t.Errorf("a converted monster's corpse listed: %d", len(got))
	}
}
