package d2monsters

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

// w39: host side of the Druid and Necromancer pets' follow logic (MONAI_SelectPetActionByRange 0x5e3510 and
// MONAI_FollowLeaderStep 0x5e2dc0, ports in d2monster/ai_pet_fb*.go).
//
//   - the owner's mode the pets see (walk, run, neutral) is derived from his movement, like the mercenary's;
//   - the leash is widened by half of the owner's pet count (PETS_CountAllPets);
//   - Spirit Wolf / Dire Wolf (DruidWolf) and Grizzly (DruidBear) run their ported thinkers instead of the
//     generic chase (see usesPetAI);
//   - the wolves' pack call (a monster skill with SrvDoFunc 98, "Teleport 2") moves the wolf next to its owner.
//     The cast animation is skipped (UNVERIFIED); the AI sleeps 10 afterwards like the exe.

// stepPetOwner records the owner's movement once per think of a pet.
func (d *Director) stepPetOwner(u *unit) {
	a := u.ally
	if a == nil || a.owner == nil {
		return
	}

	x, y := playerSubtile(a.owner)

	switch {
	case !a.ownerSeen, x == a.lastOX && y == a.lastOY:
		a.ownerMode = d2monster.ModeNeutral
	case a.owner.IsRunning():
		a.ownerMode = d2monster.ModeRun
	default:
		a.ownerMode = d2monster.ModeWalk
	}

	a.lastOX, a.lastOY, a.ownerSeen = x, y, true
}

// FBNPetCount implements d2monster.FBNPetCounter: the living minions of the pet's owner.
func (d *Director) FBNPetCount(b *d2monster.Brain) int {
	u := d.unitOf(b)
	if u == nil || u.ally == nil {
		return 0
	}

	n := 0

	for _, o := range d.units {
		if o.ally != nil && o.ally.owner == u.ally.owner && o.ally.kind == "minion" && o.m.Alive() {
			n++
		}
	}

	return n
}

// FBNPetsAdjacent implements d2monster.FBNPetsAdjacent: the other minions of the owner within 2 subtiles
// (MONAI_ScanCallbackCountAdjacentUnits; the exact radius is UNVERIFIED).
func (d *Director) FBNPetsAdjacent(b *d2monster.Brain) int {
	u := d.unitOf(b)
	if u == nil || u.ally == nil {
		return 0
	}

	n := 0

	for _, o := range d.sortedUnits() {
		if o == u || o.ally == nil || o.ally.owner != u.ally.owner || !o.m.Alive() {
			continue
		}

		if d2monster.EdgeDistance(b.X-o.b.X, b.Y-o.b.Y, b.Size) <= 2 {
			n++
		}
	}

	return n
}

// FBNRavenPecks implements d2monster.FBNSkillValue: the attacks of a Raven (Param5 + (level - 1) * Param6 of the
// Raven skill at the owner's level of it, recorded at the cast); 3 when the cast did not record it.
func (d *Director) FBNRavenPecks(b *d2monster.Brain) int {
	if u := d.unitOf(b); u != nil && u.ally != nil && u.ally.opt.Pecks > 0 {
		return u.ally.opt.Pecks
	}

	return 3
}

// callSkillFunc is SRVDO 98 (MonTeleport, Teleport 2, Baal Teleport): the wolves' call to their owner.
const callSkillFunc = 98

// castPetCall lands a pet's pack-call skill at once: the pet appears next to its owner. It returns true when the
// slot holds such a skill (the cast itself then reports failure so the AI sleeps 10, as it does after the call).
func (d *Director) castPetCall(b *d2monster.Brain, slot int) bool {
	u := d.unitOf(b)
	if u == nil || u.ally == nil || u.ally.kind != "minion" || u.ally.owner == nil || b.Profile == nil ||
		slot < 0 || slot >= d2monster.NumSkills || !b.Profile.Skills[slot].Used() {
		return false
	}

	sk := d.resolveSkill(b.Profile, slot)
	if sk == nil || sk.rec == nil || sk.rec.Srvdofunc != callSkillFunc {
		return false
	}

	if d.teleportNextToOwner(u) {
		d.footprint(u)
		u.m.StopMoving()
		d.emit("minion", "MINION call name=%s skill=%q to=(%d,%d)", u.m.Label(), sk.name, u.b.X, u.b.Y)
	}

	return true
}

// isDruidPetAI reports the monstats AIs of the Druid's wolves and bear.
func isDruidPetAI(u *unit) bool {
	if u.b == nil || u.b.Def == nil {
		return false
	}

	return strings.EqualFold(u.b.Def.Name, "DruidWolf") || strings.EqualFold(u.b.Def.Name, "DruidBear")
}

var (
	_ d2monster.FBNPetCounter   = (*Director)(nil)
	_ d2monster.FBNPetsAdjacent = (*Director)(nil)
	_ d2monster.FBNSkillValue   = (*Director)(nil)
)
