package d2skill

import (
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
)

// Wave 23 re-read of the Barbarian skills against Game.exe 1.14b (notes: w23-skills-barb.md). V marks a rule read
// in the decompilation, U an inferred one.
//
//	Bash, Stun, Concentrate   SRVST_Bash 0x5d6910 (doTable[2] wrapper below: the aurastate)
//	Berserk                   SRVST_039 0x5d82a0
//	Frenzy                    SRVDO_009 0x5d78b0, 0x5d7730 (state), 0x5d75b0 (blow)
//	Double Swing              SRVDO_070 0x5d6ee0
//	Leap                      SRVST_040 0x5d8830, 0x5d84d0 (landing), SRVDO_077 0x5d8e60, 0x5d8a70 / 0x5d8c20 (path)
//	Leap Attack               SRVST_041 0x5d9040, 0x647040 (landing), SRVDO_078 0x5d9320, 0x5d8f90 / 0x5d9170
func init() {
	doTable[doMelee] = doMeleeStateFn
	doTable[9] = doFrenzyExeFn
	doTable[70] = doDoubleSwingExeFn
	doTable[74] = doDoubleThrowExeFn
	doTable[77] = doLeapW23Fn
	doTable[78] = doLeapAttackW23Fn
}

// ---- element conversion ----

// ElemConvClass is the elemtypes.txt row of an EType name, the class byte of the damage struct (+0x65): fire 1,
// lightning 2, magic 3, cold 4, poison 5, random 10, burn 11, freeze 12. Names that are not a conversion target
// (stun, life, mana...) give 0.
func ElemConvClass(etype string) byte {
	switch etype {
	case "fire":
		return 1
	case "ltng":
		return 2
	case "mag":
		return 3
	case "cold":
		return 4
	case "pois":
		return 5
	case "rand":
		return 10
	case "burn":
		return 11
	case "frze":
		return 12
	}

	return 0
}

// ---- Bash, Stun, Concentrate, Berserk: the aurastate ----

// The Concentrate state has no timer in the exe: SRVST_Bash 0x5d6910 allocates its stat list with flag 4 and expiry 0, and
// UNIT_SetUnitMode (on a real mode change) runs UNIT_RemoveModeBoundStatLists, which switches off the state of every
// child list with flag 4 and destroys it (VERIFIED w28). So the state lasts until the hero's mode changes (ModeBound).

// berserkMinFrames is the floor of the Berserk duration (0x5d82a0: an expiry not after the current frame becomes
// frame + 10).
const berserkMinFrames = 10

// doMeleeStateFn is the do function 2 (Kick, Power Strike, Bash, Stun, Concentrate, Berserk) followed by the part
// of SRVST_Bash / SRVST_039 the old port left out (VERIFIED, 0x5d6910 and 0x5d82a0): once the blow is resolved, hit
// or miss, a skill with a valid aurastate puts it on the CASTER with the aurastat columns. Berserk lasts calc2
// frames (at least 10) and replaces the state each time; Concentrate has no timer in the exe (ends
// on the next mode change, see above). Bash and Stun have no aurastate, so nothing changes for them.
func doMeleeStateFn(c *cast) {
	doMeleeFn(c)

	if !c.res.OK || len(c.res.Melees) == 0 || c.sk.AuraState == "" {
		return
	}

	switch {
	case c.sk.SrvStFunc == 39:
		frames := c.calc(2)
		if frames <= 0 {
			frames = berserkMinFrames
		}

		c.effect(Effect{Kind: "self_state", State: c.sk.AuraState, Frames: frames, Stats: statsOf(c.env, c.sk)})
	case c.sk.SrvStFunc == stBash:
		c.effect(Effect{Kind: "self_state", State: c.sk.AuraState, ModeBound: true, Stats: statsOf(c.env, c.sk)})
	}
}

// ---- Frenzy and Double Swing: two blows per swing ----

// swingReach is "melee range plus 4" of the second blow's scan (UNIT_GetMeleeRangePlus4 0x56c2d0), in subtiles.
// VERIFIED value: UNIT_GetMeleeRange (0 for most weapons) + 4, shared with the Fend search (Options.MeleeRange).
func (p *Pipeline) swingReach() int { return p.meleeRange() + 4 }

// secondVictim is the target of the odd animation event of Frenzy and Double Swing (VERIFIED 0x5d78b0 /
// 0x5d6ee0): the swing alternates between two events; the even one strikes the skill's target, the odd one scans
// the units within melee range + 4 (SKILL_ScanRadiusForTargets, hostile filter 0x20003) passing the target's id as
// the last victim, which makes the callback 0x569a40 pick the next larger unit id, else the smallest id (the
// round robin of WhirlNext, so a lone enemy is struck again). Nobody in reach: no second blow. Without a Near hook
// the first target is struck twice.
func (c *cast) secondVictim() (Target, bool) {
	if c.p.Near == nil {
		return c.tgt, true
	}

	ux, uy := c.u.Pos()
	r := c.p.swingReach()

	var (
		ids   []string
		foes  []Foe
		first = c.tgt.Unit.ID()
	)

	for _, f := range c.p.foes(ux, uy, r) {
		dx, dy := f.X-ux, f.Y-uy
		if f.Target == nil || !f.Target.Alive() || dx*dx+dy*dy > r*r {
			continue
		}

		ids = append(ids, f.Target.ID())
		foes = append(foes, f)
	}

	i := WhirlNext(ids, first)
	if i < 0 {
		return Target{}, false
	}

	f := foes[i]

	return Target{Unit: f.Target, UX: f.X, UY: f.Y, X: f.X, Y: f.Y}, true
}

// doDoubleSwingExeFn is SRVDO_070 (0x5d6ee0, VERIFIED shape): each of the two events of the swing is one
// SRVST_Bash blow with the skill's own numbers (calc1 percent damage; the +50 percent attack speed of calc3 goes
// through SKILL_ApplyAttackRateStatList as the stat attackrate, an animation speed that this pipeline does not
// model: U). Event one strikes the target, event two the next enemy in reach (secondVictim).
func doDoubleSwingExeFn(c *cast) {
	if !c.needTarget() {
		return
	}

	o := c.meleeOpt()
	o.pct = c.calc(1)
	c.addMelee(c.p.strike(c.u, c.sk, c.lvl, c.tgt.Unit, c.env, o))

	if t, ok := c.secondVictim(); ok {
		c.addMelee(c.p.strike(c.u, c.sk, c.lvl, t.Unit, c.env, o))
	}
}

// doFrenzyExeFn is SRVDO_009 (0x5d78b0, VERIFIED shape). Like Double Swing the swing has two events with the
// same victim rule. Before each event the timed state is refreshed (SKILL_FrenzyApplyTimedStateAndAura 0x5d7730)
// but ONLY when the previous Frenzy blow hit (the exe keeps that in the skill's target-type field; a miss clears
// it): the stack counter (stat 0xa9) grows by one up to the SKILL LEVEL, the duration is auralencalc, and the
// aura stats are evaluated with the counter in place of the level (so the speed bonus climbs along the dm34 /
// dm56 curves with the counter). The blow itself (0x5d75b0) is a plain roll with calc1 percent damage, the
// element conversion of calc4 (Berserk level percent into magic) and the SrcDam scale.
func doFrenzyExeFn(c *cast) {
	if !c.needTarget() {
		return
	}

	if c.p.chain == nil {
		c.p.chain = map[string]bool{}
	}

	uid := c.u.ID()
	o := c.meleeOpt()
	o.pct = c.calc(1)

	blow := func(t Target) {
		m := c.p.strike(c.u, c.sk, c.lvl, t.Unit, c.env, o)
		c.addMelee(m)
		c.p.chain[uid] = m.Hit
	}

	if c.p.chain[uid] {
		c.frenzyState()
	}

	blow(c.tgt)

	if c.p.chain[uid] {
		c.frenzyState()
	}

	if t, ok := c.secondVictim(); ok {
		blow(t)
	}
}

// frenzyState emits one stack step of the Frenzy state.
func (c *cast) frenzyState() {
	limit := c.lvl
	if limit < 1 {
		limit = 1
	}

	frames := c.env.eval(c.sk.AuraLenCalc)
	if frames < 1 {
		frames = 1
	}

	by := make(map[int][]StatMod, limit)
	for n := 1; n <= limit; n++ {
		by[n] = statsOf(c.p.env(c.sk, n, c.u), c.sk)
	}

	c.effect(Effect{Kind: "self_state", State: c.sk.AuraState, Frames: frames, Stats: by[1], Stack: limit, CountStats: by})
}

// ---- Leap and Leap Attack ----

// octDist is UNIT_GetDistanceToPoint (0x642c30, VERIFIED): the longer axis plus half the shorter one, integer.
func octDist(dx, dy int) int {
	if dx < 0 {
		dx = -dx
	}

	if dy < 0 {
		dy = -dy
	}

	if dy < dx {
		return (dy + 2*dx) / 2
	}

	return (dx + 2*dy) / 2
}

// LeapClamp is the range rule of SRVST_Leap_ValidateLandingArea (0x5d84d0, VERIFIED): the destination is pulled
// back along the line from the caster so that its octagonal distance is at most r, the skill's aurarangecalc
// (Leap: dm12, 4 to 30 subtiles by level). A distance below 2 counts as 1. (x, y) is returned unchanged inside r.
func LeapClamp(hx, hy, x, y, r int) (int, int) {
	d := octDist(x-hx, y-hy)
	if d < 2 {
		d = 1
	}

	if r < d {
		return (x-hx)*r/d + hx, (y-hy)*r/d + hy
	}

	return x, y
}

// LeapAttackLanding is SKILL_ValidateLeapLanding (0x647040, VERIFIED): a Leap Attack does not land on the target
// but two subtiles beyond it on the line from the caster (destination + 2 * (destination - caster) / distance, the
// distance being octDist). A destination on the caster is refused. There is no range limit in the exe, only the
// clear line to the landing cell.
func LeapAttackLanding(hx, hy, tx, ty int) (int, int, bool) {
	d := octDist(tx-hx, ty-hy)
	if d == 0 {
		return 0, 0, false
	}

	return (tx-hx)*2/d + tx, (ty-hy)*2/d + ty, true
}

// LeapFrames is how long the jump lasts: the path length at the leap mode velocity, 0x1000 in 8.8 or one subtile
// per frame (d2path.LeapVelocity, V for the leap mode; that Leap Attack uses the same mode is U), at least 1.
func LeapFrames(hx, hy, x, y int) int {
	dx, dy := float64(x-hx), float64(y-hy)
	f := int(math.Ceil(math.Hypot(dx, dy) / d2path.SubtilesPerTick(d2path.LeapVelocity)))

	if f < 1 {
		return 1
	}

	return f
}

// doLeapW23Fn is SRVST_040 + SRVDO_077 (VERIFIED shape). The destination is limited by aurarangecalc (LeapClamp),
// then the jump takes LeapFrames; at the landing a player throws back everything hostile within calc1 (result flag
// 8, no damage; a monster skips it).
func doLeapW23Fn(c *cast) {
	hx, hy := c.u.Pos()
	x, y := c.aim()

	if !c.sk.AuraRangeCalc.Empty() {
		x, y = LeapClamp(hx, hy, x, y, c.env.eval(c.sk.AuraRangeCalc))
	} else {
		x, y, _ = c.clampMove(leapDistance)
	}

	if c.p.Walkable != nil && !c.p.Walkable(x, y) {
		c.fail(ReasonLOS)
		return
	}

	frames := LeapFrames(hx, hy, x, y)
	c.effect(Effect{Kind: "move", Mode: "leap", X: x, Y: y, Frames: frames})

	if r := c.calc(1); r > 0 && c.u.IsPlayer() {
		c.effect(Effect{Kind: "knock_area", X: x, Y: y, Radius: r, Dist: KnockDistance, Delay: frames})
	}
}

// leapAttackVictim is SKILL_LeapAttackFindTarget (0x5d8f90, VERIFIED): the skill's target if it is within melee
// range of the landing, else the unit remembered from the start (the same target), else the SMALLEST unit id found
// by the scan around the caster (melee range + 4, last victim -1), not the nearest.
func (c *cast) leapAttackVictim(lx, ly int) (d2missile.Target, int, int) {
	if t := c.tgt.Unit; t != nil && t.Alive() && cheb(c.tgt.UX-lx, c.tgt.UY-ly) <= leapMeleeRange {
		return t, c.tgt.UX, c.tgt.UY
	}

	var (
		ids  []string
		foes []Foe
	)

	r := c.p.swingReach()

	for _, f := range c.p.foes(lx, ly, r) {
		dx, dy := f.X-lx, f.Y-ly
		if f.Target == nil || !f.Target.Alive() || dx*dx+dy*dy > r*r {
			continue
		}

		ids = append(ids, f.Target.ID())
		foes = append(foes, f)
	}

	if i := WhirlNext(ids, ""); i >= 0 {
		return foes[i].Target, foes[i].X, foes[i].Y
	}

	return nil, 0, 0
}

// doLeapAttackW23Fn is SRVST_041 + SRVDO_078 (VERIFIED shape). A target already in melee range gets no jump (the
// exe walks the blow in instead). Otherwise the landing is LeapAttackLanding, the jump takes LeapFrames and the
// blow, its bash overlay, the knockback bit and the removal of the victim's stun come at the landing.
func doLeapAttackW23Fn(c *cast) {
	hx, hy := c.u.Pos()
	tx, ty := c.aim()
	frames := 0
	lx, ly := hx, hy

	inReach := c.tgt.Unit != nil && c.tgt.Unit.Alive() && cheb(c.tgt.UX-hx, c.tgt.UY-hy) <= leapMeleeRange
	if !inReach {
		var ok bool

		lx, ly, ok = LeapAttackLanding(hx, hy, tx, ty)
		if !ok {
			c.fail(ReasonLOS)
			return
		}

		if c.p.Walkable != nil && !c.p.Walkable(lx, ly) {
			// the exe searches the nearest free cell around the target; the target cell stands in for it (U)
			lx, ly = tx, ty
			if !c.p.Walkable(lx, ly) {
				c.fail(ReasonLOS)
				return
			}
		}

		frames = LeapFrames(hx, hy, lx, ly)
		c.effect(Effect{Kind: "move", Mode: "leap", X: lx, Y: ly, Frames: frames})
	}

	t, vx, vy := c.leapAttackVictim(lx, ly)
	if t == nil {
		return
	}

	o := c.meleeOpt()
	o.pct = c.calc(1)
	o.skillElem = c.sk.EType != "" && c.calc(4) > 0
	m := c.p.strike(c.u, c.sk, c.lvl, t, c.env, o)
	m.Delay = frames
	c.addMelee(m)

	if !m.Hit {
		return
	}

	c.effect(Effect{Kind: "knockback", Target: t, Dist: KnockDistance, Delay: frames})
	c.effect(Effect{Kind: "overlay", Overlay: OverlayBash, Target: t, X: vx, Y: vy, Delay: frames})
	c.effect(Effect{Kind: "unit_clear_state", State: "stunned", Target: t, Delay: frames})
}

// ---- Double Throw ----

// doDoubleThrowExeFn is SRVDO_074 (0x5d7340, VERIFIED shape). Each animation event throws ONE missile of the held
// throwing weapon with no angle offset, then adds the skill's to-hit bonus (stat 0x13) and calc1 as damagepercent
// (stat 0x19) to that missile; no weapon missile (ITEM_GetMissileType 0) fails. The animation has two events, so
// two missiles fly along the same line, not the +-3 degree fan the old port used. The event count of 2 is U.
func doDoubleThrowExeFn(c *cast) {
	name := c.u.ThrownMissile()
	if name == "" {
		c.fail(ReasonNoWeapon)
		return
	}

	for i := 0; i < 2; i++ {
		c.castM(name, castOpts{})
	}

	if len(c.res.Missiles) == 0 {
		c.fail(ReasonMissile)
	}
}
