package d2statlist

import "testing"

// setPiece builds a worn set item; piece is 1 + the SetItems row.
func setPiece(set, piece, slot int, lists ...[]Prop) Item {
	return Item{Code: "x", Slot: slot, SetID: set, SetPiece: piece, SetLists: lists}
}

func TestSetDistinctPiecesAndLastTier(t *testing.T) {
	defs := SetDefs{1: {
		Pieces: 3, Members: []int{10, 11, 12}, AddFuncs: []int{2, 2, 2},
		Partial: [][]Prop{{{ID: StatVitality, Value: 5}}, {{ID: StatEnergy, Value: 7}}},
		Full:    []Prop{{ID: StatStrength, Value: 9}},
	}}
	env := &Env{Sets: defs}

	// two copies of the same piece are one piece: no 2-piece bonus
	dup := []Item{setPiece(1, 12, SlotRingRight), setPiece(1, 12, SlotRingLeft)}
	if v := Compute(Hero{Level: 1}, dup, env).Stats.Get(StatVitality); v != 0 {
		t.Fatalf("duplicate piece gave the 2-piece bonus: %d", v)
	}

	two := []Item{setPiece(1, 12, SlotRingRight), setPiece(1, 13, SlotRingLeft)}
	tt := Compute(Hero{Level: 1}, two, env)

	if tt.Stats.Get(StatVitality) != 5 || tt.Stats.Get(StatEnergy) != 0 {
		t.Fatalf("two pieces: vit %d ene %d", tt.Stats.Get(StatVitality), tt.Stats.Get(StatEnergy))
	}

	// 3 of 3: the 3-piece tier (the last one) is not used, the full bonus is
	all := append(two, setPiece(1, 11, SlotAmulet))
	tt = Compute(Hero{Level: 1}, all, env)

	if tt.Stats.Get(StatEnergy) != 0 || tt.Stats.Get(StatStrength) != 9 || tt.Stats.Get(StatVitality) != 5 {
		t.Fatalf("full set: vit %d ene %d str %d",
			tt.Stats.Get(StatVitality), tt.Stats.Get(StatEnergy), tt.Stats.Get(StatStrength))
	}
}

func TestSetAddFunc1MateLists(t *testing.T) {
	defs := SetDefs{1: {Pieces: 4, Members: []int{10, 11, 12, 13}, AddFuncs: []int{0, 0, 1, 0}}}
	// the piece at index 2 (row 12): list 0 = mate 0, list 1 = mate 1, list 2 = mate 3
	lists := [][]Prop{{{ID: StatVitality, Value: 1}}, {{ID: StatEnergy, Value: 2}}, {{ID: StatStrength, Value: 4}}}
	items := []Item{
		setPiece(1, 13, SlotRightHand, lists...),
		setPiece(1, 14, SlotAmulet), // row 13, index 3
	}

	tt := Compute(Hero{Level: 1}, items, &Env{Sets: defs})
	if tt.Stats.Get(StatStrength) != 4 || tt.Stats.Get(StatVitality) != 0 || tt.Stats.Get(StatEnergy) != 0 {
		t.Fatalf("mate lists: str %d vit %d ene %d",
			tt.Stats.Get(StatStrength), tt.Stats.Get(StatVitality), tt.Stats.Get(StatEnergy))
	}
}
