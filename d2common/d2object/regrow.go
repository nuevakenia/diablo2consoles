package d2object

// Regrowing objects after a room is reactivated (wells, healing plants).
//
// SUNITINACTIVE_RestoreUnitsForRoom (0x540850) restores an object whose objects.txt
// subclass has flag 0x20 from the saved charge byte and the saved frame. VERIFIED
// against the disassembly (the decompiler hides the clamp):
//
//	gain   = 0 when savedFrame >= nowFrame + period (cannot normally happen)
//	       = min(1, (nowFrame - savedFrame) / period)    -- at most ONE charge, however long the absence
//	charges = min(saved + gain, max)
//	mode   = 2 - charges / parm2   (0 full, 1 half, 2 empty); a value above 2 resets to mode 0 and max
//	then one refill event is scheduled for each missing charge, at nowFrame + k*period, k = 1..max-charges.
//
// period is objects.txt Parm0 and must be non-zero (the exe aborts with an internal error on 0).
// max is the room-wide charge ceiling the exe reads through 0x550b10 (2 for a 1.14b well, i.e. WellCharges).

// RegrowResult is the restored state of one regrowing object.
type RegrowResult struct {
	Charges int
	Mode    int
	// RefillAfter are the delays, in frames from the restore, of the refill events still pending.
	RefillAfter []int
	OK          bool
}

// Regrow restores a well-like object. savedFrame and nowFrame are game frames.
func Regrow(saved, max, savedFrame, nowFrame, period, parm2 int) RegrowResult {
	if period <= 0 || parm2 <= 0 || saved >= max {
		if period <= 0 || parm2 <= 0 {
			return RegrowResult{}
		}

		// already full (the exe does not enter this branch when saved >= max): keep it full
		return RegrowResult{Charges: max, Mode: 2 - max/parm2, OK: true}
	}

	gain := 0

	if savedFrame < nowFrame+period {
		gain = (nowFrame - savedFrame) / period
		if gain > 1 {
			gain = 1
		}

		if gain < 0 {
			gain = 0
		}
	}

	c := saved + gain
	if c > max {
		c = max
	}

	mode := 2 - c/parm2
	if mode > 2 || mode < 0 {
		return RegrowResult{Charges: max, Mode: 0, OK: true}
	}

	res := RegrowResult{Charges: c, Mode: mode, OK: true}

	for k := 1; k <= max-c; k++ {
		res.RefillAfter = append(res.RefillAfter, k*period)
	}

	return res
}
