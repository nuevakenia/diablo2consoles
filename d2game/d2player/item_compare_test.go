package d2player

import (
	"strings"
	"testing"
)

func TestCompareLine(t *testing.T) {
	if l := compareLine("Defense:", 11, 10); !strings.Contains(l, "Defense: +10%") {
		t.Errorf("better line %q", l)
	}

	if l := compareLine("Damage:", 8, 10); !strings.Contains(l, "Damage: -20%") {
		t.Errorf("worse line %q", l)
	}

	if l := compareLine("Damage:", 5, 5); l != "" {
		t.Errorf("equal values give %q", l)
	}

	if l := compareLine("Defense:", 7, 0); !strings.Contains(l, "Defense: +7") {
		t.Errorf("no worn value: %q", l)
	}

	if better, worse := compareLine("X", 2, 1), compareLine("X", 1, 2); better == worse {
		t.Error("better and worse use the same colour")
	}
}

type fakeDescItem struct{ InventoryItem }

func (fakeDescItem) GetItemDescription() []string {
	return []string{"Cap", "Defense: 12", "One-Hand Damage: 3 to 7", "Durability: 12 of 12"}
}

func TestItemNumbers(t *testing.T) {
	def, dmg := itemNumbers(fakeDescItem{})
	if def != 12 || dmg != 5 {
		t.Errorf("defense %d damage %v", def, dmg)
	}
}
