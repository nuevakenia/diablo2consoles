package d2skill

// Client side skill visuals: the cltstfunc / cltdofunc dispatch of Game.exe
// (SKILLS\SkillsAma.cpp, SkillsNec.cpp, ... in studied-batch-10.md and
// gaps-slice-F.md). Everything here is pure geometry on subtile coordinates; the
// caller turns a Shot into a missile entity. All formulas were read from the
// decompile (CLTDO_017 0x4f1b60, 018 0x4f1d20, 019 0x4f1e70, 020 0x4f1fd0,
// 022 0x4f2340, and the helpers 0x4c1dc0 / 0x4c1e00) and are VERIFIED unless
// stated.

// Client do-function numbers (skills.txt cltdofunc) handled here.
const (
	CltDoMultipleShot    = 17 // also Teeth, Shock Wave, Prime Poisonball
	CltDoGuidedArrow     = 18 // also Bone Spirit
	CltDoChargedStrike   = 19
	CltDoStrafe          = 20
	CltDoFend            = 21
	CltDoLightningStrike = 22
)

// Missile creation kinds (MissileCreateParams.dwKind). Their full semantics are
// UNVERIFIED; the values are what each client function stores.
const (
	KindMultiShot       = 0x820
	KindHomingUnit      = 0x20
	KindHomingGround    = 0x420
	KindAimed           = 0x21
	KindLightningStrike = 0x20
)

// SubPoint is a subtile coordinate.
type SubPoint struct{ X, Y int }

// Shot is one client missile to create. From is where it starts, To where it is
// aimed (a point, or the delta for homing kinds); Loop is the exe's loop index
// (+0x58) which the per-missile hook uses to rotate bolts; Data28 / Data2C are
// the two missile data words some functions fill.
type Shot struct {
	Slot   int // index into the skill's cltmissile columns: 0 base, 1 a, 2 b, 3 c, 4 d
	Kind   int
	From   SubPoint
	To     SubPoint
	Loop   int
	Data28 int
	Data2C int
	Parent int // unit id the missile is parented to, 0 for the caster
}

// SynergyMissileSlot is CLTDO_GetSkillSynergyBonusLevel (0x4ef560): for a skill
// with dwBits4 & 4 and its aura state present on the caster, the missile column
// is picked by the aura stat (1 or less: a, 2: b, 3 or more: c); otherwise column a.
func SynergyMissileSlot(bits4 int, hasAuraState bool, auraStat int) int {
	if bits4&4 == 0 || !hasAuraState {
		return 1
	}

	switch {
	case auraStat > 2:
		return 3
	case auraStat > 1:
		return 2
	}

	return 1
}

// truncDiv2 halves toward zero (the exe's CDQ / SUB / SAR sequence).
func truncDiv2(v int) int { return v / 2 }

// scaleUp is MATH_ScaleUpShortVector (0x4c1dc0): short vectors are enlarged
// (x4 if the squared length is below 4, then x2 if still below 16).
func scaleUp(x, y int) (int, int) {
	if x*x+y*y < 4 {
		x, y = x*4, y*4
	}

	if x*x+y*y < 16 {
		x, y = x*2, y*2
	}

	return x, y
}

// perpScaledDown is MATH_PerpendicularScaledDownVector (0x4c1e00): the vector
// turned to (y, -x) and halved (truncating) until its squared length is 3 or
// less, i.e. a unit grid step.
func perpScaledDown(x, y int) (int, int) {
	a, b := -x, y

	for a*a+b*b > 3 {
		b, a = truncDiv2(b), truncDiv2(a)
	}

	return b, a
}

// MultiShotStep is the per-arrow step of Multiple Shot: the caster to target
// vector, enlarged, then turned perpendicular and cut to a grid step.
func MultiShotStep(caster, target SubPoint) (dx, dy int) {
	vx, vy := scaleUp(target.X-caster.X, target.Y-caster.Y)

	return perpScaledDown(vx, vy)
}

// MultiShotPlan is CLTDO_017_MultipleShot (0x4f1b60): count arrows, each created
// at the caster's position and aimed at a point on a line through the target,
// perpendicular to the shot; the first aim point is target - step*count/2 (integer
// division), then one step per arrow. calc2 is stored at +0x44 by the exe (the
// caller passes it on). slot is 1 (a) or, for a non bow weapon class with column
// b set, 2.
func MultiShotPlan(caster, target SubPoint, count, slot int) []Shot {
	if count < 1 {
		return nil
	}

	dx, dy := MultiShotStep(caster, target)
	ax, ay := target.X-dx*count/2, target.Y-dy*count/2
	out := make([]Shot, 0, count)

	for i := 0; i < count; i++ {
		out = append(out, Shot{Slot: slot, Kind: KindMultiShot, From: caster, To: SubPoint{ax, ay}, Loop: i})
		ax, ay = ax+dx, ay+dy
	}

	return out
}

// MultiShotSlot picks the missile column of Multiple Shot: column a, or b when
// the attack weapon class is not 1 (a bow) and column b is set.
func MultiShotSlot(weaponClass int, hasColumnB bool) int {
	if weaponClass != 1 && hasColumnB {
		return 2
	}

	return 1
}

// ChargedStrikePlan is CLTDO_019_ChargedStrike (0x4f1e70): calc1 bolts, all
// created AT the target unit and aimed at the point mirrored through it
// (2*target - caster). The loop index rotates the bolts in the per-missile hook
// (0x5c7340, not decoded). Needs a target unit; the caller checks that.
func ChargedStrikePlan(caster, target SubPoint, bolts, slot int) []Shot {
	out := make([]Shot, 0, bolts)
	mirror := SubPoint{target.X*2 - caster.X, target.Y*2 - caster.Y}

	for i := 0; i < bolts; i++ {
		out = append(out, Shot{Slot: slot, Kind: KindAimed, From: target, To: mirror, Loop: i})
	}

	return out
}

// GuidedArrowPlan is CLTDO_018_GuidedArrow (0x4f1d20): one homing missile at the
// caster; kind 0x20 with a target unit (mode 1) or 0x420 aimed at a ground point
// (mode 2). Data 0x28 holds the mode, data 0x2c the packed 16 bit pair
// ((dy << 16) + dx) of aim point minus caster.
func GuidedArrowPlan(caster, aim SubPoint, hasUnit bool, slot int) Shot {
	s := Shot{Slot: slot, Kind: KindHomingGround, From: caster, To: aim, Data28: 2}
	if hasUnit {
		s.Kind, s.Data28 = KindHomingUnit, 1
	}

	dx, dy := int16(aim.X-caster.X), int16(aim.Y-caster.Y)
	s.Data2C = int(int32(dy)*0x10000 + int32(dx))

	return s
}

// ChainTarget is a candidate enemy for the chain steps.
type ChainTarget struct {
	ID  int
	Pos SubPoint
}

// ClientScanNext is CLTDO_FindTargetInAreaWithCallback (0x4c2910, callback 0x4c28d0, VERIFIED wave 24, the client twin
// of the server scan 0x569a40): over the candidates already inside the scan area it keeps the smallest unit id ABOVE
// the excluded id, else the smallest id of all (the excluded unit itself when it is the only candidate; the fallback
// starts at the excluded id and takes every id up to it). It reports false for an empty list.
func ClientScanNext(cands []ChainTarget, excluded int) (ChainTarget, bool) {
	var above, fall ChainTarget

	haveAbove, haveFall := false, false
	aboveID, fallID := int(^uint32(0)>>1), excluded

	for _, c := range cands {
		if c.ID > excluded {
			if c.ID < aboveID {
				above, aboveID, haveAbove = c, c.ID, true
			}

			continue
		}

		if c.ID <= fallID {
			fall, fallID, haveFall = c, c.ID, true
		}
	}

	switch {
	case haveAbove:
		return above, true
	case haveFall:
		return fall, true
	}

	return ChainTarget{}, false
}

// within lists the candidates inside a Chebyshev radius of a point. The exe's area shape (CLTDO_EnumerateUnitsInArea
// 0x4c2690) is UNVERIFIED; the square matches the server scans of this port.
func within(cands []ChainTarget, at SubPoint, radius int) []ChainTarget {
	var out []ChainTarget

	for _, c := range cands {
		if maxInt(absInt(c.Pos.X-at.X), absInt(c.Pos.Y-at.Y)) <= radius {
			out = append(out, c)
		}
	}

	return out
}

// LightningStrikePlan is CLTDO_022_LightningStrike (0x4f2340): the main target needs a unit; the scan around it
// (radius calc1, filter 3, the main unit excluded: ClientScanNext, VERIFIED wave 24 the same rule as the server
// strike) picks the second unit, which gets one kind 0x20 missile parented to the main target, aimed at that unit's
// position, with calc2 in data 0x28. When the main unit is alone, the scan returns the main unit itself and the bolt
// is drawn on it, as on the server. Returns false when nothing is in range. others may include the main target.
func LightningStrikePlan(main ChainTarget, others []ChainTarget, calc1, calc2, slot int) (Shot, bool) {
	cands := within(others, main.Pos, calc1)

	found := false

	for _, c := range cands {
		if c.ID == main.ID {
			found = true
		}
	}

	if !found {
		cands = append(cands, main) // the enumeration around the unit contains the unit itself
	}

	pick, ok := ClientScanNext(cands, main.ID)
	if !ok {
		return Shot{}, false
	}

	return Shot{Slot: slot, Kind: KindLightningStrike, From: main.Pos, To: pick.Pos, Data28: calc2, Parent: main.ID}, true
}

// StrafeSlot is the missile column CLTDO_020_Strafe picks: column b when the attack weapon class is not 1 (a bow) and
// column b is set, else column a.
func StrafeSlot(weaponClass int, hasColumnB bool) int {
	if weaponClass != 1 && hasColumnB {
		return 2
	}

	return 1
}

// StrafeClientPlan is CLTDO_020_Strafe (0x4f1fd0) for one animation event: one kind 0x21 arrow from the archer to the
// current target, then the counter drops by one; when it is still above zero the next target is the scan result
// around the archer (radius aurarangecalc, filter 3, the current target excluded: ClientScanNext, VERIFIED wave 24)
// and the exe also shortens the animation. Returns the arrow, the remaining counter and the next target (ok false =
// no next target, burst ends). With the current target alone the scan hands it back, so the burst goes on at it.
func StrafeClientPlan(archer SubPoint, target ChainTarget, counter int, others []ChainTarget, rng, slot int) (shot Shot, left int, next ChainTarget, ok bool) {
	shot = Shot{Slot: slot, Kind: KindAimed, From: archer, To: target.Pos}
	left = counter - 1

	if left <= 0 {
		return shot, left, ChainTarget{}, false
	}

	cands := within(others, archer, rng)

	found := false

	for _, c := range cands {
		if c.ID == target.ID {
			found = true
		}
	}

	if !found {
		cands = append(cands, target)
	}

	next, ok = ClientScanNext(cands, target.ID)

	return shot, left, next, ok
}

// StrafeClientBurst lists every arrow of a Strafe cast for a viewer that draws the whole burst at once (a remote caster):
// StrafeClientPlan applied count times from the first target.
func StrafeClientBurst(archer SubPoint, first ChainTarget, count int, others []ChainTarget, rng, slot int) []Shot {
	var out []Shot

	cur := first

	for count > 0 {
		shot, left, next, ok := StrafeClientPlan(archer, cur, count, others, rng, slot)
		out = append(out, shot)

		if !ok {
			break
		}

		cur, count = next, left
	}

	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
