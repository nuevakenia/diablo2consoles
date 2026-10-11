package d2missile

import "math"

// The vine and recycler missiles of the Druid summons (Plague Poppy, Cycle of
// Life, Vines). Read in Game.exe 1.14b in wave 20 (V = read in the decompiled
// body, U = inferred):
//
//   - SrvDoFunc 26, 0x5ad4e0 (V): a missile without SubMissile1 is destroyed
//     (return 2). Otherwise, whenever the elapsed life is a multiple of
//     max(Param1, 1) (so on its very first frame too) it creates its
//     SubMissile1 on its own subtile (SKILL_ServerCreateMissileAtTarget) with
//     the owner, skill id and skill level of the missile, then does the
//     standard move. "plague vines" and "vines" drop their trail this way
//     every 9 frames.
//   - SrvDoFunc 29 and 33, 0x5ad910 and 0x5ada80 (V): the recycler delay
//     missiles. Their skill id must be a valid skill (else return 2). When the
//     owner exists and is not dead, on the frame the elapsed life equals
//     Param1 (45), and the owner's life (29) or mana (33) is below its
//     maximum, the owner receives maximum * calc1 / 100 of it (MulDiv, whole
//     points, clamped to the maximum), where calc1 is that of the skill the
//     missile carries (CorpseCycler / VineCycler: dm12 over their Param1/2,
//     3..12 percent for life and 1..8 for mana) evaluated for the owner at
//     the missile's level; then the progressive overlay of the missile
//     record is added to the owner (cosmetic, not modelled). Then the
//     standard move.
//   - SrvHitFunc 50, 0x5aa340 (V): with no target it returns 1 (destroy).
//     With a target it returns 2 (the damage applies, the missile stays)
//     while the elapsed life is below the total life minus sHitPar1 (15), and
//     0 (nothing happens) in the last sHitPar1 frames: the trail withers.

// vineTrail is SrvDoFunc 26. It reports false when the missile vanished.
func (s *Sim) vineTrail(m *Missile) bool {
	sp := m.Spec

	sub := s.lookup(sp.SubMissile[0])
	if sp.SubMissile[0] == "" {
		s.vanish(m)

		return false
	}

	period := sp.Param[0]
	if period < 1 {
		period = 1
	}

	if sub == nil || m.Elapsed()%period != 0 {
		return true
	}

	x, y := math.Floor(m.X)+0.5, math.Floor(m.Y)+0.5
	_, _ = s.Create(CreateParams{Spec: sub, Parent: m, Owner: m.Owner, SkillID: m.SkillID, Level: m.Level,
		Damage: m.Damage, AreaRadius: m.AreaRadius, X: x, Y: y, DestX: x, DestY: y, Stationary: true})

	return true
}

// ownerRestore is SrvDoFunc 29 (life) and 33 (mana). It reports false when
// the missile vanished.
func (s *Sim) ownerRestore(m *Missile, mana bool) bool {
	if m.SkillID < 0 {
		s.vanish(m)

		return false
	}

	if m.Owner.Restore == nil || (m.Owner.Gone != nil && m.Owner.Gone()) || m.Elapsed() != m.Spec.Param[0] {
		return true
	}

	m.Owner.Restore(mana, m.RestorePct)

	return true
}

// vineTrailHit is SrvHitFunc 50.
func vineTrailHit(m *Missile, t Target) int {
	if t == nil {
		return resKill
	}

	if m.Elapsed() < m.Total-m.Spec.SHitPar[0] {
		return resDamage
	}

	return 0
}
