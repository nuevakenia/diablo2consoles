package d2sfx

// Panel duck (Game.exe SOUND_UpdateEntries 0x4b6970, factor DAT_0072658c):
// once per tick the factor moves 5 toward 0 while the game menu (UI panel id
// 9) or the key configuration panel (id 11) is open and the panel layout mode
// is 0 or 1, otherwise 5 toward 100. VERIFIED: ids 9 and 11 from the panel
// tests at 0x4b6f9d; the meaning of layout mode 0/1 versus 2 (800x600) is
// UNVERIFIED, so callers pass "open" as the panel being open in any layout.
const panelDuckStep = 5

// PanelDuckStep moves the panel duck factor (0..100) one tick.
func PanelDuckStep(cur int, open bool) int {
	if open {
		cur -= panelDuckStep
		if cur < 0 {
			return 0
		}

		return cur
	}

	cur += panelDuckStep
	if cur > duckMax {
		return duckMax
	}

	return cur
}

// PanelDuckExempt says whether a Sounds.txt index ignores the panel duck:
// 1..0xf, 0x34..0x47 and 0x1231..0x125a (VERIFIED, 0x4dcb30; the purpose of
// the ranges is UNVERIFIED, probably UI and speech).
func PanelDuckExempt(index int) bool {
	return (index >= 1 && index <= 0xf) || (index >= 0x34 && index <= 0x47) || (index >= 0x1231 && index <= 0x125a)
}
