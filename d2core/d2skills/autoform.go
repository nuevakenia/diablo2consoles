package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// PrepareShape is for the OD2_AUTOCAST scenarios: the exe refuses a skill by
// its restrict rule (d2skill.RestrictOK, 0x645450), so before a skill is cast
// the hero is put in the form it needs (restrict 2: the first State column,
// for example Hunger needs the wolf) or taken out of every form (restrict 0).
// It returns the form state that was set, "" for none.
func (e *Engine) PrepareShape(p *d2mapentity.Player, skillID int) string {
	set := e.setOf(p.ID())

	for _, s := range formStates {
		set.Remove(s)
	}

	// Maul and Feral Rage share the forms' States.txt group 3: a still-active one of an earlier skill makes the
	// toggle of Wearwolf / Wearbear (ClearGroup, 0x56a480) end that state and count as the switch back to human
	// shape, so the form was never entered (w29: the 86i Wearbear cast reverted instead of applying the bear).
	for _, s := range formGroupStates {
		set.Remove(s)
	}

	sk := e.pipe.Skills.ByID(skillID)
	if sk == nil || sk.Restrict != 2 || sk.FormStates[0] == "" {
		return ""
	}

	set.Apply(e.frame, d2state.Instance{Name: sk.FormStates[0], Source: p.ID()})

	return sk.FormStates[0]
}
