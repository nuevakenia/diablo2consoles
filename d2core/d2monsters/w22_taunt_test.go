package d2monsters

import "testing"

// The Baal Taunt is placed by the DS1 stairs rooms of the Worldstone Keep (baal*up*.ds1 hold a preset monster named
// baaltaunt, found with the stamp scan of wave 22; no immediate in Game.exe creates the class). The Director adopts a
// DS1 placement when its monstats row is hostile: this pins that the taunter's row is.
func TestBaalTauntRowIsAdoptable(t *testing.T) {
	_, rm := loadReal(t)

	st := rm.Monster.Stats["baaltaunt"]
	if st == nil {
		t.Fatal("no baaltaunt row")
	}

	if !IsHostile(st) {
		t.Errorf("baaltaunt: enabled %v npc %v interactable %v align %d ai %q: not adopted by the Director", st.Enabled,
			st.IsNpc, st.IsInteractable, st.Alignment, st.AiKey)
	}
}
