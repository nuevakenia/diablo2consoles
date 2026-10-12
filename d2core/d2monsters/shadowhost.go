package d2monsters

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

// Host side of the Assassin's shadows (w24-ai-left). Before this file a hero's
// Shadow Warrior / Shadow Master was driven by the generic minion stand-in
// (allyStep: nearest hostile, melee). Now the ported thinkers run
// (MONAI_Think_ShadowWarrior 0x5e9ff0, MONAI_Think_ShadowMaster 0x5ea9f0):
// follow the owner by MONAI_SelectPetActionByRange, leash, target distance
// gates and the roll for the plain attack are the exe's.
//
// NOT hosted (UNVERIFIED as a feature, the AI code for it exists):
//   - the per-skill scoring of the Shadow Master (ShadowEnv: the skills.txt
//     view, hostile scans, state and resist queries) and the buff upkeep;
//   - the Shadow Warrior's copy of the hero's right or left skill (FB1ShadowBook.Copy);
//     the pre-hook that copies the owner's skills (0x5ea4f0 / 0x5ea620) has no
//     source here: a minion carries no skill list.
//
// The book below therefore answers "has skills" (so the think does not idle for
// 100 ticks as it does for a unit with no skills, unit+0xa8 == 0) and offers the
// plain attack in place of the copied skill: in reach the warrior strikes with
// A1; out of reach it runs to the target (the think's own fallback).

// isShadowAI reports whether the unit's AI is one of the shadow thinkers.
func isShadowAI(u *unit) bool {
	if u == nil || u.b == nil || u.b.Def == nil {
		return false
	}

	switch strings.ToLower(u.b.Def.Name) {
	case "shadowwarrior", "shadowmaster", "shadowmasternoinit":
		return true
	}

	return false
}

// shadowCopyCooldown is the cooldown calc the plain-attack stand-in reports for
// the warrior's skill copy: the think stores calc/3+18 frames; UNVERIFIED, the
// real value is the copied skill's calc.
const shadowCopyCooldown = 0

// HasSkills implements d2monster.FB1ShadowBook: a shadow counts as having
// skills (the plain attack at least).
func (d *Director) HasSkills(b *d2monster.Brain) bool {
	return isShadowAI(d.unitOf(b))
}

// CastQueued implements d2monster.FB1ShadowBook: no chosen skill is queued
// (the skill scoring is not hosted).
func (d *Director) CastQueued(_ *d2monster.Brain, _ int, _ d2monster.Target, _ bool) bool {
	return false
}

// Upkeep implements d2monster.FB1ShadowBook: no buff or aura upkeep.
func (d *Director) Upkeep(_ *d2monster.Brain, _ *d2monster.Target, _ bool) bool { return false }

// Choose implements d2monster.FB1ShadowBook: nothing scored, the think falls
// back to the plain attack or the walk to the target.
func (d *Director) Choose(_ *d2monster.Brain, _ d2monster.Target, _ *d2monster.Target, _ bool) (int, bool) {
	return 0, false
}

// Copy implements d2monster.FB1ShadowBook for the Shadow Warrior: the copied
// hero skill is replaced by the plain attack (UNVERIFIED stand-in), possible
// only in reach.
func (d *Director) Copy(b *d2monster.Brain, _ d2monster.Target, tgt d2monster.Target, _, _, inRange bool) (int, bool) {
	if !inRange || !d.Attack(b, d2monster.ModeAttack1, tgt) {
		return 0, false
	}

	return shadowCopyCooldown, true
}

var _ d2monster.FB1ShadowBook = (*Director)(nil)
