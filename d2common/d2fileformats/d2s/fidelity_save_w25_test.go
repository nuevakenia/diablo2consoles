package d2s

import (
	"encoding/binary"
	"testing"
)

// Wave 25 save fidelity: behaviours of SAVE_WriteStatsSection / the merc section that the earlier
// round trips did not pin down. Synthetic saves only.

func TestStatWriteSaturatesLikeTheExe(t *testing.T) {
	tb := synthTables(t)

	cases := []struct {
		name  string
		id    int
		value uint64
		want  uint64
	}{
		{"vitality in range", StatVitality, 255, 255},
		{"vitality over 10 bits saturates", StatVitality, 1 << 12, 1<<10 - 1},
		{"gold over 25 bits saturates", StatGold, 1 << 26, 1<<25 - 1},
		{"gold at the limit", StatGold, 1<<25 - 1, 1<<25 - 1},
		{"level over 7 bits saturates", StatLevel, 300, 127},
	}

	for _, tc := range cases {
		c := matrixBase(t, tb)
		c.Body.SetStat(tc.id, tc.value)

		out, err := Write(c, tb)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		back, err := Parse(out, tb)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		var got uint64

		for _, s := range back.Body.Stats {
			if s.ID == tc.id {
				got = s.Value
			}
		}

		if got != tc.want {
			t.Errorf("%s: wrote %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestSetStatKeepsAscendingOrder(t *testing.T) {
	tb := synthTables(t)
	c := matrixBase(t, tb)

	c.Body.SetStat(StatVitality, 7)
	c.Body.SetStat(StatDexterity, 9)
	c.Body.SetStat(StatStrength, 31) // existing: replaced in place

	last := -1

	for _, s := range c.Body.Stats {
		if s.ID <= last {
			t.Fatalf("stats not ascending: %+v", c.Body.Stats)
		}

		last = s.ID
	}

	roundTripExact(t, "set stat", c, tb)
}

func TestMercSectionFollowsExists(t *testing.T) {
	tb := synthTables(t)

	// an experience-only mercenary header still has an item list in the file
	c := matrixBase(t, tb)
	c.Header.Status = StatusExpansion
	c.Header.Mercenary = Mercenary{Experience: 5}

	item, err := NewItem(Item{Identified: true, Location: LocationEquipped, Equipped: 1, Code: "cap", ID: 3, Level: 1,
		Quality: QualityNormal, Defense: 3, MaxDurability: 5, Durability: 5}, tb)
	if err != nil {
		t.Fatal(err)
	}

	c.MercItems = []Item{item}

	out := roundTripExact(t, "exp-only merc", c, tb)

	back, err := Parse(out, tb)
	if err != nil || len(back.MercItems) != 1 {
		t.Fatalf("merc item lost: %v %d", err, len(back.MercItems))
	}

	// no merc at all: items are refused, not dropped
	c.Header.Mercenary = Mercenary{}
	if _, err := Write(c, tb); err == nil {
		t.Fatal("merc items without a mercenary were written")
	}
}

// TestHeaderLayoutTable pins every header field to its offset in a hand-assembled file.
func TestHeaderLayoutTable(t *testing.T) {
	tb := synthTables(t)
	c := matrixBase(t, tb)

	h := c.Header
	h.Name = "Layout"
	h.Class = Paladin
	h.Level = 77
	h.Status = StatusExpansion | StatusHardcore | StatusLadder
	h.Difficulty = [3]byte{0, 0x82, 0}
	h.MapSeed = 0xA1B2C3D4
	h.Mercenary = Mercenary{Dead: true, ID: 0x01020304, NameID: 0x0506, Type: 0x0708, Experience: 0x090A0B0C}

	out, err := Write(c, tb)
	if err != nil {
		t.Fatal(err)
	}

	le := binary.LittleEndian
	check := []struct {
		name string
		off  int
		got  uint64
		want uint64
	}{
		{"magic", 0, uint64(le.Uint32(out)), 0xAA55AA55},
		{"version", 4, uint64(le.Uint32(out[4:])), uint64(h.Version)},
		{"size", 8, uint64(le.Uint32(out[8:])), uint64(len(out))},
		{"checksum", 0xC, uint64(le.Uint32(out[0xC:])), uint64(Checksum(out))},
		{"status", 0x24, uint64(out[0x24]), 0x04 | 0x20 | 0x40},
		{"class", 0x28, uint64(out[0x28]), uint64(Paladin)},
		{"level", 0x2B, uint64(out[0x2B]), 77},
		{"normal difficulty", 0xA8, uint64(out[0xA8]), 0},
		{"nightmare difficulty", 0xA9, uint64(out[0xA9]), 0x82},
		{"hell difficulty", 0xAA, uint64(out[0xAA]), 0},
		{"map seed", 0xAB, uint64(le.Uint32(out[0xAB:])), 0xA1B2C3D4},
		{"merc dead", 0xB1, uint64(out[0xB1] & 1), 1},
		{"merc id", 0xB3, uint64(le.Uint32(out[0xB3:])), 0x01020304},
		{"merc name", 0xB7, uint64(le.Uint16(out[0xB7:])), 0x0506},
		{"merc type", 0xB9, uint64(le.Uint16(out[0xB9:])), 0x0708},
		{"merc exp", 0xBB, uint64(le.Uint32(out[0xBB:])), 0x090A0B0C},
	}

	for _, tc := range check {
		if tc.got != tc.want {
			t.Errorf("%s at 0x%X: got 0x%X want 0x%X", tc.name, tc.off, tc.got, tc.want)
		}
	}

	if string(out[0x14:0x14+6]) != "Layout" || out[0x1A] != 0 {
		t.Errorf("name field: %q", out[0x14:0x24])
	}

	back, err := Parse(out, tb)
	if err != nil {
		t.Fatal(err)
	}

	if d, act, ok := back.Header.ActiveDifficulty(); !ok || d != 1 || act != 2 {
		t.Errorf("active difficulty/act = %d/%d/%v, want 1/2/true", d, act, ok)
	}
}
