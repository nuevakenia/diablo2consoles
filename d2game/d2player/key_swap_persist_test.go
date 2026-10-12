package d2player

import (
	"encoding/json"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

func swapTestKeyMap() *KeyMap {
	km := &KeyMap{mapping: map[d2enum.Key]d2enum.GameEvent{}, controls: map[d2enum.GameEvent]*KeyBinding{}}
	km.SetPrimaryBinding(d2enum.ToggleInventoryPanel, d2enum.KeyI)
	km.SetPrimaryBinding(d2enum.ToggleCharacterPanel, d2enum.KeyC)
	km.SetPrimaryBinding(d2enum.ToggleSkillTreePanel, d2enum.KeyT)
	km.SetSecondaryBinding(d2enum.ToggleInventoryPanel, d2enum.KeyB)

	return km
}

// A saved layout must come back exactly, also when two events swap keys:
// applying one event first would otherwise unbind the other (map order).
func TestBindingsRoundTripWithSwap(t *testing.T) {
	src := swapTestKeyMap()
	src.SetPrimaryBinding(d2enum.ToggleInventoryPanel, d2enum.KeyX)
	src.SetPrimaryBinding(d2enum.ToggleCharacterPanel, d2enum.KeyI)
	src.SetPrimaryBinding(d2enum.ToggleInventoryPanel, d2enum.KeyC) // inventory C, character I: a swap

	raw, err := json.Marshal(src.ExportBindings())
	if err != nil {
		t.Fatal(err)
	}

	events := []d2enum.GameEvent{d2enum.ToggleInventoryPanel, d2enum.ToggleCharacterPanel, d2enum.ToggleSkillTreePanel}

	for i := 0; i < 40; i++ { // map iteration order is random
		var saved map[string][]string
		if err := json.Unmarshal(raw, &saved); err != nil {
			t.Fatal(err)
		}

		dst := swapTestKeyMap()
		if n := dst.ApplyBindings(saved); n != len(saved) {
			t.Fatalf("applied %d of %d", n, len(saved))
		}

		for _, ev := range events {
			if got, want := dst.GetKeysForGameEvent(ev).Primary, src.GetKeysForGameEvent(ev).Primary; got != want {
				t.Fatalf("run %d event %v: primary %v want %v", i, ev, got, want)
			}
		}
	}
}

func TestBindingsSkipUnknown(t *testing.T) {
	km := swapTestKeyMap()
	n := km.ApplyBindings(map[string][]string{
		"NoSuchEvent":          {"A", ""},
		"ToggleQuestLog":       {"NoSuchKey", ""},
		"ToggleInventoryPanel": {"Q"},
	})

	if n != 0 || km.GetKeysForGameEvent(d2enum.ToggleInventoryPanel).Primary != d2enum.KeyI {
		t.Fatalf("bad entries must be ignored, applied=%d", n)
	}
}
