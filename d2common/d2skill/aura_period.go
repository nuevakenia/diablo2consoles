package d2skill

// AuraPeriod is the pulse period of an aura skill. VERIFIED (SKILL_CalcNextPeriodicTickFrame 0x56aa90, used by
// SRVDO_065 / 066 / 081 / 082 and SKILL_StartAuraStateTimerEvent 0x5bd0f0): the skill record calc at +0x128 is
// evaluated (the perdelay column; the offset-to-column mapping is UNVERIFIED but it is the only period calc) and a
// result below 6 becomes 5. A skill without the column keeps the old fixed 25.
func AuraPeriod(env *Env, sk *Skill) int {
	if sk.PerDelay == nil {
		return 25
	}

	return PeriodFloor(env.eval(sk.PerDelay))
}

// PeriodFloor is the clamp of the exe: anything below 6 is 5.
func PeriodFloor(p int) int {
	if p < 6 {
		return 5
	}

	return p
}

// NextPeriodicTick is the absolute frame of the next pulse (0x56aa90): the next frame congruent to 1 modulo the
// period, counted from frame-1 upwards (the current frame itself when it already is one). The SRVDO callers
// turn it into a delay with + 1 - frame (UNVERIFIED how the event queue counts that +1).
func NextPeriodicTick(frame, period int) int {
	period = PeriodFloor(period)

	return ((frame-1+period)/period)*period + 1
}
