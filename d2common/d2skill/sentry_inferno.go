package d2skill

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"

// InfernoLife is SRVDO_SetMissileLifeFromCalc (0x5ca5b0, VERIFIED) as an Inferno Sentry's flame uses it
// (SRVDO_095_FetishInferno 0x5ca7b0): the missile's life is calc1 of the monster skill at the sentry's level, at
// most 255 and at least 1 (below 2 gives 1). A calc1 below 1 falls back to (missile Param2 - 1) + level in the
// exe; the port keeps the missile's own range then (0, UNVERIFIED column mapping).
func InfernoLife(calc1 int) int {
	switch {
	case calc1 < 1:
		return 0
	case calc1 < 2:
		return 1
	case calc1 > 254:
		return 255
	}

	return calc1
}

// CastTrapInferno is an Inferno Sentry's shot: one flame missile (srvmissilea of "mon inferno sentry") from the
// sentry's cell with the life calc1 gives, the damage of the placing skill (U, as for the other sentries).
func (p *Pipeline) CastTrapInferno(u Unit, placingID, monsterSkillID, monsterLevel, fromX, fromY int, tgt Target) *d2missile.Missile {
	pl, ms := p.Skills.ByID(placingID), p.Skills.ByID(monsterSkillID)
	if pl == nil || ms == nil || ms.SrvMissileA == "" {
		return nil
	}

	lvl := u.SkillLevel(placingID)
	life := InfernoLife(p.env(ms, maxInt(monsterLevel, 1), u).eval(ms.Calc[1]))

	return p.castMissile(u, pl, lvl, p.env(pl, lvl, u), ms.SrvMissileA, tgt, castOpts{hasStart: true,
		startX: float64(fromX) + 0.5, startY: float64(fromY) + 0.5, rangeLife: life})
}
