package d2skill

import (
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

// ArmageddonSeedBound is the bound of the one roll SRVDO_124 (0x5c6200) makes
// on the caster's seed when the skill starts; the result is stored in the
// skill's "target type" slot (VERIFIED).
const ArmageddonSeedBound = 0x10000

// ArmageddonSlotBound is the bound of the roll that follows each rock: the
// state-active function of the Armageddon state (srvactivefunc 146, table
// entry 0x5c65b0) re-seeds the caster's generator from the slot (low word =
// slot, high word the usual 0x29a), rolls below one million and stores the
// result back as the new slot (VERIFIED).
const ArmageddonSlotBound = 1000000

// ArmageddonTries is how many positions the active function tries before it
// gives up for this pulse (VERIFIED: the loop stops after the fifth miss).
const ArmageddonTries = 5

// Roller rolls a number in [0, n) (d2rand.Seed.Roll fits).
type Roller func(n int) int

// ArmageddonPoint picks the landing point of one meteor the way 0x5c65b0
// does. radius is aurarangecalc (Param3), (px, py) the caster's subtile. Each
// try rolls the x offset and then the y offset, each uniformly in [-radius,
// radius] (a SQUARE, not a circle), and accepts the point when ok reports a
// clear line from the caster to it that also has no blocking cell on the
// point itself (the exe: collision trace with mask 0x805, a room at the
// point and cell flags of zero there). After ArmageddonTries misses no rock
// falls this pulse and found is false. A radius below one yields no rock
// (the exe refuses a non-positive aurarangecalc).
func ArmageddonPoint(roll Roller, radius, px, py int, ok func(x, y int) bool) (x, y int, found bool) {
	if radius < 1 {
		return 0, 0, false
	}

	span := radius*2 + 1

	for try := 0; try < ArmageddonTries; try++ {
		x = roll(span) - radius + px
		y = roll(span) - radius + py

		if ok == nil || ok(x, y) {
			return x, y, true
		}
	}

	return 0, 0, false
}

// StormInterval is the pulse period of Armageddon and Hurricane: both active
// functions (0x5c65b0 and 0x5c63f0) re-arm the event after the skill's Param4
// frames, whatever it is (VERIFIED; a period below one frame is raised to one
// here because the engine ticks once a frame).
func StormInterval(param4 int) int {
	if param4 < 1 {
		return 1
	}

	return param4
}

// BloodGolemDamageShare is the damage-sharing rule of the Blood Golem
// (SKILL_DrainLeaderLifeForSharedPool 0x5c4600, VERIFIED): the golem is about
// to take dmg points; the owner pays dmg * pct / 100 (rounded toward zero) of
// them and the golem takes that much less. Nothing happens for a blow of zero
// or less, for an owner that is dead, or for an owner with less than one full
// point of life. The owner is never brought below one point; the golem's
// reduction is the full share even when the owner's clamp swallowed part of
// it (the exe subtracts the share from both sides without checking). Returns
// the owner's new life and the damage the golem still takes.
func BloodGolemDamageShare(dmg, pct, ownerLife int) (newOwnerLife, golemDmg int) {
	if dmg <= 0 || ownerLife < 1 {
		return ownerLife, dmg
	}

	share := dmg * pct / 100

	newOwnerLife = ownerLife - share
	if newOwnerLife < 1 {
		newOwnerLife = 1
	}

	golemDmg = dmg - share
	if golemDmg < 0 {
		golemDmg = 0
	}

	return newOwnerLife, golemDmg
}

// BloodGolemHealShare is ITEM_ShareHealingWithPlayerGolem (0x5c4800,
// VERIFIED): when the owner drinks a potion the golem receives heal * pct /
// 100 points (pct is the skill's Param6), limited by its missing life, and
// the owner's own healing is lowered by what the golem really took. A heal of
// zero or less changes nothing here (the exe's zero case fills the golem to
// full; see the notes). Returns the golem's gain.
func BloodGolemHealShare(heal, pct, golemLife, golemMax int) (gain int) {
	if heal <= 0 || pct <= 0 {
		return 0
	}

	gain = heal * pct / 100
	if room := golemMax - golemLife; gain > room {
		gain = room
	}

	if gain < 0 {
		gain = 0
	}

	return gain
}

// FirestormJitter is the per-missile hook of Firestorm's extra emitters (SRVDO_117, 0x5c5130, VERIFIED): the
// skill casts its first firestormmaker straight at the target, then calc1 - 1 more from the caster's position
// toward the same destination, each re-seeded with the destination x plus its index (the hook 0x5c7340 shared
// with Charged Strike) before the path stepper scatters it. It returns the heading offset in radians. The
// scatter width of that stepper is not decoded (UNVERIFIED), the same +-90 degrees as ChargedBoltAim.
func FirestormJitter(destX, index int) float64 {
	seed := d2rand.New(ChargedBoltSeed(destX, index))

	return float64(int(seed.Roll(181))-90) * math.Pi / 180
}
