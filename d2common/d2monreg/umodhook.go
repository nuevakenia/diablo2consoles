package d2monreg

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"

// The run time hooks of the monster modifiers (Game.exe 1.14b, own words, VERIFIED unless marked).
//
// Next to the init callbacks (umodfx.go) the exe keeps a second table: one row of six code pointers per modifier id,
// base 0x739410, stride 0x18 (id * 6 + hook type, 4 bytes each). MONSTER_RunUniqueModHook 0x5a1d50 walks the nine
// modifier bytes of a unit (unit data +0x1c, stops at the first 0) and calls the entry of the requested type. It hands
// every callback the modifier id and "the unit is a unique" (type word +0x16, mask 0x8). The six hook types and where
// the exe raises them:
//
//	0  0x5a5746 MONAI_ExecuteAiCommand            (start of an AI command)
//	1  0x5a57cd MONAI_ExecuteAiCommand            (end of an AI command)
//	2  event 7 of the unit (table 0x6e3940, slot 7 = 0x5a1e50); only scheduled by modifier 41
//	3  0x57a7d5 COMBAT_ApplyDamageToUnit          (a monster attacker's damage landed)
//	4  0x57b069 / 0x57b09a / 0x57b0d0 COMBAT_ServerHandleUnitHit
//	5  0x59da6a MISSILE_CreateServerMissile       (a monster created a missile)
//
// Rows that hold code (id: type -> address): 7: 3 -> 0x5a0060 (Cursed); 9: 1 0x5a0110, 2 0x5a0140; 10: 1 0x5a1320, 2
// 0x5a0740; 14: 0 0x5a1650; 15: 1 0x5a0830; 17: 1 0x5a12f0, 2 0x5a04d0, 4 0x5a06d0; 18: 1 0x5a1320, 2 0x5a06f0; 19: 0
// 0x5a08a0, 5 0x5a0920; 20: 1 0x5a09c0, 2 0x5a09f0; 21: 2 0x5a15a0; 22: 1 0x5a0d40; 23: 0 0x5a0f80; 24: 3 0x5a0be0; 27:
// 0 0x5a0b50, 5 0x5a0bc0; 29: 5 0x5a1120 (Multishot); 31: 1 0x5a1320, 2 0x5a0370; 32: 1 0x5a1320, 2 0x5a1820; 33: 1
// 0x5a1960, 2 0x5a19e0; 34: 1 0x5a1350, 2 0x5a1410; 35: 1 0x5a1580; 40: 1 0x5a1ce0; 41: 2 0x5a1d10 (timer); 42: 1
// 0x5a1320, 2 0x5a0440. Only 7, 29 and 41 are ported in this file; the others are listed for the next reader (UNVERIFIED
// which monumod row each id is for those).

// Modifier ids with a hook ported here.
const (
	UModCursed    = 7
	UModMultishot = 29
	UModTimer     = 41
)

// Hook types.
const (
	HookAICmdStart = 0
	HookAICmdEnd   = 1
	HookTimer      = 2
	HookDamage     = 3
	HookHit        = 4
	HookMissile    = 5
)

// MonTypeUniqueBit is the type word mask the hooks hand over as the "is unique" argument (+0x16, mask 0x8).
const MonTypeUniqueBit = 0x8

// MonTypeMultishotGuard is the bit (+0x16, mask 0x80) Multishot sets while it fires its extra missiles, so that the
// extra missiles do not multiply again.
const MonTypeMultishotGuard = 0x80

// CursedSkillID is the skill the Cursed hook casts: skill 0x42 (66), Amplify Damage.
const CursedSkillID = 66

// CurseRadiusMax is the largest radius (in the units of the aurarangecalc) the hook allows.
const CurseRadiusMax = 40

// CursedRoll is the Cursed hook 0x5a0060 (type 3, raised for a monster attacker whose damage landed): nothing happens
// unless the unit is a unique; then one step of the unit's own seed decides (the exe skips when the low two bits of the
// new low word are 0, so three times in four), and the curse level is the monster level / 5 + 1 (at least 1).
func CursedRoll(seed *d2rand.Seed, unique bool, monLevel int) (cast bool, level int) {
	if !unique || seed == nil {
		return false, 0
	}

	if seed.Step()&3 == 0 {
		return false, 0
	}

	return true, CursedLevel(monLevel)
}

// CursedLevel is level / 5 + 1 (the exe's multiply by 0x66666667 and shift, sign corrected), at least 1.
func CursedLevel(monLevel int) int {
	l := monLevel/5 + 1
	if l < 1 {
		l = 1
	}

	return l
}

// CurseRadius clamps the evaluated aurarangecalc of the curse to 1..40 (0x5a00e3: value <= 1 gives 1, >= 40 gives 40).
func CurseRadius(v int) int {
	if v <= 1 {
		return 1
	}

	if v >= CurseRadiusMax {
		return CurseRadiusMax
	}

	return v
}

// MultishotEligible says whether the Multishot hook 0x5a1120 fires for a missile: the carrier is a unique, the
// missile's missiles.txt row has no NoUniqueMod (dwBits4 bit 0x1000), and the owner is not already inside a Multishot
// volley (type word 0x80).
func MultishotEligible(unique, noUniqueMod, inVolley bool) bool {
	return unique && !noUniqueMod && !inVolley
}

// MultishotTargets are the two points the extra missiles aim at. (mx, my) is the monster, (tx, ty) the aim point (the
// owner's target unit, else the missile's destination). The unit step (dx, dy) = sign(m - t) gives the two points
// (tx - dy, ty + dx) and (tx + dy, ty - dx), the perpendicular neighbours of the aim point. For the missile rows
// 0x3f..0x42 (rows of the missiles.txt, UNVERIFIED which) the step is zero and both extra missiles aim at the same
// point as the original.
func MultishotTargets(mx, my, tx, ty, missileRow int) [2][2]int {
	dx, dy := sign(mx-tx), sign(my-ty)

	if missileRow > 0x3e && missileRow < 0x43 {
		dx, dy = 0, 0
	}

	return [2][2]int{{tx - dy, ty + dx}, {tx + dy, ty - dx}}
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}

	return 0
}

// TimerPeriod is the frames between the ticks of modifier 41 (0x4b = 75, 3 seconds at 25 frames per second).
const TimerPeriod = 0x4b

// TimerWakeDelay is how far ahead the tick schedules the AI think event (event 2) in frames.
const TimerWakeDelay = 2

// TimerWakes says whether the timer tick of modifier 41 (0x5a1d10 then 0x5715b0) re-arms the AI think event: the
// monster must be alive (mode neither 0 death nor 0xc dead) and in mode 1 (neutral), or be one of the base classes
// 0x6e, 0x76, 0x88 or 0xf7 in any other living mode. A dead unit does nothing and does not reschedule. The tick always
// reschedules itself after TimerPeriod frames while the unit is alive. The exe cancels a pending think event first
// whatever the mode (UNVERIFIED what that means for other modes, the port leaves the pending one).
func TimerWakes(mode, baseClass int) (alive, wake bool) {
	if mode == 0 || mode == 0xc {
		return false, false
	}

	if mode == 1 {
		return true, true
	}

	switch baseClass {
	case 0x6e, 0x76, 0x88, 0xf7:
		return true, true
	}

	return true, false
}

// The random names of the rare monsters. The client builds the name text in MONSTER_BuildUModEffectInfo 0x4a8fe0 when
// the modifier list arrives (packet 0x57): it reseeds the unit's seed with the 16 bit name seed (low word = seed, high
// word 0x29a, DRLG_InitSeed), then rolls (RAND_RollSeedModulo on the unit seed, one step each, a power of two table size
// masks) the suffix, then the prefix of the tables uniquesuffix / uniqueprefix, formats string 0x6b9 with prefix and
// suffix, then rolls 100 (RAND_RollSeedBounded) and below 50 builds again with the three rolls appellation, suffix,
// prefix and the format 0x6ba (prefix, suffix, appellation). The strings themselves are not recorded here.
// Super uniques and champions never reach this code (they take their text from superuniques / monstats), nor do the
// classes of FixedNameString.

// RareNameFormat1 and RareNameFormat2 are the string ids of the two formats.
const (
	RareNameFormat1 = 0x6b9
	RareNameFormat2 = 0x6ba
)

// RareName is the outcome of the roll: indices into the unique prefix, suffix and appellation tables (rows, in file
// order) and whether the long form (with the appellation) was chosen. The short form uses the first suffix and prefix
// rolls (Suffix1/Prefix1); the long form replaces them by the second set.
type RareName struct {
	Prefix, Suffix, Appellation int
	Long                        bool
	// Prefix1 and Suffix1 are what the short form used (also rolled, but replaced, for the long form).
	Prefix1, Suffix1 int
}

// RollRareName runs the exe's name roll for a name seed and the sizes of the three tables (0 rows: the roll does not
// advance and the index is 0, the exe then prints an empty string).
func RollRareName(nameSeed uint16, nPrefix, nSuffix, nAppellation int) RareName {
	var s d2rand.Seed

	s.Init(uint32(nameSeed))

	r := RareName{}
	r.Suffix1 = int(s.Roll(int32(nSuffix)))
	r.Prefix1 = int(s.Roll(int32(nPrefix)))
	r.Prefix, r.Suffix = r.Prefix1, r.Suffix1

	if s.Roll(100) < 50 {
		r.Long = true
		r.Appellation = int(s.Roll(int32(nAppellation)))
		r.Suffix = int(s.Roll(int32(nSuffix)))
		r.Prefix = int(s.Roll(int32(nPrefix)))
	}

	return r
}

// FixedNameString returns the string id the exe prints instead of a random name for these base classes (class ids of
// monstats, the table of the switch at 0x4a9170 / 0x4a9238), and false for every other class. UNVERIFIED which monster
// each class id is.
func FixedNameString(class int) (int, bool) {
	switch class {
	case 0x9c:
		return 0xbcd, true
	case 0xd3, 0x2c4:
		return 0xbee, true
	case 0xf2, 0x2c0:
		return 0xbf6, true
	case 0xf3, 0x14d, 0x2c1:
		return 0xbf4, true
	case 0x100, 0x2c2:
		return 0x3f6, true
	case 0x220, 0x2c5:
		return 0xbf5, true
	case 0x2c3:
		return 0x2b92, true
	}

	return 0, false
}
