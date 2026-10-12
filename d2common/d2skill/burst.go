package d2skill

import (
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

// Burst is the per-animation-event repetition mechanism of the exe's multi-hit
// skills. In Game.exe a skill such as Fend or Zeal does not hit N times in one
// call: its do function (SRVDO_013_Fend 0x5da910) runs once per attack
// animation event, hits one unit, decrements the skill's remaining-hit counter
// (the "target type" field of the Skill record), looks for the next victim and
// reschedules the animation events while the counter is positive. A Burst is
// that state kept outside the unit record: Pipeline.Do runs the first event and
// returns the Burst in DoResult.Burst when Options.EventBursts is set; the
// engine then calls Pipeline.BurstEvent at every further attack event until
// Burst.Done. Without EventBursts the whole burst runs inside Do (RunBurst).
// Other skills can reuse it by adding a BurstKind with its own victim choice.
type Burst struct {
	kind   BurstKind
	u      Unit
	sk     *Skill
	lvl    int
	env    *Env
	left   int    // hits still to deliver (exe: Skill target-type counter)
	target Target // victim of the next event
	last   string // id of the unit hit by the previous event ("" none)
	opt    strikeOpt

	// Animation clock (actionevents.go): nil keeps the fallback gap. pct is the reschedule percent after each
	// hit (Fend, Zeal, Fury: Param2; Strafe: Param6), negative for a fixed sequence (Jab). delay is the number
	// of game frames from the last event to the next one.
	clock *ActionClock
	pct   int
	delay int
	shots int  // events run so far
	rng   int  // Strafe: scan radius in subtiles
	seq   bool // a fixed sequence: the victim is the stored target, never rescanned
}

// BurstKind selects what one event of a Burst does.
type BurstKind int

const (
	// BurstMeleeChain is Fend / Zeal: one melee strike per event; the target
	// must be in melee range, else any enemy within melee range + 4 other than
	// the last victim is taken (scan filter 0x20003, VERIFIED 0x5da910).
	BurstMeleeChain BurstKind = iota
	// BurstStrafe is Strafe (SRVDO_012 0x5da6c0, VERIFIED): one arrow per event at the stored target, then the
	// next enemy within the scan radius other than the one just shot, reschedule by Param6 percent; the burst
	// ends when no other enemy is found.
	BurstStrafe
	// BurstSequence is a fixed animation sequence (Jab: one strike per do row of the sequence, no reschedule).
	BurstSequence
)

// FallbackBurstGap is the spacing in game frames of two burst events when no animation clock is available (no
// AnimData record of the hero's attack). UNVERIFIED guess, only a fallback since wave 25.
const FallbackBurstGap = 8

// NextDelay is the number of game frames between the event just run and the next one. fromClock tells whether
// it comes from the exe's event scheduling (true) or is the fallback gap.
func (b *Burst) NextDelay() (frames int, fromClock bool) {
	if b == nil || b.clock == nil || b.delay <= 0 {
		return FallbackBurstGap, false
	}

	return b.delay, true
}

// Shots is the number of events run so far.
func (b *Burst) Shots() int {
	if b == nil {
		return 0
	}

	return b.shots
}

// advance moves the animation clock past the event that just ran: the exe reschedules the animation events by
// the skill's percent (unless the sequence is fixed) and the next do event decides the delay. A mode that ends
// before another do event ends the burst.
func (b *Burst) advance() {
	if b.clock == nil || b.left <= 0 {
		return
	}

	if b.pct >= 0 {
		b.clock.Reschedule(b.pct)
	}

	t, ok := b.clock.NextDo()
	if !ok {
		b.left = 0

		return
	}

	b.delay = t - b.clock.Now()
}

// attachClock arms the burst with the cast's clock; the clock is positioned on the cast's own first event.
func (b *Burst) attachClock(c *ActionClock, pct int) {
	b.clock, b.pct = c, pct
}

// Done reports whether no event is left.
func (b *Burst) Done() bool { return b == nil || b.left <= 0 }

// Left is the number of events still to run.
func (b *Burst) Left() int {
	if b == nil {
		return 0
	}

	return b.left
}

// Last is the id of the previous event's victim.
func (b *Burst) Last() string { return b.last }

// meleeRange is the reach of Fend / Zeal in subtiles (Options.MeleeRange, 4
// when unset; the exe calls a helper whose value is not decoded: UNVERIFIED).
func (p *Pipeline) meleeRange() int {
	if p.Opt.MeleeRange > 0 {
		return p.Opt.MeleeRange
	}

	return 4
}

// newMeleeBurst arms an event burst of n hits at the cast's target.
func (p *Pipeline) newMeleeBurst(c *cast, n int) *Burst {
	return &Burst{kind: BurstMeleeChain, u: c.u, sk: c.sk, lvl: c.lvl, env: c.env, left: n, target: c.tgt, opt: c.meleeOpt()}
}

// pickMeleeVictim implements the target choice of one event: the stored
// target if alive and within melee range of the caster, otherwise the first
// enemy within range+4 that is not the previous victim. Without a Near hook
// the stored target is used as is.
func (b *Burst) pickMeleeVictim(p *Pipeline) (Target, bool) {
	ux, uy := b.u.Pos()
	reach := p.meleeRange()

	if t := b.target; t.Unit != nil && t.Unit.Alive() {
		x, y := t.UX, t.UY
		known := p.Near == nil

		if p.Near != nil {
			for _, f := range p.Near(ux, uy, reach+4) {
				if f.Target.ID() == t.Unit.ID() {
					x, y, known = f.X, f.Y, true

					break
				}
			}
		}

		if known && (p.Near == nil || cheb(x-ux, y-uy) <= reach) {
			t.UX, t.UY = x, y

			return t, true
		}
	}

	return b.scanOther(p)
}

// scanOther is the radius scan (melee range + 4, last victim excluded).
func (b *Burst) scanOther(p *Pipeline) (Target, bool) {
	if p.Near == nil {
		return Target{}, false
	}

	ux, uy := b.u.Pos()

	for _, f := range p.Near(ux, uy, p.meleeRange()+4) {
		if f.Target.ID() == b.last || !f.Target.Alive() {
			continue
		}

		return Target{Unit: f.Target, UX: f.X, UY: f.Y, X: f.X, Y: f.Y}, true
	}

	return Target{}, false
}

// BurstEvent runs one animation event of the burst and appends its strike to
// res. It returns false when nothing was hit (no victim in reach): the exe then
// ends the sequence without rescheduling.
func (p *Pipeline) BurstEvent(b *Burst, res *DoResult) bool {
	if b.Done() {
		return false
	}

	if b.clock != nil {
		b.clock.RunNextDo()
	}

	b.shots++

	if b.kind == BurstStrafe {
		return p.strafeEvent(b, res)
	}

	t, ok := b.pickMeleeVictim(p)
	if b.seq {
		t, ok = b.target, b.target.Unit != nil && b.target.Unit.Alive()
	}

	if !ok {
		b.left = 0

		return false
	}

	m := p.strike(b.u, b.sk, b.lvl, t.Unit, b.env, b.opt)
	if res.Melee == nil {
		res.Melee = m
	}

	res.Melees = append(res.Melees, m)

	b.left--
	b.last = t.Unit.ID()
	b.target = t

	if b.left > 0 && !b.seq {
		// the exe rescans after the hit (excluding the victim just hit) and
		// stores the result as the next target
		// (none found keeps the old target, which is then struck again)
		if n, ok := b.scanOther(p); ok {
			b.target = n
		}
	}

	b.advance()

	return true
}

// RunBurst delivers every remaining event at once (the one-call behaviour,
// used when the engine has no animation-event hook).
func (p *Pipeline) RunBurst(b *Burst, res *DoResult) {
	for !b.Done() {
		if !p.BurstEvent(b, res) {
			return
		}
	}
}

// ChargedBoltSeed is the seed the per-bolt hook (0x5c7340, VERIFIED) gives the
// i-th Charged Strike bolt's own random generator: the bolt's destination X
// plus its index, so every bolt of a cast follows a distinct but repeatable
// path. (The hook also sets the bolt's path type to 10 and its step counters
// to its lifetime word, clamped to 0x4d.)
func ChargedBoltSeed(destX, index int) uint32 { return uint32(destX + index) }

// ChargedBoltAim returns the angle (radians) of the i-th Charged Strike bolt:
// the exe aims every bolt at the target mirrored through the target
// (destination 2*target - caster, VERIFIED in SRVDO_011) and path type 10 then
// scatters it using the bolt's seeded generator. The scatter width (+-90
// degrees here) is UNVERIFIED because the path-type-10 stepper is not decoded.
func ChargedBoltAim(casterX, casterY, targetX, targetY, index int) float64 {
	dx := float64(targetX - casterX)
	dy := float64(targetY - casterY)

	base := 0.0
	if dx != 0 || dy != 0 {
		base = math.Atan2(dy, dx)
	}

	seed := d2rand.New(ChargedBoltSeed(targetX*2-casterX, index))
	jitter := float64(int(seed.Roll(181))-90) * math.Pi / 180

	return base + jitter
}
