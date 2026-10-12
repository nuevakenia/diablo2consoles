package d2statlist

import (
	"os"
	"path/filepath"
	"testing"
)

// Wave 31: the speed column of the worn items. ITEMGEN_InitItemBaseStats (0x555b10) puts -speed on an armor piece as
// stat 0x43 and on a weapon as stat 0x44; the totals expose the sum for the movement and attack rate rules.
func TestComputeArmorAndWeaponSpeed(t *testing.T) {
	plate := Item{Slot: SlotTorso, BaseSpeed: 10}
	tower := Item{Slot: SlotLeftHand, BaseSpeed: 10, BaseBlock: 25}
	helm := Item{Slot: SlotHead}
	sword := Item{Slot: SlotRightHand, BaseSpeed: -10, Weapon: &WeaponBase{Min: 1, Max: 2}}
	swap := Item{Slot: SlotSwapRight, BaseSpeed: 30, Weapon: &WeaponBase{Min: 1, Max: 2}}
	broken := Item{Slot: SlotGloves, BaseSpeed: 5, Broken: true}
	inv := Item{Slot: 0, BaseSpeed: 10}

	cases := []struct {
		name                     string
		items                    []Item
		armor, body, weapon, drn int
	}{
		{"naked", nil, 0, 0, 0, 0},
		{"plate and tower", []Item{plate, tower, helm}, 20, 10, 0, 0},
		{"weapon speed", []Item{sword, swap}, 0, 0, -10, 0},
		{"broken, swap and inventory pieces count for nothing", []Item{broken, swap, inv}, 0, 0, 0, 0},
		{"everything", []Item{plate, tower, sword}, 20, 10, -10, 0},
	}

	for _, c := range cases {
		got := Compute(hero(), c.items, nil)
		if got.ArmorSpeed != c.armor || got.BodyArmorSpeed != c.body || got.WeaponSpeed != c.weapon {
			t.Errorf("%s: armor %d body %d weapon %d, want %d %d %d", c.name,
				got.ArmorSpeed, got.BodyArmorSpeed, got.WeaponSpeed, c.armor, c.body, c.weapon)
		}
	}

	// the stamina stats come from the property lists
	ring := Item{Slot: SlotRingLeft, Props: []Prop{{ID: StatStaminaDrain, Value: 20}, {ID: StatStamRecovery, Value: 30}}}
	if got := Compute(hero(), []Item{ring}, nil); got.StaminaDrainPct != 20 || got.StaminaRecoveryPct != 30 {
		t.Errorf("stamina stats %d %d", got.StaminaDrainPct, got.StaminaRecoveryPct)
	}
}

// With the real tables (D2_TABLES): only body armor and shields carry a speed (5 or 10), helmets, gloves, boots and
// belts none; the weapons' column is the weapon speed modifier.
func TestBaseSpeedFromTables(t *testing.T) {
	dir := os.Getenv("D2_TABLES")
	if dir == "" {
		t.Skip("D2_TABLES not set")
	}

	armor, err := os.ReadFile(filepath.Join(dir, "d2data", "armor.txt"))
	if err != nil {
		t.Skip(err)
	}

	weapons, err := os.ReadFile(filepath.Join(dir, "d2data", "weapons.txt"))
	if err != nil {
		t.Skip(err)
	}

	bases, err := ParseBases(armor, weapons)
	if err != nil {
		t.Fatal(err)
	}

	for code, want := range map[string]int{
		"plt": 10, "ful": 10, "tow": 10, "lrg": 5, "chn": 5, "qui": 0, "cap": 0, "lgl": 0, "lbt": 0, "lbl": 0,
	} {
		if b, ok := bases[code]; !ok || b.Speed != want {
			t.Errorf("armor %s speed %d (found %v), want %d", code, b.Speed, ok, want)
		}
	}

	seenFast, seenSlow := false, false

	for code, b := range bases {
		if b.Weapon == nil {
			if b.Speed != 0 && b.Speed != 5 && b.Speed != 10 {
				t.Errorf("armor %s has speed %d", code, b.Speed)
			}

			continue
		}

		seenFast = seenFast || b.Speed < 0
		seenSlow = seenSlow || b.Speed > 0
	}

	if !seenFast || !seenSlow {
		t.Error("weapons.txt should hold both fast and slow weapons")
	}
}
