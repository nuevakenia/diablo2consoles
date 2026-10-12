package d2player

import (
	"reflect"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

func TestStatSpendCount(t *testing.T) {
	// VERIFIED 0x4a3f80: plain click 1, Shift all remaining, Control nothing special.
	cases := []struct {
		mod       d2enum.KeyMod
		remaining int
		want      int
	}{
		{0, 0, 0}, {0, 7, 1}, {d2enum.KeyModShift, 7, 7}, {d2enum.KeyModShift, 3, 3}, {d2enum.KeyModShift, 250, 250},
		{d2enum.KeyModControl, 12, 1}, {d2enum.KeyModControl | d2enum.KeyModShift, 12, 12}, {d2enum.KeyModShift, -1, 0},
	}
	for _, c := range cases {
		if got := StatSpendCount(c.mod, c.remaining); got != c.want {
			t.Errorf("mod %v remaining %d: got %d want %d", c.mod, c.remaining, got, c.want)
		}
	}
}

func TestStatSpendPackets(t *testing.T) {
	cases := map[int][]int{0: nil, 1: {1}, 32: {32}, 33: {32, 1}, 70: {32, 32, 6}}
	for n, want := range cases {
		if got := StatSpendPackets(n); !reflect.DeepEqual(got, want) {
			t.Errorf("n=%d got %v want %v", n, got, want)
		}
	}
}

func TestStatAllocPacket(t *testing.T) {
	// opcode 0x3a, word = id | (count-1)<<8, little endian
	if got := StatAllocPacket(StatAllocVitality, 32); got != [3]byte{0x3a, 0x03, 0x1f} {
		t.Fatalf("got %x", got)
	}

	if got := StatAllocPacket(StatAllocStrength, 1); got != [3]byte{0x3a, 0x00, 0x00} {
		t.Fatalf("got %x", got)
	}

	if !StatAllocServerAccepts(15, 100) || StatAllocServerAccepts(16, 1) || StatAllocServerAccepts(0, 101) ||
		StatAllocServerAccepts(0, 0) {
		t.Fatal("server range checks")
	}
}

func TestStatValueColour(t *testing.T) {
	if StatValueColour(10, 10) != StatColourNormal || StatValueColour(11, 10) != StatColourIncreased ||
		StatValueColour(9, 10) != StatColourDecreased {
		t.Fatal("colour rule")
	}

	if StatColourIncreased.FontIndex() != 3 || StatColourDecreased.FontIndex() != 1 || StatColourNormal.FontIndex() != 0 {
		t.Fatal("font indices")
	}
}

func TestVitalColour(t *testing.T) {
	// current life, mana and stamina stay white, the maxima follow the base value
	if VitalColour(6, 500, 100) != 0 || VitalColour(8, 1, 100) != 0 || VitalColour(10, 5, 100) != 0 {
		t.Fatal("current values are white")
	}

	if VitalColour(7, 150, 100) != 3 || VitalColour(9, 50, 100) != 1 || VitalColour(11, 100, 100) != 0 {
		t.Fatal("maxima")
	}
}

func TestResistColour(t *testing.T) {
	cases := []struct {
		name            string
		shown, maxRes   int
		raised, lowered bool
		want            int
	}{
		{"plain", 30, 0, false, false, 0},
		{"negative", -20, 0, false, false, 1},
		{"at cap gold", 75, 0, false, false, 4},
		{"over cap shows cap", 120, 0, false, false, 4},
		{"cap raised by max resist", 85, 10, false, false, 4},
		{"cap is 95", 99, 40, false, false, 4},
		{"blue at cap stays blue", 75, 0, true, false, 3},
		{"blue below cap", 30, 0, true, false, 3},
		{"red state below cap", 30, 0, false, true, 1},
		{"floor", -150, 0, false, false, 1},
	}
	for _, c := range cases {
		if got := ResistColour(c.shown, c.maxRes, c.raised, c.lowered); got != c.want {
			t.Errorf("%s: colour %d want %d", c.name, got, c.want)
		}
	}

	if ResistDisplayed(120, 0) != 75 || ResistDisplayed(-150, 0) != -100 || ResistDisplayed(33, 0) != 33 || ResistDisplayed(99, 40) != 95 {
		t.Fatal("displayed clamp")
	}
}

func TestDefenseColour(t *testing.T) {
	if DefenseColour(false, false) != 0 || DefenseColour(true, false) != 3 || DefenseColour(true, true) != 1 || DefenseColour(false, true) != 1 {
		t.Fatal("defense colours")
	}
}

func TestExpBarWidth(t *testing.T) {
	// golden values: level 1 -> 2 needs 500 experience (Experience.txt), the bar is 0x77 = 119 pixels
	cases := []struct {
		exp, prev, next, level, max int
		want                        int
	}{
		{0, 0, 500, 1, 99, 0},
		{250, 0, 500, 1, 99, 59},                    // 250*119/500
		{499, 0, 500, 1, 99, 118},                   //
		{500, 500, 1500, 2, 99, 0},                  // just leveled: bar empty (old code drew 33%)
		{1000, 500, 1500, 2, 99, 59},                // half of level 2
		{900, 500, 1500, 2, 99, 47},                 // 400*119/1000
		{600, 500, 1500, 99, 99, 0},                 // maximum level: nothing is drawn
		{100, 500, 1500, 2, 99, 0},                  // below the previous threshold (damaged value)
		{2000, 500, 1500, 2, 99, 0},                 // over the next threshold: width above 0x77 counts as 0
		{500_000_000, 0, 1_000_000_000, 80, 99, 59}, // large values use the >>7 scaling
	}
	for _, c := range cases {
		if got := ExpBarWidth(c.exp, c.prev, c.next, c.level, c.max); got != c.want {
			t.Errorf("%+v: got %d want %d", c, got, c.want)
		}
	}
}
