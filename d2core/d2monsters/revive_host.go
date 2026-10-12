package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

// reviveSearchRadius is the radius of SKILL_FindReviveCorpseNearUnit (0x56c140, VERIFIED): the unit search runs
// around the attack target's position with radius 10 and the filter SKILL_ReviveCorpseSearchFilter, which is
// SKILL_CanReviveOrExplodeCorpse plus the generic unit match.
const reviveSearchRadius = 10

// FindReviveCorpse implements d2monster.ReviveHost for the Death Sentry: the corpse near the sentry's current
// attack target. The exe returns the first unit the search finds; the order of that search is not known, so the
// nearest corpse to the target is taken (UNVERIFIED). Corpses of pets and mercenaries and corpses already
// reserved are skipped, a corpse must be selectable (monstats2 corpseSel, the SKILL_CanReviveOrExplodeCorpse bit).
func (d *Director) FindReviveCorpse(b *d2monster.Brain, _, _ int, near d2monster.Target) (d2monster.Target, bool) {
	var (
		best  d2monster.Target
		bestD = reviveSearchRadius*reviveSearchRadius + 1
		found bool
	)

	for _, u := range d.sortedUnits() {
		if u.b == b || u.merc != nil || u.ally != nil || u.raising || u.m.Alive() {
			continue
		}

		if u.m.Mode() != d2monster.ModeDead && u.m.CorpseAge() < 1 {
			continue
		}

		if !d.corpseSelectable(u) {
			continue
		}

		x, y := u.m.SubtilePos()
		dx, dy := near.X-x, near.Y-y

		if dd := dx*dx + dy*dy; dd < bestD {
			bestD, found = dd, true
			best = d2monster.Target{ID: corpseTargetBase + u.b.ID, X: x, Y: y, Size: 1}
		}
	}

	return best, found
}

// corpseSelectable is the monstats2 corpseSel column (flag bit 7, mapping VERIFIED w27: monstats2 loader 0x658ee0, test MONSTATS2_TestFlagBit 0x45f0b0 in SKILL_CanReviveOrExplodeCorpse 0x646a10); a monster without a row
// counts as selectable, as in the skill engine's corpse facts.
func (d *Director) corpseSelectable(u *unit) bool {
	if u.m.Stat == nil || d.asset == nil || d.asset.Records == nil || d.asset.Records.Monster.Stats2 == nil {
		return true
	}

	if s2 := d.asset.Records.Monster.Stats2[u.m.Stat.ExtraDataKey]; s2 != nil {
		return s2.IsCorpseSelectable
	}

	return true
}

// ReviveRadius implements d2monster.ReviveHost: SKILLS_CalcLinearValueByLevel(Skill1, level), which the skill
// engine computed when it armed the sentry (TrapSpec.ReviveRadius). The AI uses half of it as the largest
// distance between the target and the corpse.
func (d *Director) ReviveRadius(b *d2monster.Brain, _, _ int) int {
	if u := d.unitOf(b); u != nil && u.ally != nil && u.ally.trap != nil {
		return u.ally.trap.ReviveRadius
	}

	return 0
}
