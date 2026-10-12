package d2skill

// IronMaidenDamage is the amount the cursed unit takes when it deals dealt damage in melee: MulDiv(dealt, pct, 100)
// (SKILL_ApplyStateLinkedDamageToTarget 0x5c3ec0, VERIFIED). Zero or negative inputs give zero.
func IronMaidenDamage(dealt, pct int) int {
	if dealt <= 0 || pct <= 0 {
		return 0
	}

	return dealt * pct / 100
}

// LifeTapHeal is the life the attacker of a Life Tap cursed unit gains (event function 5, 0x5c4180, VERIFIED wave
// 23): MulDiv(damage dealt, calc1, 100), added to the attacker's current life and clamped between 1 and its
// maximum life. A current life of 0 or less (dead attacker) or a maximum life of 0 gains nothing. Life values are
// in whatever fixed-point unit the caller uses for both arguments (the exe works in 8.8).
func LifeTapHeal(dealt, pct, life, maxLife int) (newLife int, ok bool) {
	if dealt <= 0 || pct <= 0 || life <= 0 || maxLife <= 0 {
		return life, false
	}

	n := life + dealt*pct/100
	if n < 1 {
		n = 1
	}

	if n > maxLife {
		n = maxLife
	}

	return n, true
}
