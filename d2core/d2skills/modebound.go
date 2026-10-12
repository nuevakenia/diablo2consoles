package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// modeBoundState is a state held by a stat list with flag 4 and no expiry (Concentrate). The exe removes such lists in
// UNIT_RemoveModeBoundStatLists, called by UNIT_SetUnitMode whenever the unit's mode really changes (VERIFIED).
// The hero's mode is read from the animation mode: captured on the first tick after the state was applied (the
// cast itself may run just before the attack mode is entered), removal on any later change.
type modeBoundState struct {
	p      *d2mapentity.Player
	read   func() d2enum.PlayerAnimationMode
	state  string
	mode   d2enum.PlayerAnimationMode
	seeded bool
}

func (e *Engine) bindToMode(p *d2mapentity.Player, state string) {
	for _, m := range e.modeBound {
		if m.p == p && m.state == state {
			m.seeded = false

			return
		}
	}

	e.modeBound = append(e.modeBound, &modeBoundState{p: p, state: state, read: p.GetAnimationMode})
}

// tickModeBound drops the mode-bound states of heroes whose mode changed.
func (e *Engine) tickModeBound() {
	keep := e.modeBound[:0]

	for _, m := range e.modeBound {
		mode := m.read()

		if !m.seeded {
			m.mode, m.seeded = mode, true
			keep = append(keep, m)

			continue
		}

		if mode == m.mode {
			keep = append(keep, m)

			continue
		}

		e.setOf(m.p.ID()).Remove(m.state)
		e.emit("state", "STATE mode change unit=%s state=%s removed", m.p.Name(), m.state)
	}

	e.modeBound = keep
}
