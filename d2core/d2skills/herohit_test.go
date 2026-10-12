package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// A strike reads the equipment's elemental, poison, leech and strike stats, the attack rating and the
// weapon damage with enhanced damage from the totals (w23 damage).
func TestHeroUnitReadsEquipment(t *testing.T) {
	list := d2statlist.NewList()
	list.Add(48, 0, 3) // firemindam
	list.Add(49, 0, 9)
	list.Add(60, 0, 4) // lifedrainmindam
	list.Add(141, 0, 25)

	e := newTestEngine()
	st := &d2hero.HeroStatsState{}
	st.Totals = &d2statlist.Totals{Stats: list, AttackRatingFlat: 777, Weapon: d2statlist.WeaponParts{Armed: true, Min: 12, Max: 31}}
	p := &d2mapentity.Player{Stats: st}
	h := &heroUnit{e: e, p: p}

	tests := []struct {
		name string
		got  int
		want int
	}{
		{"fire min", h.itemStat("firemindam"), 3},
		{"fire max", h.itemStat("firemaxdam"), 9},
		{"life leech", h.itemStat("lifedrainmindam"), 4},
		{"deadly strike", h.itemStat("item_deadlystrike"), 25},
		{"not an item name", h.itemStat("damagepercent"), 0},
		{"attack rating", h.AttackRating(), 777},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: %d want %d", tt.name, tt.got, tt.want)
		}
	}

	if lo, hi := h.WeaponDamage(); lo != 12 || hi != 31 {
		t.Errorf("weapon damage %d-%d", lo, hi)
	}

	if g := e.heroStatGetter(p); g(49) != 9 || g(141) != 25 {
		t.Errorf("getter %d %d", g(49), g(141))
	}
}
