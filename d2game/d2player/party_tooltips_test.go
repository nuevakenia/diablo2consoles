package d2player

import (
	"reflect"
	"testing"
)

// The string id pairs are those of UI_DrawPartyScreenButtonTooltips 0x4977f0.
func TestPartyTooltipIDs(t *testing.T) {
	tests := []struct {
		name string
		k    partyTooltipKeys
		want []int
	}{
		{"hostile clear", partyHostileClear, []int{0x100a, 0x100b}},
		{"hostile set", partyHostileSet, []int{0x100c, 0x100d}},
		{"loot clear", partyLootClear, []int{0x100e, 0x100f}},
		{"loot set", partyLootSet, []int{0x1010, 0x1011}},
		{"hear clear", partyHearClear, []int{0x1014, 0x1015}},
		{"hear set", partyHearSet, []int{0x1012, 0x1013}},
		{"squelch clear", partySquelchClear, []int{0x1016}},
	}

	for _, tc := range tests {
		if got := tc.k.ids(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %#x want %#x", tc.name, got, tc.want)
		}
	}

	if got := partyLootClear.text(func(s interface{}) string { return "<" + s.(string) + ">" }); got != "<strParty11>\n<strParty12>" {
		t.Errorf("text: %q", got)
	}
}
