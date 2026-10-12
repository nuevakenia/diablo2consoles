package d2statlist

import (
	"os"
	"path/filepath"
	"testing"
)

// The gem column group comes from the host base's gemapplytype (ITEM_GetGemApplyType
// 629c10, VERIFIED), not from "has a block value": the Buckler is a shield with block 0.
func TestGemColumnFollowsGemApplyType(t *testing.T) {
	gems := GemTable{"g": {
		Weapon: []Prop{{ID: StatToHit, Value: 1}},
		Helm:   []Prop{{ID: StatDexterity, Value: 2}},
		Shield: []Prop{{ID: StatStrength, Value: 3}},
	}}

	for _, c := range []struct {
		name string
		it   Item
		want int
		prop int
	}{
		{"buckler", Item{GemKind: 3}, 3, StatStrength},
		{"weapon", Item{GemKind: 1, Weapon: &WeaponBase{}}, 1, StatToHit},
		{"helm", Item{GemKind: 2}, 2, StatDexterity},
		{"fallback block", Item{BaseBlock: 30}, 3, StatStrength},
		{"fallback weapon", Item{Weapon: &WeaponBase{}}, 1, StatToHit},
		{"fallback armor", Item{}, 2, StatDexterity},
	} {
		c.it.Sockets = []SocketedItem{{Code: "g"}}

		got := (&Env{Gems: gems}).gemProps(&c.it)
		if c.it.gemColumn() != c.want || len(got) != 1 || got[0].ID != c.prop {
			t.Errorf("%s: column %d props %v", c.name, c.it.gemColumn(), got)
		}
	}
}

// With the real tables: every base with sockets gets the column the exe reads.
func TestBaseGemKindRealTables(t *testing.T) {
	dir := os.Getenv("D2_TABLES")
	if dir == "" {
		t.Skip("D2_TABLES not set")
	}

	armor, err1 := os.ReadFile(filepath.Join(dir, "armor.txt"))
	weapons, err2 := os.ReadFile(filepath.Join(dir, "weapons.txt"))

	if err1 != nil || err2 != nil {
		t.Skip("tables missing")
	}

	b, err := ParseBases(armor, weapons)
	if err != nil {
		t.Fatal(err)
	}

	it := Item{GemKind: b["buc"].GemKind}
	if it.gemColumn() != 3 {
		t.Errorf("buckler gem column %d, want shield (3)", it.gemColumn())
	}

	lsd := Item{GemKind: b["lsd"].GemKind}
	if k := lsd.gemColumn(); k != 1 {
		t.Errorf("long sword column %d", k)
	}
}
