package d2skill

// Charge release of the assassin finishers (w23). SRVDO_AssassinMeleeFinisherHit 0x5d3ae0, VERIFIED:
//
//   - It only runs for a finisher hit whose hit-sequence entry has the hit bit (result flags bit 0) and a target
//     in melee range. For each charge state (progressive_*) the unit holds it reads the builder skill (stat 0x15e)
//     and its level (stat 0x15f, at least the unit's current level of it), and the charge count (aurastat1 of the
//     builder, 1..3).
//   - The function run is srvprgfunc<count> of the builder. With prgstack the lower ones run first: for k below
//     count, the count stat is set to k and srvprgfunc<k> runs, then srvprgfunc<count> runs. Functions outside
//     1..190 are skipped. The charge state is then removed.
//   - The progressive missile each function uses (SKILL_GetProgressiveMissileId 0x5d2370) is srvmissilea at a
//     count below 2, srvmissileb at 2 and srvmissilec at 3 or more (for a skill with the progressive flag).
//   - SRVDO_EvalSkillCalcOrStateValue gives the radius / step: prgcalc<count> (1..3) for such a skill, or its
//     aurarangecalc when that is 0.
//
// Functions: 36 = nova ring of the progressive missile around the target; 37 = ring of 64/step missiles on the
// fixed 30 unit circle (arc A); 143 = the same ring with path type 10 (arc B); 38 = the skill's damage (physical
// and elemental) to everything within the radius of the aim point; 39 = radius*radius tries of the progressive
// missile at random points of the circle around the aim point. The exact meaning of the offsets in the ring
// (target offset from the aim point) and of the nova's centre and range are UNVERIFIED.

// arcRing is the table the arc functions read: 64 entries of 30*sin(2*pi*i/64) (rounded as the exe's table),
// and the x offsets are the same table shifted by a quarter turn.
var arcRing = [64]int{
	0, 2, 5, 8, 11, 14, 16, 19, 21, 23, 24, 26, 27, 28, 29, 29,
	30, 29, 29, 28, 27, 26, 24, 23, 21, 19, 16, 14, 11, 8, 5, 2,
	0, -2, -5, -8, -11, -14, -16, -19, -21, -23, -24, -26, -27, -28, -29, -29,
	-30, -29, -29, -28, -27, -26, -24, -23, -21, -19, -16, -14, -11, -8, -5, -2,
}

// arcOffset gives the (x, y) offset of ring entry i: y = table[i], x = table[i+16] (the 0x40.. half).
func arcOffset(i int) (int, int) { return arcRing[(i+16)%64], arcRing[i%64] }

// progressiveMissile is SKILL_GetProgressiveMissileId for a unit holding the charge.
func progressiveMissile(sk *Skill, charge int) string {
	if sk.Progressive && sk.AuraState != "" {
		switch {
		case charge > 2:
			return sk.SrvMissileC
		case charge == 2:
			return sk.SrvMissileB
		}
	}

	return sk.SrvMissileA
}

// prgValue is SRVDO_EvalSkillCalcOrStateValue with the aurarangecalc fallback every finisher function uses.
func prgValue(env *Env, sk *Skill, charge int) int {
	idx := 1

	if sk.Progressive && sk.AuraState != "" {
		switch {
		case charge >= 3:
			idx = 3
		case charge == 2:
			idx = 2
		}
	}

	v := env.eval(sk.PrgCalc[idx])
	if v == 0 {
		v = env.eval(sk.AuraRangeCalc)
	}

	return v
}

// anyMeleeHit reports whether a finisher blow of this cast hit (the hit bit of the queued entry).
func anyMeleeHit(res *DoResult) bool {
	for _, m := range res.Melees {
		if m != nil && m.Hit {
			return true
		}
	}

	return res.Melee != nil && res.Melee.Hit
}

// releaseCharges runs the finisher release for the cast's blows: nothing unless a blow hit.
func (c *cast) releaseCharges() {
	if !anyMeleeHit(c.res) {
		return
	}

	for _, st := range chargeStats {
		count := c.u.Stat(st)
		if count < 1 {
			continue
		}

		b := c.p.builderOf(st)
		if b == nil {
			continue
		}

		c.releaseOne(b, st, count)
		c.effect(Effect{Kind: "clear_state", State: st})
	}
}

// releaseOne runs the srvprgfunc chain of one builder for a charge count.
func (c *cast) releaseOne(b *Skill, state string, count int) {
	lvl := maxInt(c.u.SkillLevel(b.ID), 1)
	if count > 3 {
		count = 3
	}

	env := c.p.env(b, lvl, c.u)
	bc := &cast{p: c.p, u: c.u, sk: b, lvl: lvl, tgt: c.tgt, env: env, res: c.res}

	if b.PrgStack {
		for k := 1; k < count; k++ {
			bc.runPrgFunc(b.SrvPrgFunc[k], k)
		}
	}

	bc.runPrgFunc(b.SrvPrgFunc[count], count)
}

// runPrgFunc runs one finisher function with the charge count it sees.
func (c *cast) runPrgFunc(fn, charge int) {
	if fn <= 0 || fn >= 0xbf {
		return
	}

	name := progressiveMissile(c.sk, charge)
	ax, ay := c.aim()

	switch fn {
	case 36: // SRVDO_036: nova ring of the progressive missile around the target (needs a target unit)
		if c.tgt.Unit == nil || name == "" {
			return
		}

		for i := 0; i < novaCount; i++ {
			c.ringMissile(name, ax, ay, i, castOpts{})
		}
	case 37, 143: // SRVDO_037 / SRVDO_143: arc A / arc B
		step := prgValue(c.env, c.sk, charge)
		if name == "" || step < 1 {
			return
		}

		for i := 0; i < 64; i += step {
			c.ringMissile(name, ax, ay, i, castOpts{pathType10: fn == 143, pathSeed: uint32(ax + i)})
		}
	case 38: // SRVDO_038: skill damage to everything within the radius of the aim point
		r := prgValue(c.env, c.sk, charge)

		c.effect(Effect{Kind: "area_hit", Origin: "aim", X: ax, Y: ay, Radius: r, Filter: c.sk.AuraFilter, Desc: c.desc()})
	case 40: // SRVDO_040 0x5d3890 (w37, VERIFIED): one progressive missile created at the target position
		if name == "" || c.lvl < 1 {
			return
		}

		if m := c.p.castMissile(c.u, c.sk, c.lvl, c.env, name, Target{X: ax, Y: ay},
			castOpts{hasStart: true, startX: float64(ax) + 0.5, startY: float64(ay) + 0.5, stationary: true}); m != nil {
			c.res.Missiles = append(c.res.Missiles, m)
		}
	case 41: // SRVDO_041 0x5d3910 (w37, VERIFIED): the chaos ice burst, see chaosIceBurst
		c.chaosIceBurst(name, ax, ay, prgValue(c.env, c.sk, charge))
	case 39: // SRVDO_039: radius*radius random tries of the progressive missile inside the circle
		r := prgValue(c.env, c.sk, charge)
		if name == "" || r < 1 {
			return
		}

		for i := 0; i < r*r; i++ {
			dx, dy := r-c.rollN(int32(r*2)), r-c.rollN(int32(r*2))
			if dx*dx+dy*dy > r*r {
				continue
			}

			x, y := ax+dx, ay+dy

			if c.p.Walkable != nil && !c.p.Walkable(x, y) {
				continue
			}

			if m := c.p.castMissile(c.u, c.sk, c.lvl, c.env, name, Target{X: x, Y: y},
				castOpts{hasStart: true, startX: float64(x) + 0.5, startY: float64(y) + 0.5, stationary: true}); m != nil {
				c.res.Missiles = append(c.res.Missiles, m)
			}
		}
	}
}

// ringMissile throws ring entry i from the aim point outward.
func (c *cast) ringMissile(name string, ax, ay, i int, o castOpts) {
	dx, dy := arcOffset(i)
	o.hasStart, o.startX, o.startY = true, float64(ax)+0.5, float64(ay)+0.5

	if m := c.p.castMissile(c.u, c.sk, c.lvl, c.env, name, Target{X: ax + dx, Y: ay + dy}, o); m != nil {
		c.res.Missiles = append(c.res.Missiles, m)
	}
}

// chaosIceBurst is SRVDO_041 (Royal Strike's third charge, 0x5d3910, w37). VERIFIED: the unit seed is stepped once
// and a local generator started from its low word (DRLG_InitSeed); count (SRVDO_EvalSkillCalcOrStateValue, par5 =
// 16 in the table) missiles are created at the target position, each with a heading (dx, dy), both
// rand(40) - 20 (two rolls, x then y; both zero becomes (20, 0)). Each missile gets data field 0x28 = the local
// seed's low word after the rolls and data field 0x2c = the heading as two shorts, which is what the chaos ice
// flight function (d2missile chaosIceTurn, do function 35) turns every Param1 frames.
// UNVERIFIED: that the heading is also the missile's initial destination offset from the start (taken so), and
// the local seed (the unit's own seed word is not reachable here, a roll of the caster stands in for it).
func (c *cast) chaosIceBurst(name string, ax, ay, count int) {
	if name == "" || count < 1 || c.lvl < 1 {
		return
	}

	s := newShockSeed(c.rollN(1 << 30))

	for i := 0; i < count; i++ {
		dx, dy := chaosHeading(&s)

		if m := c.p.castMissile(c.u, c.sk, c.lvl, c.env, name, Target{X: ax + dx, Y: ay + dy},
			castOpts{hasStart: true, startX: float64(ax) + 0.5, startY: float64(ay) + 0.5,
				data28: s.lo, data2C: packHeading(dx, dy)}); m != nil {
			c.res.Missiles = append(c.res.Missiles, m)
		}
	}
}

// chaosHeading rolls one chaos ice heading: x then y, each modulo 40 minus 20; (0, 0) becomes (20, 0).
func chaosHeading(s *shockSeed) (int, int) {
	dx := s.modulo(40) - 20
	dy := s.modulo(40) - 20

	if dx == 0 && dy == 0 {
		dx = 20
	}

	return dx, dy
}

// packHeading puts two signed shorts into one word (low half x), as the exe's field 0x2c holds them.
func packHeading(x, y int) uint32 { return uint32(uint16(int16(x))) | uint32(uint16(int16(y)))<<16 }
