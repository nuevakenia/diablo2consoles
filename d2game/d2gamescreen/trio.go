package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2uber"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// The endgame trio reward of Game.exe 0x5dee90 (d2uber.Trio): when Uber
// Mephisto, Uber Diablo and Uber Baal (classes 0x2c0, 0x2c1, 0x2c5) have all
// died in the game, a charm (cm2) with its charges at the maximum drops, then
// the dying monster's standard treasure class is rolled once per entry of the
// game's counter (Game +0x8c; taken here as the number of players, UNVERIFIED).

// trioReward feeds one kill to the tracker and, when it completes the trio,
// calls charm for the charm and extra for the number of standard drops.
func trioReward(t *d2uber.Trio, class, players int, charm func(code string), extra func(n int)) bool {
	d, ok := t.Kill(class, players)
	if !ok {
		return false
	}

	charm(d.Charm)

	if d.StandardNum > 0 {
		extra(d.StandardNum)
	}

	return true
}

// trioKilled is called for every monster death.
func (v *Game) trioKilled(ev d2monsters.KillEvent) {
	players := 1
	if v.gameClient != nil && v.gameClient.Roster != nil && v.gameClient.Roster.Len() > 1 {
		players = v.gameClient.Roster.Len()
	}

	trioReward(&v.trio, ev.Class, players,
		func(code string) {
			v.Infof("TRIO reward: %s with maximum charges dropped (kill of class %d)", code, ev.Class)
			v.spawnQuestItem(code)
		},
		func(n int) {
			done := 0
			if v.monsters != nil && ev.Monster != nil {
				done = v.monsters.ExtraDrops(ev.Monster, n)
			}

			v.Infof("TRIO reward: %d standard drop(s) rolled (players=%d)", done, players)
		})
}
