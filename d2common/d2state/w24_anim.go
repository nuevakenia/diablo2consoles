package d2state

// Wave 24: speed side effects of the cold states, ported from PATH_UpdateUnitVelocityAndAnimRate (0x624150),
// UNIT_SetAnimRate (0x621970) and PATH_ScaleSpeedByStatTableEntry (0x621930). Notes: d2-re-notes/w24-dot.md.
//
// Chill (state 0xb) writes the same value to stats 0x43, 0x44 and 0x45 (COMBAT_ApplyColdSlowState 0x578ca0).
// The animation module reads them like this (percent of the base rate):
//
//	walk / run                    : effective FRW + stat 0x43, minimum 25; velocity AND animation scale with it
//	attack                        : effective IAS + stat 0x44, clamp 15..175
//	cast                          : effective FCR + 100, maximum 175: stats 0x43..0x45 are NOT read
//	get hit (player 4, monster 3) : effective FHR + 50: stat 0x45 is NOT read
//	block (player 9, monster 6)   : effective FBR + 50, or + 100 with state 0x65
//	everything else               : stat 0x45 clamped 15..175 (unless the mode keeps its base rate)
//
// The "effective" values are diminishing: c*x/(c+x) with c = 120 for IAS, FHR, FCR and FBR and 150 for FRW
// (table at 0x6ea3d4, 12-byte entries: kind 1, constant, item stat 93, 99, 105, 102, 96).

// Diminishing-return constants of the speed table (VERIFIED, memory at 0x6ea3d4).
const (
	SpeedConstFRW = 150
	SpeedConstFCR = 120
	SpeedConstFHR = 120
	SpeedConstFBR = 120
	SpeedConstIAS = 120
)

// EffectiveSpeed is PATH_ScaleSpeedByStatTableEntry for an entry of kind 1: c*x/(c+x) (integer, truncated
// toward zero); x == 0 gives 0.
func EffectiveSpeed(c, x int) int {
	if x == 0 || c+x == 0 {
		return 0
	}

	return c * x / (c + x)
}

// WalkRunPct is the walk/run speed percent: effective FRW plus the chill stat (negative), minimum 25.
func WalkRunPct(frw, chillStat43 int) int {
	v := EffectiveSpeed(SpeedConstFRW, frw) + chillStat43
	if v < 25 {
		v = 25
	}

	return v
}

// AttackRatePct is the attack animation percent: effective IAS plus the stat 0x44 plus delta (dual wield
// average difference, or -30 for player mode 0x12), clamped 15..175.
func AttackRatePct(ias, chillStat44, delta int) int {
	return clampPct(EffectiveSpeed(SpeedConstIAS, ias)+chillStat44+delta, 15, 175)
}

// OtherRatePct is the percent used for modes without their own rule: stat 0x45 clamped 15..175.
func OtherRatePct(stat45 int) int { return clampPct(stat45, 15, 175) }

// CastRatePct is effective FCR + 100, at most 175 (chill is ignored).
func CastRatePct(fcr int) int {
	v := EffectiveSpeed(SpeedConstFCR, fcr) + 100
	if v > 175 {
		v = 175
	}

	return v
}

// HitRecoveryPct is the get-hit animation percent: effective FHR + 50. Chill and freeze do not change it.
func HitRecoveryPct(fhr int) int { return EffectiveSpeed(SpeedConstFHR, fhr) + 50 }

// BlockRatePct is the block animation percent: effective FBR + 50, or + 100 for a unit with state 0x65.
func BlockRatePct(fbr int, holyShield bool) int {
	add := 50
	if holyShield {
		add = 100
	}

	return EffectiveSpeed(SpeedConstFBR, fbr) + add
}

// AnimRate is the stored animation rate: base * pct / 100 (integer), never negative, capped at 0x7fff.
func AnimRate(base, pct int) int {
	v := base * pct / 100
	if v < 0 {
		v = 0
	}

	if v > 0x7fff {
		v = 0x7fff
	}

	return v
}

// FrozenSkipsEvent says whether the monster event dispatcher (MONAI_DispatchModeHandlerEvent 0x5a5980) drops an
// event of the given type for a unit with state 1 (freeze) that is not in a death mode: every type except 3
// (life regeneration, so poison keeps ticking), 4, 5, 8 and 0xc (state expiry). A frozen monster neither acts
// nor moves. VERIFIED.
func FrozenSkipsEvent(eventType int, frozen, deathMode bool) bool {
	if !frozen || deathMode {
		return false
	}

	switch eventType {
	case 3, 4, 5, 8, 0xc:
		return false
	}

	return true
}

func clampPct(v, lo, hi int) int {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}
