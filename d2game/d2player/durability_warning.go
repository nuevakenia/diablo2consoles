package d2player

// LowDurabilityWarning reports whether a worn item shows the warning icon of
// UI_DrawLowDurabilityWarningIcons 0x494470: its maximum is not 0 and
// (current * 100) / maximum, in integer division, is at most 20.
func LowDurabilityWarning(cur, maxDur int) bool {
	return maxDur != 0 && cur*100/maxDur <= lowDurPct
}
