package d2state

// Wave 24: the per-frame consumer of the poison / burn / open wounds streams, ported from Game.exe 1.14b.
// Notes: d2-re-notes/w24-dot.md.
//
// The appliers (COMBAT_ApplyPoisonState 0x578990, COMBAT_ApplyBurnState 0x578b00, the open wounds item event
// 0x5bda80) only create a statlist carrying stat 0x4a (hitpoint regeneration) = -value. The damage is taken by
// the life regeneration event (event type 3), which runs on EVERY frame and adds the whole stat to the life
// stat (stat 6, 8.8 fixed point) of the unit: monsters use MONSTER_LifeRegenEventTick 0x5a4340 (slot 3 of the
// monster mode handler table 0x6e3940), players use the handler at 0x57e800 (slot 3 of the table 0x6e2b08),
// which calls 0x57e600 (life) and 0x57e4f0 (stamina). The value is therefore per frame in 1/256 life points,
// summed over every statlist: poison, burn, open wounds and any positive regeneration all add into one number
// and there is no separate remainder per kind (the remainder lives in the life stat itself).

// Fixed-point and state constants (all VERIFIED unless noted).
const (
	// LifeUnit is one whole hit point in the 8.8 life stat.
	LifeUnit = 256

	// StatHPRegen is stat 0x4a, the stat the three appliers write.
	StatHPRegen = 0x4a

	// StatePreventHeal is state 0x34 (row 52 of the shipped states.txt, preventheal). The monster tick refuses
	// to run while the unit has it and the net regeneration is not negative.
	StatePreventHeal = 0x34
	// StateUninterruptable is state 0x36: a monster at 0 life with it is not killed, state 0x5c (death delay)
	// is switched on instead.
	StateUninterruptable = 0x36
	// StateDeathDelay is state 0x5c.
	StateDeathDelay = 0x5c
	// StatePoison, StateOpenWounds and StateBurn are the states of the three streams. Only the first two are
	// searched for the killer (statlist owner) when a monster dies of the tick; a burn death has no killer.
	StatePoison     = 2
	StateOpenWounds = 0x3e
	StateBurn       = 0x73
	// StateHealthPot is state 100: a player whose life reaches the maximum loses that statlist.
	StateHealthPot = 100

	// OpenWoundsFrames is the length of an open wounds statlist (0xc8 = 200 frames, 0x5bda80).
	OpenWoundsFrames = 200
)

// MonsterRegen is the input of one run of the monster life event.
type MonsterRegen struct {
	// HP and MaxHP are the life stat and STATS_GetMaxHitPoints, both 8.8.
	HP, MaxHP int
	// Regen is the total of stat 0x4a (STATS_GetUnitStat). BaseRegen is the unit's base stat 0x4a, used only
	// when LifeFlagState is set.
	Regen, BaseRegen int
	// LifeFlagState is STATE_UnitHasStateInMask14C: the unit has a state of states.txt mask index 32 (0x20 in
	// the loader, the column between shatter and udead; the header string sits at 0x6e90d8 and its name is
	// UNVERIFIED, the order points at "life"). When set the unit's base regeneration is left out of the sum.
	LifeFlagState bool
	// PreventHeal is state 0x34.
	PreventHeal bool
	// RoomKnown is "the unit has a room"; InTown is ROOM_IsTownRoom of that room.
	RoomKnown, InTown bool
	// Dead is mode 0 or 0xc (death / dead): the unit is not killed again.
	Dead bool
	// Uninterruptable is state 0x36.
	Uninterruptable bool
}

// MonsterRegenOut is what the run decided.
type MonsterRegenOut struct {
	// HP is the new life stat.
	HP int
	// Rearm says event 3 is scheduled again for the next frame (otherwise the stream stops until something
	// schedules it: a new poison/burn application, a damaging hit while the stat is nonzero).
	Rearm bool
	// Kill says the unit dies of the tick; Delay says state 0x5c is switched on instead of dying.
	Kill, Delay bool
	// Changed says the life stat was written.
	Changed bool
}

// RunMonsterRegen is MONSTER_LifeRegenEventTick (0x5a4340), VERIFIED against the disassembly:
//
//	net = stat 0x4a; if the unit has a state of mask 14C: net -= base stat 0x4a
//	state 0x34 and net >= 0       -> nothing, and event 3 is NOT scheduled again
//	otherwise event 3 is scheduled for the next frame; net == 0 cancels it again (nothing else happens)
//	damage (net < 0) is skipped when hp <= 255 (under one whole point) and the unit's room is a town room or
//	missing: a monster in town cannot be poisoned below one point
//	hp += net; above the maximum it is clamped and the event cancelled; below 1 (8.8) it becomes 0
//	hp == 0 and not in a death mode: killed (owner of the poison, else the open wounds statlist, is the
//	killer), or with state 0x36 the death is delayed with state 0x5c
//
// Note the kill threshold: the 8.8 life must fall under 1/256, not under one whole point.
func RunMonsterRegen(in MonsterRegen) MonsterRegenOut {
	out := MonsterRegenOut{HP: in.HP}

	net := in.Regen
	if in.LifeFlagState {
		net -= in.BaseRegen
	}

	if in.PreventHeal && net >= 0 {
		return out
	}

	out.Rearm = true

	if net == 0 {
		out.Rearm = false

		return out
	}

	if net < 0 && in.HP <= 0xff && (!in.RoomKnown || in.InTown) {
		return out
	}

	hp := in.HP + net
	if hp > in.MaxHP {
		out.Rearm = false
		hp = in.MaxHP
	}

	if hp < 1 {
		hp = 0
	}

	out.HP, out.Changed = hp, true

	if hp == 0 && !in.Dead {
		if in.Uninterruptable {
			out.Delay = true
		} else {
			out.Kill = true
		}
	}

	return out
}

// PlayerRegen is the input of one run of the player life regeneration.
type PlayerRegen struct {
	HP, MaxHP int
	Regen     int
	// Dead is UNIT_IsPlayerDeathMode (mode 0 or 0x11, or unit flag 0x10000): the whole event body is skipped.
	Dead bool
}

// PlayerRegenOut is the result of a player run.
type PlayerRegenOut struct {
	HP int
	// DropHealthPot says the statlist of state 100 is removed (life reached the maximum).
	DropHealthPot bool
}

// RunPlayerRegen is the life half of the player event 3 handler (0x57e800 -> 0x57e600), VERIFIED: the handler
// is scheduled again every frame no matter what; with stat 0x4a == 0 nothing happens; otherwise hp += stat,
// the maximum clamps (and ends the state 100 list), and the result is never below one whole point (256): a
// player cannot die from poison, burn or open wounds. There is no state 0x34 test and no town test on this
// path (UNVERIFIED whether 0x34 is handled elsewhere for players).
func RunPlayerRegen(in PlayerRegen) PlayerRegenOut {
	out := PlayerRegenOut{HP: in.HP}

	if in.Dead || in.Regen == 0 {
		return out
	}

	hp := in.HP + in.Regen
	if hp > in.MaxHP {
		hp = in.MaxHP
		out.DropHealthPot = true
	}

	if hp < LifeUnit {
		hp = LifeUnit
	}

	out.HP = hp

	return out
}

// OpenWoundsPerFrame is the per-frame 8.8 damage of an open wounds application (ITEM_ApplyOpenWoundsEvent
// 0x5bda80 -> SKILL_CreateTimedStateStatList 0x56c740 with state 0x3e, length 200, stat 0x4a = -value):
//
//	level = max(1, attacker level (stat 0xc))
//	total = ITEM_CalcPotionRecoveryTotal(level, {9, 18, 27, 36, 45}) + 40
//	value = total / 4 when the target is a player (unit type 0); halved when the target is a monster whose
//	        type flag word (data +0x16) has 0xc (champion or unique)
//
// The helper is 0x5bd9c0: piecewise linear in the level with steps at 15, 30, 45 and 60. The arithmetic is
// VERIFIED; which branch reads which register of the first comparison is UNVERIFIED (the player /4 and the
// monster /2 mapping follows the unit type test at 0x5bdb3b and the 0xc type-flag call at 0x5bdb64).
func OpenWoundsPerFrame(attackerLevel int, targetIsPlayer, targetChampOrUnique bool) int {
	if attackerLevel < 1 {
		attackerLevel = 1
	}

	v := potionRecoveryTotal(attackerLevel, [5]int{9, 18, 27, 36, 45}) + 40

	switch {
	case targetIsPlayer:
		v /= 4
	case targetChampOrUnique:
		v /= 2
	}

	return v
}

// potionRecoveryTotal is ITEM_CalcPotionRecoveryTotal (0x5bd9c0), VERIFIED.
func potionRecoveryTotal(level int, t [5]int) int {
	switch {
	case level < 2:
		return 0
	case level > 0x3c:
		return (t[3]+t[2]+t[1])*0xf + (level-0x3c)*t[4] + t[0]*0xe
	case level > 0x2d:
		return (t[2]+t[1])*0xf + (level-0x2d)*t[3] + t[0]*0xe
	case level > 0x1e:
		return (level-0x1e)*t[2] + t[1]*0xf + t[0]*0xe
	case level > 0xf:
		return (level-0xf)*t[1] + t[0]*0xe
	}

	return (level - 1) * t[0]
}

// RearmAfterHit is the tail of COMBAT_ApplyDamageToUnit (0x57a650): a monster that survives a hit that
// dealt life damage (damage struct field 0x13 nonzero) while its stat 0x4a is nonzero has event 3 cancelled
// and scheduled for the next frame, so a stopped regeneration or poison stream starts again; the states of
// the remhit column are removed (STATE_RemoveStatesInMaskE4, index 6). Returns whether event 3 is re-armed.
func RearmAfterHit(monster, survived bool, lifeDamage, regenStat int) bool {
	return monster && survived && lifeDamage != 0 && regenStat != 0
}
