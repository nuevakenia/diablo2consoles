package d2player

import "github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"

// SetHeroVoice sets the function that plays a hero voice line by its id in UNITSND_PlayUnitSoundById (0x4c8470):
// 0x13 impossible, 0x14 cantuseyet, 0x17 cantcarry. The game screen owns the repeat suppression.
func (g *GameControls) SetHeroVoice(f func(id int)) { g.heroVoice = f }

// sayHeroVoice plays a hero voice line (id 0 plays nothing).
func (g *GameControls) sayHeroVoice(id int) {
	if id != 0 && g.heroVoice != nil {
		g.heroVoice(id)
	}
}

// SetFriendlyCheck tells the controls which monsters are the hero's own units (summons, mercenary): a click on one does
// not start an attack on it.
func (g *GameControls) SetFriendlyCheck(f func(m *d2mapentity.Monster) bool) { g.isFriendly = f }
