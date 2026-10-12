package d2statlist

import "testing"

// The thorns, damage-to-mana and absorb stats land in the Totals (w23 damage).
func TestTotalsDamageEventStats(t *testing.T) {
	it := Item{Slot: SlotRightHand, Props: []Prop{
		{ID: StatDamageToMana, Value: 7}, {ID: StatAttackerTakes, Value: 12}, {ID: StatAttackerTakesLight, Value: 9},
		{ID: 142, Value: 5}, {ID: 143, Value: 3}, {ID: 144, Value: 6}, {ID: 145, Value: 4},
		{ID: 146, Value: 7}, {ID: 147, Value: 5}, {ID: 148, Value: 8}, {ID: 149, Value: 6},
	}}

	got := Compute(hero(), []Item{it}, nil)

	if got.DamageToMana != 7 || got.AttackerTakes != 12 || got.AttackerTakesLight != 9 {
		t.Fatalf("events %d %d %d", got.DamageToMana, got.AttackerTakes, got.AttackerTakesLight)
	}

	if got.AbsorbPct != [4]int{5, 6, 7, 8} || got.AbsorbFlat != [4]int{3, 4, 5, 6} {
		t.Fatalf("absorb %v %v", got.AbsorbPct, got.AbsorbFlat)
	}
}
