package d2monster

// Params are the AiTickParams a think function receives (VERIFIED layout:
// target, edge distance, in-range flag).
type Params struct {
	Target  *Target // nil when the AI's target mode does not acquire one
	Dist    int
	InRange bool
}

// Ctx is passed to a think function. Its methods are the primitives of
// monster-ai-2.md section 3; each requests one action and marks the brain as
// busy until WakeNow, or sleeps.
type Ctx struct {
	B *Brain
	W World
	Params
	acted bool
}

// Acted reports whether the think function issued an action or sleep.
func (c *Ctx) Acted() bool { return c.acted }

func (c *Ctx) busy() {
	c.acted = true
	c.B.Wake = waitForever
}

// Sleep is MONAI_ScheduleWaitTicks: the next think runs n frames from now
// (n==0 becomes 1). The original also forces mode neutral if needed; the
// engine does that when the monster is not already idle.
func (c *Ctx) Sleep(n int) {
	if n < 1 {
		n = 1
	}

	c.acted = true
	c.B.Wake = c.W.Frame() + n
}

// Attack requests an attack/skill mode; on failure the original sleeps 10.
func (c *Ctx) Attack(mode Mode, t Target) bool {
	if !c.W.Attack(c.B, mode, t) {
		c.Sleep(10)
		return false
	}

	c.busy()

	return true
}

// Cast requests the monstats skill slot at t.
func (c *Ctx) Cast(slot int, t Target) bool {
	if !c.W.Cast(c.B, slot, t) {
		c.Sleep(10)
		return false
	}

	c.busy()

	return true
}

// SetSpeed is MONAI_SetMoveSpeedOverride 0x5dcf80 (VERIFIED w37: a fatal error unless the speed is
// in -126..126, then a queued move command is stored with the speed and the second argument).
func (c *Ctx) SetSpeed(v int) { c.W.SetSpeed(c.B, v) }

// WalkTo is MONAI_WalkToTarget(target, reach): reach 7 is the usual melee
// value, 0 is tile exact. Returns false when the request failed (no sleep is
// scheduled so the caller can fall back, as the original does).
func (c *Ctx) WalkTo(t Target, reach int) bool {
	if c.move(Point{t.X, t.Y}, &t, reach, false) {
		return true
	}

	return c.walkToFailed(reach)
}

// walkToFailed is the failure tail of MONAI_QueueMoveCommand 0x5dd980 for the
// "walk to melee" call (VERIFIED wave 23): MONAI_WalkToTarget(target, 7) hands
// its 7 over as the FLAGS byte (1 = AI flag, 2 = fallback, 4 = cancel event 2),
// not as a reach. When the request is refused, flag 2 takes one LCG step: below
// 70 percent the unit wanders 4 subtiles, otherwise it sleeps 10 frames, and
// the call reports SUCCESS either way, so the caller's own fallback (its sleep,
// its attack) never runs. Other reach values are not the flags value and keep
// the plain failure result.
func (c *Ctx) walkToFailed(reach int) bool {
	if reach != meleeReach {
		return false
	}

	if c.B.Roll(100) < walkFailWanderPct {
		c.Wander(walkFailWanderDist)
	} else {
		c.Sleep(walkFailSleep)
	}

	c.acted = true

	return true
}

// Constants of the flag 2 fallback of 0x5dd980 (VERIFIED immediates 0x46, 4, 10).
const (
	walkFailWanderPct  = 70
	walkFailWanderDist = 4
	walkFailSleep      = 10
)

// RunTo is MONAI_RunToTarget (mode RN).
func (c *Ctx) RunTo(t Target, reach int) bool { return c.move(Point{t.X, t.Y}, &t, reach, true) }

func (c *Ctx) move(p Point, t *Target, reach int, run bool) bool {
	if !c.W.MoveTo(c.B, p, t, reach, run) {
		return false
	}

	c.busy()

	return true
}

// WalkToRange is MONAI_WalkToRangeOfTarget(target, maxStep, desiredDist): the
// kiting primitive. With d the distance to the target, it moves toward the
// target by min(|d-desired|, maxStep) along the (dx,dy) ratio when
// d >= desired, and away from it by the same amount when d < desired
// (VERIFIED description; the rounding of the ratio is not recorded so it is
// truncation toward zero here).
func (c *Ctx) WalkToRange(t Target, maxStep, desired int) bool {
	return c.toRange(t, maxStep, desired, false)
}

// RunToRange is MONAI_MoveToRangeOfTarget with run mode.
func (c *Ctx) RunToRange(t Target, maxStep, desired int) bool {
	return c.toRange(t, maxStep, desired, true)
}

func (c *Ctx) toRange(t Target, maxStep, desired int, run bool) bool {
	b := c.B
	dx, dy := t.X-b.X, t.Y-b.Y
	d := EdgeDistance(dx, dy, b.Size)

	step := d - desired
	if step < 0 {
		step = -step
	}

	if step > maxStep {
		step = maxStep
	}

	full := Distance(dx, dy)
	if full == 0 || step == 0 {
		return c.move(Point{b.X, b.Y}, nil, 0, run)
	}

	mx, my := dx*step/full, dy*step/full
	if d < desired {
		mx, my = -mx, -my
	}

	return c.move(Point{b.X + mx, b.Y + my}, nil, 0, run)
}

// WalkAway is MONAI_WalkAwayFromTarget(target, n): the destination is the
// monster's position plus n*sign(own-target) per axis (VERIFIED: a diagonal
// flee). It returns false when the request fails.
//
// For n above 5 the exe first calls MONAI_SetMoveSpeedOverride(0, n) (asm
// 0x5ddedb: EDX 0, first argument 0, second argument n). VERIFIED in w23 from
// 0x5dcf80 and 0x5a3c60: the setter writes only its NON-ZERO arguments into the
// queued-command record (+0x18 from EDX, +0x1c the speed percent, +0x20 the
// second argument clamped to 0x4d), so this call changes only the +0x20 byte
// and leaves a speed percent the caller queued just before (Baboon rage,
// Mosquito retreat, BloodHawk) in place. Batch 8 read it as "resets the speed
// to 0"; that was wrong. The +0x20 byte is not modelled.
func (c *Ctx) WalkAway(t Target, n int) bool {
	return c.away(t, n, false)
}

// RunAway is MONAI_RunAwayFromTarget.
func (c *Ctx) RunAway(t Target, n int) bool { return c.away(t, n, true) }

func (c *Ctx) away(t Target, n int, run bool) bool {
	b := c.B

	return c.move(Point{b.X + n*sign(b.X-t.X), b.Y + n*sign(b.Y-t.Y)}, nil, 0, run)
}

// Wander is MONAI_WanderRandomNearby(n) 0x5dcff0 (VERIFIED from the
// disassembly): four LCG steps. Step 1's low bit picks the axis that gets the
// full n (set: x, clear: y); step 2 is a bounded roll(n) (0x457b60: [0,n)) for
// the other axis; step 3's low bit negates x, step 4's low bit negates y. The
// destination is the monster's position plus the offsets (0x5dd980, walk
// mode, no target).
func (c *Ctx) Wander(n int) bool {
	b := c.B
	dx, dy := c.wanderOffset(n)

	return c.move(Point{b.X + dx, b.Y + dy}, nil, 0, false)
}

// wanderOffset draws the four LCG steps shared by 0x5dcff0 and 0x5de330.
func (c *Ctx) wanderOffset(n int) (dx, dy int) {
	b := c.B
	s1 := b.Seed.Step()
	r := int(b.Seed.Roll(int32(n)))
	sx := b.Seed.Step()
	sy := b.Seed.Step()

	dx, dy = r, n
	if s1&1 != 0 {
		dx, dy = n, r
	}

	if sx&1 != 0 {
		dx = -dx
	}

	if sy&1 != 0 {
		dy = -dy
	}

	return dx, dy
}

// WalkNearTarget is MONAI_WalkNearTargetRandom(target, n) 0x5de330 (VERIFIED
// from the disassembly in wave 23): the Wander offset scheme (see Wander)
// applied around the target's position. Step 1's low bit set: dx = n and
// dy = roll(n); clear: dx = roll(n) and dy = n; the roll is the second LCG
// step, the third step's low bit negates dx and the fourth's negates dy. The
// path distance limit is 1 and the walk (mode 2) has no target unit. Without
// a target it falls back to Wander(2). An earlier port used roll(n+1) and a
// different bit assignment.
func (c *Ctx) WalkNearTarget(t *Target, n int) bool {
	if t == nil {
		return c.Wander(2)
	}

	dx, dy := c.wanderOffset(n)

	return c.move(Point{t.X + dx, t.Y + dy}, nil, 0, false)
}

// Circle is MONAI_CircleOrStrafeTarget 0x5de5e0. VERIFIED (re-read in wave 23,
// decompile plus disassembly): it consumes one LCG step and DISCARDS it (the
// low byte only picks the value 5 or 6 that goes in a register the speed
// override ignores), calls MONAI_SetMoveSpeedOverride(0, third argument) and
// queues a walk (mode 2) toward the TARGET UNIT with path distance limit 1 and
// extra flag 4 (cancel event 2 on failure) when its last argument is set. There
// is no sideways step in the exe: the "circle" is a plain approach. The n of
// the Go signature is the exe's third argument (0, 3, 4 or 6 at the callers),
// stored in the move record's second field, whose use is UNVERIFIED (w37: not read by the walk setup 0x5dcf80, only stored), so it is
// not modelled. An earlier port strafed n subtiles perpendicular to the target.
func (c *Ctx) Circle(t Target, n int) bool {
	_ = n

	c.B.Seed.Step()

	return c.move(Point{t.X, t.Y}, &t, circleReach, false)
}

// circleReach is the path distance limit passed by 0x5de5e0 (VERIFIED, push 1).
const circleReach = 1

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}

	return 0
}
