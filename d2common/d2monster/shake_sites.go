package d2monster

// The fixed screen-shake call sites of Game.exe 1.14b (callers of GFX_StartScreenShake 0x4727c0).
// Arguments were read off the disassembly: ECX = amplitude, EDX = ramp ms, then the stack holds
// hold ms and decay ms. The missile functions are named by the exe's MISSILE_DoFunc_N. Two further
// sites (0x4d0b18 and DoFunc_28 0x4d2196) take their numbers from missile data and are not
// listed. Nothing in OD2's missile simulation emits these yet (UNVERIFIED which OD2 missile
// carries each id); they are here as tested data for the wiring.
var (
	// ShakeSceneEvent is the long rumble 0x49bb01 starts right after a day-phase change from a
	// scene event: amplitude 6, 4 s ramp, 10 s hold, 4 s decay.
	ShakeSceneEvent = Shake{Amp: 6, RampMs: 4000, HoldMs: 10000, DecayMs: 4000}

	// ShakeDoFunc65 is started by MISSILE_DoFunc_65 whenever the low six bits of its running
	// counter are zero (every 64th step): 6, 2 s, 3 s, 2 s.
	ShakeDoFunc65 = Shake{Amp: 6, RampMs: 2000, HoldMs: 3000, DecayMs: 2000}

	// ShakeMissile150 is started by MISSILE_DoFunc_36 and DoFunc_30 when the missile id is 0x96:
	// amplitude id - 0x7d = 25, no ramp, 4 s hold, no decay.
	ShakeMissile150 = Shake{Amp: 25, HoldMs: 4000}

	// ShakeMissile325 is started by MISSILE_DoFunc_35 when the missile id is 0x145: 20 pixels for 6 s.
	ShakeMissile325 = Shake{Amp: 20, HoldMs: 6000}

	// ShakeDoFunc37 is started by MISSILE_DoFunc_37 when its stage value is 10: amplitude 4,
	// 80 ms ramp, 80 ms hold, 400 ms decay.
	ShakeDoFunc37 = Shake{Amp: 4, RampMs: 80, HoldMs: 80, DecayMs: 400}

	// ShakeDoFunc53 is started by MISSILE_DoFunc_53 when its stage value is 5: amplitude 3,
	// 600 ms ramp, 4 s hold, 3 s decay.
	ShakeDoFunc53 = Shake{Amp: 3, RampMs: 600, HoldMs: 4000, DecayMs: 3000}
)
