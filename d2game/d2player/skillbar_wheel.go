package d2player

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
)

// onWheelSkill is the mouse wheel (or whatever the user bound to "select previous / next skill"): it steps the right
// skill through the skills the hero can use (Shift: the left skill). UNVERIFIED which button the exe changes; the
// order is the skill id order shared with the gamepad bumpers (d2hero.CycleSkill).
func (g *GameControls) onWheelSkill(dir int, mod d2enum.KeyMod) {
	if g.escapeMenu.IsOpen() || g.NPCMenu.IsOpen() {
		return
	}

	left := mod&d2enum.KeyModShift != 0
	from := g.activeSkillID(left)

	id, ok := d2hero.CycleSkill(g.hero.Skills, from, left, dir)
	if !ok || id == from {
		g.Infof("SKILLBAR wheel hand=%s dir=%d: nothing to select", handName(left), dir)
		return
	}

	g.Infof("SKILLBAR wheel hand=%s dir=%d %q -> %q", handName(left), dir, g.skillName(from), g.skillName(id))

	_ = g.SelectSkill(left, id)
}
