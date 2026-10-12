package d2difficulty

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

func TestCheckCharacterForGame(t *testing.T) {
	const x, hc, died, lad = d2s.StatusExpansion, d2s.StatusHardcore, d2s.StatusDied, d2s.StatusLadder

	st := func(flags uint32, prog int) uint32 { return WithProgression(flags, prog) }

	cases := []struct {
		name   string
		status uint32
		game   GateGame
		want   GateCode
	}{
		{"expansion ok", x, GateGame{Expansion: true}, GateOK},
		{"classic char in expansion game", 0, GateGame{Expansion: true}, GateNeedClassic},
		{"expansion char in classic game", x, GateGame{}, GateNeedExpansion},
		{"hardcore ok", x | hc, GateGame{Expansion: true, Hardcore: true}, GateOK},
		{"hardcore died", x | hc | died, GateGame{Expansion: true, Hardcore: true}, GateHardcoreDied},
		{"hardcore died softcore game", x | hc | died, GateGame{Expansion: true}, GateHardcoreDied},
		{"hardcore char softcore game", x | hc, GateGame{Expansion: true}, GateHardcoreInSoft},
		{"softcore char hardcore game", x, GateGame{Expansion: true, Hardcore: true}, GateSoftInHardcore},
		{"died flag on softcore is harmless", x | died, GateGame{Expansion: true}, GateOK},
		{"nightmare below 5 (exp)", st(x, 4), GateGame{Expansion: true, Difficulty: Nightmare}, GateNeedNormalWon},
		{"nightmare ok 5 (exp)", st(x, 5), GateGame{Expansion: true, Difficulty: Nightmare}, GateOK},
		{"nightmare ok 4 (classic)", st(0, 4), GateGame{Difficulty: Nightmare}, GateOK},
		{"nightmare 3 (classic)", st(0, 3), GateGame{Difficulty: Nightmare}, GateNeedNormalWon},
		{"hell below 10 (exp)", st(x, 9), GateGame{Expansion: true, Difficulty: Hell}, GateNeedNightmare},
		{"hell ok 10 (exp)", st(x, 10), GateGame{Expansion: true, Difficulty: Hell}, GateOK},
		{"hell ok 8 (classic)", st(0, 8), GateGame{Difficulty: Hell}, GateOK},
		{"hell 7 (classic)", st(0, 7), GateGame{Difficulty: Hell}, GateNeedNightmare},
		{"normal needs nothing", x, GateGame{Expansion: true}, GateOK},
		{"realm non-ladder char ladder game", x, GateGame{Expansion: true, Realm: true, Ladder: true}, GateNeedLadder},
		{"realm ladder char ladder game", x | lad, GateGame{Expansion: true, Realm: true, Ladder: true}, GateNeedLadder},
		{"realm expired ladder char ladder game", x | lad, GateGame{Expansion: true, Realm: true, Ladder: true, LadderExpired: true}, GateOK},
		{"realm expired ladder char normal game", x | lad, GateGame{Expansion: true, Realm: true, LadderExpired: true}, GateLadderOver},
		{"realm ladder char normal game", x | lad, GateGame{Expansion: true, Realm: true}, GateOK},
		{"no realm ladder ignored", x, GateGame{Expansion: true, Ladder: true}, GateOK},
		{"mismatch beats died", hc | died, GateGame{Expansion: true, Hardcore: true}, GateNeedClassic},
	}

	for _, c := range cases {
		if got := CheckCharacterForGame(c.status, c.game); got != c.want {
			t.Errorf("%s: got %#x want %#x", c.name, got, c.want)
		}
	}
}
