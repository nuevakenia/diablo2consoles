package d2player

import (
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2equip"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2ui"
)

func TestCompareRowLinesColours(t *testing.T) {
	defer d2ui.SetColorBlind(false)

	rows := []d2hero.CompareRow{
		{Name: "Defense", Text: "+12 (+8%)", Verdict: d2hero.Better},
		{Name: "Damage", Text: "-3.5 (-9%)", Verdict: d2hero.Worse},
		{Name: "Attack Speed", Text: "same", Verdict: d2hero.Same},
	}

	tests := []struct {
		name  string
		blind bool
		want  []string
	}{
		{"normal: blue better, red worse, white same", false, []string{
			"[blue]Defense: +12 (+8%)", "[red]Damage: -3.5 (-9%)", "[white]Attack Speed: same"}},
		{"colour blind: vermillion for worse, words as well", true, []string{
			"[blue]Defense: +12 (+8%) (better)", "[orange]Damage: -3.5 (-9%) (worse)", "[white]Attack Speed: same"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d2ui.SetColorBlind(tc.blind)

			got := compareRowLines(rows)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

func TestPlaceableReasons(t *testing.T) {
	tests := []struct {
		r    d2equip.Reason
		want bool
	}{
		{d2equip.ReasonOK, true},
		{d2equip.ReasonStrength, true}, // compared by what it would give
		{d2equip.ReasonDexterity, true},
		{d2equip.ReasonLevel, true},
		{d2equip.ReasonBodyLoc, false},
		{d2equip.ReasonClass, false},
		{d2equip.ReasonDualWield, false},
		{d2equip.ReasonNoAmmoWeapon, false},
		{d2equip.ReasonUnidentified, false},
	}

	for _, tc := range tests {
		if got := placeable(tc.r); got != tc.want {
			t.Errorf("placeable(%q) = %v, want %v", tc.r, got, tc.want)
		}
	}
}
