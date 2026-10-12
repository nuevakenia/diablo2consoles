package d2monster

// FallenShaman (0x5f04d0) and the forced states 13 (0x5e4be0) and 16
// (0x5e1c60). Sources: Game.exe 1.14b, read-only Ghidra; the comments say what
// is VERIFIED and what is not.

func init() {
	register("FallenShaman", TargetStandard, thinkFallenShaman)
	// Both rows are target mode 1 in the exe's state table (VERIFIED, 0x73a548
	// rows 13 and 16): with nobody inside the aggro radius the tick idles.
	regState(StateLeash, TargetStandard, thinkState13)
	regState(StateImp, TargetStandard, thinkState16)
}

// Classes the Shaman resurrects: exe monster classes 19 (Fallen) and 58
// (FallenShaman) by MONSTER_GetBaseClassId (VERIFIED, FUN_005f0420).
var shamanCorpseClasses = []int{19, 58}

// CorpseFinder is an optional extension of Senses: the nearest dead unit of one
// of classes whose corpse can be raised, within maxDist subtiles (the exe
// filter FUN_005f0420 also needs: the corpse flag set, not already being
// resurrected, mode DD).
type CorpseFinder interface {
	NearestCorpse(b *Brain, classes []int, maxDist int) (Target, bool)
}

// ShamanCorpseScanner is the exe corpse scan of the Fallen Shaman (w20, VERIFIED
// in 0x5f04d0 / 0x5f0420 / 0x5dbae0 / 0x5dbda0). radiusSq is aip4 squared. The
// host returns the LAST qualifying unit of its walk and the number of all
// qualifying units. The exe picks the family scan (Fallen and Fallen Shaman
// corpses anywhere in the radius, no unique or champion corpse) only when the
// shaman itself is a unique and not a champion; every other shaman scans the
// group of its leader for dead followers whose leader is the shaman, by EDGE
// distance compared with the squared radius (a quirk of the exe: one block holds
// the squared value for both callbacks).
type ShamanCorpseScanner interface {
	ShamanCorpseScan(b *Brain, radiusSq int) (Target, int)
}

// thinkFallenShaman is MONAI_Think_FallenShaman 0x5f04d0 (VERIFIED, re-read:
// the roll order below is the decompiled one). aip1 resurrect%, aip2 fire%,
// aip3 melee%, aip4 corpse search radius, aip5 fire range. Slots: Skill1
// Resurrect, Skill2 Shaman Fire (mode of slot 2).
//
// w24 re-read: the two calls of FUN_0059dd60 (MONSTER_HasDataFlag16) are not a
// game-state test. They test the rank bits of the shaman's monster data word
// (+0x16): mask 8 (unique) set and mask 4 (champion) clear selects the family
// scan (callback MONAI_FallenShaman_CountNearbyFallenFilter, mode 1), every
// other shaman scans the group of its leader (mode 9); the host's
// ShamanCorpseScanner does exactly that. The corpse scan is a no-op without the
// World's ShamanCorpseScanner / CorpseFinder. w38: the skill 2 roll is drawn even when the slot is undefined, as in the exe.
func thinkFallenShaman(c *Ctx) {
	b := c.B
	p := b.Profile
	t := c.Target

	if c.InRange && t != nil && b.Roll(100) < b.AIP(3) {
		c.Attack(ModeAttack1, *t)

		return
	}

	var corpse *Target

	// w20: the exe scan (squared aip4, last unit of the walk wins) when the host has it.
	if sc, ok := c.W.(ShamanCorpseScanner); ok {
		if ct, n := sc.ShamanCorpseScan(b, b.AIP(4)*b.AIP(4)); n > 0 {
			corpse = &ct
		}
	} else if cf, ok := c.W.(CorpseFinder); ok {
		if ct, found := cf.NearestCorpse(b, shamanCorpseClasses, b.AIP(4)); found {
			corpse = &ct
		}
	}

	if b.Roll(100) < b.AIP(1) {
		b.Broadcast(Command{Type: CmdAlert, Count: 1})
	}

	// batch 8: the exe asks MONAI_CanCastSkillAtTarget after the roll
	if corpse != nil && b.Roll(100) < b.AIP(1) && p.Skills[slot1].Used() && c.otherCanCast(slot1, *corpse) {
		c.Cast(slot1, *corpse)

		return
	}

	// w38: the exe rolls the Skill2 chance whether or not the slot is defined (the id test is inside
	// MONAI_QueueSkillCast), so the roll comes first and an undefined slot queues nothing.
	if t != nil && c.Dist < b.AIP(5) && b.Roll(100) < b.AIP(2) {
		if p.Skills[slot2].Used() {
			c.Cast(slot2, *t)
		}

		return
	}

	if t2, d, ok := c.W.AttackTarget(b); ok && d < b.AIP(5) && b.Roll(100) < b.AIP(2) {
		if p.Skills[slot2].Used() {
			c.Cast(slot2, t2)
		}

		return
	}

	if b.AIP(3) <= b.Roll(100) {
		c.Sleep(10)

		return
	}

	if t != nil {
		c.Circle(*t, 3)

		return
	}

	c.Sleep(10)
}

// RoomChecker is an optional extension of Senses: whether two subtile points
// lie in the same room (FUN_0061ae80 maps a point to its room/region id).
type RoomChecker interface {
	SameRoom(b *Brain, a, z Point) bool
}

// stateLeashRadius is the vertical-distance cutoff of State13 (0x28, VERIFIED
// constant, applied to FUN_005db310 of the anchor's y: UNVERIFIED which axis).
const stateLeashRadius = 0x28

// thinkState13 is MONAI_State13_Think 0x5e4be0, a leashed guard: it keeps to
// the room of its anchor (command type 10, created at its own position on the
// first tick; VERIFIED) and strikes anything that enters it.
//
// VERIFIED: needs a profile; the anchor repair (a zero anchor takes the
// monster's position); when the monster is not in the anchor's room it walks
// back; when the target is not in the anchor's room either, the monster sits on
// the anchor, using its Skill1 ring cast (FUN_005e4b10) if fewer than 25
// casts were made, else sleeps 10; the final rolls: in reach, aip3+10%
// attack mode A2 (mode 4 in the exe's queue = A1 here) else stall aip2; out of
// reach aip1% run (speed 100) else stall aip2.
// UNVERIFIED: the room test (default: within 20 subtiles of the anchor), the
// ring-cast helper (not ported: always "not cast"), the two unknown calls
// FUN_005a3c60/FUN_0053eb80 around the walk back.
func thinkState13(c *Ctx) {
	b := c.B
	p := b.Profile
	t := c.Target

	if p == nil {
		return
	}

	a := b.FindCommand(CmdAnchor)
	if a == nil {
		b.AppendCommand(Command{Type: CmdAnchor, X: b.X, Y: b.Y})
		a = b.FindCommand(CmdAnchor)
	}

	if a.X == 0 || a.Y == 0 {
		a.X, a.Y = b.X, b.Y
	}

	anchor := Point{a.X, a.Y}
	same := func(x, y Point) bool {
		if rc, ok := c.W.(RoomChecker); ok {
			return rc.SameRoom(b, x, y)
		}

		return Distance(x.X-y.X, x.Y-y.Y) <= 20
	}

	if !same(anchor, Point{b.X, b.Y}) {
		if c.moveTo(anchor) {
			return
		}
	}

	if t == nil {
		c.Sleep(10)

		return
	}

	if !same(anchor, Point{t.X, t.Y}) {
		if b.X == anchor.X && b.Y == anchor.Y {
			c.Sleep(10)

			return
		}

		if c.moveTo(anchor) {
			return
		}
	}

	if abs(anchor.Y-b.Y) > stateLeashRadius {
		if c.moveTo(anchor) {
			return
		}
	}

	if c.InRange {
		if b.Roll(100) < b.AIP(3)+10 {
			c.Attack(ModeAttack1, *t)

			return
		}

		c.Sleep(b.AIP(2))

		return
	}

	if b.Roll(100) < b.AIP(1) {
		c.SetSpeed(100)
		c.RunTo(*t, 0)

		return
	}

	c.Sleep(b.AIP(2))
}

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

// ImpStateID is the unit state that keeps a monster in AI state 16 (0x8f,
// VERIFIED constant: the think returns the class AI as soon as it is gone).
const ImpStateID = 0x8f

// state16Slots is the table at 0x6e4a70 (VERIFIED w37): two (skill slot, extra) pairs {2, 0} and {4, 0}; the slot is
// the zero-based monstats Skill index (Skill3 at +0x174, Skill5 at +0x178 of the row).
var state16Slots = [2]int{2, 4}

// state16Pre is MONAI_State16_Pre 0x5e1bc0 (VERIFIED w37): a coin flip (one seed step, low bit) picks the first of the
// two table entries; when the unit does not hold that skill the other entry is tried; with neither skill both
// AiGeneral words +0x18/+0x1c (Scratch[1], Scratch[2]) are zeroed, else Scratch[1] is the chosen slot index (2 or 4)
// and Scratch[2] the table's second word (0).
func state16Pre(c *Ctx) {
	b := c.B
	u := b.Roll(2)

	for tries := 0; tries < 2; tries++ {
		u = (u + 1) & 1

		if b.Profile.Skills[state16Slots[u]].Used() {
			b.Scratch[1] = state16Slots[u]
			b.Scratch[2] = 0

			return
		}
	}

	b.Scratch[1], b.Scratch[2] = 0, 0
}

// thinkState16 is MONAI_State16_Think 0x5e1c60, the AI of an imp-class monster right after its teleport (the skill
// Imp Teleport puts the state 0x8f on it). VERIFIED w37 flow: the Pre hook runs first (state16Pre). Without the state
// (or without the flagged owner/target) the class AI is restored (SetAiState(0)) and the think sleeps 1. Within 25
// subtiles of a living, hostile target: with a pre-chosen slot (Scratch[1] != 0), a distance below aip2 and a roll
// under aip3 cast that slot at the target (Skill id < 0: wait 20), else a 50% strike (mode 8) or a 20-tick wait;
// without a pre-chosen slot, or farther away, the unit casts Skill1 at a random point within aip2 of itself
// (MONAI_QueueRandomOffsetSkillCast, nihOffset) and sets Scratch[0] = -1. The exe reads the profile of the imp rows
// 0x1ec (skills) and 0x1ed (aips); the port reads the monster's own profile (UNVERIFIED which class that is in this
// engine). The owner test (UNIT_FindOwnerUnitIfFlagged) and the hostility test are the host's (c.Target stands in).
func thinkState16(c *Ctx) {
	b := c.B
	p := b.Profile
	t := c.Target

	state16Pre(c)

	if t == nil || !c.W.HasState(b, ImpStateID) {
		b.ClearForced(c.W.Frame())
		c.Sleep(1)

		return
	}

	if c.Dist < 25 && b.Scratch[1] != 0 {
		slot := b.Scratch[1]
		if c.Dist < b.AIP(2) && b.Roll(100) < b.AIP(3) {
			if slot >= 0 && slot < NumSkills && p.Skills[slot].Used() {
				c.Cast(slot, *t)

				return
			}

			c.Sleep(20)

			return
		}

		if b.Roll(100) < 50 {
			c.Attack(ModeSkill1, *t)

			return
		}

		c.Sleep(20)

		return
	}

	nihOffsetAIP(c, slot1, 2)
	b.Scratch[0] = -1
}
