package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Hosts of the monster director's endgame inputs (wave 22): the last town portal of a hero and her states, which
// Diablo and the Baal crab read, and the Rite of Passage introduction that lets the Ancients fight.

const (
	subtilesPerTilePortal = 5
	riteIntroMessage      = 20002 // 0x4e22, the statues' introduction (AncientsAct5IntroGossip1)
	stateColdID           = 0xb
)

// hookEndgameMonsters connects the director's hero portal and hero state hosts.
func (v *Game) hookEndgameMonsters() {
	if v.monsters == nil {
		return
	}

	v.monsters.HeroPortal = v.heroTownPortal
	v.monsters.HeroHasState = v.heroHasState
}

// heroTownPortal is PLAYER_GetLastPortalUnitId for a hero: the field end of the town portal she owns (level and
// subtile position), when one is open.
func (v *Game) heroTownPortal(p *d2mapentity.Player) (level, x, y int, ok bool) {
	for i := range v.portal.pairs {
		pr := &v.portal.pairs[i]
		if pr.Owner != p.ID() {
			continue
		}

		return pr.Field.Level, int(pr.Field.X * subtilesPerTilePortal), int(pr.Field.Y * subtilesPerTilePortal), true
	}

	return 0, 0, 0, false
}

// heroHasState asks the skill engine whether a hero has the states.txt state with the given id.
func (v *Game) heroHasState(p *d2mapentity.Player, state int) bool {
	if v.skills == nil || v.asset == nil || v.asset.Records == nil {
		return false
	}

	for name, rec := range v.asset.Records.States {
		if rec != nil && rec.ID == state {
			return v.skills.HasState(p.ID(), name)
		}
	}

	return false
}

// riteMessageHeard starts the Rite of Passage fight when the statues' introduction was spoken. The statue objects
// are not modelled in the harness: the introduction itself is the trigger.
func (v *Game) riteMessageHeard(msg int) {
	if msg == riteIntroMessage && v.monsters != nil {
		v.riteStart()
	}
}
