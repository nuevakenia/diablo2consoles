package d2skill

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"

// Wave 20, Druid summons: Vine Attack (SRVDO_130) and the corpse / vine
// cyclers (SRVST_063). V = read in the decompiled body of Game.exe 1.14b.
//
// SRVDO_130 0x5d02d0 (V): srvmissilea must be a valid missile, the caster
// gets unit flag 0x40, calc1 (min(24, ln12): Param1 + (lvl-1) * Param2) must
// be at least one. SRVDO_VineAttack_CreateMissile 0x5d0220 then makes calc1
// missiles with create flags 3 (explicit start, relative destination): the
// start is the target's position (SKILL_GetSkillTargetPosition), the
// destination is that position plus one of the four cardinal unit steps
// (0,1), (0,-1), (1,0), (-1,0) taken by index & 3 (tables 0x6e47e4 / 0x6e47f4),
// and every missile goes through the shared per-missile hook 0x5c7340 (seed =
// destination x + index, path type 10, life capped at 77). The vine missile
// itself (SrvDoFunc 26) then drops its trail every Param1 frames.
//
// SRVST_063 0x5d1090 (V): the caster must be a monster (unit type 1) with a
// leader. The target unit (the corpse) must not carry state 0x76; the state
// is set on it (each corpse feeds one cycle), and a missile of srvmissilea
// (recycler delay) is created for the leader, with the corpse as its parent.
// Without a start position it begins at its owner, the leader. Its SrvDoFunc
// 29 (33 for the mana cycler) gives the leader calc1 percent back 45 frames
// later (see d2missile/vines.go).

// vineOffsets are the cardinal steps of tables 0x6e47e4 (x) and 0x6e47f4 (y).
var vineOffsets = [4][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}

// doVineAttackFn is SRVDO_130_VineAttack.
func doVineAttackFn(c *cast) {
	name := c.missileName()
	if name == "" || c.p.Missiles.ByName(name) == nil {
		c.fail(ReasonMissile)
		return
	}

	n := c.calc(1)
	if n < 1 {
		c.fail(ReasonMissile)
		return
	}

	ax, ay := c.aim()

	for _, m := range c.p.vineMissiles(c.u, c.sk, c.lvl, c.env, name, ax, ay, n) {
		c.res.Missiles = append(c.res.Missiles, m)
	}

	if len(c.res.Missiles) == 0 {
		c.fail(ReasonMissile)
	}
}

// vineMissiles is SRVDO_VineAttack_CreateMissile: n missiles of the named
// record start on the subtile (tx, ty) and head one cardinal step further,
// each with the path type 10 hook seeded by its destination x plus its index.
func (p *Pipeline) vineMissiles(u Unit, sk *Skill, lvl int, env *Env, name string, tx, ty, n int) []*d2missile.Missile {
	var out []*d2missile.Missile

	for i := 0; i < n; i++ {
		off := vineOffsets[i&3]
		dest := Target{X: tx + off[0], Y: ty + off[1]}

		m := p.castMissile(u, sk, lvl, env, name, dest, castOpts{hasStart: true, startX: float64(tx) + 0.5, startY: float64(ty) + 0.5,
			pathType10: true, pathSeed: ChargedBoltSeed(tx+off[0], i)})
		if m != nil {
			out = append(out, m)
		}
	}

	return out
}

// CastVineAttack is a Druid vine (Plague Poppy, Cycle of Life, Vines) using
// its Vine Attack skill (sumskill1 of the summoning skill) on tgt: the vines
// come up under the target, calc1 of Vine Attack at the summoning skill's
// level of them. The damage is the summoning skill's, as for the other traps.
// It returns the vines made (none when Vine Attack is unknown or its calc is
// below one).
func (p *Pipeline) CastVineAttack(u Unit, skillID int, missile string, tgt Target) []*d2missile.Missile {
	sk := p.Skills.ByID(skillID)
	if sk == nil {
		return nil
	}

	lvl := u.SkillLevel(skillID)
	env := p.env(sk, lvl, u)

	n := 0

	if va := p.Skills.ByName(sk.SumSkill[1]); va != nil {
		n = p.env(va, lvl, u).eval(va.Calc[1])
	}

	if n < 1 {
		return nil
	}

	ax, ay := tgt.X, tgt.Y
	if tgt.Unit != nil {
		ax, ay = tgt.UX, tgt.UY
	}

	return p.vineMissiles(u, sk, lvl, env, missile, ax, ay, n)
}

// CastCycler is SRVST_063 / VineCycler for the hero that owns a cycling
// vine: the recycler delay missile of the cycler skill (CorpseCycler gives
// life, VineCycler mana) is created at the hero. The percent it gives back
// is calc1 of the cycler skill evaluated for the hero at lvl. nil when the
// cycler skill or its missile is unknown.
func (p *Pipeline) CastCycler(u Unit, cyclerSkill string, lvl int) *d2missile.Missile {
	sk := p.Skills.ByName(cyclerSkill)
	if sk == nil {
		return nil
	}

	name := sk.SrvMissileA
	if name == "" {
		name = sk.SrvMissile
	}

	env := p.env(sk, lvl, u)
	x, y := u.Pos()

	return p.castMissile(u, sk, lvl, env, name, Target{X: x, Y: y},
		castOpts{hasStart: true, startX: float64(x) + 0.5, startY: float64(y) + 0.5, stationary: true, restorePct: env.eval(sk.Calc[1])})
}

func init() {
	doTable[130] = doVineAttackFn
}
