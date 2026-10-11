package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Skill fire callback for sentries, vines and other stationary summons.
//
// thinkSentry / thinkStationary (d2common/d2monster/ai_pet.go) end in
// Actor.Cast. For a hostile monster Director.Cast plays the monstats mode and
// the strike is a monstats melee hit; for a hero's sentry that would lose the
// hero-level skill damage and the missile the skill engine fires today
// (d2skills registerTrap -> Pipeline.CastTrap). A unit armed with ArmTrap
// instead hands its Cast to the SkillFirer, which the skill engine implements
// with the same CastTrap call.
//
// STATUS (UNVERIFIED): the callback and the Director side are in place and
// tested with a fake, but the skill engine still drives sentries itself
// (trapsTick). Switching was NOT done: the AI path picks targets with
// monstats aggro/InRange (melee 7 or ranged line of sight) and paces with
// aip1/aip2, while trapsTick uses a fixed 20 frame period and a 20 subtile
// Chebyshev range, so firing cadence and target set would change. The fire
// itself (missile, level, damage, start position) is shared code in d2skills
// and covered by a parity test there.

// TrapShot is one request to fire a trap's skill.
type TrapShot struct {
	// Trap is the sentry or vine firing.
	Trap *d2mapentity.Monster
	// Owner is the hero that placed it.
	Owner *d2mapentity.Player
	// Spec is what ArmTrap registered.
	Spec TrapSpec
	// FromX, FromY is the trap's subtile.
	FromX, FromY int
	// Target is the monster aimed at.
	Target *d2mapentity.Monster
}

// TrapSpec names the skill a trap fires; the skill engine resolves it.
type TrapSpec struct {
	SkillID   int
	SkillName string
	Missile   string
	// Level is the level of the sentry's monster skill and Life the shots it may take (calc4 at that level);
	// see d2skill.SummonOrder.TrapLevel. Life 0 = unknown: the AI is then not used (TrapAIDriven false).
	Level, Life int
}

// SkillFirer is implemented by the skill engine. It returns whether a missile
// was fired.
type SkillFirer interface {
	FireTrap(TrapShot) bool
}

// SetSkillFirer injects the skill engine side. nil disables the callback.
func (d *Director) SetSkillFirer(f SkillFirer) { d.firer = f }

// ArmTrap makes m (a unit made by SpawnMinion) fire spec through the
// SkillFirer whenever its AI casts. It reports false if m is not a minion unit.
func (d *Director) ArmTrap(m *d2mapentity.Monster, spec TrapSpec) bool {
	u := d.byEntity[m.ID()]
	if u == nil || u.ally == nil {
		return false
	}

	u.ally.trap = &spec

	return true
}

// fireTrap routes a Cast of an armed unit to the SkillFirer. handled is false
// when the unit is not armed (or no firer is set): the caller does its normal
// monstats cast.
func (d *Director) fireTrap(u *unit, t d2monster.Target) (handled, ok bool) {
	if d.firer == nil || u == nil || u.ally == nil || u.ally.trap == nil {
		return false, false
	}

	tu := d.units[t.ID]
	if tu == nil || !tu.m.Alive() {
		return true, false
	}

	x, y := u.m.SubtilePos()

	return true, d.firer.FireTrap(TrapShot{Trap: u.m, Owner: u.ally.owner, Spec: *u.ally.trap, FromX: x, FromY: y, Target: tu.m})
}

// trapTick runs the ported AI of an armed trap; false means it is not armed.
func (d *Director) trapTick(u *unit) bool {
	if d.firer == nil || u.ally.trap == nil || u.b.Def == nil || !u.b.Def.Implemented {
		return false
	}

	d2monster.Tick(d, u.b)

	return true
}

// TrapAIDriven reports whether m is an armed sentry whose cadence the ported
// monster AI (MONAI_Think_AssassinSentry 0x5e9480) decides, so the skill
// engine must not fire it on its own timer. It needs a known shot budget; an
// unarmed sentry or one of another AI keeps the old driver.
func (d *Director) TrapAIDriven(m *d2mapentity.Monster) bool {
	u := d.byEntity[m.ID()]
	if d.firer == nil || u == nil || u.ally == nil || u.ally.trap == nil || u.ally.trap.Life <= 0 {
		return false
	}

	return u.b.Def != nil && u.b.Def.Implemented && u.b.Profile != nil && u.b.Profile.AI == "AssassinSentry"
}

// SkillLevel implements d2monster.SkillCalc: the level of the monster skill
// in a slot. An armed sentry has the level its placing skill gave it; every
// other unit keeps the fallback the AI used before (1 when the slot is used).
func (d *Director) SkillLevel(b *d2monster.Brain, slot int) int {
	if u := d.unitOf(b); u != nil && u.ally != nil && u.ally.trap != nil && u.ally.trap.Level > 0 {
		return u.ally.trap.Level
	}

	if b.Profile.Skills[slot].Used() {
		return 1
	}

	return 0
}

// SkillCalc4 implements d2monster.SkillCalc: the shots of an armed sentry; the
// AI's own fallback (a long life) for every other unit.
func (d *Director) SkillCalc4(b *d2monster.Brain, _, _ int) int {
	if u := d.unitOf(b); u != nil && u.ally != nil && u.ally.trap != nil && u.ally.trap.Life > 0 {
		return u.ally.trap.Life
	}

	return 1000
}
