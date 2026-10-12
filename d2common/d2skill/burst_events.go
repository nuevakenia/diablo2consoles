package d2skill

import "strings"

// clockFor asks the host for the action-event clock of the caster's attack animation (nil: none).
func (p *Pipeline) clockFor(u Unit, sk *Skill) *ActionClock {
	if p.Clock == nil {
		return nil
	}

	return p.Clock(u, sk)
}

// SkillSequence is the skills.txt seqnum of the player sequence a skill plays (SeqJab, SeqImpale), 0 for a skill
// that plays a plain attack or cast mode. Only the sequences decoded in actionevents.go are listed.
func SkillSequence(sk *Skill) int {
	switch {
	case sk == nil:
		return 0
	case sk.SrvDoFunc == doJab:
		return SeqJab
	case sk.SrvStFunc == stImpale:
		return SeqImpale
	}

	return 0
}

// doJabFn is SRVDO_Jab (0x5d9e80): one strike per do event. The three thrusts of a Jab are the three flagged
// rows of the sequence animation (SeqJab) of the hero's weapon class, not a counter: the host's clock (Pipeline.Clock)
// tells how many do events the sequence holds and when they fall. Without a clock the cast is one strike.
func doJabFn(c *cast) {
	doMeleeFn(c)

	if c.res.Melee == nil || c.p.Clock == nil {
		return
	}

	clk := c.p.clockFor(c.u, c.sk)
	if clk == nil {
		return
	}

	n := len(clk.DoEvents())
	if n < 2 {
		return
	}

	b := &Burst{kind: BurstSequence, u: c.u, sk: c.sk, lvl: c.lvl, env: c.env, left: n - 1, target: c.tgt, opt: c.meleeOpt(), seq: true}
	b.attachClock(clk, -1)
	clk.RunNextDo() // the cast's own strike is the first do event
	b.shots = 1
	b.advance()

	if c.p.Opt.EventBursts {
		c.res.Burst = b

		return
	}

	c.p.RunBurst(b, c.res)
}

// strafeReach is the scan radius of a Strafe burst (aurarangecalc, 14 when it evaluates to nothing).
func strafeReach(c *cast) int {
	rng := c.env.eval(c.sk.AuraRangeCalc)
	if rng <= 0 {
		rng = 14
	}

	return rng
}

// strafeMissile is the arrow missile of a Strafe: srvmissilea, srvmissileb for a crossbow (VERIFIED 0x5da6c0).
func strafeMissile(c *cast) string {
	name := c.sk.SrvMissileA
	if strings.Contains(c.u.RangedWeaponMissile(), "bolt") && c.sk.SrvMissileB != "" {
		name = c.sk.SrvMissileB
	}

	return name
}

// scanOtherThan is the radius scan of Strafe (filter 3): the first enemy within the radius that is not id.
func (p *Pipeline) scanOtherThan(u Unit, rng int, id string) (Target, bool) {
	ux, uy := u.Pos()

	for _, f := range p.foes(ux, uy, rng) {
		if f.Target.ID() == id || !f.Target.Alive() {
			continue
		}

		return Target{Unit: f.Target, UX: f.X, UY: f.Y}, true
	}

	return Target{}, false
}

// strafeEvent runs one event of a Strafe burst (SRVDO_012 0x5da6c0, VERIFIED): the stored target is shot when it
// still lives, else the scan picks an enemy other than the previous victim; after the arrow the counter drops
// and, while it is positive, the scan runs again excluding the unit just shot. No second enemy ends the burst.
// The first event of a cast with nobody around still shoots at the aimed point (the engine only starts the skill
// with a target or a point).
func (p *Pipeline) strafeEvent(b *Burst, res *DoResult) bool {
	t := b.target
	if t.Unit == nil || !t.Unit.Alive() {
		n, ok := p.scanOtherThan(b.u, b.rng, b.last)

		switch {
		case ok:
			t = n
		case b.shots > 1:
			b.left = 0

			return false
		}
	}

	cc := &cast{p: p, u: b.u, sk: b.sk, lvl: b.lvl, tgt: t, env: b.env, res: res}
	cc.castM(strafeMissile(cc), castOpts{pctAdd: cc.calc(2)})

	b.left--
	b.target = t

	if t.Unit != nil {
		b.last = t.Unit.ID()
	}

	if b.left > 0 {
		// SRVDO_012 callback 0x569a40 (VERIFIED wave 24, d2missile.ScanNext): when the scan finds no other unit the
		// burst goes on at the same one, so a lone enemy takes the whole burst.
		if n, ok := p.scanOtherThan(b.u, b.rng, b.last); ok {
			b.target = n
		}
	}

	b.advance()

	return true
}
