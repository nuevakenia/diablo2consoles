package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// d2monster.ShadowEnv for the Director: the skill view, the unit scan of MONAI_ShadowMaster_ClassifyTarget 0x5ea730
// and the state and resist queries the Shadow Master's scoring (0x5ea9f0) makes. The shadow is a friendly minion, so
// its enemies are named by plain unit id (see nearestEnemy).

// specialClasses are the monstats classes the scan counts as friendly special units (0x19a-0x19d, 0x19f, 0x1a0).
func specialClass(c int) bool {
	switch c {
	case 0x19a, 0x19b, 0x19c, 0x19d, 0x19f, 0x1a0:
		return true
	}

	return false
}

// ShadowSkills implements d2monster.ShadowEnv: the skill list in order.
func (d *Director) ShadowSkills(b *d2monster.Brain) []d2monster.ShadowSkill {
	_, sb := d.shadowOf(b)
	if sb == nil {
		return nil
	}

	return d.shadowViews(sb)
}

// shadowTarget is a hostile unit as the scoring names it.
func shadowTarget(o *unit) d2monster.Target {
	x, y := o.m.SubtilePos()

	return d2monster.Target{ID: o.b.ID, X: x, Y: y, Size: o.b.Size}
}

// validShadowTarget is MONAI_ShadowMaster_IsValidTargetUnit 0x5ea6b0. The monstats flag word bits are read as: 0x100
// the npc flag (must be clear), 0x8000 the killable flag (must be set); a monster that is neither a boss, nor flagged
// 0x80 (read as primeevil), needs one of the unit data bits 0xe (super unique, champion, unique). UNVERIFIED: the
// names of the three monstats bits (the masks are VERIFIED, the columns are the usual layout).
func validShadowTarget(o *unit) bool {
	if o == nil || o.m == nil || !o.m.Alive() || o.m.Stat == nil {
		return false
	}

	st := o.m.Stat
	if st.IsNpc || !st.IsKillable {
		return false
	}

	if st.IsSpecialBoss || st.IsActBoss {
		return true
	}

	return o.m.TypeFlags&(d2mapentity.MonTypeSuperUnique|d2mapentity.MonTypeChampion|d2mapentity.MonTypeUnique) != 0
}

// ShadowScan implements d2monster.ShadowEnv: the callback 0x5ea730 over every unit of the world.
func (d *Director) ShadowScan(b *d2monster.Brain, owner *d2monster.Target) d2monster.ShadowScan {
	sc := d2monster.ShadowScan{NearestDist2: 0x7fffffff, NearOwnerDist2: 0x7fffffff}
	self := d.unitOf(b)

	for _, o := range d.sortedUnits() {
		if o == self || o.m == nil || !o.m.Alive() {
			continue
		}

		if o.friendly() || o.b.Allied {
			if specialClass(o.b.Class) {
				sc.SpecialCount++
			}

			continue
		}

		if o.m.Stat == nil || IsDestructibleProp(o.m.Stat) {
			continue
		}

		t := shadowTarget(o)
		tt := t

		if owner != nil {
			ds := sqDist2(owner.X, owner.Y, t.X, t.Y)
			if ds < 0x65 {
				sc.OwnerCount++
			}

			if ds < sc.NearOwnerDist2 {
				sc.NearOwner, sc.NearOwnerDist2 = &tt, ds
			}
		}

		ds := sqDist2(b.X, b.Y, t.X, t.Y)
		if ds >= 0x401 {
			continue
		}

		sc.WithinCount++

		if ds < 0x65 {
			sc.CloseCount++
		}

		if ds < sc.NearestDist2 {
			n := t
			sc.Nearest, sc.NearestDist2 = &n, ds
		}

		if validShadowTarget(o) {
			v := t
			sc.ValidTarget = &v
		}
	}

	return sc
}

func sqDist2(ax, ay, bx, by int) int {
	dx, dy := ax-bx, ay-by

	return dx*dx + dy*dy
}

// TargetStat implements d2monster.ShadowEnv: a resist stat (STATS_GetUnitStat) of a target.
func (d *Director) TargetStat(t d2monster.Target, stat int) int {
	o := d.units[t.ID]
	if o == nil || o.m == nil {
		return 0
	}

	i, ok := fxResistStats[stat]
	if !ok {
		return 0
	}

	return MonsterResistsOf(o.m)[i]
}

// SelfHasState implements d2monster.ShadowEnv (and ShadowUpkeepEnv): the engine's state list of the shadow.
func (d *Director) SelfHasState(b *d2monster.Brain, state int) bool {
	u, sb := d.shadowOf(b)
	if u == nil {
		return false
	}

	return d.shadowStateOf(u, nil, sb.name(state)).Active
}

// ShadowTargetHasState implements d2monster.ShadowEnv.
func (d *Director) ShadowTargetHasState(t d2monster.Target, state int) bool {
	o := d.units[t.ID]
	if o == nil || o.m == nil {
		return false
	}

	name := d.shadowNameTable().name(state)
	if d.mercHooks.State == nil || name == "" {
		return false
	}

	return d.mercHooks.State(nil, 0, o.m, name).Active
}

// SelfInCastMask implements d2monster.ShadowEnv. The exe tests the unit's state bitset against a caller-supplied mask (0x63b240 overlap loop, VERIFIED w37); which states the mask holds is not decoded (UNVERIFIED).
func (d *Director) SelfInCastMask(_ *d2monster.Brain) bool { return false }

// OtherInStateGroup implements d2monster.ShadowEnv (STATE_HasOtherActiveStateInSameGroup).
func (d *Director) OtherInStateGroup(b *d2monster.Brain, state int) bool {
	u, sb := d.shadowOf(b)
	if u == nil {
		return false
	}

	return d.shadowStateOf(u, nil, sb.name(state)).OtherInGroup
}

// SelfStateStat implements d2monster.ShadowEnv: the value of a stat in the shadow's own state list.
func (d *Director) SelfStateStat(b *d2monster.Brain, state, stat int) (int, bool) {
	u, sb := d.shadowOf(b)
	if u == nil {
		return 0, false
	}

	v := d.shadowStateOf(u, nil, sb.name(state))
	if !v.Active {
		return 0, false
	}

	val, ok := v.Stats[sb.name(stat)]

	return val, ok
}

// TargetMonsterThreat implements d2monster.ShadowEnv: the byte at monstats +0xa0 + difficulty of a monster target.
// VERIFIED w37 (record layout in monster-ai.md: Drain +0xa0 with one byte per difficulty, Level is +0xaa): those bytes
// are the Drain (leech sensitivity) column, not the monster level; the caller compares it with 0x18.
func (d *Director) TargetMonsterThreat(t d2monster.Target, diff d2monster.Difficulty) (bool, int) {
	o := d.units[t.ID]
	if o == nil || o.m == nil {
		return false, 0
	}

	if o.m.Stat == nil {
		return true, 0
	}

	switch diff {
	case d2monster.Nightmare:
		return true, o.m.Stat.LeechSensitivityNightmare
	case d2monster.Hell:
		return true, o.m.Stat.LeechSensitivityHell
	}

	return true, o.m.Stat.LeechSensitivityNormal
}

// SelectActiveSkill implements d2monster.ShadowEnv (SKILL_SelectActiveSkillForUnit): the shadow's left skill.
func (d *Director) SelectActiveSkill(b *d2monster.Brain, skillID int) {
	if _, sb := d.shadowOf(b); sb != nil {
		sb.leftID, sb.leftSet = skillID, true
	}
}

// LeftSkillSet implements d2monster.ShadowEnv (UNIT_GetLeftSkill != nil).
func (d *Director) LeftSkillSet(b *d2monster.Brain) bool {
	_, sb := d.shadowOf(b)

	return sb != nil && sb.leftSet
}

// ---- d2monster.ShadowTargeting ----

// OwnerTarget implements d2monster.ShadowTargeting. UNVERIFIED / not hosted: the Director does not know the hero's
// current target (the hero's attack is run by the skill engine), so a shadow never adopts it.
func (d *Director) OwnerTarget(_ *d2monster.Brain, _ d2monster.Target) (d2monster.Target, bool) {
	return d2monster.Target{}, false
}

// ValidTarget implements d2monster.ShadowTargeting (MONAI_ShadowMaster_IsValidTargetUnit).
func (d *Director) ValidTarget(_ *d2monster.Brain, t d2monster.Target) bool {
	return validShadowTarget(d.units[t.ID])
}

// LeaderOf implements d2monster.ShadowTargeting (MONAI_GetLeaderUnit of the target).
func (d *Director) LeaderOf(t d2monster.Target) (d2monster.Target, bool) {
	o := d.units[t.ID]
	if o == nil || o.b == nil || o.b.Leader == nil {
		return d2monster.Target{}, false
	}

	l := d.units[o.b.Leader.ID]
	if l == nil || !l.m.Alive() {
		return d2monster.Target{}, false
	}

	return shadowTarget(l), true
}

var (
	_ d2monster.ShadowEnv       = (*Director)(nil)
	_ d2monster.ShadowTargeting = (*Director)(nil)
)
