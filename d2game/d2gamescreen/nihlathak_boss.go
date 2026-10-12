package d2gamescreen

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
)

// levelHallsOfVaught is the last level of Nihlathak's Temple, where the Betrayal of Harrogath ends.
const levelHallsOfVaught = 124

// nihlathakEnter puts Nihlathak (the super unique "Nihlathak Boss", class nihlathakboss) into the Halls of Vaught when the
// level has none. The natural population of the level (drlgpop, DS1 preset NihlN.ds1) leaves him out, so the Betrayal of
// Harrogath could not be finished in a played game (wave 43 full-game chain: "no monster named Nihlathak"). Where the
// original stands him is UNVERIFIED: he is placed a few tiles from the arrival stairs, like the scenario spawns.
func (v *Game) nihlathakEnter(level int) {
	if level != levelHallsOfVaught || v.localPlayer == nil {
		return
	}

	d := v.monsterDirector()
	if d == nil {
		return
	}

	if r := v.quests(); r != nil {
		if q := r.g.Quest(d2quest.QuestBetrayal); q != nil && r.g.Rec.Get(q.Slot, d2quest.FlagPrimaryGoal) {
			return // already dead
		}
	}

	for _, m := range d.Monsters() {
		if m.Alive() && strings.Contains(strings.ToLower(m.Label()), "nihlathak") {
			return
		}
	}

	centre := d2path.Point{X: int(v.localPlayer.Position.X()) + 2*monsterTestRing, Y: int(v.localPlayer.Position.Y())}

	res, err := d.SpawnSuperUnique("Nihlathak Boss", centre)
	if err != nil {
		v.Warningf("NIHLATHAK spawn: %v", err)

		return
	}

	v.Infof("NIHLATHAK the Halls of Vaught had no Nihlathak: %d monster(s) placed near the arrival", len(res.Monsters))
}
