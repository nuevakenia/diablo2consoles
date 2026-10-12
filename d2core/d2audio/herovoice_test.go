package d2audio

import "testing"

func TestVoiceHandle(t *testing.T) {
	idx := map[string]int{"druid_act1_complete_andariel": 100}
	at := map[int]string{100: "druid_act1_complete_andariel", 101: "druid_act1_complete_burial"}
	indexOf := func(h string) (int, bool) { i, ok := idx[h]; return i, ok }
	handleAt := func(i int) string { return at[i] }

	tests := []struct {
		class string
		id    int
		want  string
	}{
		{"amazon", VoiceNeedMana, "amazon_needmana_1"},
		{"Sorceress", VoiceNotInTown, "sorceress_not_in_town_1"},
		{"assassin", VoiceCantCarry, "assassin_cant_carry"},
		{"paladin", VoiceCantUse, "paladin_cantuseyet"},
		{"barbarian", 0x19, "barbarian_chat_a_1"},
		{"barbarian", 0x20, "barbarian_chat_h_1"},
		{"druid", 0x21, "druid_act1_complete_andariel"},
		{"druid", 0x22, "druid_act1_complete_burial"},
		{"druid", 0x23, ""},
		{"druid", 0x54, ""},
		{"druid", 0x05, ""},
	}
	for _, tc := range tests {
		got, ok := VoiceHandle(tc.class, tc.id, indexOf, handleAt)
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("%s %#x = %q,%v want %q", tc.class, tc.id, got, ok, tc.want)
		}
	}
}

func TestVoiceDelay(t *testing.T) {
	for id, want := range map[int]int{0x13: 0, 0x21: 12, 0x26: 0, 0x2c: 0, 0x2d: 12, 0x37: 0, 0x53: 12} {
		if got := VoiceDelay(id); got != want {
			t.Errorf("delay %#x = %d want %d", id, got, want)
		}
	}
}

func TestVoiceGate(t *testing.T) {
	var g VoiceGate
	if !g.Allow(VoiceNeedMana, 100, false, false) {
		t.Fatal("first needmana refused")
	}
	if g.Allow(VoiceNeedMana, 150, false, false) {
		t.Error("repeat within 74 ticks allowed")
	}
	if !g.Allow(VoiceNeedKey, 150, false, false) {
		t.Error("different refusal blocked")
	}
	if !g.Allow(VoiceNeedMana, 150+VoiceRepeatTicks+1, false, false) {
		t.Error("repeat after the gap refused")
	}
	if g.Allow(VoiceNeedKey, 1000, true, false) || g.Allow(0x19, 1000, true, false) {
		t.Error("voice over a playing voice allowed")
	}
	if g.Allow(VoiceNeedKey, 2000, false, true) || g.Allow(0x21, 2000, false, true) {
		t.Error("dead hero spoke")
	}
	if !g.Allow(0x21, 2000, true, false) {
		t.Error("quest line is not gated by a playing voice")
	}
}
