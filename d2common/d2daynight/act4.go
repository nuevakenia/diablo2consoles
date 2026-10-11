package d2daynight

// Act4Target is the ambient intensity Act 4 settles on per level
// (ENVIRON_UpdateAmbientIntensity 0x61b8a0, branch taken when the act index is 3):
// level 0x67 -> 0x80, 0x68 -> 0x40, 0x69 -> 0x38, 0x6a -> 0x30, every other Act 4
// level 0x10. Act 4 does not use the sun cycle at all.
func Act4Target(level int) int {
	switch level {
	case 0x67:
		return 0x80
	case 0x68:
		return 0x40
	case 0x69:
		return 0x38
	case 0x6a:
		return 0x30
	default:
		return 0x10
	}
}

// Act4Step advances the Act 4 ambient intensity by n ticks: it moves 1 per tick
// toward the level target (up first, then down, then the exe's "== 0 means the
// target" check, which only matters for a target of 0 and so never fires).
// What value the exe holds when a game starts is not known (UNVERIFIED); the
// caller decides the start value.
func Act4Step(cur, level, n int) int {
	target := Act4Target(level)

	for i := 0; i < n; i++ {
		if cur < target {
			cur++
		}

		if cur > target {
			cur--
		}

		if cur == 0 {
			cur = target
		}
	}

	return cur
}
