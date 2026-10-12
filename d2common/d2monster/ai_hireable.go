package d2monster

// Hireable is MONAI_Think_Hireable (0x5e4240, monster-ai-2.md 6.2): the AI of
// mercenaries (rogue 271, desert guard 338, Iron Wolf 359, barbarians
// 560/561). It is registered under "Hireable"; the engine assigns it to merc
// units directly because their monstats AI column is not "Hireable".

// Distances of the follow logic (VERIFIED defaults, notes 6.2).
const (
	mercTeleportDist = 100 // edge distance above which the merc teleports to the owner
	mercOuterDefault = 0x18
	mercFarDefault   = 0x10
	mercInnerDefault = 5
	mercSeekRadius   = 25 // hostile scan radius around the merc
	mercThrCap       = 0x5f
	mercThrFixed     = 0x62
)

// OwnerInfo is the snapshot of a merc's owner.
type OwnerInfo struct {
	Target
	Mode    Mode
	LevelID int
}

// MercWorld extends World with what the Hireable AI needs.
type MercWorld interface {
	World
	// Owner returns the owning player; ok is false when there is none.
	Owner(b *Brain) (OwnerInfo, bool)
	// Teleport is FollowHelper(3): move to a free tile next to the owner.
	Teleport(b *Brain) bool
	// MercLevel is the merc's level (stat 0xc).
	MercLevel(b *Brain) int
	// IsRanged is true for archers and mages (rogue, Iron Wolf).
	IsRanged(b *Brain) bool
	// ChooseAndCast is MERC_ChooseAndQueueSkill: weighted skill pick, or the
	// default attack (or walking into reach).
	ChooseAndCast(b *Brain, t Target, dist int) CastResult
}

// CastResult says what ChooseAndCast did.
type CastResult int

// Results of ChooseAndCast.
const (
	CastFailed  CastResult = iota // nothing was started
	CastBusy                      // an animation or move runs; the engine wakes the AI when it ends
	CastInstant                   // a buff took effect at once; the AI should think again soon
)

// finishCast turns a CastResult into the scheduling the think function owes.
func finishCast(c *Ctx, r CastResult) {
	switch r {
	case CastBusy:
		c.busy()
	case CastInstant:
		c.Sleep(5)
	default:
		c.Sleep(10)
	}
}

func init() {
	register("Hireable", TargetNone, thinkHireable)
}

// mercFixedThreshold lists the classes with the fixed attack threshold 0x62
// (desert guard, barbarians; VERIFIED).
func mercFixedThreshold(class int) bool { return class == 338 || class == 560 || class == 561 }

// MercLeaderOps are the optional host hooks of the faithful Hireable think (w40). A host that lacks them gets
// the fallbacks named in each comment.
type MercLeaderOps interface {
	// ClearState is STATE_ToggleUnitState(unit, id, 0).
	ClearState(b *Brain, id int)
	// FootprintBlocked is COLLISION_TestFootprintMask(room, x, y, size, 0x40) != 0 (fallback: false).
	FootprintBlocked(b *Brain) bool
	// InTownRoom is ROOM_IsTownRoom of the merc's room (fallback: false).
	InTownRoom(b *Brain) bool
	// SameRegion compares DRLG_GetRegionIdAtPointInRoom of the leader and the merc (fallback: same level).
	SameRegion(b *Brain, o OwnerInfo) bool
	// LeaderFollow is MONAI_TeleportToLeader 0x5e2810 for kind 0 (step toward the leader's destination, trying 8
	// directions in the same region) or kind 1 (leader path breadcrumbs, then PetMoveTowardTarget); run and speed
	// are its nArg5/nArg6. It returns false when the host could not start a move.
	LeaderFollow(b *Brain, o OwnerInfo, kind int, run bool, speed int) bool
}

// mercClearedState is the unit state the exe tests (0x63af30) and clears (0x63aef0) first: 0xc, VERIFIED w42 from
// the disassembly (w40 called it taunt and the Go port tested 27; states.txt 12 is inferno).
const mercClearedState = 0xc

// mercRogueClass gets alternate state 5 (GoodNpcRanged, not ported) when it has no player leader; all others state 6.
const mercRogueClass = 271 // 0x10f

// mercGuardClass: 0x152 and 0x230..0x231 take the 6 (not 12) footprint wander threshold (VERIFIED 0x5e446b).
func mercGuardClass(class int) bool { return class == 338 || class == 560 || class == 561 }

// mercEnterState is MONAI_SetAiState(n) + wait 10 for the no-leader case. State 5 has no Go think
// (UNVERIFIED stand-in: wait).
func mercEnterState(c *Ctx, n int) {
	if def := StateDef(n); def != nil {
		if c.B.baseDef == nil {
			c.B.baseDef = c.B.Def
		}

		c.B.SetAI(def)
	}

	c.Sleep(10)
}

// thinkHireable is MONAI_Think_Hireable 0x5e4240, rewritten from the exe in w40 (order of tests as in the exe).
func thinkHireable(c *Ctx) {
	w, ok := c.W.(MercWorld)
	if !ok {
		c.Sleep(25)

		return
	}

	b := c.B
	ops, _ := c.W.(MercLeaderOps)

	owner, ok := w.Owner(b)
	if !ok || !owner.IsPlayer {
		// no leader, or the leader is not a player: alternate state 5 (class 271) or 6
		if b.Class == mercRogueClass {
			mercEnterState(c, StateGoodRange)
		} else {
			mercEnterState(c, StateTownGuard)
		}

		return
	}

	if ops != nil && w.HasState(b, mercClearedState) {
		ops.ClearState(b, mercClearedState)
	}

	// mode 2 (walk) and 0xf (run): return, the animation end wakes the AI
	if b.Mode == ModeWalk || b.Mode == ModeRun {
		c.busy()

		return
	}

	outer, far, inner := mercOuterDefault, mercFarDefault, mercInnerDefault

	if band := b.Scratch[0]; band >= 0x11 && band <= 0x13 {
		outer, far, inner = 2*band, band, band>>1
	}

	d := EdgeDistance(b.X-owner.X, b.Y-owner.Y, b.Size)

	follow := func(kind int, run bool, speed int) {
		if ops != nil {
			if !ops.LeaderFollow(b, owner, kind, run, speed) {
				c.busy()
			}

			return
		}

		if speed != 0 {
			c.SetSpeed(speed)
		}

		if run {
			c.RunTo(owner.Target, inner)
		} else {
			c.WalkTo(owner.Target, inner)
		}
	}

	switch {
	case d > mercTeleportDist:
		w.Teleport(b) // kind 3; on success the exe waits 5 frames

		c.Sleep(5)

		return
	case d > outer:
		follow(1, true, 0x3c)

		return
	case d > far:
		// owner modes 2/6 (host: ModeWalk) and 3 (ModeRun)
		switch owner.Mode {
		case ModeWalk:
			follow(0, false, 0)

			return
		case ModeRun:
			follow(0, true, 0x3c)

			return
		}
	}

	if b.Mode != ModeNeutral {
		c.Sleep(5) // EVENT_CancelByTypeAndData(2, 0) then wait 5

		return
	}

	blocked := ops != nil && ops.FootprintBlocked(b)

	if blocked {
		thr := 6
		if !mercGuardClass(b.Class) {
			thr = 12
		}

		if b.Roll(128) < thr { // RAND_RollSeedPow2Mask(0x80)
			c.Wander(5)

			return
		}
	}

	if !(ops != nil && ops.InTownRoom(b)) {
		if t, dist, ok := w.AttackTarget(b); ok && dist < 0x19 {
			mercDecideAttack(c, w, t, dist)

			return
		}
	}

	if blocked {
		c.Wander(5)

		return
	}

	same := b.LevelID == owner.LevelID
	if ops != nil {
		same = ops.SameRegion(b, owner)
	}

	if !same {
		follow(0, true, 0x3c)

		return
	}

	if d < 2 {
		// step around the leader, else away, else walk to the leader's destination
		if c.WalkNearRandom(owner.Target, inner-1) {
			return
		}

		if c.WalkAway(owner.Target, inner-1) {
			return
		}

		c.SetSpeed(0x28)
		c.WalkTo(owner.Target, 0)

		return
	}

	if b.Roll(100) < 5 {
		c.WalkNearRandom(owner.Target, far)

		return
	}

	c.Sleep(5)
}

// mercDecideAttack is MERC_DecideAttack (0x5e3fa0, VERIFIED): a pressure
// counter in Scratch[0] raises the chance to hold back; ranged mercs keep the
// target within reach and shoot, melee mercs shuffle when too close.
func mercDecideAttack(c *Ctx, w MercWorld, t Target, dist int) {
	b := c.B

	thr := b.Scratch[0] + 0x28 + 2*w.MercLevel(b)
	if thr > mercThrCap {
		thr = mercThrCap
	}

	if mercFixedThreshold(b.Class) {
		thr = mercThrFixed
	}

	bump := b.Roll(100) >= thr
	if bump {
		b.Scratch[0] += 10
	} else {
		b.Scratch[0] = 0
	}

	if mercCloseBranch(b.Class) {
		// monstats AIParam1 != 0 (desert guard, barbarians): fight at arm's
		// length; edge distance < 3 and in melee reach: wait on a bump, else
		// cast; otherwise run at the target (VERIFIED, 0x5e3fa0).
		if dist < 3 && w.InRange(b, t, dist) {
			if bump {
				c.Sleep(10)

				return
			}

			finishCast(c, w.ChooseAndCast(b, t, dist))

			return
		}

		if c.RunTo(t, 1) {
			return
		}

		c.Sleep(10)

		return
	}

	// AIParam1 == 0 (rogue archer, Iron Wolf; VERIFIED): within 3 tiles half
	// of the time step to a random spot near the target, else away from it; if
	// neither works fall through to the skill choice. The skill chooser itself
	// walks into reach.
	if dist < 4 && b.Roll(100) < 0x32 {
		if c.WalkNearRandom(t, 4) {
			return
		}

		if c.WalkAway(t, 4) {
			return
		}

		finishCast(c, w.ChooseAndCast(b, t, dist))

		return
	}

	if bump {
		c.Sleep(10)

		return
	}

	finishCast(c, w.ChooseAndCast(b, t, dist))
}

// mercCloseBranch reports the classes whose monstats AIParam1 is non-zero
// (desert guard 338, barbarians 560/561): VERIFIED against the data, not the
// "ranged" flag the first notes assumed (verify-merc.md item 7, re-read from
// 0x5e3fa0 in wave 20).
func mercCloseBranch(class int) bool { return class == 338 || class == 560 || class == 561 }

// WalkNearRandom is MONAI_WalkNearTargetRandom(target, n): walk to a random
// point within n subtiles of the target (U: the notes only say the offsets
// are rolled like Wander).
func (c *Ctx) WalkNearRandom(t Target, n int) bool {
	if n < 1 {
		n = 1
	}

	b := c.B
	dx := b.Roll(2*n+1) - n
	dy := b.Roll(2*n+1) - n

	return c.move(Point{t.X + dx, t.Y + dy}, nil, 0, false)
}
