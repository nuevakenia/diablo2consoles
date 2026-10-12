package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// The hold inputs of PLRMODE_CanInterruptCurrentMode (0x57ceb0), VERIFIED against the exe: state 0x2a is the row
// "concentration" of states.txt (the Paladin aura) and its chance is the state list's base stat 0xa4, which is
// "skill_concentration" of ItemStatCost (the aura's aurastat, Param5 percent); state 0xf is the row "concentrate"
// (set by the Barbarian Concentrate and Berserk attacks, a mode-bound state, see modebound.go). When the used
// skill has dwBits4 bit 31, the hero keeps its action (the block is refused unless the hero stands in neutral)
// when state 0xf is on, or when state 0x2a is on and a 0..99 roll of the unit's own seed is below the chance.
// The roll is only made when state 0x2a is on, like the exe.
const (
	holdAuraState   = "concentration"
	holdAuraStat    = "skill_concentration"
	holdAttackState = "concentrate"
	// blockedState is state 0x36, the row "uninterruptable" (states.txt row order): PLRMODE_CanInterruptCurrentMode
	// refuses at once while it is on.
	blockedState = "uninterruptable"
)

// holdInputs fills the state part of an InterruptInput for the hero.
func (e *Engine) holdInputs(p *d2mapentity.Player, u *heroUnit, in *d2skill.InterruptInput) {
	set := e.setOf(p.ID())
	in.StateBlocked = set.Active(e.frame, blockedState)
	in.State0f = set.Active(e.frame, holdAttackState)

	aura := set.Get(e.frame, holdAuraState)
	if aura == nil || in.SkillBits4&d2skill.Bits4Hold == 0 {
		return
	}

	in.State2a = true

	for _, m := range aura.Mods {
		if m.Stat == holdAuraStat {
			in.State2aChance += m.Value
		}
	}

	if u.seed != nil {
		in.Roll = int(u.seed.Roll(100))
	}
}
