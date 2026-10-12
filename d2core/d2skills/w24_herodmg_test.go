package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// A hero with equipment totals gives the skill engine the gear's weapon range (before the percent pool), the
// attack rating without its percent, the gear's share of stats 0x19 / 0x77, and the thrown range of a javelin.
func TestHeroUnitGearNumbers(t *testing.T) {
	cases := []struct {
		name         string
		w            d2statlist.WeaponParts
		str          int
		wantMelee    [2]int
		wantMissile  [2]int
		wantBonusPct int
		wantBare     int
	}{
		{"bow", d2statlist.WeaponParts{Armed: true, Min: 23, Max: 36, BonusPct: 50}, 30,
			[2]int{23, 36}, [2]int{23, 36}, 50, 0},
		{"javelin", d2statlist.WeaponParts{Armed: true, Min: 4, Max: 12, ThrowMin: 14, ThrowMax: 30, Throwing: true, BonusPct: 45},
			30, [2]int{4, 12}, [2]int{14, 30}, 45, 0},
		{"bare hands", d2statlist.WeaponParts{Min: 1, Max: 6}, 30, [2]int{1, 6}, [2]int{1, 6}, 0, 30},
	}

	for _, tc := range cases {
		e := newTestEngine()
		p := &d2mapentity.Player{Stats: &d2hero.HeroStatsState{}}
		p.Stats.Totals = &d2statlist.Totals{Weapon: tc.w, Str: tc.str, AttackRatingFlat: 700, AttackRating: 840,
			GearDamagePct: 35, GearToHitPct: 20}
		u := e.hero(p)

		if mn, mx := u.WeaponDamage(); mn != tc.wantMelee[0] || mx != tc.wantMelee[1] {
			t.Errorf("%s: melee %d-%d", tc.name, mn, mx)
		}

		if mn, mx := u.MissileWeaponDamage(); mn != tc.wantMissile[0] || mx != tc.wantMissile[1] {
			t.Errorf("%s: missile %d-%d", tc.name, mn, mx)
		}

		if u.WeaponBonusPct() != tc.wantBonusPct || u.BareHandPct() != tc.wantBare {
			t.Errorf("%s: bonus %d bare %d", tc.name, u.WeaponBonusPct(), u.BareHandPct())
		}

		if u.AttackRating() != 700 {
			t.Errorf("%s: attack rating %d, want the flat 700", tc.name, u.AttackRating())
		}

		if u.Stat("damagepercent") != 35 || u.Stat("item_tohit_percent") != 20 || u.Stat("item_deadlystrike") != 0 {
			t.Errorf("%s: gear stats %d %d", tc.name, u.Stat("damagepercent"), u.Stat("item_tohit_percent"))
		}
	}
}
