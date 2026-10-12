package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2difficulty"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
)

// heroGate runs the load gate (d2difficulty.CheckCharacterForGame) for the session the character select screen would
// start: the hero's own difficulty, or Normal when the difficulty screen still has to choose one (that screen gates the
// choice itself).
func heroGate(h *d2hero.HeroState) d2difficulty.GateCode {
	if h == nil {
		return d2difficulty.GateOK
	}

	level := d2difficulty.Clamp(int(h.Difficulty))
	if NeedsDifficultyChoice(h) {
		level = d2difficulty.Normal
	}

	return h.CheckLocal(level)
}
