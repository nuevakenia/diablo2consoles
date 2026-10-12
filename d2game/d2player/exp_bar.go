package d2player

// The experience bar of the bottom panel. VERIFIED in Game.exe: UI_DrawExperienceBar 0x495380.
//
//	span  = threshold(level) - threshold(level-1)       (threshold(l) = experience needed to leave level l)
//	done  = exp - threshold(level-1) when exp is above it, else 0
//	when done > 0x226b901 both are shifted right by 7 (overflow guard)
//	width = done * 0x77 / span; a width above 0x77 counts as 0
//	two 1 pixel lines (y = H-0x26 and H-0x25) from x = W/2-0x90 are drawn only below the maximum level and when width > 0
//
// The hover tooltip (string 0x1043 "Experience: %u / %u") prints the experience and threshold(level), in the
// small tooltip at y = H-0x33, over the rectangle x W/2-0x92..W/2-0x17, y H-0x2b..H-0x22 (inclusive).
const (
	ExpBarMaxWidth  = 0x77
	expBarShiftFrom = 0x226b901
)

// ExpBarWidth returns the lit width in pixels (0 when nothing is drawn).
func ExpBarWidth(exp, prevThreshold, nextThreshold, level, maxLevel int) int {
	span := nextThreshold - prevThreshold
	if span <= 0 {
		return 0
	}

	done := 0
	if exp > prevThreshold {
		done = exp - prevThreshold
	}

	if done > expBarShiftFrom {
		done >>= 7
		span >>= 7

		if span == 0 {
			return 0
		}
	}

	w := done * ExpBarMaxWidth / span
	if w > ExpBarMaxWidth {
		w = 0
	}

	if level >= maxLevel {
		return 0
	}

	return w
}
