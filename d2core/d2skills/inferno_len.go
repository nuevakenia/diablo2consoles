package d2skills

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"

// infernoLenOf is SRVDO_InfernoScheduleAnimEnd's table lookup (0x5c6cb0, VERIFIED w39): the unit's class number
// (hero order Amazon 0 .. Assassin 6) indexes monstats; that row's MonStatsEx link names the monstats2 row whose
// InfernoLen byte is the delay. ok is false when no row exists (the exe then uses 1). The row is found by the
// hcIdx column, which equals the row number in 1.14 (UNVERIFIED for modded tables).
func (e *Engine) infernoLenOf(u d2skill.Unit) (int, bool) {
	h, isHero := u.(*heroUnit)
	if !isHero {
		return 0, false
	}

	class := h.p.ClassNumber()

	for _, ms := range e.asset.Records.Monster.Stats {
		if ms.ID != class {
			continue
		}

		if ex := e.asset.Records.Monster.Stats2[ms.ExtraDataKey]; ex != nil {
			return ex.InfernoLen, true
		}

		return 0, false
	}

	return 0, false
}
