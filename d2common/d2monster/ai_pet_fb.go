package d2monster

// Faithful ports of the summoned-minion thinkers: NecroPet 0x5e3c30 (with
// MONAI_Pet_ThinkMelee 0x5e3770 and MONAI_Pet_ThinkCaster 0x5e3a00), Vines
// 0x5eb7c0, Totem 0x5ecb10 and Raven 0x5ebd20. They supersede the UNVERIFIED
// simplified versions of ai_pet.go (this file's init runs later).
//
// The exe's path-following helpers (MONAI_FollowLeaderStep, a large switch of
// path probes) and the pet bookkeeping are optional host interfaces (FBN
// prefix); without them a fallback built on the existing PetWorld is used.

// FBNPetStepper is MONAI_FollowLeaderStep(unit, mode, ?, boost, maxDist):
// modes 0 follow, 1 hover, 2 shuffle, 3 teleport next to the leader, 4/5 nudge.
// It returns true when it issued a move. Mode meanings read from the decompile of MONAI_FollowLeaderStep (VERIFIED
// w37): 0 walk or run to a point 8 subtiles along the direction of the leader's path destination, trying eight
// directions in the same DRLG region, then the midpoint to the leader; 1 retrace the leader's 20-entry position
// history (player data) newest first, else step toward the leader (only after a walk to the leader's destination
// when the pet is in mode 2); 2 idle shuffle (10 percent: a short step of 3..5 toward the leader, else a walk to
// the leader's destination, then wait 15 ticks); 3 teleport to a random free point in the room of the leader and
// wait 5 ticks; 4 if another pet stands adjacent walk away from the leader, else nothing; 5 nudge near the target.
type FBNPetStepper interface {
	FBNFollowLeader(b *Brain, mode, boost, maxDist int) bool
}

// FBNPetCounter is PETS_CountAllPets for the leader (SelectPetActionByRange
// widens the leash by half of it, capped at 36).
type FBNPetCounter interface {
	FBNPetCount(b *Brain) int
}

// FBNTargetStates answers state/stat questions about a target unit (Vines
// avoid frozen-state (2) or cold-immune (stat 0x2d == 100) foes).
type FBNTargetStates interface {
	FBNTargetHasState(b *Brain, t Target, state int) bool
	FBNTargetStat(b *Brain, t Target, stat int) int
}

// FBNLeaderTeleporter is SERVER_MoveUnitToLevelPosition onto the leader's
// position (the Totem's recall). Returns true on success.
type FBNLeaderTeleporter interface {
	FBNTeleportToLeader(b *Brain) bool
}

// FBNSkillValue is SKILLS_GetSecondBasePlusPerLevelValue of the owner's skill
// 1 at its level: the number of pecks a Raven gets. Fallback 3.
type FBNSkillValue interface {
	FBNRavenPecks(b *Brain) int
}

func init() {
	fbnReg("NecroPet", thinkNecroPetB)
	fbnReg("Vines", thinkVinesB)
	fbnReg("Totem", thinkTotemB)
	fbnReg("Raven", thinkRavenB)
}

// fbnLeader is MONAI_GetLeaderUnit: the linked leader brain, else the owner
// the host reports (PetWorld/MercWorld Owner).
func fbnLeader(c *Ctx) (OwnerInfo, bool) {
	if l := c.B.Leader; l != nil {
		return OwnerInfo{Target: Target{ID: l.ID, X: l.X, Y: l.Y, Size: l.Size, IsPlayer: false},
			Mode: l.Mode, LevelID: l.LevelID}, true
	}

	if w, ok := c.W.(interface {
		Owner(b *Brain) (OwnerInfo, bool)
	}); ok {
		return w.Owner(c.B)
	}

	return OwnerInfo{}, false
}

func fbnInTown(c *Ctx) bool {
	if t, ok := c.W.(TownChecker); ok {
		return t.InTown(c.B)
	}

	return false
}

// fbnFollow is MONAI_FollowLeaderStep 0x5e2dc0 (modes read in w37, host-independent port in w39). A host that
// implements FBNPetStepper replaces all of it. Without one: mode 0 tries the eight points 8 subtiles around the
// leader (the exe starts at the direction of the leader's path point, which the AI cannot see: here the direction
// from the pet to the leader, so the first candidate lies beyond the leader, UNVERIFIED) with the speed override
// boost, else the midpoint to the leader; mode 1 follows the leader's trail (the 20-entry position history is not
// visible: UNVERIFIED, it walks to the leader and shuffles when close); mode 2 draws the 10 percent shuffle and
// then ALWAYS waits 15 and reports success (VERIFIED: an idle pet stands); mode 3 teleports next to the leader
// and waits 5; mode 4 steps away from the leader only when another pet is adjacent and otherwise waits 15; mode 5
// nudges near the leader.
func fbnFollow(c *Ctx, l OwnerInfo, mode, boost, maxDist int) bool {
	if s, ok := c.W.(FBNPetStepper); ok {
		return s.FBNFollowLeader(c.B, mode, boost, maxDist)
	}

	b := c.B
	lt := l.Target
	d := EdgeDistance(b.X-l.X, b.Y-l.Y, b.Size)

	switch mode {
	case 0:
		c.SetSpeed(boost)

		start := c.mappedDirection(l.X-b.X, l.Y-b.Y)
		for i := 0; i < 8; i++ {
			v := turretVec[(start+i)&7]
			p := Point{l.X + v[0]*8, l.Y + v[1]*8}

			if c.move(p, nil, 0, false) || c.move(Point{(l.X + p.X) >> 1, (l.Y + p.Y) >> 1}, nil, 0, false) {
				return true
			}
		}

		if fbnPetMoveToward(c, 4) {
			return true
		}

		return c.move(Point{l.X, l.Y}, nil, 0, false)
	case 1:
		if d > 5 {
			c.SetSpeed(100)

			if c.move(Point{l.X, l.Y}, nil, 0, false) {
				return true
			}
		}

		n := d >> 2
		if n < 4 {
			n = 4
		}

		c.SetSpeed(0)

		return fbnPetMoveToward(c, n)
	case 2:
		if b.Roll(100) < 10 {
			if fbnPetMoveToward(c, int(b.Seed.Roll(3))+3) {
				return true
			}

			c.SetSpeed(0x28)

			if c.move(Point{l.X, l.Y}, nil, 0, false) {
				return true
			}
		}

		c.Sleep(0xf)

		return true
	case 3:
		if w, ok := c.W.(interface{ Teleport(b *Brain) bool }); ok && w.Teleport(b) {
			c.Sleep(5)

			return true
		}

		return false
	case 4:
		if a, ok := c.W.(FBNPetsAdjacent); ok && a.FBNPetsAdjacent(b) > 0 {
			if c.WalkAway(lt, maxDist) || fbnPetMoveToward(c, maxDist) {
				return true
			}
		}

		c.Sleep(0xf)

		return true
	case 5:
		c.SetSpeed(0)

		if c.WalkNearTarget(&lt, maxDist) || c.WalkAway(lt, maxDist) {
			return true
		}

		c.SetSpeed(0x28)

		return c.move(Point{l.X, l.Y}, nil, 0, false)
	}

	return false
}
