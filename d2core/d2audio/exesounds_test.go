package d2audio

import (
	"reflect"
	"testing"
)

func TestItemSounds(t *testing.T) {
	for _, tc := range []struct {
		name             string
		base             ItemSoundRow
		special          *ItemSoundRow
		drop, use, frame int
	}{
		{"base only", ItemSoundRow{Drop: 5, Use: 6}, nil, 5, 6, 12},
		{"base frame", ItemSoundRow{Drop: 5, Use: 6, DropFrame: 9}, nil, 5, 6, 9},
		{"no drop sound: use sound ignored", ItemSoundRow{Use: 6}, nil, 0, 0, 12},
		{"unique overrides", ItemSoundRow{Drop: 5, Use: 6, DropFrame: 9}, &ItemSoundRow{Drop: 7, Use: 8}, 7, 8, 9},
		{"unique use may be 0", ItemSoundRow{Drop: 5, Use: 6}, &ItemSoundRow{Drop: 7, DropFrame: 3}, 7, 0, 3},
		{"unique without drop keeps base", ItemSoundRow{Drop: 5, Use: 6}, &ItemSoundRow{Use: 8}, 5, 6, 12},
	} {
		d, u, f := ItemSounds(tc.base, tc.special)
		if d != tc.drop || u != tc.use || f != tc.frame {
			t.Errorf("%s: got %d/%d/%d want %d/%d/%d", tc.name, d, u, f, tc.drop, tc.use, tc.frame)
		}
	}
}

func TestGroundDropPlan(t *testing.T) {
	want := []GroundDrop{{ID: SndItemFlippy}, {ID: 40, Delay: 12}}
	if got := GroundDropPlan(40, 12); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}

	if got := GroundDropPlan(0, 12); len(got) != 1 || got[0].ID != SndItemFlippy {
		t.Errorf("no drop sound: %v", got)
	}
}

func TestImpactSounds(t *testing.T) {
	for _, tc := range []struct {
		hc, unit, extra, body, layer int
	}{
		{0, 1, 0, 0, 0},
		{1, 1, 0, 0x141, 0},
		{0x11, 1, 0, 0x141, 0x17b},
		{0x10, 1, 0, 0, 0},
		{0x20, 1, 0, 0, 0x161},
		{0x23, 1, 0, 0x155, 0x164},
		{0x30, 1, 0, 0, 0x167},
		{0x31, 1, 0, 0x141, 0},
		{0x31, 2, 0, 0x141, 0x16a},
		{0x41, 1, 0, 0x141, 0x174},
		{0x40, 1, 0, 0, 0x170},
		{0x50, 1, 0, 0, 0x178},
		{0x51, 1, 0, 0x141, 0},
		{0x6c, 1, 0, 0x145, 0x17e},
		{0x70, 1, 0, 0, 0x18b},
		{0x80, 1, 0, 0, 0x956},
		{0xb9, 1, 0, 0x149, 0x187},
	} {
		b, l := ImpactSounds(tc.hc, tc.unit, tc.extra)
		if b != tc.body || l != tc.layer {
			t.Errorf("hc=%#x unit=%d: got %#x/%#x want %#x/%#x", tc.hc, tc.unit, b, l, tc.body, tc.layer)
		}
	}
}

func TestMonsterInitAndFlee(t *testing.T) {
	if !MonsterInitSound(true, false, false) || MonsterInitSound(true, true, false) ||
		MonsterInitSound(true, false, true) || MonsterInitSound(false, false, false) {
		t.Error("init gate")
	}

	if FleeThrottle(3) || !FleeThrottle(4) {
		t.Error("flee throttle")
	}
}
